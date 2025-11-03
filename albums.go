// Albums related functions

package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// Album represents a collection of photos.
type Album struct {
	ID          int
	Name        string
	Description string
	PhotoCount  int
	CoverPhoto  string // URL to the cover photo
	CreatedAt   time.Time
}

func albumsHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	userDB, err := getUserDB(username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Fetch albums from the database
	albums, err := getAlbumsForUser(userDB, username)
	if err != nil {
		log.Printf("Error fetching albums for user %s: %v", username, err)
		http.Error(w, "Failed to fetch albums.", http.StatusInternalServerError)
		return
	}

	data := struct {
		Username string
		Albums   []Album
	}{
		Username: username,
		Albums:   albums,
	}

	// Execute the "albums.html" template
	if err := tmpl.ExecuteTemplate(w, "albums.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func createAlbumHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	username, ok := isValidSession(db, r)
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

	userDB, err := getUserDB(username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// Check if an album with this name already exists
	exists, err := albumExists(userDB, payload.Name)
	if err != nil {
		http.Error(w, "Error checking for existing album.", http.StatusInternalServerError)
		return
	}
	if exists {
		http.Error(w, "An album with this name already exists.", http.StatusConflict)
		return
	}

	// Create the album
	albumID, err := createAlbum(userDB, payload.Name, payload.Description)
	if err != nil {
		http.Error(w, "Failed to create album.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"status": "success", "album_id": albumID})
}

func newAlbumHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := isValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	data := struct{ Username string }{Username: username}

	if err := tmpl.ExecuteTemplate(w, "new_album.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
