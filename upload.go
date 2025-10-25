package main

import (
	"image"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

	_ "image/jpeg" // Import for JPEG decoding
	_ "image/png"  // Import for PNG decoding

	"github.com/disintegration/imaging"
)

// Response struct for JSON responses
type uploadResponse struct {
	Status   string `json:"status"`
	Message  string `json:"message"`
	Filename string `json:"filename,omitempty"`
	ExifRead bool   `json:"exifRead"`
}
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	// Check if the method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify if the user is authenticated
	username, ok := isValidSession(db, r)
	if !ok {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(uploadResponse{
			Status:  "error",
			Message: "Authentication required",
		})
		return
	}

	// Maximum upload size of 20MB per file
	r.Body = http.MaxBytesReader(w, r.Body, 20*1024*1024)

	// Parse the multipart form, max memory of 20MB. Increased to handle larger single files if needed.
	if err := r.ParseMultipartForm(100 * 1024 * 1024); err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(uploadResponse{
			Status:  "error",
			Message: "File too large",
		})
		return
	}

	// Retrieve the single file from the "photo" input field
	file, header, err := r.FormFile("photo")
	if err != nil {
		if err == http.ErrMissingFile {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(uploadResponse{
				Status:  "error",
				Message: "No file was uploaded",
			})
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(uploadResponse{
				Status:  "error",
				Message: "Error retrieving the file",
			})
		}
		return
	}
	defer file.Close()

	// Check the file type (you can add more checks as needed)
	contentType := header.Header.Get("Content-Type")
	if contentType != "image/jpeg" && contentType != "image/png" {
		log.Printf("Invalid file type for %s: %s", header.Filename, contentType)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(uploadResponse{
			Status:  "error",
			Message: fmt.Sprintf("Invalid file type: %s", contentType),
		})
		return
	}

	// --- Save to a temporary file first ---
	// Create the temporary file within the user's dedicated upload directory.
	// This ensures it's on the same filesystem as the final destination, allowing os.Rename.
	userTempDir := filepath.Join(AppConfig.PhotoUploadDir, username)
	if err := os.MkdirAll(userTempDir, 0755); err != nil {
		handleUploadError(w, "Could not create user's temporary upload directory", http.StatusInternalServerError, err)
		return
	}

	tempFile, err := os.CreateTemp(userTempDir, "upload-*.tmp")
	if err != nil {
		handleUploadError(w, "Could not create temporary file", http.StatusInternalServerError, err)
		return
	}
	// The temporary file will be removed by os.Rename or explicitly if an error occurs before moving.
	defer tempFile.Close()

	if _, err := io.Copy(tempFile, file); err != nil {
		handleUploadError(w, "Could not save to temporary file", http.StatusInternalServerError, err)
		return
	}

	// --- EXIF Parsing (from memory) ---
	exifInfo, exifReadSuccessfully := parseExifDataFromFile(tempFile.Name())

	// Determine the date to use for the folder structure.
	// Prioritize DateTimeOriginal, then DateTimeDigitized, then DateTime from EXIF, then fallback to the current time.
	photoDate := time.Now()
	if !exifInfo.DateTimeOriginal.IsZero() {
		photoDate = exifInfo.DateTimeOriginal
	} else if !exifInfo.DateTimeDigitized.IsZero() {
		photoDate = exifInfo.DateTimeDigitized
	} else if !exifInfo.DateTime.IsZero() {
		photoDate = exifInfo.DateTime
	}

	// If EXIF data did not contain dimensions, try to get them by decoding the image config.
	// This is efficient as it doesn't decode the whole image.
	if exifInfo.ImageWidth == 0 || exifInfo.ImageLength == 0 {
		if f, err := os.Open(tempFile.Name()); err == nil {
			defer f.Close()
			config, _, err := image.DecodeConfig(f)
			if err == nil {
				exifInfo.ImageWidth = uint32(config.Width)
				exifInfo.ImageLength = uint32(config.Height)
			}
		}
	}

	// Move the temporary file to its final, permanent location.
	newFilePath, newFilename, relativePath, err := moveAndSaveFile(tempFile.Name(), header.Filename, photoDate, username)
	if err != nil {
		handleUploadError(w, "Could not save file to permanent storage", http.StatusInternalServerError, err)
		return
	}

	// Generate thumbnail synchronously before responding.
	if err := createThumbnail(newFilePath, username); err != nil {
		log.Printf("Warning: failed to create thumbnail for %s: %v", newFilePath, err)
	}

	log.Printf("File %s uploaded as %s to %s", header.Filename, newFilename, newFilePath)

	// Create a PhotoMetadata struct to hold all the data.
	photoData := &PhotoMetadata{
		Filename:    newFilename,     // The name of the file itself
		Filepath:    relativePath,    // The path relative to the media root
		UploadedBy:  username,
		UploadedAt:  time.Now(),
		ImageWidth:  int64(exifInfo.ImageWidth),
		ImageLength: int64(exifInfo.ImageLength),
		DateTime:    photoDate,       // Use the determined best date
	}

	// Send metadata to the background worker queue to be saved asynchronously.
	// This is non-blocking and prevents DB contention during mass uploads.
	photoMetadataQueue <- photoData

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadResponse{
		Status:   "success",
		Message:  "File uploaded successfully",
		Filename: newFilename,
		ExifRead: exifReadSuccessfully,
	})
}

