package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"image"
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

	// --- New Upload Flow ---

	// 1. Calculate all paths and the new filename.
	newFilename, relativePath, originalPath, thumbPath, previewPath := calculateFilePaths(header.Filename, photoDate, username)

	// 2. Generate thumbnail and preview bytes from the original file in memory.
	thumbBytes, err := generateThumbnailBytes(fileBytes)
	if err != nil {
		handleUploadError(w, "Failed to generate thumbnail", http.StatusInternalServerError, err)
		return
	}

	previewBytes, err := generatePreviewBytes(fileBytes)
	if err != nil {
		handleUploadError(w, "Failed to generate preview", http.StatusInternalServerError, err)
		return
	}

	// 3. Write all three files to disk.
	if err := writeAllFiles(fileBytes, thumbBytes, previewBytes, originalPath, thumbPath, previewPath); err != nil {
		handleUploadError(w, "Failed to write files to disk", http.StatusInternalServerError, err)
		return
	}

	log.Printf("File %s uploaded as %s", header.Filename, newFilename)

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

// calculateFilePaths determines the new filename and the full disk paths for the original, thumbnail, and preview files.
func calculateFilePaths(originalFilename string, photoDate time.Time, username string) (newFilename, relativePath, originalPath, thumbPath, previewPath string) {
	// Generate a new unique filename to prevent overwrites.
	newFilename = fmt.Sprintf("%d-%s", time.Now().Unix(), originalFilename)

	// Get date parts for the directory structure.
	year := photoDate.Format("2006")
	month := photoDate.Format("01")
	day := photoDate.Format("02")

	// The path relative to the user's media directory (e.g., "2024/05/21/1716298967-photo.jpg")
	relativePath = filepath.Join(year, month, day, newFilename)

	// Full absolute paths for each file type.
	originalPath = filepath.Join(AppConfig.PhotoUploadDir, username, "originals", relativePath)
	thumbPath = filepath.Join(AppConfig.PhotoUploadDir, username, "thumbs", relativePath)
	previewPath = filepath.Join(AppConfig.PhotoUploadDir, username, "previews", relativePath)

	return
}

// writeAllFiles creates the necessary directories and writes the original, thumbnail, and preview byte slices to disk.
func writeAllFiles(originalBytes, thumbBytes, previewBytes []byte, originalPath, thumbPath, previewPath string) error {
	// Create parent directories.
	if err := os.MkdirAll(filepath.Dir(originalPath), 0755); err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(previewPath), 0755); err != nil { return err }

	// Write files.
	if err := os.WriteFile(originalPath, originalBytes, 0644); err != nil { return err }
	if err := os.WriteFile(thumbPath, thumbBytes, 0644); err != nil { return err }
	if err := os.WriteFile(previewPath, previewBytes, 0644); err != nil { return err }

	return nil
}

// generateThumbnailBytes creates a 500px wide JPEG thumbnail from an in-memory byte slice.
func generateThumbnailBytes(imageBytes []byte) ([]byte, error) {
	image, err := vips.NewImageFromBuffer(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("govips: failed to create image from buffer for thumbnail: %w", err)
	}
	defer image.Close()

	// `Thumbnail` is highly optimized for creating thumbnails. It auto-rotates based on EXIF.
	if err := image.Thumbnail(500, 0, vips.InterestingNone); err != nil {
		return nil, fmt.Errorf("govips: failed to thumbnail image: %w", err)
	}

	// Export to JPEG format with a quality of 80.
	jpegParams := vips.NewJpegExportParams()
	jpegParams.Quality = 80
	jpegParams.StripMetadata = true // Corrected field name

	thumbBytes, _, err := image.ExportJpeg(jpegParams)
	if err != nil {
		return nil, fmt.Errorf("govips: failed to export jpeg for thumbnail: %w", err)
	}
	return thumbBytes, nil
}

// generatePreviewBytes creates a 1920px wide JPEG preview from an in-memory byte slice.
func generatePreviewBytes(imageBytes []byte) ([]byte, error) {
	image, err := vips.NewImageFromBuffer(imageBytes)
	if err != nil {
		return nil, fmt.Errorf("govips: failed to create image from buffer for preview: %w", err)
	}
	defer image.Close()

	// `Thumbnail` is highly optimized. It auto-rotates based on EXIF.
	if err := image.Thumbnail(1920, 0, vips.InterestingNone); err != nil {
		return nil, fmt.Errorf("govips: failed to thumbnail image for preview: %w", err)
	}

	// Export to JPEG format with a quality of 80.
	jpegParams := vips.NewJpegExportParams()
	jpegParams.Quality = 80
	jpegParams.StripMetadata = true // Corrected field name

	previewBytes, _, err := image.ExportJpeg(jpegParams)
	if err != nil {
		return nil, fmt.Errorf("govips: failed to export jpeg for preview: %w", err)
	}
	return previewBytes, nil
}

// createThumbnailFromBytes generates a 500px wide JPEG thumbnail from an in-memory byte slice and writes it to disk.
// This is used during the initial upload process.
func createThumbnailFromBytes(imageBytes []byte, originalPath string, username string) error {
	_, _, _, thumbPath, _ := calculateFilePaths(filepath.Base(originalPath), time.Now(), username)
	thumbBytes, err := generateThumbnailBytes(imageBytes)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return err
	}
	return os.WriteFile(thumbPath, thumbBytes, 0644)
}

// createThumbnail generates a 500px wide JPEG thumbnail for a given image file path.
// This is used for regeneration tasks.
func createThumbnail(originalPath string, username string) error {
	// Read the original file
	fileBytes, err := os.ReadFile(originalPath)
	if err != nil {
		return fmt.Errorf("failed to read original file for thumbnailing %s: %w", originalPath, err)
	}
	// Delegate to the byte-based function, which now handles path calculation and writing.
	return createThumbnailFromBytes(fileBytes, originalPath, username)
}

// createPreview generates a 1920px wide JPEG preview for a given image file path using vips.
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
	jpegParams.StripMetadata = true // Corrected field name

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
