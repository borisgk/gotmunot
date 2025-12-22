package handlers

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"tm25/internal/auth"

	_ "modernc.org/sqlite"
)

func TestLoginCookiePath(t *testing.T) {
	// 1. Setup temporary DB
	tempDir, err := os.MkdirTemp("", "tm25_auth_test")
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
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			uuid TEXT NOT NULL UNIQUE,
			username TEXT NOT NULL UNIQUE,
			password TEXT NOT NULL,
			db_path TEXT NOT NULL
		);
		CREATE TABLE sessions (
			token TEXT PRIMARY KEY,
			username TEXT NOT NULL,
			expiry DATETIME NOT NULL
		);
	`)
	if err != nil {
		t.Fatal(err)
	}

	// Create test user
	password := "password123"
	hashedPassword := auth.HashPassword(password)
	_, err = db.Exec("INSERT INTO users (uuid, username, password, db_path) VALUES (?, ?, ?, ?)",
		"test-uuid", "testuser", hashedPassword, "test-user-db-path")
	if err != nil {
		t.Fatal(err)
	}

	// Initialize handlers
	Init(db, nil) // Template can be nil for this test as we don't render it in the success path or verify rendering

	// 2. Create Request
	form := url.Values{}
	form.Add("username", "testuser")
	form.Add("password", password)
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()

	// 3. Execute Handler
	loginHandler(w, req)

	// 4. Verify Response
	resp := w.Result()
	if resp.StatusCode != http.StatusSeeOther { // Redirect after login
		t.Fatalf("Expected status 303, got %d", resp.StatusCode)
	}

	// 5. Check Cookie
	cookies := resp.Cookies()
	var sessionCookie *http.Cookie
	for _, c := range cookies {
		if c.Name == "session_token" {
			sessionCookie = c
			break
		}
	}

	if sessionCookie == nil {
		t.Fatal("Session cookie not found")
	}

	if sessionCookie.Path != "/" {
		t.Errorf("Expected cookie path '/', got '%s'", sessionCookie.Path)
	}
}
