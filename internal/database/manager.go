package database

import (
	"database/sql"
	"fmt"
	"log"
	"sync"
)

// userDBs needs to be exported or private? Private is fine if GetUserDB is public.
var userDBs = struct {
	sync.RWMutex
	connections map[string]*sql.DB
}{connections: make(map[string]*sql.DB)}

// GetUserDB returns a database connection for a specific user.
// It uses a cache to avoid repeatedly opening files.
func GetUserDB(mainDB *sql.DB, username string) (*sql.DB, error) {
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
	err := mainDB.QueryRow("SELECT db_path FROM users WHERE username = ?", username).Scan(&dbPath)
	if err != nil {
		return nil, fmt.Errorf("could not find db path for user %s: %w", username, err)
	}

	// Open the user's database
	userDB, err = OpenUserDB(dbPath)
	if err != nil {
		return nil, err
	}

	// Store the new connection in the cache.
	userDBs.connections[username] = userDB
	log.Printf("Opened and cached database connection for user: %s", username)

	return userDB, nil
}

// OpenUserDB handles the logic of opening a user's DB file and ensuring the schema is correct.
func OpenUserDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s?_busy_timeout=5000", dbPath)
	userDB, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}

	// Enable WAL mode for better concurrency.
	if _, err := userDB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		return nil, fmt.Errorf("failed to enable WAL mode on %s: %w", dbPath, err)
	}

	// Apply migrations to the user database
	if err := ApplyMigrations(userDB, UserDBMigrations); err != nil {
		return nil, fmt.Errorf("error applying user DB migrations: %w", err)
	}
	return userDB, nil
}
