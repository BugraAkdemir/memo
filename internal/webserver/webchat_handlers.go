// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"net/http"
	"strings"

	"memo/internal/beta"
	"memo/internal/webchat"
)

// handleWebChat serves /api/webchat — the ChatGPT / claude.ai web chats
// (Beta). It talks to internal/webchat directly: that package keeps its own
// state and needs nothing from the App, so there is no bridge method to add.
//
// GET  → {beta, notice_version, sites:[{...Status, consented}]}
// POST → {action: consent|revoke|connect|disconnect, site, forget?}
//
// Registered adminWrites: GET is read by a settings screen every account can
// open and carries nothing secret; every action changes what Memo does with
// the person's accounts, so only an admin may take it.
func (s *Server) handleWebChat(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, webChatState(r))
	case http.MethodPost:
		var body struct {
			Action string `json:"action"`
			Site   string `json:"site"`
			Forget bool   `json:"forget"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		site := webchat.SiteByID(strings.ToLower(strings.TrimSpace(body.Site)))
		if site == nil {
			http.Error(w, "unknown site", http.StatusBadRequest)
			return
		}
		svc := webchat.For(site)
		switch strings.ToLower(strings.TrimSpace(body.Action)) {
		case "consent":
			// Confirming the notice is itself a Beta action.
			if !beta.Enabled() {
				http.Error(w, webchat.Message(webchat.KindDisabled, site), http.StatusConflict)
				return
			}
			if err := webchat.GrantConsent(site.ID); err != nil {
				http.Error(w, "could not save the confirmation", http.StatusInternalServerError)
				return
			}
		case "revoke":
			if err := webchat.RevokeConsent(site.ID); err != nil {
				http.Error(w, "could not remove the confirmation", http.StatusInternalServerError)
				return
			}
		case "connect":
			if err := webchat.Allowed(site); err != nil {
				http.Error(w, err.Error(), http.StatusConflict)
				return
			}
			svc.StartConnect()
		case "disconnect":
			if err := svc.Disconnect(body.Forget); err != nil {
				http.Error(w, "could not remove the saved sign-in", http.StatusInternalServerError)
				return
			}
		default:
			http.Error(w, "unknown action", http.StatusBadRequest)
			return
		}
		writeJSON(w, webChatState(r))
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func webChatState(r *http.Request) map[string]any {
	type siteState struct {
		webchat.Status
		Consented bool `json:"consented"`
	}
	var sites []siteState
	for _, site := range webchat.Sites() {
		sites = append(sites, siteState{Status: webchat.For(site).Status(r.Context()), Consented: webchat.Consented(site.ID)})
	}
	return map[string]any{"beta": beta.Enabled(), "notice_version": webchat.NoticeVersion, "sites": sites}
}
