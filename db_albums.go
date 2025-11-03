package main

import (
	"database/sql"
	"fmt"
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
