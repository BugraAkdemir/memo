package webserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixedCheck returns a verify/validate closure that accepts exactly one
// value — a lightweight stand-in for VerifyRemoteDeviceToken/
// ValidateRemoteSession in tests that don't need a real FullBridge.
func fixedCheck(want string) func(string) bool {
	return func(got string) bool { return want != "" && got == want }
}

var neverPasses = func(string) bool { return false }

// TestRemoteAuthOK_LocalOnlyAlwaysPasses guards the default, most common
// case: a server bound to 127.0.0.1 (remote access never enabled, or turned
// back off) must never require a credential — this is exactly today's
// behavior for the local Flutter/CLI clients, unchanged by BUG-C1's fix.
func TestRemoteAuthOK_LocalOnlyAlwaysPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
	if !remoteAuthOK("127.0.0.1", "token", r, neverPasses, neverPasses) {
		t.Fatal("expected local-only bind to pass with no credential presented")
	}
}

// TestRemoteAuthOK_RemoteRequiresCredential is the actual BUG-C1 regression
// case: once the listener is bound for remote access (LAN mode or ngrok —
// both rebind to 0.0.0.0), a request with no credential at all must be
// rejected. Before the original fix, nothing checked the token, so every
// one of these previously-dangerous requests (wipe, agent permission grant,
// import, shutdown) sailed through unauthenticated.
func TestRemoteAuthOK_RemoteRequiresCredential(t *testing.T) {
	sensitivePaths := []string{
		"/api/wipe",
		"/api/agent/permission",
		"/api/import",
		"/api/shutdown",
		"/api/uninstall",
		"/api/providers", // BUG-H1: plaintext provider API keys
	}
	for _, path := range sensitivePaths {
		r := httptest.NewRequest(http.MethodPost, path, nil)
		if remoteAuthOK("0.0.0.0", "token", r, fixedCheck("memo-sometoken"), neverPasses) {
			t.Errorf("path %s: expected remote request with no credential to be rejected", path)
		}
	}
}

func TestRemoteAuthOK_NoneModeAlwaysPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/api/wipe", nil)
	if !remoteAuthOK("0.0.0.0", "none", r, neverPasses, neverPasses) {
		t.Fatal("expected auth_mode none to pass even with no credential — this is the deliberately insecure opt-in")
	}
}

// TestRemoteAuthOK_LoopbackSourcePasses is the local-app regression guard:
// once remote access is enabled (0.0.0.0 bind) with a strict mode, the
// installed desktop app — which always talks to 127.0.0.1 — must keep
// working with no credential at all, in every mode. Before this exemption
// existed, enabling LAN access locked the machine's own app out until the
// user submitted a password.
func TestRemoteAuthOK_LoopbackSourcePasses(t *testing.T) {
	for _, mode := range []string{"token", "password", "token_password"} {
		r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
		r.RemoteAddr = "127.0.0.1:54321"
		if !remoteAuthOK("0.0.0.0", mode, r, neverPasses, neverPasses) {
			t.Errorf("mode %s: expected loopback request to pass with no credential", mode)
		}
	}
}

// TestRemoteAuthOK_LoopbackIPv6Passes covers the ::1 source — a client
// pointed at "localhost" can resolve to IPv6 first.
func TestRemoteAuthOK_LoopbackIPv6Passes(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
	r.RemoteAddr = "[::1]:54321"
	if !remoteAuthOK("0.0.0.0", "password", r, neverPasses, neverPasses) {
		t.Fatal("expected ::1 loopback request to pass with no credential")
	}
}

// TestRemoteAuthOK_LoopbackDoesNotExemptLANSource guards the "pointed at
// the LAN IP from the same machine" case: the REMOTE_ADDR is the
// interface's non-loopback address, so the gate must still apply — a
// loopback exemption must be keyed on the actual source IP, never on
// "the client thinks it's talking to itself".
func TestRemoteAuthOK_LoopbackDoesNotExemptLANSource(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
	r.RemoteAddr = "192.168.1.50:54321"
	if remoteAuthOK("0.0.0.0", "password", r, neverPasses, neverPasses) {
		t.Fatal("expected LAN-source request without credential to be rejected even from the same machine")
	}
}

