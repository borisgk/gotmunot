package main

import (
	"database/sql"
	"fmt"
	"log"
	"path/filepath"
	"time"
)

// albumExists checks if an album with the given name already exists for the user.
func albumExists(userDB *sql.DB, name string) (bool, error) {
	var count int
	err := userDB.QueryRow("SELECT COUNT(*) FROM albums WHERE name = ?", name).Scan(&count)
	if err != nil {
		return false, fmt.Errorf("failed to query for album existence: %w", err)
	}
	return count > 0, nil
}

// createAlbum inserts a new album into the user's database.
func createAlbum(userDB *sql.DB, name, description string) (int64, error) {
	stmt, err := userDB.Prepare(`
		INSERT INTO albums (name, description, created_at)
		VALUES (?, ?, ?)
	`)
	if err != nil {
		return 0, fmt.Errorf("failed to prepare album insert statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(name, description, time.Now())
	if err != nil {
		return 0, fmt.Errorf("failed to execute album insert: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("failed to get last insert ID for album: %w", err)
	}

	return id, nil
}

// getAlbumsForUser retrieves all albums for a given user from their database.
func getAlbumsForUser(userDB *sql.DB, username string) ([]Album, error) {
	query := `
		SELECT
			a.id,
			a.name,
			a.description,
			a.created_at,
			(SELECT COUNT(*) FROM album_photos ap WHERE ap.album_id = a.id) as photo_count,
			p.filepath
		FROM
			albums a
		LEFT JOIN
			photos p ON a.cover_photo_id = p.id
		ORDER BY
			a.created_at DESC;
	`
	rows, err := userDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query for albums: %w", err)
	}
	defer rows.Close()

	var albums []Album
	for rows.Next() {
		var album Album
		var coverPhotoPath sql.NullString // Use sql.NullString to handle NULL cover photos

		if err := rows.Scan(&album.ID, &album.Name, &album.Description, &album.CreatedAt, &album.PhotoCount, &coverPhotoPath); err != nil {
			return nil, fmt.Errorf("failed to scan album row: %w", err)
		}

		// If a cover photo exists, construct its thumbnail URL. Otherwise, use a placeholder.
		if coverPhotoPath.Valid && coverPhotoPath.String != "" {
			album.CoverPhoto = filepath.Join("/media", username, "thumbs", coverPhotoPath.String)
		} else {
			album.CoverPhoto = "/static/img/placeholder.png"
		}

		albums = append(albums, album)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during album rows iteration: %w", err)
	}

	return albums, nil
}

// AlbumListItem is a lightweight struct for album lists.
type AlbumListItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// getAlbumListForUser retrieves a simple list of album IDs and names for a user.
func getAlbumListForUser(userDB *sql.DB) ([]AlbumListItem, error) {
	query := `SELECT id, name FROM albums ORDER BY name ASC;`
	rows, err := userDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query for album list: %w", err)
	}
	defer rows.Close()

	var albumList []AlbumListItem
	for rows.Next() {
		var item AlbumListItem
		if err := rows.Scan(&item.ID, &item.Name); err != nil {
			return nil, fmt.Errorf("failed to scan album list item: %w", err)
		}
		albumList = append(albumList, item)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during album list rows iteration: %w", err)
	}

	return albumList, nil
}

// addPhotosToAlbum adds a list of photos (by filename) to a specific album.
// It also sets the album's cover photo to the first photo in the list if no cover is set.
func addPhotosToAlbum(userDB *sql.DB, albumID int64, filenames []string) (int, error) {
	if len(filenames) == 0 {
		return 0, nil // Nothing to do
	}

	tx, err := userDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback() // Rollback on error

	// Prepare statements
	photoIDStmt, err := tx.Prepare("SELECT id FROM photos WHERE filename = ?")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare photo ID select statement: %w", err)
	}
	defer photoIDStmt.Close()

	insertStmt, err := tx.Prepare("INSERT OR IGNORE INTO album_photos (album_id, photo_id) VALUES (?, ?)")
	if err != nil {
		return 0, fmt.Errorf("failed to prepare album_photos insert statement: %w", err)
	}
	defer insertStmt.Close()

	var firstPhotoID int64 = -1
	var photosAdded int = 0

	for _, filename := range filenames {
		var photoID int64
		err := photoIDStmt.QueryRow(filename).Scan(&photoID)
		if err != nil {
			// Log but continue, so one bad filename doesn't stop the whole batch
			fmt.Printf("Could not find photo ID for filename %s: %v\n", filename, err)
			continue
		}

		if firstPhotoID == -1 {
			firstPhotoID = photoID
		}

		result, err := insertStmt.Exec(albumID, photoID)
		if err != nil {
			return 0, fmt.Errorf("failed to insert into album_photos for photo %d: %w", photoID, err)
		}
		rowsAffected, _ := result.RowsAffected()
		if rowsAffected > 0 {
			photosAdded++
		}
	}

	// If a cover photo isn't set for the album, set it to the first photo we added.
	if firstPhotoID != -1 {
		_, err := tx.Exec(`
			UPDATE albums
			SET cover_photo_id = ?
			WHERE id = ? AND cover_photo_id IS NULL
		`, firstPhotoID, albumID)
		if err != nil {
			return 0, fmt.Errorf("failed to update cover photo: %w", err)
		}
	}

	return photosAdded, tx.Commit()
}

