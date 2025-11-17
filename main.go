// /home/ubuntu/go/src/tm25/main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/davidbyttow/govips/v2/vips"
)

// photoMetadataQueue is a channel that acts as a queue for saving photo metadata.
var photoMetadataQueue chan *PhotoMetadata

func main() {
	// Defer vips shutdown until the application exits.
	defer vips.Shutdown()

	// Define a handler function for the root path ("/").
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Redirect to the content page
		http.Redirect(w, r, "/gallery", http.StatusSeeOther)
	})

	// Define a handler function for the /status path
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "TM25 is running!")
	})

	// Define handlers for the main pages shown in the top menu
	// Handler for the gallery page
	http.HandleFunc("/gallery", galleryHandler)
	// Handler for the albums page
	http.HandleFunc("/albums", albumsHandler)
	http.HandleFunc("/album/", albumDetailHandler)
	http.HandleFunc("/albums/new", newAlbumHandler)
	http.HandleFunc("/api/albums", createAlbumHandler)
	http.HandleFunc("/api/albums/list", getAlbumListHandler)
	http.HandleFunc("/api/albums/add-photos", addPhotosToAlbumHandler)

	http.HandleFunc("/api/album/", albumActionHandler)
	// Initialize and start the metadata saving queue and worker.
	// A buffer of 100 can handle bursts of uploads.
	photoMetadataQueue = make(chan *PhotoMetadata, 100)
	go startMetadataSaveWorker()

	// Handler for the login page
	http.HandleFunc("/login", loginHandler)
	// Handler for logout page
	http.HandleFunc("/logout", logoutHandler)
	//Handler for the upload
	http.HandleFunc("/upload", uploadHandler)
	// Handler for the upload page
	http.HandleFunc("/upload-page", uploadPageHandler)
	http.HandleFunc("/photo/info/", photoInfoHandler)

	// API for single photo operations (e.g., DELETE)
	http.HandleFunc("/api/photo/", photoActionHandler)
	// API for batch photo operations
	http.HandleFunc("/api/photos/batch-update-date", startBatchUpdateDateHandler)
	http.HandleFunc("/api/photo/update-date", updatePhotoDateHandler)
	// API for login
	http.HandleFunc("/api/login", apiLoginHandler)
	http.HandleFunc("/api/photos/delete", batchDeletePhotosHandler)

	// API for downloading zipped previews
	http.HandleFunc("/api/photos/download-previews", downloadPreviewsHandler)
	// API for async downloads
	http.HandleFunc("/api/downloads/start", startDownloadHandler)
	http.HandleFunc("/api/downloads/status", getDownloadStatusHandler)
	http.HandleFunc("/api/downloads/cancel", cancelDownloadHandler)
	http.HandleFunc("/api/downloads/file", serveDownloadHandler)
	// Generic Task API
	http.HandleFunc("/api/tasks/status", getTaskStatusHandler)
	http.HandleFunc("/api/tasks/cancel", cancelTaskHandler)

	// Serve static files (CSS, JS, etc.)
	http.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir("static/css"))))
	http.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir("static/js"))))
	http.Handle("/static/fonts/", http.StripPrefix("/static/fonts/", http.FileServer(http.Dir("static/fonts"))))
	http.Handle("/static/img/", http.StripPrefix("/static/img/", http.FileServer(http.Dir("static/img"))))
	http.HandleFunc("/logo.png", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "logo.png")
	})

	// Securely serve all media (originals, thumbs, previews) from the photoUploadDir.
	http.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		sessionUser, ok := isValidSession(db, r)
		if !ok {
			// If the session is invalid, redirect to the login page,
			// Return an unauthorized error, which the frontend will catch.
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Security Check: Ensure the logged-in user is accessing their own media.
		// URL path is like /media/user1/thumbs/2023/10/28/file.jpg.webp
		pathParts := strings.SplitN(strings.TrimPrefix(r.URL.Path, "/media/"), "/", 2)
		if len(pathParts) < 2 || pathParts[0] != sessionUser {
			http.Error(w, "Forbidden", http.StatusForbidden)
			return
		}

		http.StripPrefix("/media/", http.FileServer(http.Dir(AppConfig.PhotoUploadDir))).ServeHTTP(w, r)
	})

	// Print a message indicating that the server is starting.
	fmt.Println("Starting TM25 Web Server on port 9030...")

	// Start the HTTP server on port 9030.
	// If there's an error starting the server, log.Fatal will log the error and exit.
	log.Fatal(http.ListenAndServe(":9030", nil))
}

