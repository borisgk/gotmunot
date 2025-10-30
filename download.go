package main

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadPreviewsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Get filenames from query parameters
	filenames := r.URL.Query()["filename"]
	if len(filenames) == 0 {
		http.Error(w, "No filenames provided", http.StatusBadRequest)
		return
	}

	// 3. Create a buffer to write our zip archive to in memory.
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 4. Add files to the zip.
	for _, filename := range filenames {
		photo, err := getPhotoByFilename(filename)
		if err != nil {
			log.Printf("Could not find photo '%s' for zipping: %v", filename, err)
			continue // Skip this file
		}

		// Security check: ensure user owns the photo
		if photo.UploadedBy != username {
			log.Printf("Security alert: User '%s' attempted to download preview for photo '%s' owned by '%s'", username, filename, photo.UploadedBy)
			continue
		}

		// Construct the path to the preview file on disk
		previewPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "previews", photo.Filepath)

		// Read the file content
		fileData, err := os.ReadFile(previewPath)
		if err != nil {
			log.Printf("Could not read preview file '%s': %v", previewPath, err)
			continue
		}

		// Strip the timestamp prefix from the filename for the zip entry
		// e.g., "1698512345-my-photo.jpg" -> "my-photo.jpg"
		_, strippedFilename, _ := strings.Cut(filename, "-")
		if strippedFilename == "" {
			strippedFilename = filename // Fallback if there's no hyphen
		}

		// Create a new file header in the zip archive
		f, err := zipWriter.Create(strippedFilename)
		if err != nil {
			log.Printf("Could not create zip entry for '%s': %v", strippedFilename, err)
			continue
		}

		// Write the file data to the zip entry
		_, err = f.Write(fileData)
		if err != nil {
			log.Printf("Could not write data to zip for '%s': %v", strippedFilename, err)
			continue
		}
	}

	zipWriter.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"previews-%d.zip\"", time.Now().Unix()))
	w.Write(buf.Bytes())
}

func startDownloadHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Decode JSON body
	var payload struct {
		Filenames []string `json:"filenames"`
		Type      string   `json:"type"` // "originals" or "previews"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. Generate a unique task ID
	taskID := fmt.Sprintf("download-%d", time.Now().UnixNano())

	// 4. Initialize progress tracking for this task
	taskProgressMap.Lock()
	initialFilename := ""
	if len(payload.Filenames) > 0 {
		initialFilename = payload.Filenames[0]
	}
	taskProgressMap.tasks[taskID] = &TaskProgress{Total: len(payload.Filenames), Processed: 0, Filename: initialFilename}
	taskProgressMap.Unlock()

	// 5. Start the zipping process in a new goroutine
	go createZipArchive(taskID, username, payload.Filenames, payload.Type)

	// 6. Immediately respond with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func createZipArchive(taskID, username string, filenames []string, archiveType string) {
	log.Printf("Starting zip creation for task %s", taskID)

	// Create a temporary file to store the zip archive.
	zipFile, err := os.CreateTemp("", fmt.Sprintf("%s-*.zip", taskID))
	if err != nil {
		log.Printf("Task %s: failed to create temp zip file: %v", taskID, err)
		updateTaskError(taskID, "Failed to create archive file on server.")
		return
	}
	defer zipFile.Close()
	zipWriter := zip.NewWriter(zipFile)

	for i, filename := range filenames {
		// Check for cancellation at the beginning of each loop iteration.
		taskProgressMap.RLock()
		cancelled := taskProgressMap.tasks[taskID].Cancelled
		taskProgressMap.RUnlock()
		if cancelled {
			log.Printf("Task %s was cancelled by the user.", taskID)
			break // Exit the loop
		}
		// Update progress before processing
		taskProgressMap.Lock()
		taskProgressMap.tasks[taskID].Processed = i + 1
		taskProgressMap.tasks[taskID].Filename = filename
		taskProgressMap.Unlock()

		photo, err := getPhotoByFilename(filename)
		if err != nil || photo.UploadedBy != username {
			log.Printf("Task %s: skipping file %s (not found or permission denied)", taskID, filename)
			continue
		}

		var sourcePath string
		if archiveType == "originals" {
			sourcePath = filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)
		} else {
			sourcePath = filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "previews", photo.Filepath)
		}

		fileData, err := os.ReadFile(sourcePath)
		if err != nil {
			log.Printf("Task %s: could not read file %s: %v", taskID, sourcePath, err)
			continue
		}

		_, strippedFilename, _ := strings.Cut(filename, "-")
		if strippedFilename == "" {
			strippedFilename = filename
		}

		f, err := zipWriter.Create(strippedFilename)
		if err != nil {
			log.Printf("Task %s: could not create zip entry for %s: %v", taskID, strippedFilename, err)
			continue
		}
		_, err = f.Write(fileData)
		if err != nil {
			log.Printf("Task %s: could not write data to zip for %s: %v", taskID, strippedFilename, err)
			continue
		}
	}

	zipWriter.Close()

	// Final progress update
	taskProgressMap.RLock()
	isCancelled := taskProgressMap.tasks[taskID].Cancelled
	taskProgressMap.RUnlock()
	if isCancelled {
		os.Remove(zipFile.Name()) // Clean up the partial zip file
		return
	}
	log.Printf("Finished zip creation for task %s. File: %s", taskID, zipFile.Name())
	taskProgressMap.Lock()
	taskProgressMap.tasks[taskID].Processed = len(filenames)
	taskProgressMap.tasks[taskID].Complete = true
	taskProgressMap.tasks[taskID].DownloadURL = fmt.Sprintf("/api/downloads/file?id=%s", taskID)
	taskProgressMap.Unlock()
}

func getDownloadStatusHandler(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	taskProgressMap.RLock()
	progress, ok := taskProgressMap.tasks[taskID]
	taskProgressMap.RUnlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

func serveDownloadHandler(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	// The temp filename is based on the task ID.
	// e.g., /tmp/download-123456-*.zip
	tempDir := os.TempDir()
	files, err := filepath.Glob(filepath.Join(tempDir, fmt.Sprintf("%s-*.zip", taskID)))
	if err != nil || len(files) == 0 {
		http.Error(w, "Download file not found or expired.", http.StatusNotFound)
		return
	}
	filePath := files[0]

	// Clean up the file after serving.
	defer os.Remove(filePath)

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"archive-%d.zip\"", time.Now().Unix()))
	http.ServeFile(w, r, filePath)
}

func cancelDownloadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	taskProgressMap.Lock()
	if task, ok := taskProgressMap.tasks[taskID]; ok {
		task.Cancelled = true
		task.Complete = true // Mark as complete to stop polling
	}
	taskProgressMap.Unlock()

	w.WriteHeader(http.StatusOK)
}
