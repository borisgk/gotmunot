package handlers

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"tm25/internal/auth"
	"tm25/internal/database"
)

func startBatchUpdateDateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Decode JSON body
	var payload struct {
		Filenames []string `json:"filenames"`
		StartDate string   `json:"start_date"` // Expecting "YYYY-MM-DDTHH:MM"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 3. Parse the start date
	startDate, err := time.Parse("2006-01-02T15:04", payload.StartDate)
	if err != nil {
		http.Error(w, "Invalid date format", http.StatusBadRequest)
		return
	}

	// 4. Generate a unique task ID and initialize progress
	taskID := fmt.Sprintf("batch-date-update-%d", time.Now().UnixNano())
	taskProgressMap.Lock()
	taskProgressMap.tasks[taskID] = &TaskProgress{Total: len(payload.Filenames)}
	taskProgressMap.Unlock()

	// 5. Start the background task
	go func(id, user string, filenames []string, startTime time.Time) {
		log.Printf("Starting batch date update for task %s", id)
		totalFiles := len(filenames)

		// Need userDB for the updates
		userDB, err := database.GetUserDB(db, user)
		if err != nil {
			log.Printf("Task %s: failed to get user DB: %v", id, err)
			updateTaskError(id, "Failed to access user database.")
			return
		}

		for i, filename := range filenames {
			// Check for cancellation
			taskProgressMap.RLock()
			if taskProgressMap.tasks[id].Cancelled {
				taskProgressMap.RUnlock()
				log.Printf("Task %s cancelled.", id)
				return
			}
			taskProgressMap.RUnlock()

			// Calculate the new date for this specific photo
			newDate := startTime.Add(time.Duration(i) * time.Minute)

			// Update progress before processing
			taskProgressMap.Lock()
			taskProgressMap.tasks[id].Processed = i + 1
			taskProgressMap.tasks[id].Filename = filename
			taskProgressMap.Unlock()

			// Perform the update
			// We need a way to call UpdatePhotoDateAndPath using the new database package.
			if err := database.UpdatePhotoDateAndPath(userDB, user, filename, newDate); err != nil {
				log.Printf("Task %s: failed to update date for %s: %v", id, filename, err)
				// Continue to the next file
			}
		}

		log.Printf("Batch date update complete for task %s", id)
		updateTaskComplete(id, totalFiles)
	}(taskID, username, payload.Filenames, startDate)

	// 6. Respond immediately with the task ID
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"task_id": taskID})
}

func updatePhotoDateHandler(w http.ResponseWriter, r *http.Request) {
	// 1. Authenticate user
	username, ok := auth.IsValidSession(db, r)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// 2. Ensure method is POST
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// 3. Decode JSON body
	var payload struct {
		Filename string `json:"filename"`
		NewDate  string `json:"new_date"` // Expecting "YYYY-MM-DDTHH:MM"
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 4. Parse the new date string
	newDate, err := time.Parse("2006-01-02T15:04", payload.NewDate)
	if err != nil {
		http.Error(w, "Invalid date format. Please use YYYY-MM-DDTHH:MM.", http.StatusBadRequest)
		return
	}

	userDB, err := database.GetUserDB(db, username)
	if err != nil {
		http.Error(w, "Could not access user database.", http.StatusInternalServerError)
		return
	}

	// 5. Call the core logic function
	err = database.UpdatePhotoDateAndPath(userDB, username, payload.Filename, newDate)
	if err != nil {
		if err.Error() == "forbidden" {
			http.Error(w, "Forbidden", http.StatusForbidden)
		} else if err == sql.ErrNoRows {
			http.Error(w, "Photo not found", http.StatusNotFound)
		} else {
			log.Printf("Error updating photo date for %s: %v", payload.Filename, err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
		}
		return
	}

	// 6. Respond with success
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}
