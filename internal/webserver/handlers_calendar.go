// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"memo/internal/calendar"
	"memo/internal/config"
)

// CalendarBridge is the subset of App methods required by calendar handlers.
type CalendarBridge interface {
	ListCalendarEvents(from, to time.Time) ([]calendar.Event, error)
	AddCalendarEvent(title string, startTime time.Time, description string) (calendar.Event, error)
	DeleteCalendarEvent(id string) error
	GetCalendarSettings() config.CalendarConfig
	UpdateCalendarSettings(leadMinutes int) error
	GetLearningSettings() config.LearningConfig
	UpdateLearningSettings(singleModelEnabled bool, modelID string) error
}

// handleCalendarEvents handles GET /api/calendar/events and POST /api/calendar/events.
func (s *Server) handleCalendarEvents(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(CalendarBridge)
	if !ok {
		http.Error(w, "calendar not available", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		fromStr := r.URL.Query().Get("from")
		toStr := r.URL.Query().Get("to")

		from := time.Now().Truncate(24 * time.Hour)
		to := from.Add(30 * 24 * time.Hour)

		if fromStr != "" {
			if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
				from = t
			}
		}
		if toStr != "" {
			if t, err := time.Parse(time.RFC3339, toStr); err == nil {
				to = t
			}
		}

		events, err := bridge.ListCalendarEvents(from, to)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if events == nil {
			events = []calendar.Event{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(events)

	case http.MethodPost:
		var body struct {
			Title       string `json:"title"`
			StartTime   string `json:"start_time"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if body.Title == "" {
			jsonError(w, "title is required", http.StatusBadRequest)
			return
		}
		startTime, err := time.Parse(time.RFC3339, body.StartTime)
		if err != nil {
			jsonError(w, "invalid start_time: use RFC3339", http.StatusBadRequest)
			return
		}
		event, err := bridge.AddCalendarEvent(body.Title, startTime, body.Description)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(event)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCalendarEvent handles DELETE /api/calendar/events/{id}.
func (s *Server) handleCalendarEvent(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(CalendarBridge)
	if !ok {
		http.Error(w, "calendar not available", http.StatusNotImplemented)
		return
	}

	if r.Method != http.MethodDelete {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/calendar/events/")
	if id == "" {
		jsonError(w, "event id required", http.StatusBadRequest)
		return
	}

	if err := bridge.DeleteCalendarEvent(id); err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// handleCalendarSettings handles GET/PUT /api/calendar/settings.
func (s *Server) handleCalendarSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(CalendarBridge)
	if !ok {
		http.Error(w, "calendar not available", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bridge.GetCalendarSettings())

	case http.MethodPut:
		var body struct {
			ReminderLeadMinutes int `json:"reminder_lead_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := bridge.UpdateCalendarSettings(body.ReminderLeadMinutes); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bridge.GetCalendarSettings())

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLearningSettings handles GET/PUT /api/learning/settings.
func (s *Server) handleLearningSettings(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(CalendarBridge)
	if !ok {
		http.Error(w, "learning not available", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bridge.GetLearningSettings())

	case http.MethodPut:
		var body struct {
			SingleModelEnabled bool   `json:"single_model_enabled"`
			ModelID            string `json:"model_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if err := bridge.UpdateLearningSettings(body.SingleModelEnabled, body.ModelID); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bridge.GetLearningSettings())

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
