package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	_ "modernc.org/sqlite"
)

var db *sql.DB
var tmpl *template.Template

func init() {
	fmt.Println("Initializing TM25...")

	// Load application configuration
	LoadConfig()

	// Ensure the "static" directory exists.
	if _, err := os.Stat("static"); os.IsNotExist(err) {
		fmt.Println("Creating 'static' directory...")
		if err := os.Mkdir("static", 0755); err != nil {
			log.Fatalf("Error creating 'static' directory: %v", err)
		}
	}
	// Create static/css directory if not exists
	if _, err := os.Stat("static/css"); os.IsNotExist(err) {
		fmt.Println("Creating 'static/css' directory...")
		if err := os.MkdirAll("static/css", 0755); err != nil { // use MkdirAll for nested directories
			log.Fatalf("Error creating 'static/css' directory: %v", err)
		}
	}
	// Create static/js directory if not exists
	if _, err := os.Stat("static/js"); os.IsNotExist(err) {
		fmt.Println("Creating 'static/js' directory...")
		if err := os.MkdirAll("static/js", 0755); err != nil {
			log.Fatalf("Error creating 'static/js' directory: %v", err)
		}
	}
	// Create the photo upload directory if it doesn't exist.
	if _, err := os.Stat(AppConfig.PhotoUploadDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.PhotoUploadDir)
		if err := os.MkdirAll(AppConfig.PhotoUploadDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", AppConfig.PhotoUploadDir, err)
		}
	}

	// Ensure "templates" directory exists
	if _, err := os.Stat("templates"); os.IsNotExist(err) {
		fmt.Println("Creating 'templates' directory...")
		if err := os.Mkdir("templates", 0755); err != nil {
			log.Fatalf("Error creating 'templates' directory: %v", err)
		}
	}

	// Ensure the "data" directory for databases exists.
	if _, err := os.Stat(AppConfig.DataDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.DataDir)
		if err := os.Mkdir(AppConfig.DataDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", AppConfig.DataDir, err)
		}
	}

	// Initialize the database connection.
	var err error
	dbPath := filepath.Join(AppConfig.DataDir, "users.db")
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("Error opening database: %v", err)
	}

	// Create the users table if it doesn't exist.
	_, err = db.Exec(`
        CREATE TABLE IF NOT EXISTS users (
            id INTEGER PRIMARY KEY AUTOINCREMENT,
            uuid TEXT UNIQUE NOT NULL,
            username TEXT UNIQUE,
            password TEXT
        )
    `)
	if err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}

	// Initialize the photos database (global)
	dbPathPhotos := filepath.Join(AppConfig.DataDir, "photos.db")
	photosDB, err = sql.Open("sqlite", dbPathPhotos)
	if err != nil {
		log.Fatalf("Error opening photos database: %v", err)
	}

	// Create the photos table if it doesn't exist.
	_, err = photosDB.Exec(`
		CREATE TABLE IF NOT EXISTS photos (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			filename TEXT,
			filepath TEXT UNIQUE,
			uploaded_by TEXT,
			uploaded_at DATETIME,
			image_width INTEGER,
			image_length INTEGER,
			date_time DATETIME
		)
	`)
	if err != nil {
		log.Fatalf("Error creating photos table: %v", err)
	}

	// Prepare the insert statement for saving photo metadata.
	// This is more efficient as the SQL is parsed only once.
	insertPhotoStmt, err = photosDB.Prepare(`
		INSERT INTO photos (
			filename, filepath, uploaded_by, uploaded_at, 
			image_width, image_length, date_time
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		log.Fatalf("Error preparing insert photo statement: %v", err)
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
		log.Fatalf("Error creating sessions table: %v", err)
	}

	// Add default users if they don't exist.
	users := map[string]string{
		"user1": "password123",
		"user2": "securepass",
	}
	for username, password := range users {
		var count int
		db.QueryRow("SELECT COUNT(*) FROM users WHERE username = ?", username).Scan(&count)
		if count == 0 {
			userUUID := uuid.New().String()
			hashedPassword := hashPassword(password)
			_, err := db.Exec("INSERT INTO users (uuid, username, password) VALUES (?, ?, ?)", userUUID, username, hashedPassword)
			if err != nil {
				log.Fatalf("Error adding default user %s: %v", username, err)
			}
		}
	}

	funcMap := template.FuncMap{
		"toThumbPath": func(username, originalPath string) string {
			return filepath.Join(username, "thumbs", originalPath) + ".webp"
		},
		"toPreviewPath": func(username, originalPath string) string {
			return filepath.Join(username, "previews", originalPath)
		},
	}
	// Parse the templates
	tmpl, err = template.New("").Funcs(funcMap).ParseFiles("templates/login.html", "templates/gallery.html", "templates/upload.html", "templates/service.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}
}
