package auth

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 3 * time.Hour

// Session struct to represent a session.
type Session struct {
	Token    string
	Username string
	Expiry   time.Time
}

// HashPassword hashes the given password using bcrypt.
func HashPassword(password string) string {
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Error hashing password: %v", err)
	}
	return string(hashedPassword)
}

// CheckPasswordHash compares a hashed password with a plain text password.
func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// CreateSession creates a session in the database
func CreateSession(db *sql.DB, token, username string) error {
	expiry := time.Now().Add(sessionDuration)
	_, err := db.Exec("INSERT INTO sessions (token, username, expiry) VALUES (?, ?, ?)", token, username, expiry)
	return err
}

// GetSession retrieves a session from the database
func GetSession(db *sql.DB, token string) (string, time.Time, error) {
	var username string
	var expiry time.Time
	err := db.QueryRow("SELECT username, expiry FROM sessions WHERE token = ?", token).Scan(&username, &expiry)
	if err != nil {
		//if the session does not exist, or if an error occurs
		return "", time.Time{}, err
	}

	if time.Now().After(expiry) {
		//if the session is expired
		return "", time.Time{}, fmt.Errorf("session expired")
	}
	return username, expiry, nil
}

// ExtendSession extends a session's duration
func ExtendSession(db *sql.DB, token string) error {
	newExpiry := time.Now().Add(sessionDuration)
	_, err := db.Exec("UPDATE sessions SET expiry = ? WHERE token = ?", newExpiry, token)
	if err != nil {
		log.Printf("Error extending session: %v", err)
	}
	return err
}

// DeleteSession deletes a session from the database
func DeleteSession(db *sql.DB, token string) {
	_, err := db.Exec("DELETE FROM sessions WHERE token = ?", token)
	if err != nil {
		log.Printf("Error deleting session: %v", err)
	}
}

// IsValidSession checks if a session is valid. Return username and validity.
func IsValidSession(db *sql.DB, r *http.Request) (string, bool) {
	cookie, err := r.Cookie("session_token")
	if err != nil {
		return "", false
	}

	sessionToken := cookie.Value
	username, expiry, err := GetSession(db, sessionToken)
	if err != nil {
		return "", false
	}

	// To avoid database contention, only extend the session if it's close to expiring.
	// For example, if it expires in the next hour.
	if time.Until(expiry) < 1*time.Hour {
		go ExtendSession(db, sessionToken)
	}

	return username, true
}

// GenerateSessionToken generates a random session token
func GenerateSessionToken() string {
	// Generate 32 random bytes for a secure token.
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		// This is a critical error, in a real app you might want to panic.
		// For now logging as fatal.
		log.Printf("FATAL: could not generate random bytes for session token: %v", err)
		return fmt.Sprintf("%d", time.Now().UnixNano()) // Fallback to less secure method
	}
	return hex.EncodeToString(b)
}

// SessionDuration is exported if needed elsewhere
func GetSessionDuration() time.Duration {
	return sessionDuration
}
