// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
)

// handleGoogleAccountConnection serves the Developer screen's "connect
// Google account" flow for the gemini-sub subscription provider.
//
//   - GET  -> {connected, email}
//   - POST {"connect": true}  -> starts the OAuth loopback flow and returns
//     {"auth_url": "..."}; the caller opens it in a browser and then polls
//     GET until "connected" flips true (the connection is finalized in the
//     background once the user finishes authorizing).
//   - POST {"connect": false} -> revokes + clears the token, removes the
//     marker provider, returns {connected:false}.
//
// It is a separate file from devgateway_handlers.go on purpose: the whole
// gemini-sub feature is meant to be self-contained (see internal/geminisub).
func (s *Server) handleGoogleAccountConnection(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		connected, email := s.fullBridge.GoogleAccountState()
		writeJSON(w, map[string]any{"connected": connected, "email": email})

	case http.MethodPost:
		var body struct {
			Connect bool `json:"connect"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if body.Connect {
			authURL, err := s.fullBridge.StartGoogleAuth()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeJSON(w, map[string]any{"auth_url": authURL})
			return
		}
		if err := s.fullBridge.DisconnectGoogleAccount(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		connected, email := s.fullBridge.GoogleAccountState()
		writeJSON(w, map[string]any{"connected": connected, "email": email})

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}