// handleUploadError is a helper to standardize JSON error responses for the upload handler.
func handleUploadError(w http.ResponseWriter, message string, statusCode int, err error) {
	log.Printf("Upload error: %s - %v", message, err)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(uploadResponse{
		Status:  "error",
		Message: message,
	})
}

// parseExifDataFromFile opens a file from its path and parses EXIF data.
func parseExifDataFromFile(filePath string) (ExifInfo, bool) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Printf("Could not open file %s for EXIF parsing: %v", filePath, err)
		return ExifInfo{}, false
	}
	defer file.Close()
	return parseExifData(file)
}

// moveAndSaveFile moves a file from a temporary path to its final destination
// in the uploads directory with a unique name based on the photo's date.
func moveAndSaveFile(tempPath, originalFilename string, photoDate time.Time, username string) (string, string, string, error) {
	// Get date parts from the provided photoDate to create the directory structure.
	year := photoDate.Format("2006")
	month := photoDate.Format("01")
	day := photoDate.Format("02")

	// Construct the target directory path: /data/tmunot/<username>/originals/YEAR/MONTH/DAY
	targetDir := filepath.Join(AppConfig.PhotoUploadDir, username, "originals", year, month, day)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", "", "", fmt.Errorf("failed to create upload directory: %w", err)
	}

	// Generate a new unique filename to prevent overwrites and improve security.
	// Format: <timestamp>-<original_filename>
	newFilename := fmt.Sprintf("%d-%s", time.Now().Unix(), originalFilename)
	relativePath := filepath.Join(year, month, day, newFilename)
	newFilePath := filepath.Join(targetDir, newFilename)

	// Move the file from the temporary path to the new path.
	// os.Rename is an atomic operation and works across directories on the same filesystem.
	if err := os.Rename(tempPath, newFilePath); err != nil {
		return "", "", "", fmt.Errorf("failed to move temp file to final destination: %w", err)
	}

	return newFilePath, newFilename, relativePath, nil
}

// createThumbnail generates a 500px wide WebP thumbnail for the given image.
// The thumbnail is saved in a 'thumbs' subdirectory, mirroring the original's path.
func createThumbnail(originalPath string, username string) error {
	// Open the original image file.
	srcImage, err := imaging.Open(originalPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("failed to open image for thumbnailing: %w", err)
	}

	// Resize the image to a width of 500px, preserving the aspect ratio.
	thumb := imaging.Resize(srcImage, 500, 0, imaging.Lanczos)

	// Determine the path for the thumbnail.
	// originalPath is like /data/tmunot/<user>/originals/YYYY/MM/DD/file.jpg
	// We want to create /data/tmunot/<user>/thumbs/YYYY/MM/DD/
	basePath := strings.TrimPrefix(originalPath, filepath.Join(AppConfig.PhotoUploadDir, username, "originals"))
	baseDir := filepath.Dir(basePath)

	// Construct the full directory path for the thumbnail.
	thumbDir := filepath.Join(AppConfig.PhotoUploadDir, username, "thumbs", baseDir)
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// The thumbnail will have the same filename as the original.
	thumbFilename := filepath.Base(originalPath)
	thumbPath := filepath.Join(thumbDir, thumbFilename)

	// Save the thumbnail image as a JPEG with 80% quality.
	if err := imaging.Save(thumb, thumbPath, imaging.JPEGQuality(80)); err != nil {
		return fmt.Errorf("failed to save thumbnail jpeg: %w", err)
	}

	log.Printf("Created thumbnail at %s", thumbPath)
	return nil
}

// createPreview generates a 1920px wide JPEG preview for the given image.
// The preview is saved in a 'previews' subdirectory, mirroring the original's path.
func createPreview(originalPath string, username string) error {
	// Open the original image file.
	srcImage, err := imaging.Open(originalPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("failed to open image for preview: %w", err)
	}

	// Resize the image to a width of 1920px, preserving the aspect ratio.
	preview := imaging.Resize(srcImage, 1920, 0, imaging.Lanczos)

	// Determine the path for the preview.
	// originalPath is like /data/tmunot/<user>/originals/YYYY/MM/DD/file.jpg
	// We want to create /data/tmunot/<user>/previews/YYYY/MM/DD/
	basePath := strings.TrimPrefix(originalPath, filepath.Join(AppConfig.PhotoUploadDir, username, "originals"))
	baseDir := filepath.Dir(basePath)
	previewDir := filepath.Join(AppConfig.PhotoUploadDir, username, "previews", baseDir)
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	previewFilename := filepath.Base(originalPath)
	previewPath := filepath.Join(previewDir, previewFilename)

	// Save the preview image as a JPEG with 80% quality.
	err = imaging.Save(preview, previewPath, imaging.JPEGQuality(80))
	if err != nil {
		return fmt.Errorf("failed to save preview jpeg: %w", err)
	}

	log.Printf("Created preview at %s", previewPath)
	return nil
}
