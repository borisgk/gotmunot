package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

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
	// Create the photo upload directory if it doesn't exist.
	if _, err := os.Stat(AppConfig.PhotoUploadDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.PhotoUploadDir)
		if err := os.MkdirAll(AppConfig.PhotoUploadDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", AppConfig.PhotoUploadDir, err)
		}
	}
	// Create the thumbnail directory if it doesn't exist.
	if _, err := os.Stat(AppConfig.ThumbsDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.ThumbsDir)
		if err := os.MkdirAll(AppConfig.ThumbsDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", AppConfig.ThumbsDir, err)
		}
	}
	// Create the preview directory if it doesn't exist.
	if _, err := os.Stat(AppConfig.PreviewsDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", AppConfig.PreviewsDir)
		if err := os.MkdirAll(AppConfig.PreviewsDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", AppConfig.PreviewsDir, err)
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
            username TEXT UNIQUE,
            password TEXT
        )
    `)
	if err != nil {
		log.Fatalf("Error creating users table: %v", err)
	}
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

	// Add some default users
	_, err = db.Exec("INSERT OR IGNORE INTO users (username, password) VALUES (?, ?), (?, ?)", "user1", hashPassword("password123"), "user2", hashPassword("securepass"))
	if err != nil {
		log.Fatalf("Error adding default users: %v", err)
	}

	funcMap := template.FuncMap{
		"toThumbPath": func(originalPath string) string {
			return filepath.Join("thumbs", originalPath) + ".webp"
		},
		"toPreviewPath": func(originalPath string) string {
			return filepath.Join("previews", originalPath)
		},
	}
	// Parse the templates
	tmpl, err = template.New("").Funcs(funcMap).ParseFiles("templates/login.html", "templates/content.html", "templates/upload.html", "templates/service.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}
}
