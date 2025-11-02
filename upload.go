package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"image"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	_ "image/jpeg" // Import for JPEG decoding
	_ "image/png"  // Import for PNG decoding

	"path/filepath"

	"github.com/davidbyttow/govips/v2/vips"
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

	// Maximum upload size of 50MB per file
	r.Body = http.MaxBytesReader(w, r.Body, 50*1024*1024)

	// Parse the multipart form, max memory of 20MB. Increased to handle larger single files if needed.
	if err := r.ParseMultipartForm(50 * 1024 * 1024); err != nil {
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

	// 2. Generate preview and thumbnail bytes, and get their dimensions.
	previewBytes, thumbBytes, previewWidth, previewHeight, thumbWidth, thumbHeight, err := generatePreviewAndThumbnailBytes(fileBytes)
	if err != nil {
		handleUploadError(w, "Failed to generate derivatives", http.StatusInternalServerError, err)
		return
	}

	// Store dimensions for saving to the DB
	photoData := &PhotoMetadata{
		ThumbWidth:    thumbWidth,
		ThumbHeight:   thumbHeight,
		PreviewWidth:  previewWidth,
		PreviewHeight: previewHeight,
	}
	// 3. Write all three files (original, thumbnail, preview) to disk.
	if err := writeAllFiles(fileBytes, thumbBytes, previewBytes, originalPath, thumbPath, previewPath); err != nil {
		handleUploadError(w, "Failed to write files to disk", http.StatusInternalServerError, err)
		return
	}

	log.Printf("File %s uploaded as %s", header.Filename, newFilename)

	// Create a PhotoMetadata struct to hold all the data.
	photoData.Filename = newFilename
	photoData.Filepath = relativePath
	photoData.UploadedBy = username
	photoData.UploadedAt = time.Now()
	photoData.ImageWidth = int64(exifInfo.ImageWidth)
	photoData.ImageLength = int64(exifInfo.ImageLength)
	photoData.DateTime = photoDate

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
	if err := os.MkdirAll(filepath.Dir(originalPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(thumbPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(previewPath), 0755); err != nil {
		return err
	}

	// Write files.
	if err := os.WriteFile(originalPath, originalBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(thumbPath, thumbBytes, 0644); err != nil {
		return err
	}
	log.Printf("Thumbnail created at %s", thumbPath)
	if err := os.WriteFile(previewPath, previewBytes, 0644); err != nil {
		return err
	}
	log.Printf("Preview created at %s", previewPath)

	return nil
}

// generatePreviewAndThumbnailBytes creates a preview from the original image,
// and then creates a thumbnail from that preview to save processing.
func generatePreviewAndThumbnailBytes(originalImageBytes []byte) (previewBytes, thumbBytes []byte, previewWidth, previewHeight, thumbWidth, thumbHeight int, finalErr error) {
	jpegParams := vips.NewJpegExportParams()
	jpegParams.Quality = 80
	jpegParams.StripMetadata = true

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to generate the preview
	go func() {
		defer wg.Done()
		img, err := vips.NewImageFromBuffer(originalImageBytes)
		if err != nil {
			finalErr = fmt.Errorf("govips: failed to create image for preview: %w", err)
			return
		}
		defer img.Close()

		if err = img.Thumbnail(1920, 0, vips.InterestingNone); err != nil {
			finalErr = fmt.Errorf("govips: failed to generate preview: %w", err)
			return
		}
		previewWidth = img.Width()
		previewHeight = img.Height()

		pBytes, _, err := img.ExportJpeg(jpegParams)
		if err != nil {
			finalErr = fmt.Errorf("govips: failed to export preview jpeg: %w", err)
			return
		}
		previewBytes = pBytes
	}()

	// Goroutine to generate the thumbnail
	go func() {
		defer wg.Done()
		img, err := vips.NewImageFromBuffer(originalImageBytes)
		if err != nil {
			finalErr = fmt.Errorf("govips: failed to create image for thumbnail: %w", err)
			return
		}
		defer img.Close()

		if err = img.Thumbnail(500, 0, vips.InterestingNone); err != nil {
			finalErr = fmt.Errorf("govips: failed to generate thumbnail: %w", err)
			return
		}
		thumbWidth = img.Width()
		thumbHeight = img.Height()

		tBytes, _, err := img.ExportJpeg(jpegParams)
		if err != nil {
			finalErr = fmt.Errorf("govips: failed to export thumbnail jpeg: %w", err)
			return
		}
		thumbBytes = tBytes
	}()

	wg.Wait()

	return
}
