// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"memo/internal/models"
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
//   - POST {"action":"submit_callback","callback_url":"http://localhost:1455/
//     auth/callback?code=…&state=…"} -> the new state. What makes a sign-in
//     started on a remote server finishable from a browser on another
//     machine; see cliproxy.Manager.SubmitCallbackURL.
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
		case "submit_callback":
			// The sign-in's authorization code, which is a credential until it
			// is exchanged: admin-only, like the login that produced it.
			var cb struct {
				CallbackURL string `json:"callback_url"`
			}
			if err := json.NewDecoder(r.Body).Decode(&cb); err != nil {
				http.Error(w, "bad json", http.StatusBadRequest)
				return
			}
			if err := s.fullBridge.SubmitSubscriptionCallback(cb.CallbackURL); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
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

// handleProviderModel is the model selector's endpoint (chat top bar, /model).
//
//   - GET ?name=<provider> -> {"models":[{id,owned_by}], "current": "<model>"}
//     the provider's live model list; the stored key is used on the server and
//     never returned.
//   - PUT {"name","model","activate"} -> switches that provider's model,
//     rewriting only Model (every other setting survives); with activate it
//     also makes the provider the active one, so a selector needs one call.
//
// Gated on the models permission at the route: unlike /api/providers/models
// (which takes a key from the caller), this one spends a STORED key.
func (s *Server) handleProviderModel(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "bridge not available", http.StatusServiceUnavailable)
		return
	}
	switch r.Method {
	case http.MethodGet:
		name := strings.TrimSpace(r.URL.Query().Get("name"))
		if name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		ctx := r.Context()
		// ?fresh=1 is the picker's background refresh: it may wait a moment for
		// current quota figures. A plain open answers from the cache at once.
		if r.URL.Query().Get("fresh") == "1" {
			ctx = models.WithQuotaWait(ctx, 3*time.Second)
		}
		list, current, err := s.fullBridge.ListProviderModels(ctx, name)
		if err != nil {
			writeJSON(w, map[string]any{"models": []models.ProviderModel{}, "current": current, "error": err.Error()})
			return
		}
		if list == nil {
			list = []models.ProviderModel{}
		}
		writeJSON(w, map[string]any{"models": list, "current": current})

	case http.MethodPut:
		var body struct {
			Name     string `json:"name"`
			Model    string `json:"model"`
			Activate bool   `json:"activate"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if strings.TrimSpace(body.Name) == "" || strings.TrimSpace(body.Model) == "" {
			http.Error(w, "name and model are required", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.SetProviderModel(body.Name, body.Model); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if body.Activate {
			s.fullBridge.SetActiveProvider(body.Name)
		}
		writeJSON(w, map[string]any{"ok": true})

	default:
		http.Error(w, "GET or PUT", http.StatusMethodNotAllowed)
	}
}