// getAlbumDetails retrieves the details for a single album.
func getAlbumDetails(userDB *sql.DB, albumID int64) (*Album, error) {
	var album Album
	query := `SELECT id, name, description FROM albums WHERE id = ?`
	err := userDB.QueryRow(query, albumID).Scan(&album.ID, &album.Name, &album.Description)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("album with ID %d not found", albumID)
		}
		return nil, fmt.Errorf("failed to query for album details: %w", err)
	}
	return &album, nil
}

// getPhotosForAlbum retrieves all photos associated with a specific album ID for a user.
func getPhotosForAlbum(userDB *sql.DB, username string, albumID int64) ([]PhotoMetadata, error) {
	log.Printf("Attempting to get photos for album ID %d for user '%s'", albumID, username)

	query := `
		SELECT
			p.id,
			p.filename,
			p.filepath,
			p.uploaded_at,
			p.date_time,
			p.image_width,
			p.image_length
		FROM
			photos p
		JOIN
			album_photos ap ON p.id = ap.photo_id
		WHERE
			ap.album_id = ?
		ORDER BY
			p.date_time DESC;
	`
	rows, err := userDB.Query(query, albumID)
	if err != nil {
		log.Printf("ERROR: Failed to query for photos in album %d: %v", albumID, err)
		return nil, fmt.Errorf("failed to query for photos in album %d: %w", albumID, err) // Keep original error wrapping
	}
	defer rows.Close()

	var photos []PhotoMetadata
	for rows.Next() {
		var p PhotoMetadata
		var dt sql.NullTime

		err := rows.Scan(&p.ID, &p.Filename, &p.Filepath, &p.UploadedAt,
			&dt,
			&p.ImageWidth,
			&p.ImageLength,
		)
		if err != nil {
			log.Printf("ERROR: Failed to scan photo row for album %d: %v", albumID, err)
			return nil, fmt.Errorf("failed to scan photo row for album: %w", err) // Keep original error wrapping
		}

		if dt.Valid {
			p.DateTime = dt.Time
		} else {
			p.DateTime = p.UploadedAt
		}

		// Pre-calculate paths for the template.
		p.ThumbPath = filepath.Join("/media", username, "thumbs", p.Filepath)
		p.PreviewPath = filepath.Join("/media", username, "previews", p.Filepath)

		photos = append(photos, p)
	}

	if err = rows.Err(); err != nil {
		log.Printf("ERROR: An error occurred during photo rows iteration for album %d: %v", albumID, err)
		return nil, fmt.Errorf("error during photo rows iteration for album: %w", err) // Keep original error wrapping
	}

	log.Printf("Successfully retrieved %d photos for album ID %d", len(photos), albumID)
	return photos, nil
}

// updateAlbum updates an album's name and description in the database.
func updateAlbum(userDB *sql.DB, albumID int64, name, description string) error {
	stmt, err := userDB.Prepare(`
		UPDATE albums
		SET name = ?, description = ?
		WHERE id = ?
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare album update statement: %w", err)
	}
	defer stmt.Close()

	result, err := stmt.Exec(name, description, albumID)
	if err != nil {
		return fmt.Errorf("failed to execute album update: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected after update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("no album found with ID %d to update", albumID)
	}

	return nil
}
