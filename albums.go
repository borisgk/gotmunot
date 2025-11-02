// Albums related functions

package main

import "net/http"

func albumsHandler(w http.ResponseWriter, r *http.Request) {
	// Execute the "albums.html" template
	if err := tmpl.ExecuteTemplate(w, "albums.html", nil); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
