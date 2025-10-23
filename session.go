package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"
)

const sessionDuration = 3 * time.Hour

// Session struct to represent a session.
type Session struct {
	Token    string
	Username string
	Expiry   time.Time
}

// createSession creates a session in the database
func createSession(db *sql.DB, token, username string) error {
	expiry := time.Now().Add(sessionDuration)
	_, err := db.Exec("INSERT INTO sessions (token, username, expiry) VALUES (?, ?, ?)", token, username, expiry)
	return err
}

// getSession retrieves a session from the database
func getSession(db *sql.DB, token string) (string, error) {
	var username string
	var expiry time.Time
	err := db.QueryRow("SELECT username, expiry FROM sessions WHERE token = ?", token).Scan(&username, &expiry)
	if err != nil {
		//if the session does not exist, or if an error occurs
		return "", err
	}

	if time.Now().After(expiry) {
		//if the session is expired
		deleteSession(db, token)
		return "", fmt.Errorf("session expired")
	}

	return username, nil
}

// deleteSession deletes a session from the database
func deleteSession(db *sql.DB, token string) {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		log.Printf("Error deleting session: %v", err)
	}
}

// isValidSession checks if a session is valid
func isValidSession(db *sql.DB, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return "", false
	}

	sessionToken := cookie.Value
	username, err := getSession(db, sessionToken)
	if err != nil {
		return "", false
	}

	return username, true
}

// Generate a random session token
func generateSessionToken() string {
	// In a real application, use a cryptographically secure random number generator.
	return fmt.Sprintf("%d", time.Now().UnixNano())
}
