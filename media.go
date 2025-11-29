package main

import (
	"net/http"
	"path"
	"strings"
)

func mediaHandler(w http.ResponseWriter, r *http.Request) {
	sessionUser, ok := isValidSession(db, r)
	if !ok {
		// If the session is invalid, redirect to the login page,
		// Return an unauthorized error, which the frontend will catch.
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// Security Check: Ensure the logged-in user is accessing their own media.
	// Clean the path to resolve ".." segments.
	// r.URL.Path starts with /media/
	relativePath := strings.TrimPrefix(r.URL.Path, "/media/")
	cleanPath := path.Clean(relativePath)

	// Ensure the cleaned path starts with the session user's directory.
	// We check for "username/" or exact "username" to avoid partial matches (e.g. "user1" matching "user10").
	if !strings.HasPrefix(cleanPath, sessionUser+"/") && cleanPath != sessionUser {
		http.Error(w, "Forbidden", http.StatusForbidden)
		return
	}

	// Update the request URL to use the cleaned path, preventing the FileServer from seeing the ".."
	r.URL.Path = "/media/" + cleanPath

	http.StripPrefix("/media/", http.FileServer(http.Dir(AppConfig.PhotoUploadDir))).ServeHTTP(w, r)
}
