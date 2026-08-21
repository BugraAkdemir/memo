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

	"memo/internal/config"
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

// isLoopbackIP reports whether the request's source IP is the loopback
// interface (127.0.0.0/8 or ::1). Requests from localhost are trusted by
// the same token the backend uses for its local-only bind: loopback traffic
// can only originate from software running on this machine, so remote auth
// gates — which exist purely to protect against *remote* callers — don't
// apply. This keeps the installed desktop app (always talking to
// 127.0.0.1) working without a credential even after remote access has been
// enabled with a strict auth mode, while a request from the LAN IP (the
// "192.168.1.xxx'den şifre istesin" case) still goes through the gate.
func isLoopbackIP(ip string) bool {
	if ip == "" {
		return false
	}
	parsed := net.ParseIP(ip)
	return parsed != nil && parsed.IsLoopback()
}

// forwardedHeaders are the standard markers a reverse proxy or tunnel client
// adds to a request it relays to a local origin. cloudflared in particular
// always injects Cf-Connecting-Ip/X-Forwarded-For on every request it
// proxies through a Cloudflare Tunnel — including one configured with
// `service: http://localhost:<port>`, which is a completely ordinary,
// commonly-suggested way to expose a self-hosted app.
var forwardedHeaders = []string{"X-Forwarded-For", "X-Real-Ip", "Cf-Connecting-Ip", "Forwarded"}

