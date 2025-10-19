package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"

	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

var db *sql.DB
var tmpl *template.Template

// photoUploadDir defines the directory where uploaded photos are stored.
const photoUploadDir = "/data/tmunot"

// Constants for subdirectories.
const thumbsDir = "/data/tmunot/thumbs"
const previewsDir = "/data/tmunot/previews"

// dataDir defines the directory for database files.
const dataDir = "data"

func init() {
	fmt.Println("Initializing TM25...")

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
	if _, err := os.Stat(photoUploadDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", photoUploadDir)
		if err := os.MkdirAll(photoUploadDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", photoUploadDir, err)
		}
	}
	// Create the thumbnail directory if it doesn't exist.
	if _, err := os.Stat(thumbsDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", thumbsDir)
		if err := os.MkdirAll(thumbsDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", thumbsDir, err)
		}
	}
	// Create the preview directory if it doesn't exist.
	if _, err := os.Stat(previewsDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", previewsDir)
		if err := os.MkdirAll(previewsDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", previewsDir, err)
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
	if _, err := os.Stat(dataDir); os.IsNotExist(err) {
		fmt.Printf("Creating '%s' directory...\n", dataDir)
		if err := os.Mkdir(dataDir, 0755); err != nil {
			log.Fatalf("Error creating '%s' directory: %v", dataDir, err)
		}
	}

	// Initialize the database connection.
	var err error
	dbPath := filepath.Join(dataDir, "users.db")
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

// hashPassword hashes the given password using bcrypt.
func hashPassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}
	return string(hashedPassword)
}

// checkPasswordHash compares a hashed password with a plain text password.
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
