package handlers

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"time"

	"tm25/internal/auth"
	"tm25/internal/models"
)

type loginPageData struct {
	RedirectURL string
	Error       string
}

func loginHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// Display the login form
		redirectURL := r.URL.Query().Get("redirect_url")
		data := loginPageData{RedirectURL: redirectURL}
		err := tmpl.ExecuteTemplate(w, "login.html", data)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	case http.MethodPost:
		// Handle login submission
		r.ParseForm()
		username := r.FormValue("username")
		password := r.FormValue("password")
		redirectURL := r.FormValue("redirect_url")

		renderLoginWithError := func(msg string) {
			w.WriteHeader(http.StatusUnauthorized)
			data := loginPageData{
				RedirectURL: redirectURL,
				Error:       msg,
			}
			tmpl.ExecuteTemplate(w, "login.html", data)
		}

		// Retrieve user from the database.
		var user models.User
		err := db.QueryRow("SELECT id, uuid, username, password, db_path FROM users WHERE username = ?", username).Scan(&user.ID, &user.UUID, &user.Username, &user.Password, &user.DBPath)
		if err != nil {
			if err == sql.ErrNoRows {
				renderLoginWithError("Invalid username or password - please try again")
			} else {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
			}
			return
		}

		// Check the password.
		if !auth.CheckPasswordHash(password, user.Password) {
			renderLoginWithError("Invalid username or password - please try again")
			return
		}

		// Create a session
		sessionToken := auth.GenerateSessionToken()
		err = auth.CreateSession(db, sessionToken, user.Username)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		// Set a session cookie
		http.SetCookie(w, &http.Cookie{
			Name:     "session_token",
			Value:    sessionToken,
			Expires:  time.Now().Add(auth.GetSessionDuration()),
			Path:     "/",
			HttpOnly: true, // Important for security
		})

		// After successful login, check if there's a redirect URL.
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
	var user models.User
	err := db.QueryRow("SELECT id, uuid, username, password, db_path FROM users WHERE username = ?", creds.Username).Scan(&user.ID, &user.UUID, &user.Username, &user.Password, &user.DBPath)
	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Check the password.
	if !auth.CheckPasswordHash(creds.Password, user.Password) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	// Create a session
	sessionToken := auth.GenerateSessionToken()
	err = auth.CreateSession(db, sessionToken, user.Username)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set a session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    sessionToken,
		Expires:  time.Now().Add(auth.GetSessionDuration()),
		Path:     "/", // Set cookie for the whole site
		HttpOnly: true,
	})

	w.WriteHeader(http.StatusOK)
}

func logoutHandler(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("session_token")
	if err == nil {
		auth.DeleteSession(db, cookie.Value)
		//expire the cookie by setting the expiration in the past
		cookie.Expires = time.Now().AddDate(0, 0, -1)
		http.SetCookie(w, cookie)
	}
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