// TestRemoteAuthOK_LoopbackDoesNotExemptForwardedRequest is the Cloudflare
// Tunnel regression: a request relayed by a local reverse proxy/tunnel to
// 127.0.0.1 (e.g. a Cloudflare Tunnel ingress rule of `service: http://
// localhost:8090`, found live) has a genuinely loopback RemoteAddr — same as
// the real desktop client — but carries a proxy-forwarding header the
// desktop client never sends. Before this fix, isLoopbackIP alone let this
// straight through with zero credential, from anywhere on the public
// internet.
func TestRemoteAuthOK_LoopbackDoesNotExemptForwardedRequest(t *testing.T) {
	headers := []string{"X-Forwarded-For", "X-Real-Ip", "Cf-Connecting-Ip", "Forwarded"}
	for _, h := range headers {
		r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
		r.RemoteAddr = "127.0.0.1:54321"
		r.Header.Set(h, "203.0.113.7")
		if remoteAuthOK("0.0.0.0", "password", r, neverPasses, neverPasses) {
			t.Errorf("header %s: expected a forwarded loopback request without credential to be rejected", h)
		}
	}
}

// TestRemoteAuthOK_LoopbackForwardedRequestStillPassesWithCredential guards
// against the forwarded-request fix accidentally becoming a hard block: a
// tunnel-relayed request presenting a genuinely valid credential must still
// pass — the fix only removes the free pass, it doesn't add a new gate.
func TestRemoteAuthOK_LoopbackForwardedRequestStillPassesWithCredential(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/wipe", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.7")
	r.Header.Set("Authorization", "Bearer memo-sometoken")
	if !remoteAuthOK("0.0.0.0", "token", r, fixedCheck("memo-sometoken"), neverPasses) {
		t.Fatal("expected a forwarded loopback request with a valid token to pass")
	}
}

func TestRemoteAuthOK_TokenMode(t *testing.T) {
	verify := fixedCheck("memo-sometoken")

	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("X-Memo-Token", "memo-sometoken")
	if !remoteAuthOK("0.0.0.0", "token", r, verify, neverPasses) {
		t.Fatal("expected a valid device token to be accepted in token mode")
	}

	r2 := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r2.Header.Set("X-Memo-Token", "attacker-guess")
	if remoteAuthOK("0.0.0.0", "token", r2, verify, neverPasses) {
		t.Fatal("expected a wrong device token to be rejected in token mode")
	}

	// A session token being independently valid must NOT matter in
	// token-only mode — the point of choosing "token" over
	// "token_password" is that session tokens aren't accepted.
	r3 := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r3.Header.Set("X-Memo-Token", "some-session-jwt")
	alwaysValidSession := func(string) bool { return true }
	if remoteAuthOK("0.0.0.0", "token", r3, verify, alwaysValidSession) {
		t.Fatal("expected token mode to never consult validateSession")
	}
}

func TestRemoteAuthOK_PasswordMode(t *testing.T) {
	validateSession := fixedCheck("valid-session-jwt")

	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("Authorization", "Bearer valid-session-jwt")
	if !remoteAuthOK("0.0.0.0", "password", r, neverPasses, validateSession) {
		t.Fatal("expected a valid session token to be accepted in password mode")
	}

	// A device token, even one that would independently verify, must NOT
	// satisfy password-only mode.
	r2 := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r2.Header.Set("X-Memo-Token", "memo-realdevicetoken")
	alwaysValidDevice := func(string) bool { return true }
	if remoteAuthOK("0.0.0.0", "password", r2, alwaysValidDevice, validateSession) {
		t.Fatal("expected password mode to never consult verifyDevice")
	}
}

func TestRemoteAuthOK_TokenPasswordModeIsOR(t *testing.T) {
	verify := fixedCheck("memo-devicetoken")
	validateSession := fixedCheck("valid-session-jwt")

	deviceReq := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	deviceReq.Header.Set("X-Memo-Token", "memo-devicetoken")
	if !remoteAuthOK("0.0.0.0", "token_password", deviceReq, verify, validateSession) {
		t.Fatal("expected a valid device token to satisfy token_password mode")
	}

	sessionReq := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	sessionReq.Header.Set("X-Memo-Token", "valid-session-jwt")
	if !remoteAuthOK("0.0.0.0", "token_password", sessionReq, verify, validateSession) {
		t.Fatal("expected a valid session token to satisfy token_password mode")
	}

	badReq := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	badReq.Header.Set("X-Memo-Token", "neither-of-the-above")
	if remoteAuthOK("0.0.0.0", "token_password", badReq, verify, validateSession) {
		t.Fatal("expected a credential matching neither check to fail token_password mode")
	}
}

