// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"memo/internal/routine"
)

// RoutineBridge is the subset of App methods required by routine handlers,
// mirroring CalendarBridge's independent-interface pattern in
// handlers_calendar.go.
type RoutineBridge interface {
	ListRoutines() []routine.Routine
	GetRoutine(id string) (*routine.Routine, error)
	ParseRoutineText(ctx context.Context, text string) (routine.Draft, error)
	CreateRoutineFromDraft(originalText string, d routine.Draft, whatsAppTargetJID string, autoApproveTools bool, language string, utcOffsetMinutes *int) (*routine.Routine, error)
	UpdateRoutine(r routine.Routine) (*routine.Routine, error)
	DeleteRoutine(id string) error
	SyncRoutineUTCOffsets(minutes int) (int, error)
}

// handleRoutines handles GET /api/routines (list) and POST /api/routines (create).
func (s *Server) handleRoutines(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(RoutineBridge)
	if !ok {
		http.Error(w, "routines not available", http.StatusNotImplemented)
		return
	}

	switch r.Method {
	case http.MethodGet:
		list := bridge.ListRoutines()
		if list == nil {
			list = []routine.Routine{}
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(list)

	case http.MethodPost:
		var body struct {
			OriginalText      string        `json:"original_text"`
			Draft             routine.Draft `json:"draft"`
			WhatsAppTargetJID string        `json:"whatsapp_target_jid"`
			AutoApproveTools  bool          `json:"auto_approve_tools"`
			// Language is the client's current UI locale ("tr"/"en") at the
			// moment of creation — see routine.Routine.Language's doc comment
			// (BUG-M1). Optional: an old client that never sends it leaves the
			// routine defaulting to Turkish everywhere it's read.
			Language string `json:"language"`
			// UTCOffsetMinutes is the client's current UTC offset in minutes at
			// the moment of creation — see Schedule.UTCOffsetMinutes's doc
			// comment (BUG-M4). Optional: an old client that never sends it
			// leaves the routine falling back to the backend host's own local
			// time, exactly matching pre-fix behavior.
			UTCOffsetMinutes *int `json:"utc_offset_minutes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		created, err := bridge.CreateRoutineFromDraft(body.OriginalText, body.Draft, body.WhatsAppTargetJID, body.AutoApproveTools, body.Language, body.UTCOffsetMinutes)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(created)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleParseRoutine handles POST /api/routines/parse — turns free text into
// a Draft for the confirmation-card UI step.
func (s *Server) handleParseRoutine(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(RoutineBridge)
	if !ok {
		http.Error(w, "routines not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil || strings.TrimSpace(body.Text) == "" {
		jsonError(w, "text is required", http.StatusBadRequest)
		return
	}

	draft, err := bridge.ParseRoutineText(r.Context(), body.Text)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(draft)
}

// handleRoutine handles PUT /api/routines/{id} and DELETE /api/routines/{id}.
func (s *Server) handleRoutine(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(RoutineBridge)
	if !ok {
		http.Error(w, "routines not available", http.StatusNotImplemented)
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/routines/")
	if id == "" {
		jsonError(w, "routine id required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodPut:
		// Decode onto a copy of the existing stored routine rather than a
		// fresh zero-value struct — encoding/json's Decode only overwrites
		// fields actually present in the request body, so any field a caller
		// omits (e.g. a client whose model only round-trips a subset of
		// fields, like a bare enable/disable toggle) keeps its current
		// persisted value instead of silently zeroing (BUG-C2: this used to
		// wipe weekdays/context_source/auto_approve_tools/whatsapp_target_jid
		// on every update that didn't explicitly resend them).
		existing, err := bridge.GetRoutine(id)
		if err != nil {
			jsonError(w, err.Error(), http.StatusNotFound)
			return
		}
		rt := *existing
		if err := json.NewDecoder(r.Body).Decode(&rt); err != nil {
			jsonError(w, "invalid request body", http.StatusBadRequest)
			return
		}
		rt.ID = id
		updated, err := bridge.UpdateRoutine(rt)
		if err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(updated)

	case http.MethodDelete:
		if err := bridge.DeleteRoutine(id); err != nil {
			jsonError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleRoutinesSyncOffset handles POST /api/routines/sync-offset — a client
// reports its current wall-clock UTC offset every time it (re)connects (see
// connectionStatusProvider in chat_provider.dart), and every routine's
// Schedule.UTCOffsetMinutes is updated to match (routine.Store.SyncUTCOffset).
// This is what lets a DST transition, or a permanent relocation, self-correct
// the next time the client talks to the backend, rather than staying frozen
// at whatever offset was in effect the moment each routine was created
// (BUG_REPORT TD-1).
func (s *Server) handleRoutinesSyncOffset(w http.ResponseWriter, r *http.Request) {
	bridge, ok := s.bridge.(RoutineBridge)
	if !ok {
		http.Error(w, "routines not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body struct {
		UTCOffsetMinutes int `json:"utc_offset_minutes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		jsonError(w, "invalid request body", http.StatusBadRequest)
		return
	}

	changed, err := bridge.SyncRoutineUTCOffsets(body.UTCOffsetMinutes)
	if err != nil {
		jsonError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]int{"changed": changed})
}
