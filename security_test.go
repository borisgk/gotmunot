package main

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestMediaHandlerDirectoryTraversal(t *testing.T) {
	// 1. Setup temporary directory for photos
	tmpDir := t.TempDir()
	AppConfig.PhotoUploadDir = tmpDir

	// 2. Create user directories
	user1Dir := filepath.Join(tmpDir, "user1")
	user2Dir := filepath.Join(tmpDir, "user2")
	os.MkdirAll(user1Dir, 0755)
	os.MkdirAll(user2Dir, 0755)

	// 3. Create a secret file in user2
	secretFile := filepath.Join(user2Dir, "secret.txt")
	if err := os.WriteFile(secretFile, []byte("secret content"), 0644); err != nil {
		t.Fatalf("Failed to create secret file: %v", err)
	}

	// 4. Setup in-memory DB for session validation
	var err error
	db, err = sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open in-memory DB: %v", err)
	}
	defer db.Close()

	// Create sessions table
	_, err = db.Exec(`CREATE TABLE sessions (token TEXT PRIMARY KEY, username TEXT, expiry DATETIME)`)
	if err != nil {
		t.Fatalf("Failed to create sessions table: %v", err)
	}

	// Create a valid session for user1
	token := "valid-token-user1"
	_, err = db.Exec("INSERT INTO sessions (token, username, expiry) VALUES (?, ?, ?)", token, "user1", time.Now().Add(1*time.Hour))
	if err != nil {
		t.Fatalf("Failed to insert session: %v", err)
	}

	// 5. Create a request with directory traversal
	// Attempt to access user2's file as user1
	req, err := http.NewRequest("GET", "/media/user1/../user2/secret.txt", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "session_token", Value: token})

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(mediaHandler)

	handler.ServeHTTP(rr, req)

	// 6. Check the status code
	// BEFORE FIX: This might return 200 OK if traversal is allowed.
	// AFTER FIX: This should return 403 Forbidden.
	if rr.Code == http.StatusOK {
		t.Errorf("Security Vulnerability: Directory traversal allowed! Got 200 OK, expected 403 Forbidden.")
	} else if rr.Code != http.StatusForbidden {
		t.Errorf("Expected 403 Forbidden, got %v", rr.Code)
	}
}