func TestRemoteAuthOK_UnknownModeFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("X-Memo-Token", "anything")
	alwaysPasses := func(string) bool { return true }
	if remoteAuthOK("0.0.0.0", "totally-bogus-mode", r, alwaysPasses, alwaysPasses) {
		t.Fatal("expected an unrecognized auth mode to fail closed, not open")
	}
}

func TestRemoteAuthOK_RemoteWithCorrectBearerTokenPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	r.Header.Set("Authorization", "Bearer memo-sometoken")
	if !remoteAuthOK("0.0.0.0", "token", r, fixedCheck("memo-sometoken"), neverPasses) {
		t.Fatal("expected matching Authorization: Bearer credential to be accepted")
	}
}

// TestRemoteAuthOK_EmptyCredentialFailsClosed covers a token that hasn't
// been generated yet (shouldn't normally happen once remote access has been
// enabled — SetRemoteAccess auto-provisions a device — but must never fail
// open).
func TestRemoteAuthOK_EmptyCredentialFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	// An attacker presenting an empty credential too must not match an
	// empty expectation.
	r.Header.Set("X-Memo-Token", "")
	if remoteAuthOK("0.0.0.0", "token", r, fixedCheck(""), neverPasses) {
		t.Fatal("expected an empty presented credential to never authenticate")
	}
}

// ── Faz 5.1 (yapacam.md): first-run bootstrap + admin-only gating ──────

func TestIsSetupBootstrapPath(t *testing.T) {
	yes := []string{
		"/api/setup/status", "/api/v1/setup/status",
		"/api/setup/create-admin", "/api/v1/setup/create-admin",
		"/api/setup/create-device", "/api/v1/setup/create-device",
	}
	for _, p := range yes {
		if !isSetupBootstrapPath(p) {
			t.Errorf("expected %q to be recognized as a setup bootstrap path", p)
		}
	}
	no := []string{
		"/api/setup", "/api/accounts", "/api/auth/login", "/api/setup/status/",
		// Regression guard for the bug create-device fixes: these two stay
		// authenticated on purpose — BootstrapTokenAuth (create-device) is
		// the unauthenticated path for a first-run client with no
		// credential at all; these are for a client that already has one.
		"/api/remote-access", "/api/remote-access/devices",
	}
	for _, p := range no {
		if isSetupBootstrapPath(p) {
			t.Errorf("expected %q to NOT be recognized as a setup bootstrap path", p)
		}
	}
}

func TestHandleSetupStatus_ReportsNeedsSetupAndAuthMode(t *testing.T) {
	stub := &swarmStubBridge{needsSetup: true}
	s := New(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w := httptest.NewRecorder()
	s.handleSetupStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"needs_setup":true`) {
		t.Errorf("expected needs_setup:true in body, got %s", w.Body.String())
	}
}

// TestHandleSetupStatus_ReportsInstallID covers the field clients use to
// notice their saved sign-in belongs to a backend that no longer exists —
// a wipe+reinstall reuses the same origin, so localStorage alone can never
// tell (reported live from a Raspberry Pi, 2026-08-13). The route is
// unauthenticated by design (isSetupBootstrapPath), which is exactly why a
// client locked out by stale state can still read it.
func TestHandleSetupStatus_ReportsInstallID(t *testing.T) {
	stub := &swarmStubBridge{needsSetup: true, installID: "abc123"}
	s := New(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w := httptest.NewRecorder()
	s.handleSetupStatus(w, req)

	if !strings.Contains(w.Body.String(), `"install_id":"abc123"`) {
		t.Errorf("expected install_id in body, got %s", w.Body.String())
	}
}

// TestHandleSetupStatus_ToleratesMissingInstallID: a backend that could not
// persist an id must still serve the route — clients fall back to their
// unauthorized-probe layer rather than losing the bootstrap endpoint.
func TestHandleSetupStatus_ToleratesMissingInstallID(t *testing.T) {
	stub := &swarmStubBridge{needsSetup: true}
	s := New(stub)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	w := httptest.NewRecorder()
	s.handleSetupStatus(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"install_id":""`) {
		t.Errorf("expected an empty install_id, got %s", w.Body.String())
	}
}

