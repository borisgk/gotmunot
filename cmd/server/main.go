package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"tm25/internal/auth"
	"tm25/internal/config"
	"tm25/internal/database"
	"tm25/internal/handlers"

	"github.com/davidbyttow/govips/v2/vips"
	_ "modernc.org/sqlite"
)

func main() {
	// 1. Initialize VIPS
	vips.Startup(nil)
	defer vips.Shutdown()

	// 2. Load Configuration
	config.LoadConfig()

	// 3. Create necessary directories
	dirs := []string{
		config.AppConfig.PhotoUploadDir,
		config.AppConfig.DataDir,
		filepath.Join(config.AppConfig.DataDir, "users"), // For user databases
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Fatalf("Could not create directory %s: %v", dir, err)
		}
	}

	// 4. Initialize Database
	dbPath := filepath.Join(config.AppConfig.DataDir, "users.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	// 5. Apply Migrations
	if err := database.ApplyMigrations(db, database.MainDBMigrations); err != nil {
		log.Fatalf("Could not apply migrations: %v", err)
	}

	// 6. Create default user (if needed)
	// We check if the user exists first using a simpler query or just rely on INSERT OR IGNORE logic from previous code.
	// We need to hash the password.
	hashedPassword := auth.HashPassword("test")
	// The new user needs a database path.
	userDBPath := filepath.Join(config.AppConfig.DataDir, "users", "test.db")
	uuid := "550e8400-e29b-41d4-a716-446655440000" // Fixed UUID for test user

	_, err = db.Exec(`
		INSERT INTO users (uuid, username, password, db_path) 
		VALUES (?, ?, ?, ?)
		ON CONFLICT(username) DO NOTHING
	`, uuid, "test", hashedPassword, userDBPath)
	if err != nil {
		log.Printf("Error creating default user: %v", err)
	} else {
		// Ensure the user's database exists and is migrated.
		// We use database.GetUserDB which handles creation/migration of user DB connection,
		// but since we aren't in a handler context, we can just ensure it opens correctly.
		// However, GetUserDB caches the connection in the package variable.
		// We can just call it to prime the cache and ensure it works.
		if _, err := database.GetUserDB(db, "test"); err != nil {
			log.Printf("Error initializing default user database: %v", err)
		}
	}

	// 7. Parse Templates
	tmpl, err := template.ParseGlob("templates/*.html")
	if err != nil {
		log.Fatalf("Error parsing templates: %v", err)
	}

	// 8. Initialize Handlers Package
	handlers.Init(db, tmpl)
	handlers.SetupRoutes()

	// 9. Start Background Worker
	go handlers.StartMetadataSaveWorker()

	// 10. Start Server
	log.Println("Server processing photos at:", config.AppConfig.PhotoUploadDir)
	log.Println("Server database at:", config.AppConfig.DataDir)
	log.Println("Server starting on", config.AppConfig.Port, "...")
	if err := http.ListenAndServe(config.AppConfig.Port, nil); err != nil {
		log.Fatal(err)
	}
}
