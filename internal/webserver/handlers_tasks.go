package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
)

// GET /api/tasks/running — live view of every executing task list.
func (s *Server) handleTasksRunning(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, s.fullBridge.ListRunningTasks())
}

// POST /api/tasks/{id}/{pause|resume|cancel|skip|inject}
func (s *Server) handleTaskControlByID(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusMethodNotAllowed)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	rest := strings.TrimPrefix(r.URL.Path, "/api/tasks/")
	parts := strings.SplitN(rest, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		http.Error(w, "expected /api/tasks/{id}/{action}", http.StatusBadRequest)
		return
	}
	id, action := parts[0], parts[1]

	switch action {
	case "pause":
		s.fullBridge.StopTaskList(id)
		writeJSON(w, map[string]string{"status": "paused"})
	case "resume":
		if err := s.fullBridge.StartTaskList(context.Background(), id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "running"})
	case "cancel":
		if err := s.fullBridge.CancelTaskList(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "cancelled"})
	case "skip":
		if err := s.fullBridge.SkipCurrentItem(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"status": "skipped"})
	case "inject":
		var body struct {
			Text string `json:"text"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
			http.Error(w, "text required", http.StatusBadRequest)
			return
		}
		reply, err := s.fullBridge.InjectTaskMessage(r.Context(), id, body.Text)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]string{"reply": reply})
	default:
		http.Error(w, "unknown action", http.StatusBadRequest)
	}
}
