// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
)

// MoodBridge is the subset of App methods required by mood handlers.
type MoodBridge interface {
	GetMoodScore() float64
	UpdateMoodConfig(enabled bool) error
	GetSelfInterestEnabled() bool
	UpdateSelfInterestConfig(enabled bool) error
	GetSystemManagementEnabled() bool
	UpdateSystemManagementConfig(enabled bool) error
}

// handleMoodScore handles GET /api/mood/score
func (s *Server) handleMoodScore(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(MoodBridge)
	if !ok {
		http.Error(w, "mood not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]any{
		"score": bridge.GetMoodScore(),
	})
}

// handleMoodSettings handles GET/PUT /api/mood/settings
func (s *Server) handleMoodSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(MoodBridge)
	if !ok {
		http.Error(w, "mood not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"score":         bridge.GetMoodScore(),
			"enabled":       true,
			"self_interest": bridge.GetSelfInterestEnabled(),
		})
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := bridge.UpdateMoodConfig(body.Enabled); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"score":         bridge.GetMoodScore(),
			"enabled":       body.Enabled,
			"self_interest": bridge.GetSelfInterestEnabled(),
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSystemManagementSettings handles GET/PUT /api/mood/system-management
func (s *Server) handleSystemManagementSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(MoodBridge)
	if !ok {
		http.Error(w, "mood not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{"enabled": bridge.GetSystemManagementEnabled()})
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := bridge.UpdateSystemManagementConfig(body.Enabled); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"enabled": body.Enabled})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleSelfInterestSettings handles GET/PUT /api/mood/self-interest
func (s *Server) handleSelfInterestSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(MoodBridge)
	if !ok {
		http.Error(w, "mood not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, map[string]any{
			"enabled": bridge.GetSelfInterestEnabled(),
		})
	case http.MethodPut:
		var body struct {
			Enabled bool `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := bridge.UpdateSelfInterestConfig(body.Enabled); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{
			"enabled": body.Enabled,
		})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}
