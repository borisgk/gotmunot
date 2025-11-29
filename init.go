package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"sync"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var db *sql.DB
var tmpl *template.Template

// userDBs is a cache for active user database connections.
var userDBs = struct {
	sync.RWMutex
	connections map[string]*sql.DB
}{connections: make(map[string]*sql.DB)}

func Initialize() error {
	fmt.Println("Initializing TM25...")

	// Configure govips to be less verbose. Only log errors.
	vips.LoggingSettings(nil, vips.LogLevelError)
	vips.Startup(nil) // Use default vips configuration

	// Load application configuration
	LoadConfig()

	// Ensure the "static" directory exists.
	if _, err := os.Stat("static"); os.IsNotExist(err) {
		fmt.Println("Creating 'static' directory...")
		if err := os.Mkdir("static", 0755); err != nil {
			return fmt.Errorf("error creating 'static' directory: %w", err)
		}
	}
	// Create static/css directory if not exists
	if _, err := os.Stat("static/css"); os.IsNotExist(err) {
		fmt.Println("Creating 'static/css' directory...")
		if err := os.MkdirAll("static/css", 0755); err != nil { // use MkdirAll for nested directories
			return fmt.Errorf("error creating 'static/css' directory: %w", err)
		}
	}
	// Create static/js directory if not exists
	if _, err := os.Stat("static/js"); os.IsNotExist(err) {
		fmt.Println("Creating 'static/js' directory...")
		if err := os.MkdirAll("static/js", 0755); err != nil {
			return fmt.Errorf("error creating 'static/js' directory: %w", err)
		}
	}
	// Create the photo upload directory if it doesn't exist.
	if _, err := os.Stat(AppConfig.PhotoUploadDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.PhotoUploadDir)
		if err := os.MkdirAll(AppConfig.PhotoUploadDir, 0755); err != nil {
			return fmt.Errorf("error creating '%s' directory: %w", AppConfig.PhotoUploadDir, err)
		}
	}

	// Ensure "templates" directory exists
	if _, err := os.Stat("templates"); os.IsNotExist(err) {
		fmt.Println("Creating 'templates' directory...")
		if err := os.Mkdir("templates", 0755); err != nil {
			return fmt.Errorf("error creating 'templates' directory: %w", err)
		}
	}

	// Ensure the "data" directory for databases exists.
	if _, err := os.Stat(AppConfig.DataDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.DataDir)
		if err := os.Mkdir(AppConfig.DataDir, 0755); err != nil {
			return fmt.Errorf("error creating '%s' directory: %w", AppConfig.DataDir, err)
		}
	}

	// Initialize the database connection.
	var err error
	// Add _busy_timeout to the DSN to make SQLite wait if the DB is locked.
	dbPath := filepath.Join(AppConfig.DataDir, "users.db?_busy_timeout=5000")
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		return fmt.Errorf("error opening database: %w", err)
	}

	// Enable Write-Ahead Logging for better concurrency.
	_, err = db.Exec("PRAGMA journal_mode=WAL;")
	if err != nil {
		return fmt.Errorf("error enabling WAL mode for users.db: %w", err)
	}
	log.Println("Users database WAL mode enabled.")

	// Create the users table if it doesn't exist.
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            uuid TEXT UNIQUE,
            username TEXT UNIQUE,
            password TEXT,
            db_path TEXT
        )
    `)
	if err != nil {
		return fmt.Errorf("error creating users table: %w", err)
	}

	log.Println("Photos database initialized.")

	// Create the sessions table if it doesn't exist.
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS sessions (
            token TEXT PRIMARY KEY,
            username TEXT,
            expiry DATETIME
        )
    `)
	if err != nil {
		return fmt.Errorf("error creating sessions table: %w", err)
	}

	// Add default users if they don't exist.
	users := map[string]string{
		"user1": "password123",
		"user2": "securepass",
		"Boris": "bobkimel8-",
	}
	for username, password := range users {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
		if count == 0 {
			userUUID := uuid.New().String()
			hashedPassword := hashPassword(password)
			dbPath := filepath.Join(AppConfig.DataDir, fmt.Sprintf("%s.db", userUUID))

			_, err := db.Exec("INSERT INTO users (uuid, username, password, db_path) VALUES (?, ?, ?, ?)", userUUID, username, hashedPassword, dbPath)
			if err != nil {
				return fmt.Errorf("error adding default user %s: %w", username, err)
			}

			// Create and initialize the user's personal database
			userDB, err := openUserDB(dbPath)
			if err != nil {
				return fmt.Errorf("could not create database for user %s: %w", username, err)
			}
			userDB.Close() // Close connection after creation
		}
	}

	// Parse the templates
	tmpl, err = template.ParseGlob("templates/*.html")
	if err != nil {
		return fmt.Errorf("error parsing templates: %w", err)
	}

	return nil
}

// getUserDB returns a database connection for a specific user.
// It uses a cache to avoid repeatedly opening files.
func getUserDB(username string) (*sql.DB, error) {
	userDBs.RLock()
	userDB, ok := userDBs.connections[username]
	userDBs.RUnlock()

	if ok {
		return userDB, nil
	}

	// Connection not in cache, so we need a write lock to add it.
	userDBs.Lock()
	defer userDBs.Unlock()

	// Double-check if another goroutine created it while we were waiting for the lock.
	userDB, ok = userDBs.connections[username]
	if ok {
		return userDB, nil
	}

	// Get the user's database path from the main users.db
	var dbPath string
	err := db.QueryRow("SELECT db_path FROM users WHERE username = ?", username).Scan(&dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not find db path for user %s: %w", username, err)
	}

	// Open the user's database
	userDB, err = openUserDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Store the new connection in the cache.
	userDBs.connections[username] = userDB
	log.Printf("Opened and cached database connection for user: %s", username)

	return userDB, nil
}

// openUserDB handles the logic of opening a user's DB file and ensuring the schema is correct.
func openUserDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_busy_timeout=5000", dbPath)
	userDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency.
	if _, err := userDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode on %s: %w", dbPath, err)
	}

	// Create the photos table, now without 'uploaded_by'.
	_, err = userDB.Exec(`
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
		return nil, err
	}

	// Create the albums table.
	_, err = userDB.Exec(`
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
		return nil, err
	}

	// Create the album_photos join table.
	_, err = userDB.Exec(`
		CREATE TABLE IF NOT EXISTS album_photos (
			album_id INTEGER NOT NULL,
			photo_id INTEGER NOT NULL,
			PRIMARY KEY (album_id, photo_id),
			FOREIGN KEY(album_id) REFERENCES albums(id) ON DELETE CASCADE,
			FOREIGN KEY(photo_id) REFERENCES photos(id) ON DELETE CASCADE
		)
	`)
	if err != nil {
		return nil, err
	}

	// Create the settings table.
	_, err = userDB.Exec(`
		CREATE TABLE IF NOT EXISTS settings (
			name TEXT NOT NULL PRIMARY KEY,
			value TEXT NOT NULL
		)`)
	return userDB, err
}
