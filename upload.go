package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"strings"

	"github.com/chai2010/webp"
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

	// --- EXIF Parsing (from memory) ---
	exifInfo, exifReadSuccessfully := parseExifData(file)

	// Determine the date to use for the folder structure.
	// Prioritize DateTimeOriginal from EXIF, then DateTime from EXIF, then fallback to the current time.
	photoDate := time.Now()
	if !exifInfo.DateTimeOriginal.IsZero() {
		photoDate = exifInfo.DateTimeOriginal
	} else if !exifInfo.DateTime.IsZero() {
		photoDate = exifInfo.DateTime
	}

	// Rewind the file reader to the beginning so it can be saved to disk
	_, err = file.Seek(0, io.SeekStart)
	if err != nil {
		log.Printf("Error seeking file %s: %v", header.Filename, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(uploadResponse{
			Status:  "error",
			Message: "Could not process file",
		})
		return
	}

	// Move the file to the correct folder
	newFilePath, newFilename, relativePath, err := saveUploadedFile(file, header.Filename, photoDate, username)
	if err != nil {
		log.Printf("Error moving file %s: %v", header.Filename, err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(uploadResponse{
			Status:  "error",
			Message: "Could not save file",
		})
		return
	}
	log.Printf("File %s uploaded as %s to %s", header.Filename, newFilename, newFilePath)

	// Start thumbnail and preview generation in the background.
	// This allows the handler to return a response to the client immediately.
	go func(path, name, user string) {
		if err := createThumbnail(path, user); err != nil {
			log.Printf("Warning: failed to create thumbnail for %s: %v", name, err)
		}
	}(newFilePath, newFilename, username)

	go func(path, name, user string) {
		if err := createPreview(path, user); err != nil {
			log.Printf("Warning: failed to create preview for %s: %v", name, err)
		}
	}(newFilePath, newFilename, username)

	// Create a PhotoMetadata struct to hold all the data.
	photoData := &PhotoMetadata{
		Filename:              newFilename, // The name of the file itself
		Filepath:              relativePath,  // The path relative to the media root
		Filesize:              header.Size,
		ContentType:           contentType,
		UploadedBy:            username,
		UploadedAt:            time.Now(),
		Make:                  exifInfo.Make,
		Model:                 exifInfo.Model,
		ImageDescription:      exifInfo.ImageDescription,
		ImageWidth:            int64(exifInfo.ImageWidth),
		ImageLength:           int64(exifInfo.ImageLength),
		XResolution:           exifInfo.XResolution,
		YResolution:           exifInfo.YResolution,
		ResolutionUnit:        int64(exifInfo.ResolutionUnit),
		Orientation:           int64(exifInfo.Orientation),
		Software:              exifInfo.Software,
		DateTime:              exifInfo.DateTime,
		Artist:                exifInfo.Artist,
		Copyright:             exifInfo.Copyright,
		ExposureTime:           exifInfo.ExposureTime,
		ExposureProgram:       int64(exifInfo.ExposureProgram),
		FNumber:               exifInfo.FNumber,
		ISOSpeedRatings:       int64(exifInfo.ISOSpeedRatings),
		ShutterSpeedValue:     exifInfo.ShutterSpeedValue,
		ApertureValue:         exifInfo.ApertureValue,
		ExposureBiasValue:     exifInfo.ExposureBiasValue,
		MaxApertureValue:      exifInfo.MaxApertureValue,
		MeteringMode:          int64(exifInfo.MeteringMode),
		LightSource:           int64(exifInfo.LightSource),
		Flash:                 int64(exifInfo.Flash),
		FocalLength:           exifInfo.FocalLength,
		FocalLengthIn35mmFilm: int64(exifInfo.FocalLengthIn35mmFilm),
		LensMake:              exifInfo.LensMake,
		LensModel:             exifInfo.LensModel,
		DateTimeOriginal:      exifInfo.DateTimeOriginal,
		DateTimeDigitized:     exifInfo.DateTimeDigitized,
		SubSecTime:            exifInfo.SubSecTime,
		GPSLat:                exifInfo.GPSLatitude,
		GPSLon:                exifInfo.GPSLongitude,
		GPSAltitude:           exifInfo.GPSAltitude,
		GPSTimeStamp:          exifInfo.GPSTimeStamp,
		GPSSpeed:              exifInfo.GPSSpeed,
		GPSImgDirection:       exifInfo.GPSImgDirection,
	}

	// Save metadata to the database.
	if _, err := savePhotoMetadata(photoData); err != nil {
		log.Printf("Error saving metadata for %s: %v", header.Filename, err)
		// Not returning an error to the client, as the file upload itself was successful.
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(uploadResponse{
		Status:   "success",
		Message:  "File uploaded successfully",
		Filename: newFilename,
		ExifRead: exifReadSuccessfully,
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

	thumbFilename := filepath.Base(originalPath) + ".webp"
	thumbPath := filepath.Join(thumbDir, thumbFilename)

	// Create the thumbnail file.
	thumbFile, err := os.Create(thumbPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer thumbFile.Close()

	// Encode the thumbnail image as WebP with 80% quality.
	if err := webp.Encode(thumbFile, thumb, &webp.Options{Quality: 80}); err != nil {
		return fmt.Errorf("failed to encode thumbnail to webp: %w", err)
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
