package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User struct to represent a user.
type User struct {
	ID       int
	UUID     string
	Username string
	Password string // Hash!
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

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Display the login form
		redirectURL := r.URL.Query().Get("redirect_url")
		data := struct {
			RedirectURL string
		}{RedirectURL: redirectURL}
		err := tmpl.ExecuteTemplate(w, "login.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		// Handle login submission
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")

		// Retrieve user from the database.
		var user User
		err := db.QueryRow("SELECT id, uuid, username, password FROM users WHERE username = ?", username).Scan(&user.ID, &user.UUID, &user.Username, &user.Password)
		if err != nil {
			if err == sql.ErrNoRows {
				http.Error(w, "Invalid username or password - please try again", http.StatusUnauthorized)
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check the password.
		if !checkPasswordHash(password, user.Password) {
			http.Error(w, "Invalid username or password", http.StatusUnauthorized)
			return
		}

		// Create a session
		sessionToken := generateSessionToken()
		err = createSession(db, sessionToken, user.Username)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set a session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  time.Now().Add(sessionDuration),
			HttpOnly: true, // Important for security
		})

		// After successful login, check if there's a redirect URL.
		redirectURL := r.FormValue("redirect_url")
		if redirectURL != "" {
			http.Redirect(w, r, redirectURL, http.StatusSeeOther)
		} else {
			// Otherwise, redirect to the default gallery page.
			http.Redirect(w, r, "/gallery", http.StatusSeeOther)
		}

	default:
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
	}
}

func apiLoginHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	var creds struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Retrieve user from the database.
	var user User
	err := db.QueryRow("SELECT id, uuid, username, password FROM users WHERE username = ?", creds.Username).Scan(&user.ID, &user.UUID, &user.Username, &user.Password)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check the password.
	if !checkPasswordHash(creds.Password, user.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create a session
	sessionToken := generateSessionToken()
	err = createSession(db, sessionToken, user.Username)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(sessionDuration),
		Path:     "/", // Set cookie for the whole site
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		deleteSession(db, cookie.Value)
		//expire the cookie by setting the expiration in the past
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(w, cookie)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}