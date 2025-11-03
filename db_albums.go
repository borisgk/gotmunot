package main

import (
	"database/sql"
	"fmt"
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
