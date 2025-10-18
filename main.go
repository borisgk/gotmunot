// /home/ubuntu/go/src/tm25/main.go
package main

import (
	"database/sql"
	"fmt"
	"log"
	"encoding/json"
	"net/http"
	"time"
	"strings"
	"path/filepath"
	"sync"
	"os"
)

// User struct to represent a user.
type User struct {
	ID       int
	Username string
	Password string // Hash!
}

// TaskProgress holds the state of a long-running task.
type TaskProgress struct {
	Processed int    `json:"processed"`
	Total     int    `json:"total"`
	Filename  string `json:"filename"`
	Complete  bool   `json:"complete"`
	Error     string `json:"error,omitempty"`
}

// taskProgressMap safely stores the progress of multiple concurrent tasks.
var taskProgressMap = struct {
	sync.RWMutex
	tasks map[string]*TaskProgress
}{tasks: make(map[string]*TaskProgress)}

func main() {
	// Define a handler function for the root path ("/").
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Redirect to the content page
		http.Redirect(w, r, "/content", http.StatusSeeOther)
	})

	// Define a handler function for the /status path
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "TM25 is running!")
	})

	// Handler for the login page
	http.HandleFunc("/login", loginHandler)
	// Handler for logout page
	http.HandleFunc("/logout", logoutHandler)
	// Handler for the secure area
	http.HandleFunc("/content", contentHandler)
	//Handler for the upload
	http.HandleFunc("/upload", uploadHandler)
	// Handler for the upload page
	http.HandleFunc("/upload-page", uploadPageHandler)
	// Handler for the service page
	http.HandleFunc("/service", servicePageHandler)
	// Handlers for polling-based thumbnail regeneration
	http.HandleFunc("/service/regenerate-thumbnails/start", startRegenerateThumbnailsHandler)
	http.HandleFunc("/service/regenerate-thumbnails/status", getRegenerateThumbnailsStatusHandler)
	// Handlers for polling-based preview regeneration
	http.HandleFunc("/service/regenerate-previews/start", startRegeneratePreviewsHandler)
	http.HandleFunc("/service/regenerate-previews/status", getRegeneratePreviewsStatusHandler)

	// Serve static files from the "static" directory.
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Serve uploaded photos from the photoUploadDir.
	mediaFS := http.FileServer(http.Dir(photoUploadDir))
	http.Handle("/media/", http.StripPrefix("/media/", mediaFS))

	// Serve thumbnails securely through a custom handler.
	http.HandleFunc("/media/thumbs/", thumbnailHandler)
	// Serve previews securely through a custom handler.
	http.HandleFunc("/media/previews/", previewHandler)

	// Print a message indicating that the server is starting.
	fmt.Println("Starting TM25 Web Server on port 9030...")

	// Start the HTTP server on port 9030.
	// If there's an error starting the server, log.Fatal will log the error and exit.
	log.Fatal(http.ListenAndServe(":9030", nil))
}

func thumbnailHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	if _, ok := isValidSession(db, r); !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the requested file path from the URL.
	// e.g., /media/thumbs/2023/10/28/some-photo.jpg.webp -> 2023/10/28/some-photo.jpg.webp
	requestedFile := strings.TrimPrefix(r.URL.Path, "/media/thumbs/")

	// Basic security check to prevent directory traversal attacks.
	if strings.Contains(requestedFile, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Construct the full, absolute path to the file on the filesystem.
	fullPath := filepath.Join(thumbsDir, requestedFile)

	// Serve the file. http.ServeFile handles Content-Type, caching headers, etc.
	http.ServeFile(w, r, fullPath)
}

func previewHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	if _, ok := isValidSession(db, r); !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the requested file path from the URL.
	requestedFile := strings.TrimPrefix(r.URL.Path, "/media/previews/")

	// Basic security check to prevent directory traversal attacks.
	if strings.Contains(requestedFile, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Construct the full, absolute path to the file on the filesystem.
	fullPath := filepath.Join(previewsDir, requestedFile)

	// Serve the file.
	http.ServeFile(w, r, fullPath)
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Display the login form
		err := tmpl.ExecuteTemplate(w, "login.html", nil)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		// Handle login submission
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Retrieve user from the database.
		var user User
		err := db.QueryRow("SELECT id, username, password FROM users WHERE username = ?", username).Scan(&user.ID, &user.Username, &user.Password)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Invalid username or password - please try again", http.StatusUnauthorized)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check the password.
		if !checkPasswordHash(password, user.Password) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// Create a session
		sessionToken := generateSessionToken()
		err = createSession(db, sessionToken, user.Username)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set a session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  time.Now().Add(15 * time.Minute),
			HttpOnly: true, // Important for security
		})

		// Redirect to a secure area
		http.Redirect(w, r, "/content", http.StatusSeeOther)

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func contentHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Get recent photos
	photos, err := getRecentPhotos()
	if err != nil {
		log.Printf("Error getting recent photos: %v", err)
		// If we can't get photos, we can still render the page but with an empty photo slice.
		// So we'll set photos to an empty slice and continue.
		photos = []PhotoMetadata{}
	}

	// Create a struct to hold all the data for the template
	data := struct {
		Username string
		Photos   []PhotoMetadata
	}{
		Username: username,
		Photos:   photos,
	}

	// Execute the "content.html" template and pass the data.
	if err := tmpl.ExecuteTemplate(w, "content.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func uploadPageHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Execute the "upload.html" template and pass the username.
	if err := tmpl.ExecuteTemplate(w, "upload.html", struct{ Username string }{username}); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

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
	if _, ok := isValidSession(db, r); !ok {
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
	go func(id string) {
		log.Println("Starting thumbnail regeneration process for task:", id)

		allPhotos, err := getAllPhotos()
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
			originalPath := filepath.Join(photoUploadDir, photo.Filepath)

			if _, err := os.Stat(originalPath); os.IsNotExist(err) {
				log.Printf("Skipping missing file: %s", originalPath)
			} else {
				if err := createThumbnail(originalPath); err != nil {
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
	}(taskID)

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
	if _, ok := isValidSession(db, r); !ok {
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
	go func(id string) {
		log.Println("Starting preview regeneration process for task:", id)

		allPhotos, err := getAllPhotos()
		if err != nil {
			log.Printf("Error getting all photos for preview regeneration: %v", err)
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Error = "Could not retrieve photo list."
			taskProgressMap.tasks[id].Complete = true
			taskProgressMap.Unlock()
			return
		}

		totalPhotos := len(allPhotos)
		log.Printf("Found %d photos to process for task %s.", totalPhotos, id)

		for i, photo := range allPhotos {
			originalPath := filepath.Join(photoUploadDir, photo.Filepath)

			if _, err := os.Stat(originalPath); os.IsNotExist(err) {
				log.Printf("Skipping missing file: %s", originalPath)
			} else {
				if err := createPreview(originalPath); err != nil {
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
		taskProgressMap.Lock()
		taskProgressMap.tasks[id].Complete = true
		taskProgressMap.Unlock()
	}(taskID)

	// Immediately respond with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func getRegeneratePreviewsStatusHandler(w http.ResponseWriter, r *http.Request) {
	// This handler can be the same as the thumbnail status handler
	// as the logic is identical (just looks up a task ID in the map).
	getRegenerateThumbnailsStatusHandler(w, r)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		deleteSession(db, cookie.Value)
		//expire the cookie by setting the expiration in the past
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(w, cookie)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
