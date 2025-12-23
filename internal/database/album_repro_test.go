package database_test

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	"tm25/internal/database"
	"tm25/internal/models"

	_ "modernc.org/sqlite"
)

func TestGetPhotosForAlbum_ColumnMismatch(t *testing.T) {
	// 1. Setup temporary DB
	tempDir, err := os.MkdirTemp("", "tm25_album_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_user.db")
	userDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer userDB.Close()

	// 2. Initialize schema (using the implementation from migrations.go logic)
	// We can manually run the CREATE TABLE statements here for simplicity and because we can't easily import `MainDBMigrations` if it's in the same package but different file and we are `database_test` package.
	// Actually, let's just use the `database` package for the test to access `ScanPhoto` and `GetPhotosForAlbum`.
	// Wait, `database.GetPhotosForAlbum` is exported, so `database_test` is fine. But `ScanPhoto` is exported? Yes.

	// Create tables
	_, err = userDB.Exec(`
		CREATE TABLE photos (
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
		);
		CREATE TABLE albums (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			description TEXT,
			created_at DATETIME NOT NULL,
			cover_photo_id INTEGER
		);
		CREATE TABLE album_photos (
			album_id INTEGER NOT NULL,
			photo_id INTEGER NOT NULL,
			PRIMARY KEY (album_id, photo_id)
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// 3. Insert specific test data to exercise the query
	_, err = userDB.Exec(`
		INSERT INTO photos (filename, filepath, uploaded_at, image_width, image_length, date_time, thumb_width, thumb_height, preview_width, preview_height)
		VALUES ('test.jpg', '2023/10/20/test.jpg', ?, 100, 100, ?, 50, 50, 80, 80)
	`, time.Now().Format(database.SqliteTimeLayout), time.Now().Format(database.SqliteTimeLayout))
	if err != nil {
		t.Fatal(err)
	}

	// Get the photo ID
	var photoID int64
	err = userDB.QueryRow("SELECT id FROM photos").Scan(&photoID)
	if err != nil {
		t.Fatal(err)
	}

	// Create album
	_, err = userDB.Exec("INSERT INTO albums (name, created_at) VALUES ('Test Album', ?)", time.Now().Format(database.SqliteTimeLayout))
	if err != nil {
		t.Fatal(err)
	}
	var albumID int64
	err = userDB.QueryRow("SELECT id FROM albums").Scan(&albumID)
	if err != nil {
		t.Fatal(err)
	}

	// Add photo to album
	_, err = userDB.Exec("INSERT INTO album_photos (album_id, photo_id) VALUES (?, ?)", albumID, photoID)
	if err != nil {
		t.Fatal(err)
	}

	// 4. Call `GetPhotosForAlbum` and assert it SUCCEEDS
	photos, err := database.GetPhotosForAlbum(userDB, "testuser", albumID)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if len(photos) != 1 {
		t.Fatalf("Expected 1 photo, got %d", len(photos))
	}
	p := photos[0]
	if p.Filename != "test.jpg" {
		t.Errorf("Expected filename test.jpg, got %s", p.Filename)
	}

	// Perform manual check with a wrapper that mimics the function to pinpoint if it's the scan
	var pmanual models.PhotoMetadata
	err = database.ScanPhoto(userDB.QueryRow("SELECT * FROM photos"), &pmanual)
	// This scan SHOULD succeed if we select *, but GetPhotosForAlbum selects specific columns
}
