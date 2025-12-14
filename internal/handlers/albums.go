package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"tm25/internal/auth"
	"tm25/internal/database"
	"tm25/internal/models"
)

func albumsHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Fetch albums from the database
	albums, err := database.GetAlbumsForUser(userDB, username)
	if err != nil {
		log.Printf("Error fetching albums for user %s: %v", username, err)
		http.Error(w, "Failed to fetch albums.", http.StatusInternalServerError)
		return
	}

	data := struct {
		Username    string
		Albums      []models.Album
		CurrentPage string
	}{
		Username:    username,
		Albums:      albums,
		CurrentPage: "albums",
	}

	// Execute the "albums.html" template
	if err := tmpl.ExecuteTemplate(w, "albums.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func albumDetailHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	// Parse album ID from URL, e.g., /album/123
	idStr := strings.TrimPrefix(r.URL.Path, "/album/")
	albumID, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid album ID", http.StatusBadRequest)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Fetch album details and photos
	album, err := database.GetAlbumDetails(userDB, albumID)
	if err != nil {
		http.Error(w, "Album not found.", http.StatusNotFound)
		return
	}

	photos, err := database.GetPhotosForAlbum(userDB, username, albumID)
	if err != nil {
		http.Error(w, "Failed to fetch photos for album.", http.StatusInternalServerError)
		return
	}

	// Group photos by date for the template (reusing gallery logic)
	var dayGroups []models.DayGroup
	if len(photos) > 0 {
		currentDateStr := ""
		var currentGroup *models.DayGroup

		for i := range photos {
			p := &photos[i]
			photoDateStr := database.GetPhotoDateString(p)
			if photoDateStr != currentDateStr {
				if currentGroup != nil {
					dayGroups = append(dayGroups, *currentGroup)
				}
				currentGroup = &models.DayGroup{Date: database.GetPhotoTime(p)}
				currentDateStr = photoDateStr
			}
			currentGroup.Photos = append(currentGroup.Photos, *p)
			currentGroup.Count++
		}
		if currentGroup != nil {
			dayGroups = append(dayGroups, *currentGroup)
		}
	}

	// Sort photos within each day group
	for i := range dayGroups {
		sort.Slice(dayGroups[i].Photos, func(j, k int) bool {
			return database.GetPhotoTime(&dayGroups[i].Photos[j]).Before(database.GetPhotoTime(&dayGroups[i].Photos[k]))
		})
	}

	data := struct {
		Username    string
		Album       *models.Album
		TotalPhotos int // For photogrid template
		FilterYear  int // For photogrid template
		DayGroups   []models.DayGroup
		CurrentPage string
	}{
		Username:    username,
		Album:       album,
		TotalPhotos: len(photos),
		FilterYear:  0, // No year filtering on this page
		DayGroups:   dayGroups,
		CurrentPage: "albums",
	}

	if err := tmpl.ExecuteTemplate(w, "album_detail.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func getAlbumListHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	albumList, err := database.GetAlbumListForUser(userDB)
	if err != nil {
		http.Error(w, "Failed to retrieve album list.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(albumList)
}

func createAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate input
	payload.Name = strings.TrimSpace(payload.Name)
	if payload.Name == "" {
		http.Error(w, "Album name cannot be empty", http.StatusBadRequest)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Check if an album with this name already exists
	exists, err := database.AlbumExists(userDB, payload.Name)
	if err != nil {
		http.Error(w, "Error checking for existing album.", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "An album with this name already exists.", http.StatusConflict)
		return
	}

	// Create the album
	albumID, err := database.CreateAlbum(userDB, payload.Name, payload.Description)
	if err != nil {
		http.Error(w, "Failed to create album.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "album_id": albumID})
}

func addPhotosToAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var payload struct {
		AlbumID   int64    `json:"album_id"`
		Filenames []string `json:"filenames"`
	}

	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if payload.AlbumID == 0 || len(payload.Filenames) == 0 {
		http.Error(w, "Album ID and filenames are required", http.StatusBadRequest)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	photosAdded, err := database.AddPhotosToAlbum(userDB, payload.AlbumID, payload.Filenames)
	if err != nil {
		log.Printf("Error adding photos to album %d for user %s: %v", payload.AlbumID, username, err)
		http.Error(w, "Failed to add photos to album.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":       "success",
		"photos_added": photosAdded,
	})
}

func newAlbumHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct {
		Username    string
		CurrentPage string
	}{
		Username:    username,
		CurrentPage: "albums",
	}

	if err := tmpl.ExecuteTemplate(w, "new_album.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
