package main

import (
	"encoding/json"
	"net/http"
	"sync"
)

// TaskProgress holds the state of a long-running task.
type TaskProgress struct {
	Processed           int      `json:"processed,omitempty"`
	Total               int      `json:"total,omitempty"`
	Filename            string   `json:"filename,omitempty"`
	Complete            bool     `json:"complete"`
	Error               string   `json:"error,omitempty"`
	Cancelled           bool     `json:"cancelled,omitempty"`
	DownloadURL         string   `json:"download_url,omitempty"`
	GeneratedThumbnails []string `json:"generated_thumbnails,omitempty"`
}

// taskProgressMap safely stores the progress of multiple concurrent tasks.
var taskProgressMap = struct {
	sync.RWMutex
	tasks map[string]*TaskProgress
}{tasks: make(map[string]*TaskProgress)}

func getTaskStatusHandler(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	taskProgressMap.RLock()
	progress, ok := taskProgressMap.tasks[taskID]
	taskProgressMap.RUnlock()

	if !ok {
		http.Error(w, "Task not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(progress)
}

func cancelTaskHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "Missing task ID", http.StatusBadRequest)
		return
	}

	updateTaskCancelled(taskID)
	w.WriteHeader(http.StatusOK)
}

func updateTaskCancelled(taskID string) {
	taskProgressMap.Lock()
	defer taskProgressMap.Unlock()
	if task, ok := taskProgressMap.tasks[taskID]; ok {
		task.Cancelled = true
		task.Complete = true // Mark as complete to stop polling
		task.Error = "Task cancelled by user."
	}
}

func updateTaskComplete(taskID string, total int) {
	taskProgressMap.Lock()
	defer taskProgressMap.Unlock()
	if task, ok := taskProgressMap.tasks[taskID]; ok {
		if !task.Cancelled {
			task.Processed = total
			task.Complete = true
		}
	}
}

func updateTaskError(taskID, message string) {
	taskProgressMap.Lock()
	defer taskProgressMap.Unlock()
	if task, ok := taskProgressMap.tasks[taskID]; ok {
		task.Error = message
		task.Complete = true
	}
}
