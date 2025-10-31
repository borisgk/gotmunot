// /home/ubuntu/go/src/tm25/main.go
package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
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

	// Initialize and start the metadata saving queue and worker.
	// A buffer of 100 can handle bursts of uploads.
	photoMetadataQueue = make(chan *PhotoMetadata, 100)
	go startMetadataSaveWorker()

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

	// Check if we need to show a specific preview after login.
	showPreview := r.URL.Query().Get("show_preview")

	// Check for a year filter from the query parameters.
	yearStr := r.URL.Query().Get("year")
	year, _ := strconv.Atoi(yearStr) // Atoi returns 0 on error, which we use to mean "no filter".

	// Get all photos matching the filter.
	userDB, err := getUserDB(username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	photos, err := getPhotos(userDB, username, year)
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
			// When filtering by year, skip any photos that don't match the filter year.
			// This prevents incorrect grouping from the last day of the previous year.
			if year > 0 && getPhotoTime(&p).Year() != year {
				continue
			}
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

	// Sort photos within each day group in ascending order.
	for i := range dayGroups {
		sort.Slice(dayGroups[i].Photos, func(j, k int) bool {
			// getPhotoTime gets the best available time for sorting.
			return getPhotoTime(&dayGroups[i].Photos[j]).Before(getPhotoTime(&dayGroups[i].Photos[k]))
		})
	}

	// Get the total number of photos for the frontend to know when to stop loading.
	// The count must also be filtered by year.
	totalPhotos, err := getTotalPhotoCount(userDB, year)
	if err != nil {
		log.Printf("Error getting total photo count: %v", err)
		totalPhotos = 0 // Default to 0 on error
	}

	// Get total count for the "All" link, regardless of year filter.
	allPhotosCount, err := getTotalPhotoCount(userDB, 0)
	if err != nil {
		log.Printf("Error getting total count for 'All' photos: %v", err)
		allPhotosCount = 0
	}

	// Get photo counts for the year bar
	photoCounts, err := getPhotoCountsByYear(userDB)
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
		Username       string
		DayGroups      []DayGroup
		TotalPhotos    int
		AllPhotosCount int
		ShowPreview    string
		FilterYear     int
		Years          []int
		PhotoCounts    map[int]int
	}{
		Username:       username,
		DayGroups:      dayGroups,
		AllPhotosCount: allPhotosCount,
		ShowPreview:    showPreview,
		TotalPhotos:    totalPhotos,
		FilterYear:     year,
		Years:          years,
		PhotoCounts:    photoCounts,
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

func photoInfoHandler(w http.ResponseWriter, r *http.Request) {
	// First, verify the user has a valid session.
	username, ok := isValidSession(db, r)
	if !ok {
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

	userDB, err := getUserDB(username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Retrieve photo metadata from the database.
	photoData, err := getPhotoByFilename(userDB, username, filename)
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
	username, ok := isValidSession(db, r)
	if !ok {
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
		handleDeletePhoto(w, username, filename)
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
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

func handleDeletePhoto(w http.ResponseWriter, username, filename string) {
	// 1. Get photo metadata from DB to find its filepath.
	err := deletePhoto(username, filename)
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

	// 4. Iterate and delete each photo
	// We'll collect errors but not stop on the first one.
	var errors []string
	for _, filename := range payload.Filenames {
		if err := deletePhoto(username, filename); err != nil {
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

// deletePhoto contains the core logic to delete a single photo and its files.
func deletePhoto(username, filename string) error {
	userDB, err := getUserDB(username)
	if err != nil {
		return err
	}

	// Get photo metadata from DB to find its filepath.
	photo, err := getPhotoByFilename(userDB, username, filename)
	if err != nil {
		return err // Propagate error (e.g., sql.ErrNoRows)
	}

	// 2. Construct paths for all three files.
	originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)
	previewPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "previews", photo.Filepath)
	thumbPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "thumbs", photo.Filepath) // No .webp extension

	// 3. Delete the files. We'll log errors but continue, to ensure we try to delete everything.
	if err := os.Remove(originalPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete original file %s: %v", originalPath, err)
	}
	if err := os.Remove(previewPath); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete preview file %s: %v", previewPath, err)
	}
	if err := os.Remove(thumbPath + ".webp"); err != nil && !os.IsNotExist(err) {
		log.Printf("Warning: could not delete thumbnail file %s: %v", thumbPath, err)
	}

	// 4. Delete the database record.
	if err := deletePhotoByFilename(userDB, filename); err != nil {
		log.Printf("Error deleting photo record for %s: %v", filename, err)
		return fmt.Errorf("error deleting photo from database: %w", err)
	}
	return nil
}

// getPhotoTime returns the most relevant time.Time for a photo.
func getPhotoTime(p *PhotoMetadata) time.Time {
	// DateTime is now the pre-calculated best date, so we use it directly.
	if !p.DateTime.IsZero() {
		return p.DateTime
	}
	return p.UploadedAt // Fallback for any old data that might not have DateTime
}
