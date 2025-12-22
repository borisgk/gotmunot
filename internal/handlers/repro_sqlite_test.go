package handlers

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestSQLiteTimeRoundTrip(t *testing.T) {
	// 1. Setup temporary DB
	tempDir, err := os.MkdirTemp("", "tm25_sqlite_test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	// Initialize tables
	_, err = db.Exec(`
		CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			expiry DATETIME NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// 2. Insert Session with time.Time (Converted to Unix)
	token := "test-token"
	username := "testuser"
	expiry := time.Now().Add(1 * time.Hour)
	expiryUnix := expiry.Unix()
	t.Logf("Original Expiry: %v (Unix: %d)", expiry, expiryUnix)

	_, err = db.Exec("INSERT INTO sessions (token, username, expiry) VALUES (?, ?, ?)", token, username, expiryUnix)
	if err != nil {
		t.Fatalf("Failed to insert session: %v", err)
	}

	// 3. Read Session back
	var readUsername string
	var readExpiryUnix int64
	err = db.QueryRow("SELECT username, expiry FROM sessions WHERE token = ?", token).Scan(&readUsername, &readExpiryUnix)
	if err != nil {
		t.Fatalf("Failed to scan session: %v", err)
	}

	readExpiry := time.Unix(readExpiryUnix, 0)
	t.Logf("Read Expiry: %v (Unix: %d)", readExpiry, readExpiryUnix)

	// 4. Compare
	// Precision loss is expected (seconds vs nanoseconds)
	if readExpiryUnix != expiryUnix {
		t.Errorf("Time mismatch! Original: %d, Read: %d", expiryUnix, readExpiryUnix)
	}

	// 5. Check logic from auth.go
	if time.Now().After(readExpiry) {
		t.Errorf("Session considered expired immediately! Now: %v, Expiry: %v", time.Now(), readExpiry)
	}
}