// TestHandleSetupStatus_ReportsLoopback mirrors remoteAuthOK's trust model
// for clients: the same request source that passes with no credential must
// be told so via the loopback field, so the desktop app can skip its login
// gate instead of demanding a password the backend would never check.
func TestHandleSetupStatus_ReportsLoopback(t *testing.T) {
	stub := &swarmStubBridge{needsSetup: false}
	s := New(stub)

	local := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	local.RemoteAddr = "127.0.0.1:54321"
	wLocal := httptest.NewRecorder()
	s.handleSetupStatus(wLocal, local)
	if !strings.Contains(wLocal.Body.String(), `"loopback":true`) {
		t.Errorf("expected loopback:true for a 127.0.0.1 source, got %s", wLocal.Body.String())
	}

	remote := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	wRemote := httptest.NewRecorder()
	s.handleSetupStatus(wRemote, remote)
	if !strings.Contains(wRemote.Body.String(), `"loopback":false`) {
		t.Errorf("expected loopback:false for a LAN source, got %s", wRemote.Body.String())
	}
}

// TestHandleSetupStatus_ForwardedLoopbackReportsFalse is the client-facing
// half of the Cloudflare Tunnel fix: a request relayed to 127.0.0.1 through
// a local reverse proxy/tunnel must be told loopback:false, so the web UI
// actually shows its login gate instead of skipping straight to the app.
func TestHandleSetupStatus_ForwardedLoopbackReportsFalse(t *testing.T) {
	stub := &swarmStubBridge{needsSetup: false}
	s := New(stub)

	r := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("Cf-Connecting-Ip", "203.0.113.7")
	w := httptest.NewRecorder()
	s.handleSetupStatus(w, r)
	if !strings.Contains(w.Body.String(), `"loopback":false`) {
		t.Errorf("expected loopback:false for a tunnel-forwarded loopback source, got %s", w.Body.String())
	}
}

