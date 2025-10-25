package main

import (
	"image"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
	"strings"


	_ "image/jpeg" // Import for JPEG decoding
	_ "image/png"  // Import for PNG decoding

	"github.com/davidbyttow/govips/v2/vips"
	"path/filepath"
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

	// --- Read file into memory ---
	// This allows us to perform multiple operations (EXIF, save, thumbnail) without re-reading from disk.
	fileBytes, err := io.ReadAll(file)
	if err != nil {
		handleUploadError(w, "Could not read uploaded file into memory", http.StatusInternalServerError, err)
		return
	}

	// Check the file type from the detected content
	contentType := http.DetectContentType(fileBytes)
	if contentType != "image/jpeg" && contentType != "image/png" {
		handleUploadError(w, fmt.Sprintf("Invalid file type: %s", contentType), http.StatusBadRequest, nil)
		return
	}

	// --- EXIF Parsing (from memory) ---
	exifInfo, exifReadSuccessfully := parseExifData(bytes.NewReader(fileBytes))

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
		config, _, err := image.DecodeConfig(bytes.NewReader(fileBytes))
		if err == nil {
			exifInfo.ImageWidth = uint32(config.Width)
			exifInfo.ImageLength = uint32(config.Height)
		}
	}

	// Save the original file from the in-memory bytes.
	newFilePath, newFilename, relativePath, err := saveUploadedFile(bytes.NewReader(fileBytes), header.Filename, photoDate, username)
	if err != nil {
		handleUploadError(w, "Could not save file to permanent storage", http.StatusInternalServerError, err)
		return
	}

	// Generate thumbnail synchronously before responding.
	if err := createThumbnailFromBytes(fileBytes, newFilePath, username); err != nil {
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

// saveUploadedFile saves the uploaded file to the uploads directory with a unique name.
func saveUploadedFile(file io.Reader, originalFilename string, photoDate time.Time, username string) (string, string, string, error) {
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

	// Create the new file
	newFile, err := os.Create(newFilePath)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to create new file: %w", err)
	}
	defer newFile.Close()

	// Copy the file content
	_, err = io.Copy(newFile, file)
	if err != nil {
		// If copy fails, we should remove the partially created file.
		os.Remove(newFilePath)
		return "", "", "", fmt.Errorf("failed to copy file content: %w", err)
	}

	return newFilePath, newFilename, relativePath, nil
}

// createThumbnailFromBytes generates a 500px wide JPEG thumbnail from an in-memory byte slice.
// This is used during the initial upload process.
func createThumbnailFromBytes(imageBytes []byte, originalPath string, username string) error {
	// Determine the path for the thumbnail.
	// originalPath is like /data/tmunot/<user>/originals/YYYY/MM/DD/file.jpg
	// We want to create /data/tmunot/<user>/thumbs/YYYY/MM/DD/file.jpg
	basePath := strings.TrimPrefix(originalPath, filepath.Join(AppConfig.PhotoUploadDir, username, "originals"))
	baseDir := filepath.Dir(basePath)

	// The reader is passed for uploads to create the thumbnail from memory.
	// For regeneration tasks, originalPath is a file on disk. We will prefer the path.

	// Construct the full directory path for the thumbnail.
	thumbDir := filepath.Join(AppConfig.PhotoUploadDir, username, "thumbs", baseDir)
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// The thumbnail will have the same filename as the original.
	thumbFilename := filepath.Base(originalPath)
	thumbPath := filepath.Join(thumbDir, thumbFilename)

	image, err := vips.NewImageFromBuffer(imageBytes)
	if err != nil {
		return fmt.Errorf("govips: failed to create image from buffer for %s: %w", originalPath, err)
	}
	defer image.Close()

	// `Thumbnail` is highly optimized for creating thumbnails. It auto-rotates based on EXIF.
	if err := image.Thumbnail(500, 0, vips.InterestingNone); err != nil {
		return fmt.Errorf("govips: failed to thumbnail image %s: %w", originalPath, err)
	}

	// Export to JPEG format with a quality of 80.
	jpegParams := vips.NewJpegExportParams()
	jpegParams.Quality = 80
	jpegParams.StripMetadata = true


	imageBytes, _, err = image.ExportJpeg(jpegParams)
	if err != nil {
		return fmt.Errorf("govips: failed to export jpeg for %s: %w", originalPath, err)
	}

	if err = os.WriteFile(thumbPath, imageBytes, 0644); err != nil {
		return fmt.Errorf("govips: failed to write jpeg thumbnail to %s: %w", thumbPath, err)
	}

	log.Printf("Created JPEG thumbnail at %s", thumbPath)
	return nil
}

// createThumbnail generates a 500px wide JPEG thumbnail for the given image from a file path.
// This is used for regeneration tasks.
func createThumbnail(originalPath string, username string) error {
	// Read the original file
	fileBytes, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file for thumbnailing %s: %w", originalPath, err)
	}
	// Delegate to the byte-based function
	return createThumbnailFromBytes(fileBytes, originalPath, username)
}

// createPreview generates a 1920px wide JPEG preview for the given image using vips.
func createPreview(originalPath string, username string) error {
	image, err := vips.NewImageFromFile(originalPath)
	if err != nil {
		return fmt.Errorf("govips: failed to open image for preview %s: %w", originalPath, err)
	}
	defer image.Close()

	// Determine the path for the preview.
	// originalPath is like /data/tmunot/<user>/originals/YYYY/MM/DD/file.jpg
	// We want to create /data/tmunot/<user>/previews/YYYY/MM/DD/
	basePath := strings.TrimPrefix(originalPath, filepath.Join(AppConfig.PhotoUploadDir, username, "originals"))
	baseDir := filepath.Dir(basePath)

	// Construct the full directory path for the preview.
	previewDir := filepath.Join(AppConfig.PhotoUploadDir, username, "previews", baseDir)
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	// The preview will have the same filename as the original.
	previewFilename := filepath.Base(originalPath)
	previewPath := filepath.Join(previewDir, previewFilename)

	// `Thumbnail` is highly optimized. It auto-rotates based on EXIF.
	if err := image.Thumbnail(1920, 0, vips.InterestingNone); err != nil {
		return fmt.Errorf("govips: failed to thumbnail image for preview %s: %w", originalPath, err)
	}

	// Export to JPEG format with a quality of 80.
	jpegParams := vips.NewJpegExportParams()
	jpegParams.Quality = 80
	jpegParams.StripMetadata = true

	imageBytes, _, err := image.ExportJpeg(jpegParams)
	if err != nil {
		return fmt.Errorf("govips: failed to export jpeg preview for %s: %w", originalPath, err)
	}

	if err = os.WriteFile(previewPath, imageBytes, 0644); err != nil {
		return fmt.Errorf("govips: failed to write jpeg preview to %s: %w", previewPath, err)
	}

	log.Printf("Created preview at %s", previewPath)
	return nil
}
