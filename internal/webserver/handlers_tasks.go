package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
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

// GET /api/tasks/events — Server-Sent Events stream of live Self-Driving
// task-loop events (planning / executing / step_done / item_done / paused /
// escalation / finished), each an enriched JSON line carrying the list's
// chat_id and current step/item progress. The chat UI filters by chat_id so a
// task's progress shows up in the chat that started it.
func (s *Server) handleTasksEvents(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotFound)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Transfer-Encoding", "chunked")

	ch, unsubscribe := s.fullBridge.SubscribeTaskEvents()
	defer unsubscribe()

	// Prime the client with the current state of anything already running.
	for _, line := range s.fullBridge.RunningTaskEventSnapshot() {
		fmt.Fprintf(w, "data: %s\n\n", line)
	}
	flusher.Flush()

	ctx := r.Context()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "data: %s\n\n", line)
			flusher.Flush()
		case <-keepalive.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
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