func TestHandleSetupCreateAdmin_SucceedsWhileSetupNeeded(t *testing.T) {
	stub := &swarmStubBridge{
		needsSetup: true,
		createAdminAccount: func(username, password string) (string, error) {
			if username != "alice" || password != "hunter2" {
				t.Errorf("got username=%q password=%q, want alice/hunter2", username, password)
			}
			return "a-fresh-session-token", nil
		},
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/create-admin", strings.NewReader(`{"username":"alice","password":"hunter2"}`))
	w := httptest.NewRecorder()
	s.handleSetupCreateAdmin(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "a-fresh-session-token") {
		t.Errorf("expected the session token in the response body, got %s", w.Body.String())
	}
}

// TestHandleSetupCreateAdmin_ClosesPermanentlyAfterFirstSuccess is the
// literal yapacam.md acceptance criterion: "ilk başarılı çağrıdan sonra
// kalıcı olarak kapanır (sunucu yeniden başlasa bile, hesap listesi dolu
// olduğu sürece hep 403/404)".
func TestHandleSetupCreateAdmin_ClosesPermanentlyAfterFirstSuccess(t *testing.T) {
	stub := &swarmStubBridge{
		needsSetup: false, // simulates a server restarted after bootstrap already completed
		createAdminAccount: func(username, password string) (string, error) {
			return "", errors.New("setup already completed")
		},
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/create-admin", strings.NewReader(`{"username":"mallory","password":"hunter2"}`))
	w := httptest.NewRecorder()
	s.handleSetupCreateAdmin(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

// TestHandleSetupCreateDevice_SucceedsWhileSetupNeeded and the two tests
// below guard the actual reported bug: before create-device existed, the
// token-only setup screen's first call went through the normal,
// authenticated remote-access endpoints and 401'd for any non-loopback
// client (reported live from a Raspberry Pi over LAN, 2026-08-13).
func TestHandleSetupCreateDevice_SucceedsWhileSetupNeeded(t *testing.T) {
	stub := &swarmStubBridge{
		needsSetup: true,
		bootstrapTokenAuth: func(deviceName string) (string, error) {
			if deviceName != "Auth setup" {
				t.Errorf("got deviceName=%q, want %q", deviceName, "Auth setup")
			}
			return "a-fresh-device-token", nil
		},
	}
	s := New(stub)

	// No X-Memo-Token/Authorization header at all — matching a genuine
	// first-run client, and the whole point of this endpoint being in
	// isSetupBootstrapPath's exemption list (see remoteAuthMiddleware,
	// which routes here without ever calling this handler with a missing
	// credential rejected).
	req := httptest.NewRequest(http.MethodPost, "/api/setup/create-device", strings.NewReader(`{"name":"Auth setup"}`))
	w := httptest.NewRecorder()
	s.handleSetupCreateDevice(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "a-fresh-device-token") {
		t.Errorf("expected the device token in the response body, got %s", w.Body.String())
	}
}

// TestHandleSetupCreateDevice_ClosesPermanentlyAfterFirstSuccess mirrors
// TestHandleSetupCreateAdmin_ClosesPermanentlyAfterFirstSuccess — same
// self-gating contract, so a second bootstrap attempt (or an attempt
// against a server that already completed setup by either path) must be
// rejected, not silently mint another unauthenticated device token forever.
func TestHandleSetupCreateDevice_ClosesPermanentlyAfterFirstSuccess(t *testing.T) {
	stub := &swarmStubBridge{
		needsSetup: false, // simulates a server restarted after bootstrap already completed
		bootstrapTokenAuth: func(deviceName string) (string, error) {
			return "", errors.New("setup already completed")
		},
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/create-device", strings.NewReader(`{"name":"Another device"}`))
	w := httptest.NewRecorder()
	s.handleSetupCreateDevice(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleRemoteAccess_PutForbiddenForNonAdminSession(t *testing.T) {
	stub := &swarmStubBridge{
		sessionRole: func(token string) (string, bool) {
			if token == "user-session" {
				return "user", true
			}
			return "", false
		},
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPut, "/api/remote-access", strings.NewReader(`{"auth_mode":"none"}`))
	req.Header.Set("X-Memo-Token", "user-session")
	w := httptest.NewRecorder()
	s.handleRemoteAccess(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403 for a non-admin session, body: %s", w.Code, w.Body.String())
	}
}

func TestHandleRemoteAccess_PutAllowedForAdminSession(t *testing.T) {
	stub := &swarmStubBridge{
		sessionRole: func(token string) (string, bool) {
			if token == "admin-session" {
				return "admin", true
			}
			return "", false
		},
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPut, "/api/remote-access", strings.NewReader(`{}`))
	req.Header.Set("X-Memo-Token", "admin-session")
	w := httptest.NewRecorder()
	s.handleRemoteAccess(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected an admin session to be allowed through, got 403: %s", w.Body.String())
	}
}

// TestHandleRemoteAccess_PutAllowedWithNoRoleInformation is the
// non-disruption guarantee callerIsAdmin's doc comment describes: every
// pre-Faz-5.1 install (no accounts at all — device tokens or local-only
// access) must keep working exactly as before, since SessionRole reports
// ok=false whenever the presented credential isn't a recognized account
// session.
func TestHandleRemoteAccess_PutAllowedWithNoRoleInformation(t *testing.T) {
	stub := &swarmStubBridge{
		sessionRole: func(token string) (string, bool) { return "", false },
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPut, "/api/remote-access", strings.NewReader(`{}`))
	req.Header.Set("X-Memo-Token", "some-legacy-device-token")
	w := httptest.NewRecorder()
	s.handleRemoteAccess(w, req)

	if w.Code == http.StatusForbidden {
		t.Fatalf("expected a request with no recognized session role to be allowed through unchanged, got 403: %s", w.Body.String())
	}
}

func TestHandleAccounts_PostForbiddenForNonAdmin(t *testing.T) {
	stub := &swarmStubBridge{
		sessionRole: func(token string) (string, bool) { return "user", true },
	}
	s := New(stub)

	req := httptest.NewRequest(http.MethodPost, "/api/accounts", strings.NewReader(`{"username":"bob","password":"x","role":"user"}`))
	req.Header.Set("X-Memo-Token", "user-session")
	w := httptest.NewRecorder()
	s.handleAccounts(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("got status %d, want 403, body: %s", w.Code, w.Body.String())
	}
}
