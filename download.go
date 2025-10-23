package main

import (
	"archive/zip"
	"bytes"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func downloadPreviewsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Get filenames from query parameters
	filenames := r.URL.Query()["filename"]
	if len(filenames) == 0 {
		http.Error(w, "No filenames provided", http.StatusBadRequest)
		return
	}

	// 3. Create a buffer to write our zip archive to in memory.
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 4. Add files to the zip.
	for _, filename := range filenames {
		photo, err := getPhotoByFilename(filename)
		if err != nil {
			log.Printf("Could not find photo '%s' for zipping: %v", filename, err)
			continue // Skip this file
		}

		// Security check: ensure user owns the photo
		if photo.UploadedBy != username {
			log.Printf("Security alert: User '%s' attempted to download preview for photo '%s' owned by '%s'", username, filename, photo.UploadedBy)
			continue
		}

		// Construct the path to the preview file on disk
		previewPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "previews", photo.Filepath)

		// Read the file content
		fileData, err := os.ReadFile(previewPath)
		if err != nil {
			log.Printf("Could not read preview file '%s': %v", previewPath, err)
			continue
		}

		// Strip the timestamp prefix from the filename for the zip entry
		// e.g., "1698512345-my-photo.jpg" -> "my-photo.jpg"
		_, strippedFilename, _ := strings.Cut(filename, "-")
		if strippedFilename == "" {
			strippedFilename = filename // Fallback if there's no hyphen
		}

		// Create a new file header in the zip archive
		f, err := zipWriter.Create(strippedFilename)
		if err != nil {
			log.Printf("Could not create zip entry for '%s': %v", strippedFilename, err)
			continue
		}

		// Write the file data to the zip entry
		_, err = f.Write(fileData)
		if err != nil {
			log.Printf("Could not write data to zip for '%s': %v", strippedFilename, err)
			continue
		}
	}

	zipWriter.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"previews-%d.zip\"", time.Now().Unix()))
	w.Write(buf.Bytes())
}

func downloadOriginalsHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := isValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Get filenames from query parameters
	filenames := r.URL.Query()["filename"]
	if len(filenames) == 0 {
		http.Error(w, "No filenames provided", http.StatusBadRequest)
		return
	}

	// 3. Create a buffer to write our zip archive to in memory.
	buf := new(bytes.Buffer)
	zipWriter := zip.NewWriter(buf)

	// 4. Add files to the zip.
	for _, filename := range filenames {
		photo, err := getPhotoByFilename(filename)
		if err != nil {
			log.Printf("Could not find photo '%s' for zipping: %v", filename, err)
			continue // Skip this file
		}

		// Security check: ensure user owns the photo
		if photo.UploadedBy != username {
			log.Printf("Security alert: User '%s' attempted to download original for photo '%s' owned by '%s'", username, filename, photo.UploadedBy)
			continue
		}

		// Construct the path to the original file on disk
		originalPath := filepath.Join(AppConfig.PhotoUploadDir, photo.UploadedBy, "originals", photo.Filepath)

		// Read the file content
		fileData, err := os.ReadFile(originalPath)
		if err != nil {
			log.Printf("Could not read original file '%s': %v", originalPath, err)
			continue
		}

		// Strip the timestamp prefix from the filename for the zip entry
		_, strippedFilename, _ := strings.Cut(filename, "-")
		if strippedFilename == "" {
			strippedFilename = filename // Fallback if there's no hyphen
		}

		// Create a new file header in the zip archive
		f, err := zipWriter.Create(strippedFilename)
		if err != nil {
			log.Printf("Could not create zip entry for '%s': %v", strippedFilename, err)
			continue
		}

		// Write the file data to the zip entry
		_, err = f.Write(fileData)
		if err != nil {
			log.Printf("Could not write data to zip for '%s': %v", strippedFilename, err)
			continue
		}
	}

	zipWriter.Close()

	w.Header().Set("Content-Type", "application/zip")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"originals-%d.zip\"", time.Now().Unix()))
	w.Write(buf.Bytes())
}