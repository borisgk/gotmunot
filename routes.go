package main

import (
	"net/http"
)

func setupRoutes() {
	// General
	http.HandleFunc("/", rootHandler)
	http.HandleFunc("/status", statusHandler)

	// Gallery & Albums
	http.HandleFunc("/gallery", galleryHandler)
	http.HandleFunc("/albums", albumsHandler)
	http.HandleFunc("/album/", albumDetailHandler)
	http.HandleFunc("/albums/new", newAlbumHandler)
	http.HandleFunc("/api/albums", createAlbumHandler)
	http.HandleFunc("/api/albums/list", getAlbumListHandler)
	http.HandleFunc("/api/albums/add-photos", addPhotosToAlbumHandler)
	http.HandleFunc("/api/album/", albumActionHandler)

	// Auth
	http.HandleFunc("/login", loginHandler)
	http.HandleFunc("/logout", logoutHandler)
	http.HandleFunc("/api/login", apiLoginHandler)

	// Settings
	http.HandleFunc("/settings", settingsHandler)

	// Upload
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/upload-page", uploadPageHandler)

	// Photos
	http.HandleFunc("/photo/info/", photoInfoHandler)
	http.HandleFunc("/api/photo/", photoActionHandler)
	http.HandleFunc("/api/photos/batch-update-date", startBatchUpdateDateHandler)
	http.HandleFunc("/api/photo/update-date", updatePhotoDateHandler)
	http.HandleFunc("/api/photos/delete", batchDeletePhotosHandler)

	// Downloads
	http.HandleFunc("/api/photos/download-previews", downloadPreviewsHandler)
	http.HandleFunc("/api/downloads/start", startDownloadHandler)
	http.HandleFunc("/api/downloads/status", getDownloadStatusHandler)
	http.HandleFunc("/api/downloads/cancel", cancelDownloadHandler)
	http.HandleFunc("/api/downloads/file", serveDownloadHandler)

	// Tasks
	http.HandleFunc("/api/tasks/status", getTaskStatusHandler)
	http.HandleFunc("/api/tasks/cancel", cancelTaskHandler)

	// Static
	http.Handle("/static/css/", http.StripPrefix("/static/css/", http.FileServer(http.Dir("static/css"))))
	http.Handle("/static/js/", http.StripPrefix("/static/js/", http.FileServer(http.Dir("static/js"))))
	http.Handle("/static/fonts/", http.StripPrefix("/static/fonts/", http.FileServer(http.Dir("static/fonts"))))
	http.Handle("/static/img/", http.StripPrefix("/static/img/", http.FileServer(http.Dir("static/img"))))
	http.HandleFunc("/logo.png", logoHandler)

	// Media
	http.HandleFunc("/media/", mediaHandler)
}
