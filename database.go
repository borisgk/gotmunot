package main

import (
	"database/sql"
	"log"
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
	Make             sql.NullString
	Model            sql.NullString
	ImageDescription sql.NullString
	ImageWidth       sql.NullInt64
	ImageLength      sql.NullInt64
	XResolution      sql.NullFloat64
	YResolution      sql.NullFloat64
	ResolutionUnit   sql.NullInt64
	Orientation      sql.NullInt64
	Software         sql.NullString
	DateTime         sql.NullTime
	Artist           sql.NullString
	Copyright        sql.NullString
	ExposureTime      sql.NullString
	ExposureProgram   sql.NullInt64
	FNumber           sql.NullFloat64
	ISOSpeedRatings   sql.NullInt64
	ShutterSpeedValue sql.NullString
	ApertureValue     sql.NullFloat64
	ExposureBiasValue sql.NullString
	MaxApertureValue  sql.NullFloat64
	MeteringMode      sql.NullInt64
	LightSource       sql.NullInt64
	Flash             sql.NullInt64
	FocalLength           sql.NullFloat64
	FocalLengthIn35mmFilm sql.NullInt64
	LensMake              sql.NullString
	LensModel             sql.NullString
	DateTimeOriginal sql.NullTime
	DateTimeDigitized sql.NullTime
	SubSecTime        sql.NullString
	GPSLat           sql.NullFloat64
	GPSLon           sql.NullFloat64
	GPSAltitude      sql.NullFloat64
	GPSTimeStamp     sql.NullTime
	GPSSpeed         sql.NullFloat64
	GPSImgDirection  sql.NullFloat64
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

// getPhotos retrieves a paginated list of photos.
func getPhotos(limit, offset int) ([]PhotoMetadata, error) {
	rows, err := photosDB.Query(`
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
        ORDER BY COALESCE(date_time_original, date_time, uploaded_at) DESC
        LIMIT ?
        OFFSET ?
    `, limit, offset)

	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		if err := rows.Scan(
			&p.ID, &p.Filename, &p.Filepath, &p.Filesize, &p.ContentType, &p.UploadedBy, &p.UploadedAt,
			&p.Make, &p.Model, &p.ImageDescription, &p.ImageWidth, &p.ImageLength, &p.XResolution, &p.YResolution,
			&p.ResolutionUnit, &p.Orientation, &p.Software, &p.DateTime, &p.Artist, &p.Copyright,
			&p.ExposureTime, &p.ExposureProgram, &p.FNumber, &p.ISOSpeedRatings, &p.ShutterSpeedValue,
			&p.ApertureValue, &p.ExposureBiasValue, &p.MaxApertureValue, &p.MeteringMode, &p.LightSource, &p.Flash,
			&p.FocalLength, &p.FocalLengthIn35mmFilm, &p.LensMake, &p.LensModel,
			&p.DateTimeOriginal, &p.DateTimeDigitized, &p.SubSecTime,
			&p.GPSLat, &p.GPSLon, &p.GPSAltitude, &p.GPSTimeStamp, &p.GPSSpeed, &p.GPSImgDirection,
		); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}

	return photos, rows.Err()
}

// getTotalPhotoCount returns the total number of photos in the database.
func getTotalPhotoCount() (int, error) {
	var count int
	err := photosDB.QueryRow("SELECT COUNT(*) FROM photos").Scan(&count)
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

	err := row.Scan(
		&p.ID, &p.Filename, &p.Filepath, &p.Filesize, &p.ContentType, &p.UploadedBy, &p.UploadedAt,
		&p.Make, &p.Model, &p.ImageDescription, &p.ImageWidth, &p.ImageLength, &p.XResolution, &p.YResolution,
		&p.ResolutionUnit, &p.Orientation, &p.Software, &p.DateTime, &p.Artist, &p.Copyright,
		&p.ExposureTime, &p.ExposureProgram, &p.FNumber, &p.ISOSpeedRatings, &p.ShutterSpeedValue,
		&p.ApertureValue, &p.ExposureBiasValue, &p.MaxApertureValue, &p.MeteringMode, &p.LightSource, &p.Flash,
		&p.FocalLength, &p.FocalLengthIn35mmFilm, &p.LensMake, &p.LensModel,
		&p.DateTimeOriginal, &p.DateTimeDigitized, &p.SubSecTime,
		&p.GPSLat, &p.GPSLon, &p.GPSAltitude, &p.GPSTimeStamp, &p.GPSSpeed, &p.GPSImgDirection,
	)

	if err != nil {
		return p, err
	}

	return p, nil
}

// getAllPhotos retrieves the filepath and filename for all photos in the database.
func getAllPhotos() ([]PhotoMetadata, error) {
	rows, err := photosDB.Query(`SELECT id, filename, filepath FROM photos ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		if err := rows.Scan(&p.ID, &p.Filename, &p.Filepath); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return photos, nil
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