// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleGoogleAccountConnection serves the Settings → Gemini Subscription
// flow for the gemini-sub subscription provider.
//
//   - GET  -> {connected, email, model}
//   - POST {"connect": true}  -> starts the OAuth loopback flow and returns
//     {"auth_url": "..."}; the caller opens it in a browser and then polls
//     GET until "connected" flips true.
//   - POST {"connect": false} -> revokes + clears the token, returns state.
//   - POST {"model": "gemini-..."} -> switches the model the marker provider
//     (and Memo's own chat, when active) uses, returns state.
//
// It is a separate file from devgateway_handlers.go on purpose: the whole
// gemini-sub feature is meant to be self-contained (see internal/geminisub).
func (s *Server) handleGoogleAccountConnection(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	writeState := func() {
		connected, email, model := s.fullBridge.GoogleAccountState()
		writeJSON(w, map[string]any{"connected": connected, "email": email, "model": model})
	}

	switch r.Method {
	case http.MethodGet:
		writeState()

	case http.MethodPost:
		var body struct {
			Connect *bool  `json:"connect"`
			Model   string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		// Model switch (no connect/disconnect intent).
		if body.Connect == nil && strings.TrimSpace(body.Model) != "" {
			if err := s.fullBridge.SetGoogleAccountModel(body.Model); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeState()
			return
		}

		connect := body.Connect != nil && *body.Connect
		if connect {
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
		writeState()

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}
