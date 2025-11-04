// /home/ubuntu/go/src/tm25/gallery.go
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
)

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

		for i := range photos {
			p := &photos[i] // Use a pointer to modify the original photo in the slice.

			// Pre-calculate paths for the template before any other logic.
			p.ThumbPath = filepath.Join("/media", username, "thumbs", p.Filepath)
			p.PreviewPath = filepath.Join("/media", username, "previews", p.Filepath)

			// When filtering by year, skip any photos that don't match the filter year.
			// This prevents incorrect grouping from the last day of the previous year.
			if year > 0 && getPhotoTime(p).Year() != year {
				continue
			}
			photoDateStr := getPhotoDateString(p)
			if photoDateStr != currentDateStr {
				if currentGroup != nil {
					dayGroups = append(dayGroups, *currentGroup)
				}
				currentGroup = &DayGroup{Date: getPhotoTime(p)}
				currentDateStr = photoDateStr
			}
			currentGroup.Photos = append(currentGroup.Photos, *p)
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

func albumActionHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Extract album ID from URL, e.g., /api/album/123
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid album ID", http.StatusBadRequest)
		return
	}
	albumID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		http.Error(w, "Invalid album ID format", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPatch:
		var payload struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		userDB, err := getUserDB(username)
		if err != nil {
			http.Error(w, "Could not access user database", http.StatusInternalServerError)
			return
		}

		if err := updateAlbum(userDB, albumID, payload.Name, payload.Description); err != nil {
			log.Printf("Error updating album %d: %v", albumID, err)
			http.Error(w, "Failed to update album", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		// Return the updated data as confirmation
		json.NewEncoder(w).Encode(payload)
	case http.MethodDelete:
		userDB, err := getUserDB(username)
		if err != nil {
			http.Error(w, "Could not access user database", http.StatusInternalServerError)
			return
		}

		if err := deleteAlbum(userDB, albumID); err != nil {
			log.Printf("Error deleting album %d: %v", albumID, err)
			http.Error(w, "Failed to delete album", http.StatusInternalServerError)
			return
		}

		log.Printf("Successfully deleted album %d for user %s", albumID, username)
		w.WriteHeader(http.StatusNoContent) // 204 No Content
	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
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
