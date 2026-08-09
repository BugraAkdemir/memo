package webserver

import (
	"net/http"
	"net/http/httptest"
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
