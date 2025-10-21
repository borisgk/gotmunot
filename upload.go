package main

import (
	"database/sql"
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
	newFilePath, newFilename, relativePath, err := saveUploadedFile(file, header.Filename, photoDate)
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
	go func(path, name string) {
		if err := createThumbnail(path); err != nil {
			log.Printf("Warning: failed to create thumbnail for %s: %v", name, err)
		}
	}(newFilePath, newFilename)

	go func(path, name string) {
		if err := createPreview(path); err != nil {
			log.Printf("Warning: failed to create preview for %s: %v", name, err)
		}
	}(newFilePath, newFilename)

	// Create a PhotoMetadata struct to hold all the data.
	photoData := &PhotoMetadata{
		Filename:              newFilename, // The name of the file itself
		Filepath:              relativePath,  // The path relative to the media root
		Filesize:              header.Size,
		ContentType:           contentType,
		UploadedBy:            username,
		UploadedAt:            time.Now(),
		Make:                  sql.NullString{String: exifInfo.Make, Valid: exifInfo.Make != ""},
		Model:                 sql.NullString{String: exifInfo.Model, Valid: exifInfo.Model != ""},
		ImageDescription:      sql.NullString{String: exifInfo.ImageDescription, Valid: exifInfo.ImageDescription != ""},
		ImageWidth:            sql.NullInt64{Int64: int64(exifInfo.ImageWidth), Valid: exifInfo.ImageWidth > 0},
		ImageLength:           sql.NullInt64{Int64: int64(exifInfo.ImageLength), Valid: exifInfo.ImageLength > 0},
		XResolution:           sql.NullFloat64{Float64: exifInfo.XResolution, Valid: exifInfo.XResolution > 0},
		YResolution:           sql.NullFloat64{Float64: exifInfo.YResolution, Valid: exifInfo.YResolution > 0},
		ResolutionUnit:        sql.NullInt64{Int64: int64(exifInfo.ResolutionUnit), Valid: exifInfo.ResolutionUnit > 0},
		Orientation:           sql.NullInt64{Int64: int64(exifInfo.Orientation), Valid: exifInfo.Orientation > 0},
		Software:              sql.NullString{String: exifInfo.Software, Valid: exifInfo.Software != ""},
		DateTime:              sql.NullTime{Time: exifInfo.DateTime, Valid: !exifInfo.DateTime.IsZero()},
		Artist:                sql.NullString{String: exifInfo.Artist, Valid: exifInfo.Artist != ""},
		Copyright:             sql.NullString{String: exifInfo.Copyright, Valid: exifInfo.Copyright != ""},
		ExposureTime:           sql.NullString{String: exifInfo.ExposureTime, Valid: exifInfo.ExposureTime != ""},
		ExposureProgram:       sql.NullInt64{Int64: int64(exifInfo.ExposureProgram), Valid: exifInfo.ExposureProgram > 0},
		FNumber:               sql.NullFloat64{Float64: exifInfo.FNumber, Valid: exifInfo.FNumber > 0},
		ISOSpeedRatings:       sql.NullInt64{Int64: int64(exifInfo.ISOSpeedRatings), Valid: exifInfo.ISOSpeedRatings > 0},
		ShutterSpeedValue:     sql.NullString{String: exifInfo.ShutterSpeedValue, Valid: exifInfo.ShutterSpeedValue != ""},
		ApertureValue:         sql.NullFloat64{Float64: exifInfo.ApertureValue, Valid: exifInfo.ApertureValue > 0},
		ExposureBiasValue:     sql.NullString{String: exifInfo.ExposureBiasValue, Valid: exifInfo.ExposureBiasValue != ""},
		MaxApertureValue:      sql.NullFloat64{Float64: exifInfo.MaxApertureValue, Valid: exifInfo.MaxApertureValue > 0},
		MeteringMode:          sql.NullInt64{Int64: int64(exifInfo.MeteringMode), Valid: exifInfo.MeteringMode > 0},
		LightSource:           sql.NullInt64{Int64: int64(exifInfo.LightSource), Valid: exifInfo.LightSource > 0},
		Flash:                 sql.NullInt64{Int64: int64(exifInfo.Flash), Valid: exifInfo.Flash > 0},
		FocalLength:           sql.NullFloat64{Float64: exifInfo.FocalLength, Valid: exifInfo.FocalLength > 0},
		FocalLengthIn35mmFilm: sql.NullInt64{Int64: int64(exifInfo.FocalLengthIn35mmFilm), Valid: exifInfo.FocalLengthIn35mmFilm > 0},
		LensMake:              sql.NullString{String: exifInfo.LensMake, Valid: exifInfo.LensMake != ""},
		LensModel:             sql.NullString{String: exifInfo.LensModel, Valid: exifInfo.LensModel != ""},
		DateTimeOriginal:      sql.NullTime{Time: exifInfo.DateTimeOriginal, Valid: !exifInfo.DateTimeOriginal.IsZero()},
		DateTimeDigitized:     sql.NullTime{Time: exifInfo.DateTimeDigitized, Valid: !exifInfo.DateTimeDigitized.IsZero()},
		SubSecTime:            sql.NullString{String: exifInfo.SubSecTime, Valid: exifInfo.SubSecTime != ""},
		GPSLat:                sql.NullFloat64{Float64: exifInfo.GPSLatitude, Valid: exifInfo.GPSLatitude != 0},
		GPSLon:                sql.NullFloat64{Float64: exifInfo.GPSLongitude, Valid: exifInfo.GPSLongitude != 0},
		GPSAltitude:           sql.NullFloat64{Float64: exifInfo.GPSAltitude, Valid: exifInfo.GPSAltitude != 0},
		GPSTimeStamp:          sql.NullTime{Time: exifInfo.GPSTimeStamp, Valid: !exifInfo.GPSTimeStamp.IsZero()},
		GPSSpeed:              sql.NullFloat64{Float64: exifInfo.GPSSpeed, Valid: exifInfo.GPSSpeed > 0},
		GPSImgDirection:       sql.NullFloat64{Float64: exifInfo.GPSImgDirection, Valid: exifInfo.GPSImgDirection > 0},
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
func saveUploadedFile(file io.Reader, originalFilename string, photoDate time.Time) (string, string, string, error) {
	// Get date parts from the provided photoDate to create the directory structure.
	year := photoDate.Format("2006")
	month := photoDate.Format("01")
	day := photoDate.Format("02")

	// Construct the target directory path: /data/tmunot/originals/YEAR/MONTH/DAY
	targetDir := filepath.Join(AppConfig.PhotoUploadDir, "originals", year, month, day)
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
func createThumbnail(originalPath string) error {
	// Open the original image file.
	srcImage, err := imaging.Open(originalPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("failed to open image for thumbnailing: %w", err)
	}

	// Resize the image to a width of 500px, preserving the aspect ratio.
	thumb := imaging.Resize(srcImage, 500, 0, imaging.Lanczos)

	// Determine the path for the thumbnail.
	// It will be /data/tmunot/thumbs/YEAR/MONTH/DAY/original-filename.webp
	relPath, err := filepath.Rel(AppConfig.PhotoUploadDir, originalPath)
	if err != nil {
		return fmt.Errorf("could not determine relative path for thumbnail: %w", err)
	}
	// relPath is "originals/YYYY/MM/DD/filename.jpg", we want "thumbs/YYYY/MM/DD/"
	thumbDir := filepath.Join(AppConfig.PhotoUploadDir, "thumbs", filepath.Dir(strings.TrimPrefix(relPath, "originals"+string(filepath.Separator))))
	if err := os.MkdirAll(thumbDir, 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	// The thumbnail filename keeps the original name but adds a .webp extension.
	thumbFilename := filepath.Base(relPath) + ".webp"
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
func createPreview(originalPath string) error {
	// Open the original image file.
	srcImage, err := imaging.Open(originalPath, imaging.AutoOrientation(true))
	if err != nil {
		return fmt.Errorf("failed to open image for preview: %w", err)
	}

	// Resize the image to a width of 1920px, preserving the aspect ratio.
	preview := imaging.Resize(srcImage, 1920, 0, imaging.Lanczos)

	// Determine the path for the preview.
	// It will be /data/tmunot/previews/YEAR/MONTH/DAY/original-filename.jpg
	relPath, err := filepath.Rel(AppConfig.PhotoUploadDir, originalPath)
	if err != nil {
		return fmt.Errorf("could not determine relative path for preview: %w", err)
	}
	// relPath is "originals/YYYY/MM/DD/filename.jpg", we want "previews/YYYY/MM/DD/"
	previewDir := filepath.Join(AppConfig.PhotoUploadDir, "previews", filepath.Dir(strings.TrimPrefix(relPath, "originals"+string(filepath.Separator))))
	if err := os.MkdirAll(previewDir, 0755); err != nil {
		return fmt.Errorf("failed to create preview directory: %w", err)
	}

	// The preview filename is the same as the original file's base name.
	previewFilename := filepath.Base(relPath)
	previewPath := filepath.Join(previewDir, previewFilename)

	// Save the preview image as a JPEG with 80% quality.
	err = imaging.Save(preview, previewPath, imaging.JPEGQuality(80))
	if err != nil {
		return fmt.Errorf("failed to save preview jpeg: %w", err)
	}

	log.Printf("Created preview at %s", previewPath)
	return nil
}
