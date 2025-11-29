package main

import (
	"log"
)

// startMetadataSaveWorker is a long-running goroutine that processes metadata
// save requests from a channel, serializing all database writes.
func startMetadataSaveWorker() {
	log.Println("Starting metadata save worker...")
	for photoData := range photoMetadataQueue {
		photoID, err := savePhotoMetadata(photoData)
		if err != nil {
			log.Printf("BACKGROUND_ERROR: Failed to save metadata for %s: %v", photoData.Filename, err)
		} else {
			log.Printf("Metadata for %s saved successfully from queue with ID %d.", photoData.Filename, photoID)
		}
	}
}
