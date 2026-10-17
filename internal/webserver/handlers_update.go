// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"errors"
	"net/http"

	"memo/internal/updater"
)

// GET /api/update/check[?force=1] — is there a newer Memo, and engine updates.
func (s *Server) handleUpdateCheck(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	info, err := s.fullBridge.CheckForUpdate(r.Context(), r.URL.Query().Get("force") == "1")
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadGateway)
		return
	}
	writeJSON(w, info)
}

// GET /api/update/status — progress of the running update.
func (s *Server) handleUpdateStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	writeJSON(w, s.fullBridge.UpdateStatus())
}

// POST /api/update/apply {"kind":"app"} or {"kind":"component","component":"llama"}
// starts an update and returns at once (202); poll /api/update/status.
func (s *Server) handleUpdateApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	var req struct {
		Kind      string `json:"kind"`
		Component string `json:"component"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 4096)).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.ApplyUpdate(req.Kind, req.Component); err != nil {
		code := http.StatusBadRequest
		if errors.Is(err, updater.ErrBusy) {
			code = http.StatusConflict
		}
		http.Error(w, err.Error(), code)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	writeJSON(w, map[string]string{"status": "started"})
}
