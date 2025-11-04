package main

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

// PhotoMetadata struct to represent photo metadata.
type PhotoMetadata struct {
	ID            int
	Filename      string
	Filepath      string
	UploadedBy    string
	UploadedAt    time.Time
	ImageWidth    int64
	ImageLength   int64
	DateTime      time.Time
	ThumbWidth    int
	ThumbHeight   int
	PreviewWidth  int
	PreviewHeight int

	// Fields populated at runtime, not stored in DB.
	ThumbPath   string
	PreviewPath string
}

// getPhotos retrieves all photos for a user, with an optional year filter.
func getPhotos(userDB *sql.DB, username string, year int) ([]PhotoMetadata, error) {
	query := `SELECT id, filename, filepath, uploaded_at, 
		image_width, image_length, date_time,
		thumb_width, thumb_height, preview_width, preview_height
		FROM photos WHERE 1=1`
	args := []interface{}{}

	if year > 0 {
		query += " AND CAST(SUBSTR(date_time, 1, 4) AS INTEGER) = ?"
		args = append(args, year)
	}

	query += ` ORDER BY date_time DESC`

	return queryPhotos(userDB, username, query, args...)
}

// getTotalPhotoCount returns the total number of photos in the database.
func getTotalPhotoCount(userDB *sql.DB, year int) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM photos WHERE 1=1"
	args := []interface{}{}

	if year > 0 {
		query += " AND CAST(SUBSTR(date_time, 1, 4) AS INTEGER) = ?"
		args = append(args, year)
	}

	err := userDB.QueryRow(query, args...).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

// getPhotoByFilename retrieves all metadata for a single photo by its filename.
func getPhotoByFilename(userDB *sql.DB, username, filename string) (PhotoMetadata, error) {
	var p PhotoMetadata
	row := userDB.QueryRow(`
        SELECT 
            id, filename, filepath, uploaded_at, image_width, image_length, date_time,
			thumb_width, thumb_height, preview_width, preview_height
        FROM photos
        WHERE filename = ?
    `, filename)

	if err := scanPhoto(row, &p); err != nil {
		return p, err
	}

	// Manually set the username since it's not in the user-specific DB.
	p.UploadedBy = username
	return p, nil
}

// getPhotoCountsByYear retrieves a map of year to photo count for a user.
func getPhotoCountsByYear(userDB *sql.DB) (map[int]int, error) {
	rows, err := userDB.Query(`
		SELECT
			CAST(SUBSTR(date_time, 1, 4) AS INTEGER) as year,
			COUNT(*) as count
		FROM photos
		WHERE year IS NOT NULL
		GROUP BY year
	`)
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

// queryPhotos is a helper function to run a query and scan the results into a slice of PhotoMetadata.
func queryPhotos(userDB *sql.DB, username, query string, args ...interface{}) ([]PhotoMetadata, error) {
	rows, err := userDB.Query(query, args...)
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
		// Manually set the username since it's not in the user-specific DB.
		p.UploadedBy = username
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// savePhotoMetadata saves photo metadata to the database.
func savePhotoMetadata(p *PhotoMetadata) (int64, error) {
	userDB, err := getUserDB(p.UploadedBy)
	if err != nil {
		return 0, fmt.Errorf("could not get user database for %s: %w", p.UploadedBy, err)
	}

	tx, err := userDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error, if Commit is not called

	stmt, err := tx.Prepare(`
		INSERT INTO photos (
			filename, filepath, uploaded_at, 
			image_width, image_length, date_time,
			thumb_width, thumb_height, preview_width, preview_height
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare statement within transaction: %w", err)
	}
	defer stmt.Close() // Close the statement when the function returns

	result, err := stmt.Exec(
		p.Filename, p.Filepath, p.UploadedAt,
		p.ImageWidth, p.ImageLength, p.DateTime,
		p.ThumbWidth, p.ThumbHeight, p.PreviewWidth, p.PreviewHeight,
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
func deletePhotoByFilename(userDB *sql.DB, filename string) error {
	tx, err := userDB.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error

	// First, get the photo ID from the filename.
	var photoID int64
	err = tx.QueryRow("SELECT id FROM photos WHERE filename = ?", filename).Scan(&photoID)
	if err != nil {
		if err == sql.ErrNoRows {
			// If the photo doesn't exist, there's nothing to do.
			return nil
		}
		return fmt.Errorf("failed to find photo ID for filename %s: %w", filename, err)
	}

	// Delete all associations from the album_photos table.
	if _, err := tx.Exec("DELETE FROM album_photos WHERE photo_id = ?", photoID); err != nil {
		return fmt.Errorf("failed to delete from album_photos: %w", err)
	}

	// Delete the photo from the photos table.
	if _, err := tx.Exec("DELETE FROM photos WHERE id = ?", photoID); err != nil {
		return fmt.Errorf("failed to delete from photos: %w", err)
	}

	return tx.Commit()
}

// updatePhotoDateAndPath moves a photo's files to a new directory structure based on a new date
// and updates its metadata in the database within a single transaction.
func updatePhotoDateAndPath(username, filename string, newDate time.Time) error {
	userDB, err := getUserDB(username)
	if err != nil {
		return err
	}

	// Get current photo metadata to know the old path and verify ownership.
	photo, err := getPhotoByFilename(userDB, username, filename)
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
		_, err := userDB.Exec("UPDATE photos SET date_time = ? WHERE filename = ?", newDate, filename)
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
	tx, err := userDB.Begin()
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
	if err := os.MkdirAll(filepath.Dir(newOriginalPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newPreviewPath), 0755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(newThumbPath), 0755); err != nil {
		return err
	}

	// Rename/move the files.
	if err := os.Rename(oldOriginalPath, newOriginalPath); err != nil {
		return err
	}
	if err := os.Rename(oldPreviewPath, newPreviewPath); err != nil {
		return err
	}
	if err := os.Rename(oldThumbPath, newThumbPath); err != nil {
		return err
	}

	// TODO: Optionally, clean up old empty directories. This is a non-trivial task
	// and can be skipped for now.

	// 3. If all operations succeeded, commit the transaction.
	return tx.Commit()
}

// scanPhoto is a helper to scan a photo row into a PhotoMetadata struct.
func scanPhoto(scanner interface{ Scan(...interface{}) error }, p *PhotoMetadata) error {
	// Use sql.Null types for scanning to handle potential NULL values from the database.
	var imageWidth, imageLength sql.NullInt64
	var thumbWidth, thumbHeight, previewWidth, previewHeight sql.NullInt64
	var dateTime sql.NullTime

	err := scanner.Scan(
		&p.ID, &p.Filename, &p.Filepath, &p.UploadedAt,
		&imageWidth, &imageLength, &dateTime, &thumbWidth, &thumbHeight,
		&previewWidth, &previewHeight,
	)
	if err != nil {
		return err
	}

	// Assign values from sql.Null types to the struct, falling back to zero values if NULL.
	p.ImageWidth = imageWidth.Int64
	p.ImageLength = imageLength.Int64
	p.DateTime = dateTime.Time
	p.ThumbWidth = int(thumbWidth.Int64)
	p.ThumbHeight = int(thumbHeight.Int64)
	p.PreviewWidth = int(previewWidth.Int64)
	p.PreviewHeight = int(previewHeight.Int64)

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
