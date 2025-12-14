package handlers

import (
	"log"

	"tm25/internal/database"
	"tm25/internal/models"
)

// photoMetadataQueue is a channel that acts as a queue for saving photo metadata.
var photoMetadataQueue = make(chan *models.PhotoMetadata, 100)

// StartMetadataSaveWorker is a long-running goroutine that processes metadata
// save requests from a channel, serializing all database writes.
func StartMetadataSaveWorker() {
	log.Println("Starting metadata save worker...")
	for photoData := range photoMetadataQueue {
		// userDB is needed. We need to get it from the manager.
		// photoData has UploadedBy.
		userDB, err := database.GetUserDB(db, photoData.UploadedBy)
		if err != nil {
			log.Printf("BACKGROUND_ERROR: Failed to get user DB for %s: %v", photoData.UploadedBy, err)
			continue
		}

		photoID, err := database.SavePhotoMetadata(userDB, photoData)
		if err != nil {
			log.Printf("BACKGROUND_ERROR: Failed to save metadata for %s: %v", photoData.Filename, err)
		} else {
			log.Printf("Metadata for %s saved successfully from queue with ID %d.", photoData.Filename, photoID)
		}
	}
}
