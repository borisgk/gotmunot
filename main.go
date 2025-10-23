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
	"sort"
)

// User struct to represent a user.
type User struct {
	ID       int
	Username string
	Password string // Hash!
}

// TaskProgress holds the state of a long-running task.
type TaskProgress struct {
	Processed int    `json:"processed,omitempty"`
	Total     int    `json:"total,omitempty"`
	Filename  string `json:"filename,omitempty"`
	Complete  bool   `json:"complete"`
	Error     string `json:"error,omitempty"`
	Cancelled bool   `json:"cancelled,omitempty"`
	DownloadURL string `json:"download_url,omitempty"`
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
		http.Redirect(w, r, "/gallery", http.StatusSeeOther)
	})

	// Define a handler function for the /status path
	http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "TM25 is running!")
	})

	// Handler for the login page
	http.HandleFunc("/login", loginHandler)
	// Handler for logout page
	http.HandleFunc("/logout", logoutHandler)
	// Handler for the gallery page
	http.HandleFunc("/gallery", galleryHandler)
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
	// API for batch photo operations
	http.HandleFunc("/api/photos/delete", batchDeletePhotosHandler)
	http.HandleFunc("/api/photos/regenerate", batchRegenerateHandler)
	// API for downloading zipped previews
	http.HandleFunc("/api/photos/download-previews", downloadPreviewsHandler)
	// API for async downloads
	http.HandleFunc("/api/downloads/start", startDownloadHandler)
	http.HandleFunc("/api/downloads/status", getDownloadStatusHandler)
	http.HandleFunc("/api/downloads/cancel", cancelDownloadHandler)
	http.HandleFunc("/api/downloads/file", serveDownloadHandler)

	// Serve static files (CSS, JS, etc.)
	http.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir("static/css"))))
	http.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir("static/js"))))

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
		http.Redirect(w, r, "/gallery", http.StatusSeeOther)

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

// DayGroup is a struct to hold photos grouped by a specific date.
type DayGroup struct {
	Date   time.Time
	Photos []PhotoMetadata
	Count  int
}

func galleryHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Check for a year filter from the query parameters.
	yearStr := r.URL.Query().Get("year")
	year, _ := strconv.Atoi(yearStr) // Atoi returns 0 on error, which we use to mean "no filter".

	// Define the number of photos per page for the initial load.
	const initialLimit = 50

	// Get the first page of photos.
	photos, err := getPhotos(username, year, initialLimit, 0)
	if err != nil {
		log.Printf("Error getting recent photos: %v", err)
		// If we can't get photos, we can still render the page but with an empty photo slice.
		photos = []PhotoMetadata{}
	}

	// Group photos by date for the template
	var dayGroups []DayGroup
	if len(photos) > 0 {
		currentDateStr := ""
		var currentGroup *DayGroup

		for _, p := range photos {
			photoDateStr := getPhotoDateString(&p)
			if photoDateStr != currentDateStr {
				if currentGroup != nil {
					dayGroups = append(dayGroups, *currentGroup)
				}
				currentGroup = &DayGroup{Date: getPhotoTime(&p)}
				currentDateStr = photoDateStr
			}
			currentGroup.Photos = append(currentGroup.Photos, p)
			currentGroup.Count++
		}
		if currentGroup != nil {
			dayGroups = append(dayGroups, *currentGroup)
		}
	}

	// Get the total number of photos for the frontend to know when to stop loading.
	// The count must also be filtered by year.
	totalPhotos, err := getTotalPhotoCount(username, year)
	if err != nil {
		log.Printf("Error getting total photo count: %v", err)
		totalPhotos = 0 // Default to 0 on error
	}

	// Get total count for the "All" link, regardless of year filter.
	allPhotosCount, err := getTotalPhotoCount(username, 0)
	if err != nil {
		log.Printf("Error getting total count for 'All' photos: %v", err)
		allPhotosCount = 0
	}

	// Get photo counts for the year bar
	photoCounts, err := getPhotoCountsByYear(username)
	if err != nil {
		log.Printf("Error getting photo counts by year: %v", err)
		photoCounts = make(map[int]int) // Ensure it's not nil
	}

	// Get a sorted list of years from the map keys.
	var years []int
	for year := range photoCounts {
		years = append(years, year)
	}
	sort.Ints(years)

	// Create a struct to hold all the data for the template
	data := struct {
		Username    string
		DayGroups   []DayGroup
		TotalPhotos int
		AllPhotosCount int
		Limit       int
		FilterYear  int
		Years       []int
		PhotoCounts map[int]int
	}{
		Username:    username,
		DayGroups:   dayGroups,
		AllPhotosCount: allPhotosCount,
		TotalPhotos: totalPhotos,
		Limit:       initialLimit,
		FilterYear:  year,
		Years:       years,
		PhotoCounts: photoCounts,
	}

	// Execute the "gallery.html" template and pass the data.
	if err := tmpl.ExecuteTemplate(w, "gallery.html", data); err != nil {
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
	offsetStr := r.URL.Query().Get("offset")
	limitStr := r.URL.Query().Get("limit")
	yearStr := r.URL.Query().Get("year")

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50 // Default limit
	}

	year, _ := strconv.Atoi(yearStr)

	// Retrieve photos from the database
	photos, err := getPhotos(username, year, limit, offset)
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
	err := deletePhoto(filename)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Photo not found", http.StatusNotFound)
		} else {
			log.Printf("Error getting photo info for deletion %s: %v", filename, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	log.Printf("Successfully deleted photo '%s' and its associated files.", filename)
	w.WriteHeader(http.StatusNoContent) // 204 No Content is a good response for a successful DELETE.
}

func batchDeletePhotosHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	if _, ok := isValidSession(db, r); !ok {
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
		Filenames []string `json:"filenames"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 4. Iterate and delete each photo
	// We'll collect errors but not stop on the first one.
	var errors []string
	for _, filename := range payload.Filenames {
		if err := deletePhoto(filename); err != nil {
			log.Printf("Failed to delete photo %s during batch operation: %v", filename, err)
			errors = append(errors, fmt.Sprintf("Failed to delete %s: %v", filename, err.Error()))
		}
	}

	// 5. Respond
	if len(errors) > 0 {
		http.Error(w, fmt.Sprintf("Completed with %d errors. See logs for details.", len(errors)), http.StatusMultiStatus)
		return
	}

	w.WriteHeader(http.StatusNoContent) // All successful
}

func batchRegenerateHandler(w http.ResponseWriter, r *http.Request) {
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
		Filenames []string `json:"filenames"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 4. Iterate and start regeneration in goroutines
	for _, filename := range payload.Filenames {
		go func(fname string) {
			photo, err := getPhotoByFilename(fname)
			if err != nil {
				log.Printf("Failed to find photo '%s' for regeneration: %v", fname, err)
				return
			}

			// Ensure the user owns this photo before regenerating
			if photo.UploadedBy != username {
				log.Printf("Security alert: User '%s' attempted to regenerate photo '%s' owned by '%s'", username, fname, photo.UploadedBy)
				return
			}

			originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)
			createThumbnail(originalPath, photo.UploadedBy)
			createPreview(originalPath, photo.UploadedBy)
		}(filename)
	}

	// 5. Respond immediately
	w.WriteHeader(http.StatusAccepted) // 202 Accepted is a good response for starting a background task.
}

// deletePhoto contains the core logic to delete a single photo and its files.
func deletePhoto(filename string) error {
	// Get photo metadata from DB to find its filepath.
	photo, err := getPhotoByFilename(filename)
	if err != nil {
		return err // Propagate error (e.g., sql.ErrNoRows)
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
		return fmt.Errorf("error deleting photo from database: %w", err)
	}
	return nil
}

// getPhotoTime returns the most relevant time.Time for a photo.
func getPhotoTime(p *PhotoMetadata) time.Time {
	if p.DateTimeOriginal.Valid {
		return p.DateTimeOriginal.Time
	} else if p.DateTime.Valid {
		return p.DateTime.Time
	}
	return p.UploadedAt
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