// isForwardedRequest reports whether r carries any standard proxy-forwarding
// header. Used only as a "don't extend loopback trust here" signal (see
// isLoopbackIP's comment above) — never to read an actual client identity
// out of these headers. That distinction matters: this codebase already had
// one incident from trusting X-Forwarded-For's *value* for the rate limiter
// (an attacker-controlled header, since a request can reach this server
// directly with no proxy in front at all) — this is the opposite,
// conservative use. A locally-run reverse proxy/tunnel (cloudflared, ngrok,
// nginx, ssh -L) that forwards to 127.0.0.1 makes the resulting connection's
// RemoteAddr *genuinely* loopback at the TCP level, so it cannot be told
// apart from the real local desktop client (also 127.0.0.1) by IP alone —
// but the desktop app is a bare HTTP client that never sets any of these
// headers, while every reverse proxy/tunnel does by convention, so their
// mere presence reliably means "this loopback connection was relayed from
// somewhere else" without the header's contents needing to be trusted for
// anything. A remote attacker cannot forge this signal without first being
// the loopback-sourced relay itself, at which point they'd already have to
// run software on the machine.
func isForwardedRequest(r *http.Request) bool {
	for _, h := range forwardedHeaders {
		if r.Header.Get(h) != "" {
			return true
		}
	}
	return false
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
		// remember extends the session lifetime (remoteauth.
		// RememberSessionTTL) — the "beni hatırla" checkbox on the login
		// gate. Absent = false keeps old clients on the default short TTL.
		Remember bool `json:"remember"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	token, role, err := s.fullBridge.LoginRemotePassword(requestIP(r), req.Username, req.Password, req.Remember)
	if err != nil {
		if seconds, locked := asLockedError(err); locked {
			w.Header().Set("Retry-After", strconv.Itoa(int(seconds)+1))
			http.Error(w, "too many failed attempts", http.StatusTooManyRequests)
			return
		}
		http.Error(w, "invalid credentials", http.StatusUnauthorized)
		return
	}
	// permissions lets the client gate its own UI (hide tabs/nav entries)
	// immediately, without a second round-trip — resolved fresh right here
	// rather than trusted from anywhere else, same "live account list,
	// not the JWT claim" principle SessionRole already follows. Best-effort:
	// if resolution somehow fails right after a successful login, the
	// client just falls back to showing everything (fail open, matching
	// this whole permission system's philosophy) rather than the login
	// itself failing.
	perms, _ := s.fullBridge.SessionPermissions(token)
	writeJSON(w, map[string]interface{}{"session_token": token, "role": role, "permissions": perms})
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

// callerHasPermission reports whether r's caller may proceed on an action
// gated by one of AccountPermissions' checkboxes (Faz 5.1.1, yapacam.md).
// check picks which field matters for this call site. Same fail-open shape
// as callerIsAdmin, deliberately: no credential, an unrecognized/expired
// session, or a plain device token (SessionPermissions returns ok=false
// for all three) is allowed, not denied — this is what keeps every
// pre-Faz-5.1.1 install (no Accounts at all) and every admin session
// completely unaffected; only a recognized "user"-role session whose
// account has this specific box unchecked is ever turned away.
func (s *Server) callerHasPermission(r *http.Request, check func(config.AccountPermissions) bool) bool {
	if s.fullBridge == nil {
		return true
	}
	cred := remoteCredential(r)
	if cred == "" {
		return true
	}
	perms, ok := s.fullBridge.SessionPermissions(cred)
	if !ok {
		return true
	}
	return check(perms)
}

// requirePermission wraps handler so a caller lacking the checked
// permission gets 403 instead of reaching it — but only for
// state-changing requests (anything but GET/HEAD). Read-only requests
// always pass through ungated: several of these endpoint groups (models/
// providers status, agent-enabled state, whatsapp/telegram/calendar/
// routine listings) are polled ambiently by screens the restricted account
// can still see even without the matching permission (e.g. the chat
// header's active-model badge) — blocking those reads would break
// ordinary chat for a restricted account, not just the specific
// tab/screen this permission is actually meant to gate. The real UI-level
// restriction (not being able to open that tab/screen at all) lives on
// the client; this is the server-side backstop against a client bypassing
// its own UI. See requirePermissionStrict for the one group (memory) that
// needs reads gated too.
func (s *Server) requirePermission(handler http.HandlerFunc, check func(config.AccountPermissions) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead && !s.callerHasPermission(r, check) {
			http.Error(w, "forbidden: missing permission", http.StatusForbidden)
			return
		}
		handler(w, r)
	}
}

// requirePermissionStrict is requirePermission without the GET/HEAD
// exemption — used only for the memory permission, where the point of
// denying it is that the account can't see the shared memory contents at
// all, not merely that it can't edit them (see AccountPermissions.Memory's
// doc comment).
func (s *Server) requirePermissionStrict(handler http.HandlerFunc, check func(config.AccountPermissions) bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.callerHasPermission(r, check) {
			http.Error(w, "forbidden: missing permission", http.StatusForbidden)
			return
		}
		handler(w, r)
	}
}

func hasModelsPerm(p config.AccountPermissions) bool   { return p.Models }
func hasMemoryPerm(p config.AccountPermissions) bool   { return p.Memory }
func hasAgentPerm(p config.AccountPermissions) bool    { return p.Agent }
func hasCalendarPerm(p config.AccountPermissions) bool { return p.Calendar }
func hasWhatsAppPerm(p config.AccountPermissions) bool { return p.WhatsApp }
func hasTelegramPerm(p config.AccountPermissions) bool { return p.Telegram }
func hasRoutinesPerm(p config.AccountPermissions) bool { return p.Routines }

// isSetupBootstrapPath reports whether path is one of the unauthenticated
// first-run setup endpoints (Faz 5.1, yapacam.md) — all three must be
// reachable with no credential (there is none yet), including their
// /api/v1/ aliases. GET /api/setup/status only ever reveals a boolean and
// the current auth mode (no secrets); POST /api/setup/create-admin and
// POST /api/setup/create-device each enforce their own one-time-only gate
// internally (App.NeedsSetup) rather than relying on remoteAuthMiddleware,
// since after the first successful call they must keep returning 403
// forever, not start requiring a credential that no client fetching this
// "is setup needed" screen would have.
func isSetupBootstrapPath(path string) bool {
	return path == "/api/setup/status" || path == "/api/v1/setup/status" ||
		path == "/api/setup/create-admin" || path == "/api/v1/setup/create-admin" ||
		path == "/api/setup/create-device" || path == "/api/v1/setup/create-device"
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
		// loopback mirrors remoteAuthOK's trust model exactly, including
		// the isForwardedRequest exception — a client relayed through a
		// local reverse proxy/tunnel to 127.0.0.1 must still see false
		// here and go through the real login gate, not just skip it
		// because the connection happens to be loopback at the TCP level.
		"loopback": isLoopbackIP(requestIP(r)) && !isForwardedRequest(r),
		// install_id lets a client detect that this backend is not the one
		// it stored its auth state against — a wipe+reinstall, or the same
		// browser pointed at a different Memo. Opaque random value, no
		// secret material (see internal/app/install_id.go). Empty when the
		// backend could not persist one; clients must tolerate that.
		"install_id": s.fullBridge.InstallID(),
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

// handleSetupCreateDevice implements POST /api/setup/create-device — the
// token-only counterpart to handleSetupCreateAdmin above, same reasoning:
// unauthenticated (see isSetupBootstrapPath), only actually succeeds while
// App.NeedsSetup() is true (App.BootstrapTokenAuth's own self-gating).
// Returns the plaintext device token exactly once, the same as the
// authenticated POST /api/remote-access/devices normally would.
func (s *Server) handleSetupCreateDevice(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "POST only", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	token, err := s.fullBridge.BootstrapTokenAuth(req.Name)
	if err != nil {
		status := http.StatusBadRequest
		if !s.fullBridge.NeedsSetup() {
			status = http.StatusForbidden
		}
		http.Error(w, err.Error(), status)
		return
	}
	writeJSON(w, map[string]string{"token": token})
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
			Username    string                    `json:"username"`
			Password    string                    `json:"password"`
			Role        string                    `json:"role"`
			Permissions config.AccountPermissions `json:"permissions"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "bad json", http.StatusBadRequest)
			return
		}
		if err := s.fullBridge.CreateAccount(req.Username, req.Password, req.Role, req.Permissions); err != nil {
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

// handleAccountPermissions implements PUT /api/accounts/{id}/permissions
// (admin-only) — editing an existing account's checkbox state (Faz 5.1.1)
// after creation, e.g. turning Memory access on for an account that didn't
// have it checked initially.
func (s *Server) handleAccountPermissions(w http.ResponseWriter, r *http.Request) {
	if s.fullBridge == nil {
		http.Error(w, "not available", http.StatusNotImplemented)
		return
	}
	if r.Method != http.MethodPut {
		http.Error(w, "PUT only", http.StatusMethodNotAllowed)
		return
	}
	if !s.callerIsAdmin(r) {
		http.Error(w, "forbidden: admin only", http.StatusForbidden)
		return
	}
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing account id", http.StatusBadRequest)
		return
	}
	var perms config.AccountPermissions
	if err := json.NewDecoder(r.Body).Decode(&perms); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if err := s.fullBridge.UpdateAccountPermissions(id, perms); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, map[string]bool{"ok": true})
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
