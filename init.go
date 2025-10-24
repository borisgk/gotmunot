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
	db.Exec("ALTER TABLE users ADD COLUMN uuid TEXT UNIQUE")
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

	// Initialize the photos database
	initPhotosDB()

	// Add default users if they don't exist, and ensure they have UUIDs.
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
		} else {
			// Ensure existing users have a UUID
			var existingUUID sql.NullString
			db.QueryRow("SELECT uuid FROM users WHERE username = ?", username).Scan(&existingUUID)
			if !existingUUID.Valid || existingUUID.String == "" {
				db.Exec("UPDATE users SET uuid = ? WHERE username = ?", uuid.New().String(), username)
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
