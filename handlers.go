package main

import (
	"fmt"
	"net/http"
)

func rootHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/gallery", http.StatusSeeOther)
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "TM25 is running!")
}

func logoHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "logo.png")
}
