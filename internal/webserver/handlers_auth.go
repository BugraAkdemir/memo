// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// isRemoteLoginPath reports whether path is the password-login endpoint
// (including the /api/v1/ alias) — exempt from remoteAuthMiddleware's
// credential check for the same reason isSwarmWorkerAddPath is: you need to
// call this endpoint precisely because you don't have a credential yet.
func isRemoteLoginPath(path string) bool {
	return path == "/api/auth/login" || path == "/api/v1/auth/login"
}

// requestIP returns r's connecting IP with any ephemeral port stripped —
// used only to key the brute-force limiter, matching rateLimitMiddleware's
// own IP-extraction logic.
func requestIP(r *http.Request) string {
	if host, _, err := net.SplitHostPort(r.RemoteAddr); err == nil {
		return host
	}
	return r.RemoteAddr
}

// handleRemoteLogin implements POST /api/auth/login for "password"/
// "token_password" auth modes — the one endpoint reachable without any
// credential at all (see isRemoteLoginPath). On success it returns a
// session token the client then presents as X-Memo-Token/Bearer on every
// subsequent request, validated by ValidateRemoteSession alongside (or
// instead of, depending on mode) a device token.
func (s *Server) handleRemoteLogin(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	token, role, err := s.fullBridge.LoginRemotePassword(requestIP(r), req.Username, req.Password)
	if err != nil {
		if seconds, locked := asLockedError(err); locked {
			w.Header().Set("Retry-After", strconv.Itoa(int(seconds)+1))
			http.Error(w, "too many failed attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{"session_token": token, "role": role})
}

// asLockedError extracts a lockout duration (in seconds) from err if it
// carries a positive one, without this package needing to import
// internal/app's unexported remoteLoginError type — errors.As matches
// structurally on the exact method signature below, which remoteLoginError
// happens to implement. remoteLoginError.LockedFor() also returns 0 for a
// plain bad-credentials rejection (same concrete type either way), so a
// positive duration — not just a successful type match — is what actually
// distinguishes a lockout from an ordinary failure here.
func asLockedError(err error) (seconds float64, ok bool) {
	var le interface{ LockedFor() time.Duration }
	if errors.As(err, &le) {
		if d := le.LockedFor(); d > 0 {
			return d.Seconds(), true
		}
	}
	return 0, false
}

// handleRemoteDevices implements GET (list) and POST (create) on
// /api/remote-access/devices. Behind remoteAuthMiddleware like everything
// else under /api/ — managing devices requires already being authenticated
// by some means (unlike the login endpoint above).
func (s *Server) handleRemoteDevices(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.ListRemoteDevices())
	case http.MethodPost:
		if !s.callerIsAdmin(r) {
			http.Error(w, "forbidden: admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		token, err := s.fullBridge.CreateRemoteDevice(req.Name)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		// The plaintext token is returned here and only here — see
		// config.RemoteDevice's doc comment.
		writeJSON(w, map[string]string{"token": token})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// handleRemoteDeviceByID implements DELETE /api/remote-access/devices/{id}.
func (s *Server) handleRemoteDeviceByID(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodDelete {
		http.Error(w, "DELETE only", http.StatusMethodNotAllowed)
		return
	}
	if !s.callerIsAdmin(r) {
		http.Error(w, "forbidden: admin only", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing device id", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.RevokeRemoteDevice(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
}

// callerIsAdmin reports whether r's caller may proceed on an admin-only
// action (auth mode changes, device/account management, server config,
// restart — Faz 5.1, yapacam.md). Deliberately permissive by default: a
// request with no session-role information at all — no credential (local
// mode, or auth mode "none"), or a plain device token, or an unrecognized/
// expired credential — is treated as allowed, not denied. This preserves
// every existing user's pre-Faz-5.1 behavior (full access, no accounts,
// no roles) unchanged; only a *recognized* session token whose account is
// explicitly Role "user" is ever turned away. remoteAuthMiddleware has
// already rejected the request earlier in the chain if the configured
// auth mode required a credential this request didn't have — this check
// only ever runs on requests that already passed that gate.
func (s *Server) callerIsAdmin(r *http.Request) bool {
	if s.fullBridge == nil {
		return true
	}
	cred := remoteCredential(r)
	if cred == "" {
		return true
	}
	role, ok := s.fullBridge.SessionRole(cred)
	if !ok {
		return true
	}
	return role == "admin"
}

// isSetupBootstrapPath reports whether path is one of the unauthenticated
// first-run setup endpoints (Faz 5.1, yapacam.md) — both must be reachable
// with no credential (there is none yet), including their /api/v1/
// aliases. GET /api/setup/status only ever reveals a boolean and the
// current auth mode (no secrets); POST /api/setup/create-admin enforces
// its own one-time-only gate internally (App.NeedsSetup) rather than
// relying on remoteAuthMiddleware, since after the first successful call
// it must keep returning 403 forever, not start requiring a credential
// that no client fetching this "is setup needed" screen would have.
func isSetupBootstrapPath(path string) bool {
	return path == "/api/setup/status" || path == "/api/v1/setup/status" ||
		path == "/api/setup/create-admin" || path == "/api/v1/setup/create-admin"
}

// handleSetupStatus implements GET /api/setup/status — unauthenticated
// (see isSetupBootstrapPath), polled by the minimal web UI on load to
// decide whether to show the first-run bootstrap screen, the
// username/password login form, or the legacy token-entry screen.
func (s *Server) handleSetupStatus(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "GET only", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{
		"needs_setup": s.fullBridge.NeedsSetup(),
		"auth_mode":   s.fullBridge.GetRemoteAuthMode(),
	})
}

// handleSetupCreateAdmin implements POST /api/setup/create-admin —
// unauthenticated (see isSetupBootstrapPath), only actually succeeds while
// App.NeedsSetup() is true. Returns a session token so the web UI can log
// straight in without a second round-trip to /api/auth/login.
func (s *Server) handleSetupCreateAdmin(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	token, err := s.fullBridge.CreateAdminAccount(req.Username, req.Password)
	if err != nil {
		status := http.StatusBadRequest
		if !s.fullBridge.NeedsSetup() {
			status = http.StatusForbidden
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, map[string]string{"session_token": token, "role": "admin"})
}

// handleAccounts implements GET (list) and POST (create) on /api/accounts.
// Behind remoteAuthMiddleware like everything else under /api/ — managing
// accounts requires already being authenticated by some means. POST is
// additionally admin-only (see callerIsAdmin); GET is left open to any
// authenticated caller, matching handleRemoteDevices' listing behavior.
func (s *Server) handleAccounts(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, s.fullBridge.ListAccounts())
	case http.MethodPost:
		if !s.callerIsAdmin(r) {
			http.Error(w, "forbidden: admin only", http.StatusForbidden)
			return
		}
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Role     string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.CreateAccount(req.Username, req.Password, req.Role); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "GET or POST", http.StatusMethodNotAllowed)
	}
}

// handleAccountByID implements DELETE /api/accounts/{id} (admin-only) and
// POST /api/accounts/{id}/password (password change — role logic lives in
// ChangeAccountPassword, see its doc comment).
func (s *Server) handleAccountByID(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing account id", http.StatusBadRequest)
		return
	}
	switch r.Method {
	case http.MethodDelete:
		if !s.callerIsAdmin(r) {
			http.Error(w, "forbidden: admin only", http.StatusForbidden)
			return
		}
		if err := s.fullBridge.DeleteAccount(id); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	case http.MethodPost:
		var req struct {
			CurrentPassword string `json:"current_password"`
			NewPassword     string `json:"new_password"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.ChangeAccountPassword(remoteCredential(r), id, req.CurrentPassword, req.NewPassword); err != nil {
			http.Error(w, err.Error(), changePasswordStatus(err))
			return
		}
		writeJSON(w, map[string]bool{"ok": true})
	default:
		http.Error(w, "DELETE or POST", http.StatusMethodNotAllowed)
	}
}

// changePasswordStatus maps ChangeAccountPassword's error strings to HTTP
// status codes — the errors already carry the distinction in their message,
// matching the codebase's string-based status mapping convention (see
// handleRemoteDeviceByID).
func changePasswordStatus(err error) int {
	msg := err.Error()
	switch {
	case msg == "valid session token is required":
		return http.StatusUnauthorized
	case strings.HasPrefix(msg, "account not found"):
		return http.StatusNotFound
	case msg == "only admins can change another account's password":
		return http.StatusForbidden
	default:
		return http.StatusBadRequest
	}
}
