// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// handleSubscriptions serves Settings -> Subscriptions, the bundled
// CLIProxyAPI sidecar that fronts Antigravity / Claude / Codex accounts.
//
//   - GET  -> models.SubscriptionsState (bundled?, running?, accounts, models,
//     the sign-in in flight). Side-effect free: it never starts the sidecar.
//   - POST {"action":"login","provider":"antigravity"} -> {"auth_url": "..."};
//     the sidecar opens the browser itself, the UI shows the link as a
//     fallback and polls GET until the account appears.
//   - POST {"action":"cancel_login"} / {"action":"logout","provider":"..."}
//     -> the new state.
//
// Wrapped in adminWrites at the route: signing an account in or out is an
// admin action, while the read stays available to a restricted account's
// always-built settings screen.
func (s *Server) handleSubscriptions(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.SubscriptionsState(r.Context()))

	case http.MethodPost:
		var body struct {
			Action   string `json:"action"`
			Provider string `json:"provider"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		switch strings.ToLower(strings.TrimSpace(body.Action)) {
		case "login":
			url, err := s.fullBridge.StartSubscriptionLogin(strings.ToLower(strings.TrimSpace(body.Provider)))
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, map[string]any{"auth_url": url})
		case "cancel_login":
			s.fullBridge.CancelSubscriptionLogin()
			writeJSON(w, s.fullBridge.SubscriptionsState(r.Context()))
		case "logout":
			if err := s.fullBridge.LogoutSubscription(strings.ToLower(strings.TrimSpace(body.Provider))); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			writeJSON(w, s.fullBridge.SubscriptionsState(r.Context()))
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
		}

	default:
		http.Error(w, "GET or POST only", http.StatusMethodNotAllowed)
	}
}
