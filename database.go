package main

import (
	"errors"
	"database/sql"
	"log"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

var photosDB *sql.DB
var insertPhotoStmt *sql.Stmt

// PhotoMetadata struct to represent photo metadata.
type PhotoMetadata struct {
	ID               int
	Filename         string
	Filepath         string
	Filesize         int64
	ContentType      string
	UploadedBy       string
	UploadedAt       time.Time
	Make             string
	Model            string
	ImageDescription string
	ImageWidth       int64
	ImageLength      int64
	XResolution      float64
	YResolution      float64
	ResolutionUnit   int64
	Orientation      int64
	Software         string
	DateTime         time.Time
	Artist           string
	Copyright        string
	ExposureTime      string
	ExposureProgram   int64
	FNumber           float64
	ISOSpeedRatings   int64
	ShutterSpeedValue string
	ApertureValue     float64
	ExposureBiasValue string
	MaxApertureValue  float64
	MeteringMode      int64
	LightSource       int64
	Flash             int64
	FocalLength           float64
	FocalLengthIn35mmFilm int64
	LensMake              string
	LensModel             string
	DateTimeOriginal time.Time
	DateTimeDigitized time.Time
	SubSecTime        string
	GPSLat           float64
	GPSLon           float64
	GPSAltitude      float64
	GPSTimeStamp     time.Time
	GPSSpeed         float64
	GPSImgDirection  float64
}

// AspectRatio calculates the width/height ratio of the photo.
// It returns a default of 3:2 if dimensions are not available.
func (p *PhotoMetadata) AspectRatio() float64 {
	if p.ImageWidth > 0 && p.ImageLength > 0 {
		return float64(p.ImageWidth) / float64(p.ImageLength)
	}
	return 1.5 // Default to a 3:2 aspect ratio
}

// initPhotosDB initializes the photos database.
func initPhotosDB() {
	var err error
	dbPath := filepath.Join(AppConfig.DataDir, "photos.db")
	photosDB, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Error opening photos database: %v", err)
	}

	// Create the photos table if it doesn't exist.
	_, err = photosDB.Exec(`
		CREATE TABLE IF NOT EXISTS photos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT,
			filepath TEXT UNIQUE,
			filesize INTEGER,
			content_type TEXT,
			uploaded_by TEXT,
			uploaded_at DATETIME,
			make TEXT,
			model TEXT,
			image_description TEXT,
			image_width INTEGER,
			image_length INTEGER,
			x_resolution REAL,
			y_resolution REAL,
			resolution_unit INTEGER,
			orientation INTEGER,
			software TEXT,
			date_time DATETIME,
			artist TEXT,
			copyright TEXT,
			exposure_time TEXT,
			exposure_program INTEGER,
			f_number REAL,
			iso_speed_ratings INTEGER,
			shutter_speed_value TEXT,
			aperture_value REAL,
			exposure_bias_value TEXT,
			max_aperture_value REAL,
			metering_mode INTEGER,
			light_source INTEGER,
			flash INTEGER,
			focal_length REAL,
			focal_length_in_35mm_film INTEGER,
			lens_make TEXT,
			lens_model TEXT,
			date_time_original DATETIME,
			date_time_digitized DATETIME,
			subsec_time TEXT,
			gps_lat REAL,
			gps_lon REAL,
			gps_altitude REAL,
			gps_timestamp DATETIME,
			gps_speed REAL,
			gps_img_direction REAL
		)
	`)
	if err != nil {
		log.Fatalf("Error creating photos table: %v", err)
	}

	// Prepare the insert statement for saving photo metadata.
	// This is more efficient as the SQL is parsed only once.
	insertPhotoStmt, err = photosDB.Prepare(`
		INSERT INTO photos (
			filename, filepath, filesize, content_type, uploaded_by, uploaded_at, 
			make, model, image_description, image_width, image_length, x_resolution, y_resolution, 
			resolution_unit, orientation, software, date_time, artist, copyright,
			exposure_time, exposure_program, f_number, iso_speed_ratings, shutter_speed_value, 
			aperture_value, exposure_bias_value, max_aperture_value, metering_mode, light_source, flash, 
			focal_length, focal_length_in_35mm_film, lens_make, lens_model, 
			date_time_original, date_time_digitized, subsec_time, 
			gps_lat, gps_lon, gps_altitude, gps_timestamp, gps_speed, gps_img_direction
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatalf("Error preparing insert photo statement: %v", err)
	}

	log.Println("Photos database initialized.")
}

// getPhotos retrieves all photos for a user, with an optional year filter.
func getPhotos(username string, year int) ([]PhotoMetadata, error) {
	query := `SELECT id, filename, filepath, filesize, content_type, uploaded_by, uploaded_at,
		make, model, image_description, image_width, image_length, x_resolution, y_resolution,
		resolution_unit, orientation, software, date_time, artist, copyright,
		exposure_time, exposure_program, f_number, iso_speed_ratings, shutter_speed_value,
		aperture_value, exposure_bias_value, max_aperture_value, metering_mode, light_source, flash,
		focal_length, focal_length_in_35mm_film, lens_make, lens_model,
		date_time_original, date_time_digitized, subsec_time,
		gps_lat, gps_lon, gps_altitude, gps_timestamp, gps_speed, gps_img_direction
		FROM photos WHERE uploaded_by = ?`
	args := []interface{}{username}

	if year > 0 {
		query += " AND CAST(SUBSTR(COALESCE(date_time_original, date_time, uploaded_at), 1, 4) AS INTEGER) = ?"
		args = append(args, year)
	}

	query += ` ORDER BY COALESCE(date_time_original, date_time, uploaded_at) DESC`

	return queryPhotos(query, args...)
}

// getTotalPhotoCount returns the total number of photos in the database.
func getTotalPhotoCount(username string, year int) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM photos WHERE uploaded_by = ?"
	args := []interface{}{username}

	if year > 0 {
		query += " AND CAST(SUBSTR(COALESCE(date_time_original, date_time, uploaded_at), 1, 4) AS INTEGER) = ?"
		args = append(args, year)
	}

	err := photosDB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// getPhotoByFilename retrieves all metadata for a single photo by its filename.
func getPhotoByFilename(filename string) (PhotoMetadata, error) {
	var p PhotoMetadata
	row := photosDB.QueryRow(`
        SELECT 
            id, filename, filepath, filesize, content_type, uploaded_by, uploaded_at, 
            make, model, image_description, image_width, image_length, x_resolution, y_resolution, 
            resolution_unit, orientation, software, date_time, artist, copyright, 
            exposure_time, exposure_program, f_number, iso_speed_ratings, shutter_speed_value, 
            aperture_value, exposure_bias_value, max_aperture_value, metering_mode, light_source, flash, 
            focal_length, focal_length_in_35mm_film, lens_make, lens_model, 
            date_time_original, date_time_digitized, subsec_time, 
            gps_lat, gps_lon, gps_altitude, gps_timestamp, gps_speed, gps_img_direction
        FROM photos
        WHERE filename = ?
    `, filename)

	if err := scanPhoto(row, &p); err != nil {
		return p, err
	}

	return p, nil
}

// getDistinctYears retrieves a sorted list of distinct years for a user's photos.
func getDistinctYears(username string) ([]int, error) {
	// Use COALESCE to find the best available date for each photo, then extract the year.
	// The order is: EXIF original date, EXIF modification date, then upload date.
	rows, err := photosDB.Query(`
		SELECT DISTINCT CAST(SUBSTR(COALESCE(date_time_original, date_time, uploaded_at), 1, 4) AS INTEGER) as year
		FROM photos
		WHERE uploaded_by = ? 
		AND year IS NOT NULL
		ORDER BY year ASC
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var years []int
	for rows.Next() {
		var year int
		if err := rows.Scan(&year); err != nil {
			return nil, err
		}
		years = append(years, year)
	}
	return years, rows.Err()
}

// getPhotoCountsByYear retrieves a map of year to photo count for a user.
func getPhotoCountsByYear(username string) (map[int]int, error) {
	rows, err := photosDB.Query(`
		SELECT
			CAST(SUBSTR(COALESCE(date_time_original, date_time, uploaded_at), 1, 4) AS INTEGER) as year,
			COUNT(*) as count
		FROM photos
		WHERE uploaded_by = ? AND year IS NOT NULL
		GROUP BY year
	`, username)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := make(map[int]int)
	for rows.Next() {
		var year, count int
		if err := rows.Scan(&year, &count); err != nil {
			return nil, err
		}
		counts[year] = count
	}
	return counts, rows.Err()
}

// getAllPhotos retrieves the filepath and filename for all photos in the database.
func getAllPhotos(username string) ([]PhotoMetadata, error) {
	query := `SELECT id, filename, filepath, filesize, content_type, uploaded_by, uploaded_at,
		make, model, image_description, image_width, image_length, x_resolution, y_resolution,
		resolution_unit, orientation, software, date_time, artist, copyright,
		exposure_time, exposure_program, f_number, iso_speed_ratings, shutter_speed_value,
		aperture_value, exposure_bias_value, max_aperture_value, metering_mode, light_source, flash,
		focal_length, focal_length_in_35mm_film, lens_make, lens_model,
		date_time_original, date_time_digitized, subsec_time,
		gps_lat, gps_lon, gps_altitude, gps_timestamp, gps_speed, gps_img_direction
		FROM photos WHERE uploaded_by = ? ORDER BY id`
	return queryPhotos(query, username)
}

// queryPhotos is a helper function to run a query and scan the results into a slice of PhotoMetadata.
func queryPhotos(query string, args ...interface{}) ([]PhotoMetadata, error) {
	rows, err := photosDB.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		if err := scanPhoto(rows, &p); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// savePhotoMetadata saves photo metadata to the database.
func savePhotoMetadata(p *PhotoMetadata) (int64, error) {
	result, err := insertPhotoStmt.Exec(
		p.Filename, p.Filepath, p.Filesize, p.ContentType, p.UploadedBy, p.UploadedAt,
		p.Make, p.Model, p.ImageDescription, p.ImageWidth, p.ImageLength, p.XResolution, p.YResolution,
		p.ResolutionUnit, p.Orientation, p.Software, p.DateTime, p.Artist, p.Copyright,
		p.ExposureTime, p.ExposureProgram, p.FNumber, p.ISOSpeedRatings, p.ShutterSpeedValue,
		p.ApertureValue, p.ExposureBiasValue, p.MaxApertureValue, p.MeteringMode, p.LightSource, p.Flash,
		p.FocalLength, p.FocalLengthIn35mmFilm, p.LensMake, p.LensModel,
		p.DateTimeOriginal, p.DateTimeDigitized, p.SubSecTime,
		p.GPSLat, p.GPSLon, p.GPSAltitude, p.GPSTimeStamp, p.GPSSpeed, p.GPSImgDirection,
	)
	if err != nil {
		return 0, err
	}
	return result.LastInsertId()
}

// deletePhotoByFilename deletes a photo's record from the database by its filename.
func deletePhotoByFilename(filename string) error {
	_, err := photosDB.Exec("DELETE FROM photos WHERE filename = ?", filename)
	if err != nil {
		return err
	}
	return nil
}

// updatePhotoDateAndPath moves a photo's files to a new directory structure based on a new date
// and updates its metadata in the database within a single transaction.
func updatePhotoDateAndPath(filename, username string, newDate time.Time) error {
	// Get current photo metadata to know the old path and verify ownership.
	photo, err := getPhotoByFilename(filename)
	if err != nil {
		return err // Propagate sql.ErrNoRows or other DB errors.
	}

	// Security check: ensure the user owns the photo.
	if photo.UploadedBy != username {
		return errors.New("forbidden")
	}

	// --- Calculate new paths ---
	year := newDate.Format("2006")
	month := newDate.Format("01")
	day := newDate.Format("02")
	newRelativePath := filepath.Join(year, month, day, filename)

	// If the path hasn't changed, we only need to update the date in the DB.
	if newRelativePath == photo.Filepath {
		_, err := photosDB.Exec("UPDATE photos SET date_time_original = ? WHERE filename = ?", newDate, filename)
		return err
	}

	// --- Prepare file paths for moving ---
	baseUploadDir := filepath.Join(AppConfig.PhotoUploadDir, username)
	// Old paths
	oldOriginalPath := filepath.Join(baseUploadDir, "originals", photo.Filepath)
	oldPreviewPath := filepath.Join(baseUploadDir, "previews", photo.Filepath)
	oldThumbPath := filepath.Join(baseUploadDir, "thumbs", photo.Filepath+".webp")
	// New paths
	newOriginalPath := filepath.Join(baseUploadDir, "originals", newRelativePath)
	newPreviewPath := filepath.Join(baseUploadDir, "previews", newRelativePath)
	newThumbPath := filepath.Join(baseUploadDir, "thumbs", newRelativePath+".webp")

	// --- Perform file move and DB update in a transaction ---
	tx, err := photosDB.Begin()
	if err != nil {
		return err
	}
	// Defer a rollback in case of error.
	defer tx.Rollback()

	// 1. Update the database record with the new date and path.
	stmt, err := tx.Prepare("UPDATE photos SET date_time_original = ?, filepath = ? WHERE filename = ?")
	if err != nil {
		return err
	}
	defer stmt.Close()
	if _, err := stmt.Exec(newDate, newRelativePath, filename); err != nil {
		return err
	}

	// 2. Move the files on the filesystem.
	// Create the destination directories first.
	if err := os.MkdirAll(filepath.Dir(newOriginalPath), 0755); err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(newPreviewPath), 0755); err != nil { return err }
	if err := os.MkdirAll(filepath.Dir(newThumbPath), 0755); err != nil { return err }

	// Rename/move the files.
	if err := os.Rename(oldOriginalPath, newOriginalPath); err != nil { return err }
	if err := os.Rename(oldPreviewPath, newPreviewPath); err != nil { return err }
	if err := os.Rename(oldThumbPath, newThumbPath); err != nil { return err }

	// TODO: Optionally, clean up old empty directories. This is a non-trivial task
	// and can be skipped for now.

	// 3. If all operations succeeded, commit the transaction.
	return tx.Commit()
}

// scanPhoto is a helper to scan a photo row into a PhotoMetadata struct.
func scanPhoto(scanner interface{ Scan(...interface{}) error }, p *PhotoMetadata) error {
	// Use sql.Null types for scanning to handle potential NULL values from the database.
	var make, model, imageDescription, software, artist, copyright, exposureTime, shutterSpeedValue, exposureBiasValue, lensMake, lensModel, subSecTime sql.NullString
	var imageWidth, imageLength, resolutionUnit, orientation, exposureProgram, isoSpeedRatings, meteringMode, lightSource, flash, focalLengthIn35mmFilm sql.NullInt64
	var xResolution, yResolution, fNumber, apertureValue, maxApertureValue, focalLength, gpsLat, gpsLon, gpsAltitude, gpsSpeed, gpsImgDirection sql.NullFloat64
	var dateTime, dateTimeOriginal, dateTimeDigitized, gpsTimestamp sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.Filename, &p.Filepath, &p.Filesize, &p.ContentType, &p.UploadedBy, &p.UploadedAt,
		&make, &model, &imageDescription, &imageWidth, &imageLength, &xResolution, &yResolution,
		&resolutionUnit, &orientation, &software, &dateTime, &artist, &copyright,
		&exposureTime, &exposureProgram, &fNumber, &isoSpeedRatings, &shutterSpeedValue,
		&apertureValue, &exposureBiasValue, &maxApertureValue, &meteringMode, &lightSource, &flash,
		&focalLength, &focalLengthIn35mmFilm, &lensMake, &lensModel,
		&dateTimeOriginal, &dateTimeDigitized, &subSecTime,
		&gpsLat, &gpsLon, &gpsAltitude, &gpsTimestamp, &gpsSpeed, &gpsImgDirection,
	)
	if err != nil {
		return err
	}

	// Assign values from sql.Null types to the struct, falling back to zero values if NULL.
	p.Make = make.String
	p.Model = model.String
	p.ImageDescription = imageDescription.String
	p.ImageWidth = imageWidth.Int64
	p.ImageLength = imageLength.Int64
	p.XResolution = xResolution.Float64
	p.YResolution = yResolution.Float64
	p.ResolutionUnit = resolutionUnit.Int64
	p.Orientation = orientation.Int64
	p.Software = software.String
	p.DateTime = dateTime.Time
	p.Artist = artist.String
	p.Copyright = copyright.String
	p.ExposureTime = exposureTime.String
	p.ExposureProgram = exposureProgram.Int64
	p.FNumber = fNumber.Float64
	p.ISOSpeedRatings = isoSpeedRatings.Int64
	p.ShutterSpeedValue = shutterSpeedValue.String
	p.ApertureValue = apertureValue.Float64
	p.ExposureBiasValue = exposureBiasValue.String
	p.MaxApertureValue = maxApertureValue.Float64
	p.MeteringMode = meteringMode.Int64
	p.LightSource = lightSource.Int64
	p.Flash = flash.Int64
	p.FocalLength = focalLength.Float64
	p.FocalLengthIn35mmFilm = focalLengthIn35mmFilm.Int64
	p.LensMake = lensMake.String
	p.LensModel = lensModel.String
	p.DateTimeOriginal = dateTimeOriginal.Time
	p.DateTimeDigitized = dateTimeDigitized.Time
	p.SubSecTime = subSecTime.String
	p.GPSLat = gpsLat.Float64
	p.GPSLon = gpsLon.Float64
	p.GPSAltitude = gpsAltitude.Float64
	p.GPSTimeStamp = gpsTimestamp.Time
	p.GPSSpeed = gpsSpeed.Float64
	p.GPSImgDirection = gpsImgDirection.Float64

	return nil
}

// getPhotoDateString returns the date part of a photo's most relevant timestamp.
func getPhotoDateString(p *PhotoMetadata) string {
	if !p.DateTimeOriginal.IsZero() {
		return p.DateTimeOriginal.Format("2006-01-02")
	} else if !p.DateTime.IsZero() {
		return p.DateTime.Format("2006-01-02")
	} else {
		return p.UploadedAt.Format("2006-01-02")
	}
}