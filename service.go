package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

func servicePageHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Execute the "service.html" template and pass the username.
	if err := tmpl.ExecuteTemplate(w, "service.html", struct{ Username string }{username}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func startRegenerateThumbnailsHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure user is authenticated
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate a unique task ID
	taskID := fmt.Sprintf("%d", time.Now().UnixNano())

	// Initialize progress tracking for this task
	taskProgressMap.Lock()
	taskProgressMap.tasks[taskID] = &TaskProgress{}
	taskProgressMap.Unlock()

	// Start the regeneration in a new goroutine
	go func(id, user string) {
		log.Println("Starting thumbnail regeneration process for task:", id)

		allPhotos, err := getAllPhotos(user)
		if err != nil {
			log.Printf("Error getting all photos for regeneration: %v", err)
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Error = "Could not retrieve photo list."
			taskProgressMap.tasks[id].Complete = true
			taskProgressMap.Unlock()
			return
		}

		totalPhotos := len(allPhotos)
		log.Printf("Found %d photos to process for task %s.", totalPhotos, id)

		for i, photo := range allPhotos {
			originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)

			if _, err := os.Stat(originalPath); os.IsNotExist(err) {
				log.Printf("Skipping missing file: %s", originalPath)
			} else {
				if err := createThumbnail(originalPath, photo.UploadedBy); err != nil {
					log.Printf("Warning: failed to regenerate thumbnail for %s: %v", photo.Filename, err)
				}
			}

			// Update progress
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Processed = i + 1
			taskProgressMap.tasks[id].Total = totalPhotos
			taskProgressMap.tasks[id].Filename = photo.Filename
			taskProgressMap.Unlock()
		}

		log.Println("Thumbnail regeneration process complete for task:", id)
		taskProgressMap.Lock()
		taskProgressMap.tasks[id].Complete = true
		taskProgressMap.Unlock()
	}(taskID, username)

	// Immediately respond with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func getRegenerateThumbnailsStatusHandler(w http.ResponseWriter, r *http.Request) {
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

func startRegeneratePreviewsHandler(w http.ResponseWriter, r *http.Request) {
	// Ensure user is authenticated
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Generate a unique task ID
	taskID := fmt.Sprintf("previews-%d", time.Now().UnixNano())

	// Initialize progress tracking for this task
	taskProgressMap.Lock()
	taskProgressMap.tasks[taskID] = &TaskProgress{}
	taskProgressMap.Unlock()

	// Start the regeneration in a new goroutine
	go func(id, user string) {
		log.Println("Starting preview regeneration process for task:", id)

		allPhotos, err := getAllPhotos(user)
		if err != nil {
			log.Printf("Error getting all photos for preview regeneration: %v", err)
			updateTaskError(id, "Could not retrieve photo list.")
			return
		}

		totalPhotos := len(allPhotos)
		log.Printf("Found %d photos to process for task %s.", totalPhotos, id)

		for i, photo := range allPhotos {
			originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)

			if _, err := os.Stat(originalPath); os.IsNotExist(err) {
				log.Printf("Skipping missing file: %s", originalPath)
			} else {
				if err := createPreview(originalPath, photo.UploadedBy); err != nil {
					log.Printf("Warning: failed to regenerate preview for %s: %v", photo.Filename, err)
				}
			}

			// Update progress
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Processed = i + 1
			taskProgressMap.tasks[id].Total = totalPhotos
			taskProgressMap.tasks[id].Filename = photo.Filename
			taskProgressMap.Unlock()
		}

		log.Println("Preview regeneration process complete for task:", id)
		updateTaskComplete(id, totalPhotos)
	}(taskID, username)

	// Immediately respond with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func getRegeneratePreviewsStatusHandler(w http.ResponseWriter, r *http.Request) {
	// This handler can be the same as the thumbnail status handler
	// as the logic is identical (just looks up a task ID in the map).
	getRegenerateThumbnailsStatusHandler(w, r)
}
