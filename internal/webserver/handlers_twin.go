package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"memo/internal/autonomy"
	"memo/internal/twin"
)

// Optional capability keeps smaller bridges independent of the twin subsystem.
type TwinBridge interface {
	CallTwin(context.Context, string, json.RawMessage) (any, error)
}

// twinOperations is the one list of owner operations served over HTTP. Routes
// and the handler both read it: they used to keep separate copies, and an
// operation present in the app but missing from either answered 404.
var twinOperations = []string{
	"state", "activity", "contact", "merge", "preferences", "reply_model",
	"cancel_action", "hold_action", "send_action", "approve_action",
	"contact_timeline", "check_availability", "git_project_status",
	"twin_commitments", "dev_status",
	"resolve_commitment", "task_from_action", "briefing_read",
}

func isTwinOperation(op string) bool {
	for _, known := range twinOperations {
		if op == known {
			return true
		}
	}
	return false
}

func (s *Server) handleTwin(w http.ResponseWriter, r *http.Request) {
	var bridge TwinBridge
	if b, ok := s.bridge.(TwinBridge); ok {
		bridge = b
	} else if b, ok := s.fullBridge.(TwinBridge); ok {
		bridge = b
	}
	if bridge == nil {
		writeMailJSON(w, 503, map[string]string{"error": "twin_unavailable"})
		return
	}
	op := strings.TrimPrefix(r.URL.Path, "/api/twin/")
	// Versioned routes normalize paths through Server's route wrapper.
	if strings.HasPrefix(r.URL.Path, "/api/v1/twin/") {
		op = strings.TrimPrefix(r.URL.Path, "/api/v1/twin/")
	}
	raw := json.RawMessage(`{}`)
	if op == "state" || op == "activity" {
		if r.Method != http.MethodGet {
			w.WriteHeader(405)
			return
		}
	} else {
		if r.Method != http.MethodPost {
			w.WriteHeader(405)
			return
		}
		b, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 32*1024))
		if err != nil || !json.Valid(b) {
			writeMailJSON(w, 400, map[string]string{"error": "twin_invalid"})
			return
		}
		raw = b
	}
	if !isTwinOperation(op) {
		w.WriteHeader(404)
		return
	}
	result, err := bridge.CallTwin(r.Context(), op, raw)
	if err != nil {
		status, code := 500, "twin_failed"
		if op == "send_action" || op == "approve_action" {
			code = "mail_reply_send_uncertain"
		}
		switch {
		case errors.Is(err, twin.ErrDeliveryConflict):
			status, code = 409, "mail_reply_not_pending"
		case errors.Is(err, twin.ErrSentHistory):
			status, code = 409, "mail_reply_sent_history"
		case errors.Is(err, twin.ErrDeliveryDenied):
			status, code = 403, "mail_reply_contact_denied"
		case errors.As(err, new(*autonomy.BlockedError)):
			status, code = 403, "mail_reply_blocked"
		case errors.Is(err, twin.ErrInvalid):
			status, code = 400, "twin_invalid"
		case errors.Is(err, twin.ErrAliasConflict):
			status, code = 409, "twin_alias_conflict"
		case errors.Is(err, twin.ErrNotFound):
			status, code = 404, "twin_not_found"
		case errors.Is(err, autonomy.ErrAutonomousCommandForbidden), errors.Is(err, autonomy.ErrContactScope):
			status, code = 403, "twin_denied"
		}
		writeMailJSON(w, status, map[string]string{"error": code})
		return
	}
	writeMailJSON(w, 200, result)
}
