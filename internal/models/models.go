package models

import "time"

// User struct to represent a user.
type User struct {
	ID       int
	UUID     string
	Username string
	Password string // Hash!
	DBPath   string
}

// Album represents a collection of photos.
type Album struct {
	ID          int
	Name        string
	Description string
	PhotoCount  int
	CoverPhoto  string // URL to the cover photo
	// The CreatedAt field is part of the Album struct in the database, but not used in the JSON response for the list view.
	CreatedAt time.Time
}

// AlbumListItem is a lightweight struct for album lists.
type AlbumListItem struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// PhotoMetadata struct to represent photo metadata.
type PhotoMetadata struct {
	ID            int
	Filename      string
	Filepath      string
	UploadedBy    string
	UploadedAt    time.Time
	ImageWidth    int64
	ImageLength   int64
	DateTime      time.Time
	ThumbWidth    int
	ThumbHeight   int
	PreviewWidth  int
	PreviewHeight int

	// Fields populated at runtime, not stored in DB.
	ThumbPath   string
	PreviewPath string
}

// DayGroup is a struct to hold photos grouped by a specific date.
type DayGroup struct {
	Date   time.Time
	Photos []PhotoMetadata
	Count  int
}
