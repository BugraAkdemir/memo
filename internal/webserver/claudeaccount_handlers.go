// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleClaudeAccountConnection serves the Settings → Claude Subscription flow
// for the claude-sub provider.
//
//   - GET  -> {connected, account, source, model}
//   - POST {"connect": true} -> signs in WITHOUT a browser when the machine
//     already holds a Claude Code login, returning {connected: true, source:
//     "claude-code-file"}. Otherwise it starts the hosted flow and returns
//     {connected: false, auth_url, state} for the user to open and paste back.
//   - POST {"code": "...", "state": "..."} -> completes the hosted flow.
//     "code" accepts either the bare code or the whole URL the browser ended on
//     (the state is then read out of it, so nothing else has to be copied).
//   - POST {"connect": false} -> clears the token, returns state.
//   - POST {"model": "claude-..."} -> switches the model, returns state.
//
// A separate file from devgateway_handlers.go on purpose: the claude-sub
// feature is meant to be self-contained (see internal/claudesub).
func (s *Server) handleClaudeAccountConnection(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	writeState := func() {
		connected, account, source, model := s.fullBridge.ClaudeAccountState()
		writeJSON(w, map[string]any{
			"connected": connected,
			"account":   account,
			"source":    source,
			"model":     model,
		})
	}

	switch r.Method {
	case http.MethodGet:
		writeState()

	case http.MethodPost:
		var body struct {
			Connect *bool  `json:"connect"`
			Code    string `json:"code"`
			State   string `json:"state"`
			Model   string `json:"model"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}

		// Model switch (no connect/disconnect intent).
		if body.Connect == nil && strings.TrimSpace(body.Model) != "" {
			if err := s.fullBridge.SetClaudeAccountModel(body.Model); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			writeState()
			return
		}

		// Completing the hosted flow: the user came back with a code. This is
		// checked before "connect" because a client that posts both (a retry
		// that resends the whole body) must complete rather than restart.
		if strings.TrimSpace(body.Code) != "" {
			if err := s.fullBridge.CompleteClaudeAuth(body.Code, body.State); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeState()
			return
		}

		if body.Connect != nil && *body.Connect {
			connected, source, authURL, state, err := s.fullBridge.ConnectClaudeAccount()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			if connected {
				_, account, _, model := s.fullBridge.ClaudeAccountState()
				writeJSON(w, map[string]any{
					"connected": true,
					"source":    source,
					"account":   account,
					"model":     model,
				})
				return
			}
			writeJSON(w, map[string]any{
				"connected": false,
				"source":    source,
				"auth_url":  authURL,
				"state":     state,
			})
			return
		}

		if err := s.fullBridge.DisconnectClaudeAccount(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeState()

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}
