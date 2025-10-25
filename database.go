package main

import (
	"errors"
	"database/sql"
	"os"
	"path/filepath"
	"time"
	"fmt"

	_ "modernc.org/sqlite"
)

// PhotoMetadata struct to represent photo metadata.
type PhotoMetadata struct {
	ID               int
	Filename         string
	Filepath         string
	UploadedBy       string
	UploadedAt       time.Time
	ImageWidth       int64
	ImageLength      int64
	DateTime         time.Time
}


// getPhotos retrieves all photos for a user, with an optional year filter.
func getPhotos(username string, year int) ([]PhotoMetadata, error) {
	query := `SELECT id, filename, filepath, uploaded_by, uploaded_at, image_width, image_length, date_time
		FROM photos WHERE uploaded_by = ?`
	args := []interface{}{username}

	if year > 0 {
		query += " AND CAST(SUBSTR(date_time, 1, 4) AS INTEGER) = ?"
		args = append(args, year)
	}

	query += ` ORDER BY date_time DESC`

	return queryPhotos(query, args...)
}

// getTotalPhotoCount returns the total number of photos in the database.
func getTotalPhotoCount(username string, year int) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM photos WHERE uploaded_by = ?"
	args := []interface{}{username}

	if year > 0 {
		query += " AND CAST(SUBSTR(date_time, 1, 4) AS INTEGER) = ?"
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
            id, filename, filepath, uploaded_by, uploaded_at, 
            image_width, image_length, date_time
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
		SELECT DISTINCT CAST(SUBSTR(date_time, 1, 4) AS INTEGER) as year
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
			CAST(SUBSTR(date_time, 1, 4) AS INTEGER) as year,
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
	query := `SELECT id, filename, filepath, uploaded_by, uploaded_at, image_width, image_length, date_time
		FROM photos WHERE uploaded_by = ? ORDER BY date_time`
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
	tx, err := photosDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error, if Commit is not called

	stmt, err := tx.Prepare(`
		INSERT INTO photos (
			filename, filepath, uploaded_by, uploaded_at, 
			image_width, image_length, date_time
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement within transaction: %w", err)
	}
	defer stmt.Close() // Close the statement when the function returns

	result, err := stmt.Exec(
		p.Filename, p.Filepath, p.UploadedBy, p.UploadedAt,
		p.ImageWidth, p.ImageLength, p.DateTime,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to execute insert statement: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID: %w", err)
	}
	return id, nil
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
		_, err := photosDB.Exec("UPDATE photos SET date_time = ? WHERE filename = ?", newDate, filename)
		return err
	}

	// --- Prepare file paths for moving ---
	baseUploadDir := filepath.Join(AppConfig.PhotoUploadDir, username)
	// Old paths
	oldOriginalPath := filepath.Join(baseUploadDir, "originals", photo.Filepath)
	oldPreviewPath := filepath.Join(baseUploadDir, "previews", photo.Filepath)
	oldThumbPath := filepath.Join(baseUploadDir, "thumbs", photo.Filepath)
	// New paths
	newOriginalPath := filepath.Join(baseUploadDir, "originals", newRelativePath)
	newPreviewPath := filepath.Join(baseUploadDir, "previews", newRelativePath)
	newThumbPath := filepath.Join(baseUploadDir, "thumbs", newRelativePath)

	// --- Perform file move and DB update in a transaction ---
	tx, err := photosDB.Begin()
	if err != nil {
		return err
	}
	// Defer a rollback in case of error.
	defer tx.Rollback()

	// 1. Update the database record with the new date and path.
	stmt, err := tx.Prepare("UPDATE photos SET date_time = ?, filepath = ? WHERE filename = ?")
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
	var imageWidth, imageLength sql.NullInt64
	var dateTime sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.Filename, &p.Filepath, &p.UploadedBy, &p.UploadedAt,
		&imageWidth, &imageLength, &dateTime,
	)
	if err != nil {
		return err
	}

	// Assign values from sql.Null types to the struct, falling back to zero values if NULL.
	p.ImageWidth = imageWidth.Int64
	p.ImageLength = imageLength.Int64
	p.DateTime = dateTime.Time

	return nil
}

// getPhotoDateString returns the date part of a photo's most relevant timestamp.
func getPhotoDateString(p *PhotoMetadata) string {
	if !p.DateTime.IsZero() {
		return p.DateTime.Format("2006-01-02")
	} else {
		return p.UploadedAt.Format("2006-01-02")
	}
}