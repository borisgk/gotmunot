package handlers

import (
	"net/http"

	"tm25/internal/auth"
)

func settingsHandler(w http.ResponseWriter, r *http.Request) {
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Redirect(w, r, "/login?redirect_url=/settings", http.StatusSeeOther)
		return
	}

	data := struct {
		Username    string
		CurrentPage string
	}{
		Username:    username,
		CurrentPage: "settings",
	}
	if err := tmpl.ExecuteTemplate(w, "settings.html", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
