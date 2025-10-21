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
	"strconv"
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
	// Handler for photo info
	http.HandleFunc("/photo/info/", photoInfoHandler)
	// API endpoint for paginated photos
	http.HandleFunc("/api/photos", photosAPIHandler)
	http.HandleFunc("/service/regenerate-thumbnails/status", getRegenerateThumbnailsStatusHandler)
	// Handlers for polling-based preview regeneration
	http.HandleFunc("/service/regenerate-previews/start", startRegeneratePreviewsHandler)
	http.HandleFunc("/service/regenerate-previews/status", getRegeneratePreviewsStatusHandler)

	// API for single photo operations (e.g., DELETE)
	http.HandleFunc("/api/photo/", photoActionHandler)
	// Serve static files from the "static" directory.
	fs := http.FileServer(http.Dir("static"))
	http.Handle("/static/", http.StripPrefix("/static/", fs))

	// Securely serve all media (originals, thumbs, previews) from the photoUploadDir.
	http.HandleFunc("/media/", func(w http.ResponseWriter, r *http.Request) {
		sessionUser, ok := isValidSession(db, r)
		if !ok {
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

	// Define the number of photos per page for the initial load.
	const initialLimit = 50

	// Get the first page of photos.
	photos, err := getPhotos(username, initialLimit, 0)
	if err != nil {
		log.Printf("Error getting recent photos: %v", err)
		// If we can't get photos, we can still render the page but with an empty photo slice.
		photos = []PhotoMetadata{}
	}

	// Get the total number of photos for the frontend to know when to stop loading.
	totalPhotos, err := getTotalPhotoCount(username)
	if err != nil {
		log.Printf("Error getting total photo count: %v", err)
		totalPhotos = 0 // Default to 0 on error
	}

	// Create a struct to hold all the data for the template
	data := struct {
		Username    string
		Photos      []PhotoMetadata
		TotalPhotos int
		Limit       int
	}{
		Username:    username,
		Photos:      photos,
		TotalPhotos: totalPhotos,
		Limit:       initialLimit,
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

func photosAPIHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Parse query parameters for pagination
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")

	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		page = 1
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50 // Default limit
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Retrieve photos from the database
	photos, err := getPhotos(username, limit, offset)
	if err != nil {
		log.Printf("Error getting photos for API: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	// If no photos are returned, it might be the end of the list.
	// Send an empty array instead of an error.
	if photos == nil {
		photos = []PhotoMetadata{}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photos)
}

func photoInfoHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	if _, ok := isValidSession(db, r); !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the requested filename from the URL path.
	// e.g., /photo/info/1698512345-my-photo.jpg -> 1698512345-my-photo.jpg
	filename := strings.TrimPrefix(r.URL.Path, "/photo/info/")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}

	// Retrieve photo metadata from the database.
	photoData, err := getPhotoByFilename(filename)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Photo not found", http.StatusNotFound)
		} else {
			log.Printf("Error getting photo info for %s: %v", filename, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(photoData)
}

func photoActionHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	if _, ok := isValidSession(db, r); !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Get the requested filename from the URL path.
	// e.g., /api/photo/1698512345-my-photo.jpg -> 1698512345-my-photo.jpg
	filename := strings.TrimPrefix(r.URL.Path, "/api/photo/")
	if filename == "" {
		http.Error(w, "Missing filename", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodDelete:
		handleDeletePhoto(w, r, filename)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func handleDeletePhoto(w http.ResponseWriter, r *http.Request, filename string) {
	// 1. Get photo metadata from DB to find its filepath.
	photo, err := getPhotoByFilename(filename)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Photo not found", http.StatusNotFound)
		} else {
			log.Printf("Error getting photo info for deletion %s: %v", filename, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// 2. Construct paths for all three files.
	originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)
	previewPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "previews", photo.Filepath)
	thumbPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "thumbs", photo.Filepath+".webp")

	// 3. Delete the files. We'll log errors but continue, to ensure we try to delete everything.
	if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete original file %s: %v", originalPath, err)
	}
	if err := os.Remove(previewPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete preview file %s: %v", previewPath, err)
	}
	if err := os.Remove(thumbPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete thumbnail file %s: %v", thumbPath, err)
	}

	// 4. Delete the database record.
	if err := deletePhotoByFilename(filename); err != nil {
		log.Printf("Error deleting photo record for %s: %v", filename, err)
		http.Error(w, "Error deleting photo from database", http.StatusInternalServerError)
		return
	}

	log.Printf("Successfully deleted photo '%s' and its associated files.", filename)
	w.WriteHeader(http.StatusNoContent) // 204 No Content is a good response for a successful DELETE.
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
		taskProgressMap.Lock()
		taskProgressMap.tasks[id].Complete = true
		taskProgressMap.Unlock()
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
