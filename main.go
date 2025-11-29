// /home/ubuntu/go/src/tm25/main.go
package main

import (
	"fmt"
	"log"
	"net/http"

	"github.com/davidbyttow/govips/v2/vips"
)

// photoMetadataQueue is a channel that acts as a queue for saving photo metadata.
var photoMetadataQueue chan *PhotoMetadata

func main() {
	// Defer vips shutdown until the application exits.
	defer vips.Shutdown()

	// Initialize and start the metadata saving queue and worker.
	// A buffer of 100 can handle bursts of uploads.
	photoMetadataQueue = make(chan *PhotoMetadata, 100)
	go startMetadataSaveWorker()

	// Setup all routes
	setupRoutes()

	// Print a message indicating that the server is starting.
	fmt.Println("Starting TM25 Web Server on port 9030...")

	// Start the HTTP server on port 9030.
	// If there's an error starting the server, log.Fatal will log the error and exit.
	log.Fatal(http.ListenAndServe(":9030", nil))
}
