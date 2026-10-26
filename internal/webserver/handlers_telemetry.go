// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"net/http"

	"memo/internal/telemetry"
)

// The optional anonymous daily summary (docs/TELEMETRY.md): what Settings ›
// Privacy and the first-run choice screen talk to.
//
// Gates: reading the status and the preview is ambient (same as the usage
// stats the Settings tab already reads); answering, resetting the id and
// deleting the server-side data change the whole install, so those three are
// admin-only. The bridge is OPTIONAL (like agentChatNoActivate): a bridge that
// does not implement it answers 501 instead of forcing every test double to.

type telemetryBridge interface {
	TelemetryStatus() telemetry.Status
	SetTelemetryConsent(yes bool) error
	TelemetryPreview(ctx context.Context) (*telemetry.PreviewResult, error)
	TelemetryResetID(ctx context.Context) error
	TelemetryDelete(ctx context.Context) error
}

func (s *Server) telemetry(w http.ResponseWriter) (telemetryBridge, bool) {
	if s.fullBridge != nil {
		if tb, ok := s.fullBridge.(telemetryBridge); ok {
			return tb, true
		}
	}
	http.Error(w, "telemetry is not available", http.StatusNotImplemented)
	return nil, false
}

func (s *Server) handleTelemetryStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if tb, ok := s.telemetry(w); ok {
		writeJSON(w, tb.TelemetryStatus())
	}
}

// handleTelemetryPreview returns the report that would be sent next as the
// exact bytes (a string), so the screen shows what leaves the machine rather
// than a re-rendering of it.
func (s *Server) handleTelemetryPreview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	tb, ok := s.telemetry(w)
	if !ok {
		return
	}
	p, err := tb.TelemetryPreview(r.Context())
	if err != nil {
		http.Error(w, "preview failed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{"day": p.Day, "empty": p.Empty, "json": string(p.JSON)})
}

// handleTelemetryConsent records an explicit yes or no: {"share": true|false}.
// Anything else — a missing field, a string, extra fields — is refused, so an
// answer can never be assumed.
func (s *Server) handleTelemetryConsent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	tb, ok := s.telemetry(w)
	if !ok {
		return
	}
	var req struct {
		Share *bool `json:"share"`
	}
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<10))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil || req.Share == nil {
		http.Error(w, `body must be {"share": true} or {"share": false}`, http.StatusBadRequest)
		return
	}
	if dec.More() {
		http.Error(w, "unexpected extra data", http.StatusBadRequest)
		return
	}
	if err := tb.SetTelemetryConsent(*req.Share); err != nil {
		http.Error(w, "could not save the answer", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tb.TelemetryStatus())
}

func (s *Server) handleTelemetryResetID(w http.ResponseWriter, r *http.Request) {
	s.telemetryAction(w, r, func(tb telemetryBridge, ctx context.Context) error { return tb.TelemetryResetID(ctx) })
}

func (s *Server) handleTelemetryDelete(w http.ResponseWriter, r *http.Request) {
	s.telemetryAction(w, r, func(tb telemetryBridge, ctx context.Context) error { return tb.TelemetryDelete(ctx) })
}

func (s *Server) telemetryAction(w http.ResponseWriter, r *http.Request, do func(telemetryBridge, context.Context) error) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	tb, ok := s.telemetry(w)
	if !ok {
		return
	}
	if err := do(tb, r.Context()); err != nil {
		http.Error(w, "the request could not be completed", http.StatusInternalServerError)
		return
	}
	writeJSON(w, tb.TelemetryStatus())
}
