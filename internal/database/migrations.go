package database

import (
	"database/sql"
	"fmt"
	"log"
)

// Migration represents a single database migration.
type Migration struct {
	Version     int
	Description string
	Up          func(*sql.Tx) error
}

// MainDBMigrations defines the migrations for the main users.db.
var MainDBMigrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema for users and sessions",
		Up: func(tx *sql.Tx) error {
			// Create users table
			_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS users (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					uuid TEXT UNIQUE,
					username TEXT UNIQUE,
					password TEXT,
					db_path TEXT
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create users table: %w", err)
			}

			// Create sessions table
			_, err = tx.Exec(`
				CREATE TABLE IF NOT EXISTS sessions (
					token TEXT PRIMARY KEY,
					username TEXT,
					expiry DATETIME
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create sessions table: %w", err)
			}
			return nil
		},
	},
}

// UserDBMigrations defines the migrations for individual user databases.
var UserDBMigrations = []Migration{
	{
		Version:     1,
		Description: "Initial schema for photos, albums, and settings",
		Up: func(tx *sql.Tx) error {
			// Create photos table
			_, err := tx.Exec(`
				CREATE TABLE IF NOT EXISTS photos (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					filename TEXT,
					filepath TEXT UNIQUE,
					uploaded_at DATETIME,
					image_width INTEGER,
					image_length INTEGER,
					date_time DATETIME,
					thumb_width INTEGER,
					thumb_height INTEGER,
					preview_width INTEGER,
					preview_height INTEGER
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create photos table: %w", err)
			}

			// Create albums table
			_, err = tx.Exec(`
				CREATE TABLE IF NOT EXISTS albums (
					id INTEGER PRIMARY KEY AUTOINCREMENT,
					name TEXT NOT NULL,
					description TEXT,
					created_at DATETIME NOT NULL,
					cover_photo_id INTEGER,
					FOREIGN KEY(cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create albums table: %w", err)
			}

			// Create album_photos join table
			_, err = tx.Exec(`
				CREATE TABLE IF NOT EXISTS album_photos (
					album_id INTEGER NOT NULL,
					photo_id INTEGER NOT NULL,
					PRIMARY KEY (album_id, photo_id),
					FOREIGN KEY(album_id) REFERENCES albums(id) ON DELETE CASCADE,
					FOREIGN KEY(photo_id) REFERENCES photos(id) ON DELETE CASCADE
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create album_photos table: %w", err)
			}

			// Create settings table
			_, err = tx.Exec(`
				CREATE TABLE IF NOT EXISTS settings (
					name TEXT NOT NULL PRIMARY KEY,
					value TEXT NOT NULL
				)
			`)
			if err != nil {
				return fmt.Errorf("failed to create settings table: %w", err)
			}
			return nil
		},
	},
	{
		Version:     2,
		Description: "Normalize date formats in photos and albums",
		Up: func(tx *sql.Tx) error {
			// Normalize photos table
			rows, err := tx.Query("SELECT id, uploaded_at, date_time FROM photos")
			if err != nil {
				return err
			}
			defer rows.Close()

			type photoUpdate struct {
				id         int
				uploadedAt string
				dateTime   string
			}
			var updates []photoUpdate

			for rows.Next() {
				var id int
				var uploadedAt, dateTime sql.NullString
				if err := rows.Scan(&id, &uploadedAt, &dateTime); err != nil {
					return err
				}

				newUploadedAt := ""
				if uploadedAt.Valid {
					newUploadedAt = parseFlexibleTime(uploadedAt.String).Format(SqliteTimeLayout)
				}

				newDateTime := ""
				if dateTime.Valid {
					newDateTime = parseFlexibleTime(dateTime.String).Format(SqliteTimeLayout)
				}

				if (uploadedAt.Valid && newUploadedAt != uploadedAt.String) || (dateTime.Valid && newDateTime != dateTime.String) {
					updates = append(updates, photoUpdate{id, newUploadedAt, newDateTime})
				}
			}
			rows.Close()

			for _, u := range updates {
				_, err := tx.Exec("UPDATE photos SET uploaded_at = ?, date_time = ? WHERE id = ?", u.uploadedAt, u.dateTime, u.id)
				if err != nil {
					return err
				}
			}

			// Normalize albums table
			arows, err := tx.Query("SELECT id, created_at FROM albums")
			if err != nil {
				return err
			}
			defer arows.Close()

			type albumUpdate struct {
				id        int
				createdAt string
			}
			var aupdates []albumUpdate

			for arows.Next() {
				var id int
				var createdAt string
				if err := arows.Scan(&id, &createdAt); err != nil {
					return err
				}

				newCreatedAt := parseFlexibleTime(createdAt).Format(SqliteTimeLayout)
				if newCreatedAt != createdAt {
					aupdates = append(aupdates, albumUpdate{id, newCreatedAt})
				}
			}
			arows.Close()

			for _, u := range aupdates {
				_, err := tx.Exec("UPDATE albums SET created_at = ? WHERE id = ?", u.createdAt, u.id)
				if err != nil {
					return err
				}
			}

			return nil
		},
	},
}

// ApplyMigrations applies the given migrations to the database.
func ApplyMigrations(db *sql.DB, migrations []Migration) error {
	// Ensure schema_migrations table exists
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS schema_migrations (
			version INTEGER PRIMARY KEY,
			applied_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("failed to create schema_migrations table: %w", err)
	}

	// Get current version
	var currentVersion int
	err = db.QueryRow("SELECT COALESCE(MAX(version), 0) FROM schema_migrations").Scan(&currentVersion)
	if err != nil {
		return fmt.Errorf("failed to get current migration version: %w", err)
	}

	for _, migration := range migrations {
		if migration.Version > currentVersion {
			log.Printf("Applying migration %d: %s", migration.Version, migration.Description)

			tx, err := db.Begin()
			if err != nil {
				return fmt.Errorf("failed to begin transaction for migration %d: %w", migration.Version, err)
			}

			if err := migration.Up(tx); err != nil {
				tx.Rollback()
				return fmt.Errorf("migration %d failed: %w", migration.Version, err)
			}

			_, err = tx.Exec("INSERT INTO schema_migrations (version) VALUES (?)", migration.Version)
			if err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to commit migration %d: %w", migration.Version, err)
			}
		}
	}

	return nil
}
