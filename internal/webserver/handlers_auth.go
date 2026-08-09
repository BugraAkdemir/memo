// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"
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
	token, err := s.fullBridge.LoginRemotePassword(requestIP(r), req.Username, req.Password)
	if err != nil {
		if seconds, locked := asLockedError(err); locked {
			w.Header().Set("Retry-After", strconv.Itoa(int(seconds)+1))
			http.Error(w, "too many failed attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	writeJSON(w, map[string]string{"session_token": token})
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
