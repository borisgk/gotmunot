package database

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMigration_Version2_Normalization(t *testing.T) {
	// 1. Setup temporary DB
	tempDir, err := os.MkdirTemp("", "tm25_migration_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test_user.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// 2. Apply only Version 1
	if err := ApplyMigrations(db, UserDBMigrations[:1]); err != nil {
		t.Fatalf("Failed to apply Version 1: %v", err)
	}

	// 3. Insert "bad" legacy data
	// Use Go's String() format which was causing issues
	legacyTime := time.Date(2023, 10, 20, 15, 30, 45, 0, time.UTC).String()

	_, err = db.Exec(`
		INSERT INTO photos (filename, filepath, uploaded_at, date_time)
		VALUES (?, ?, ?, ?)
	`, "test.jpg", "2023/10/20/test.jpg", legacyTime, legacyTime)
	if err != nil {
		t.Fatalf("Failed to insert legacy photo data: %v", err)
	}

	_, err = db.Exec(`
		INSERT INTO albums (name, created_at)
		VALUES (?, ?)
	`, "Legacy Album", legacyTime)
	if err != nil {
		t.Fatalf("Failed to insert legacy album data: %v", err)
	}

	// 4. Apply Version 2
	if err := ApplyMigrations(db, UserDBMigrations); err != nil {
		t.Fatalf("Failed to apply Version 2: %v", err)
	}

	// 5. Verify normalization
	expectedTime := "2023-10-20 15:30:45"

	var photoUploadedAt, photoDateTime string
	err = db.QueryRow("SELECT CAST(uploaded_at AS TEXT), CAST(date_time AS TEXT) FROM photos WHERE filename = ?", "test.jpg").Scan(&photoUploadedAt, &photoDateTime)
	if err != nil {
		t.Fatalf("Failed to query normalized photo data: %v", err)
	}

	if photoUploadedAt != expectedTime {
		t.Errorf("Photo uploaded_at not normalized: got %s, want %s", photoUploadedAt, expectedTime)
	}
	if photoDateTime != expectedTime {
		t.Errorf("Photo date_time not normalized: got %s, want %s", photoDateTime, expectedTime)
	}

	var albumCreatedAt string
	err = db.QueryRow("SELECT CAST(created_at AS TEXT) FROM albums WHERE name = ?", "Legacy Album").Scan(&albumCreatedAt)
	if err != nil {
		t.Fatalf("Failed to query normalized album data: %v", err)
	}

	if albumCreatedAt != expectedTime {
		t.Errorf("Album created_at not normalized: got %s, want %s", albumCreatedAt, expectedTime)
	}
}
