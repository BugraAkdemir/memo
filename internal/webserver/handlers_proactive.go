// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
)

// handleProactiveSettings gets (GET) or updates (POST) the proactive learning
// configuration.
func (s *Server) handleProactiveSettings(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.GetProactiveSettings())
	case http.MethodPost:
		var req struct {
			Enabled bool   `json:"enabled"`
			Level   string `json:"level"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetProactiveSettings(req.Enabled, req.Level); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s.fullBridge.GetProactiveSettings())
	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}

// handleProactivePending returns the in-flight suggestion for mobile polling.
// Responds with {"pending": null} when there is nothing to show.
func (s *Server) handleProactivePending(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	writeJSON(w, map[string]any{"pending": s.fullBridge.GetPendingSuggestion()})
}

// handleProactiveRespond records the user's answer to a pending suggestion.
func (s *Server) handleProactiveRespond(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID       string `json:"id"`
		Response string `json:"response"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.RespondToSuggestion(req.ID, req.Response); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleProactivePatterns lists the learned patterns (Learning Profile view).
func (s *Server) handleProactivePatterns(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	writeJSON(w, map[string]any{"patterns": s.fullBridge.ListLearnedPatterns()})
}

// handleProactiveForget deletes a single learned pattern by id.
func (s *Server) handleProactiveForget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.ForgetPattern(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}

// handleProactiveClear wipes all observations and learned patterns.
func (s *Server) handleProactiveClear(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost || s.fullBridge == nil {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if err := s.fullBridge.ClearLearningData(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]string{"ok": "true"})
}