// startMetadataSaveWorker is a long-running goroutine that processes metadata
// save requests from a channel, serializing all database writes.
func startMetadataSaveWorker() {
	log.Println("Starting metadata save worker...")
	for photoData := range photoMetadataQueue {
		photoID, err := savePhotoMetadata(photoData)
		if err != nil {
			log.Printf("BACKGROUND_ERROR: Failed to save metadata for %s: %v", photoData.Filename, err)
		} else {
			log.Printf("Metadata for %s saved successfully from queue with ID %d.", photoData.Filename, photoID)
		}
	}
}

func uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username    string
		CurrentPage string
	}{
		Username:    username,
		CurrentPage: "upload",
	}
	if err := tmpl.ExecuteTemplate(w, "upload.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func startBatchUpdateDateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Decode JSON body
	var payload struct {
		Filenames []string `json:"filenames"`
		StartDate string   `json:"start_date"` // Expecting "YYYY-MM-DDTHH:MM"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. Parse the start date
	startDate, err := time.Parse("2006-01-02T15:04", payload.StartDate)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}

	// 4. Generate a unique task ID and initialize progress
	taskID := fmt.Sprintf("batch-date-update-%d", time.Now().UnixNano())
	taskProgressMap.Lock()
	taskProgressMap.tasks[taskID] = &TaskProgress{Total: len(payload.Filenames)}
	taskProgressMap.Unlock()

	// 5. Start the background task
	go func(id, user string, filenames []string, startTime time.Time) {
		log.Printf("Starting batch date update for task %s", id)
		totalFiles := len(filenames)

		for i, filename := range filenames {
			// Check for cancellation
			taskProgressMap.RLock()
			if taskProgressMap.tasks[id].Cancelled {
				taskProgressMap.RUnlock()
				log.Printf("Task %s cancelled.", id)
				return
			}
			taskProgressMap.RUnlock()

			// Calculate the new date for this specific photo
			newDate := startTime.Add(time.Duration(i) * time.Minute)

			// Update progress before processing
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Processed = i + 1
			taskProgressMap.tasks[id].Filename = filename
			taskProgressMap.Unlock()

			// Perform the update
			if err := updatePhotoDateAndPath(user, filename, newDate); err != nil {
				log.Printf("Task %s: failed to update date for %s: %v", id, filename, err)
				// Continue to the next file
			}
		}

		log.Printf("Batch date update complete for task %s", id)
		updateTaskComplete(id, totalFiles)
	}(taskID, username, payload.Filenames, startDate)

	// 6. Respond immediately with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func updatePhotoDateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Ensure method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 3. Decode JSON body
	var payload struct {
		Filename string `json:"filename"`
		NewDate  string `json:"new_date"` // Expecting "YYYY-MM-DDTHH:MM"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 4. Parse the new date string
	newDate, err := time.Parse("2006-01-02T15:04", payload.NewDate)
	if err != nil {
		http.Error(w, "Invalid date format. Please use YYYY-MM-DDTHH:MM.", http.StatusBadRequest)
		return
	}

	// 5. Call the core logic function
	err = updatePhotoDateAndPath(username, payload.Filename, newDate)
	if err != nil {
		if err.Error() == "forbidden" {
			http.Error(w, "Forbidden", http.StatusForbidden)
		} else if err == sql.ErrNoRows {
			http.Error(w, "Photo not found", http.StatusNotFound)
		} else {
			log.Printf("Error updating photo date for %s: %v", payload.Filename, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// 6. Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
