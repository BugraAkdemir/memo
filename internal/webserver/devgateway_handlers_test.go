// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/provider"
)

func TestDevGatewayAuthOK_NotRequiredAlwaysPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if !devGatewayAuthOK(r, false, "memo-sometoken") {
		t.Fatal("expected requireAPIKey=false to always pass, regardless of headers")
	}
}

func TestDevGatewayAuthOK_RequiredWithNoKeyFails(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	if devGatewayAuthOK(r, true, "memo-sometoken") {
		t.Fatal("expected a request with no key to be rejected")
	}
}

func TestDevGatewayAuthOK_XAPIKeyHeaderPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.Header.Set("x-api-key", "memo-sometoken")
	if !devGatewayAuthOK(r, true, "memo-sometoken") {
		t.Fatal("expected matching x-api-key (the real Anthropic client header) to be accepted")
	}
}

func TestDevGatewayAuthOK_BearerFallbackPasses(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.Header.Set("Authorization", "Bearer memo-sometoken")
	if !devGatewayAuthOK(r, true, "memo-sometoken") {
		t.Fatal("expected matching Authorization: Bearer to be accepted as a fallback")
	}
}

func TestDevGatewayAuthOK_WrongKeyFails(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.Header.Set("x-api-key", "attacker-guess")
	if devGatewayAuthOK(r, true, "memo-sometoken") {
		t.Fatal("expected mismatched key to be rejected")
	}
}

func TestDevGatewayAuthOK_EmptyStoredTokenFailsClosed(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.Header.Set("x-api-key", "")
	if devGatewayAuthOK(r, true, "") {
		t.Fatal("expected an empty stored token to never authenticate, even against an empty presented key")
	}
}

// TestDevGatewayRequireKey_NonLoopbackForcesKeyEvenWhenConfigOff guards K2: a
// remote caller (LAN, tunnel, ngrok) hitting /v1/* must never be able to
// skip the API-key check just because the user left "Require API Key" off
// for local-CLI convenience — that toggle's whole point is letting a
// same-machine process (Claude Code, etc.) skip typing a key, not opening
// the gateway to the network once remote access is on.
func TestDevGatewayRequireKey_NonLoopbackForcesKeyEvenWhenConfigOff(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.RemoteAddr = "192.0.2.55:54321" // TEST-NET-1, definitely non-loopback
	if !devGatewayRequireKey(r, false) {
		t.Fatal("expected a non-loopback caller to require the API key regardless of the configured toggle")
	}
}

func TestDevGatewayRequireKey_LoopbackSkipsKeyWhenConfigOff(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	if devGatewayRequireKey(r, false) {
		t.Fatal("expected a genuine same-machine loopback caller to still be able to skip the key when the toggle is off")
	}
}

// TestDevGatewayRequireKey_LoopbackButForwardedStillRequiresKey guards the
// tunnel case: cloudflared/ngrok/nginx relay a remote connection to
// 127.0.0.1, so RemoteAddr looks loopback at the TCP level even though the
// real caller is remote — the forwarded-header signal must override the
// loopback pass, same as remoteAuthMiddleware's own reasoning.
func TestDevGatewayRequireKey_LoopbackButForwardedStillRequiresKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	r.Header.Set("X-Forwarded-For", "203.0.113.9")
	if !devGatewayRequireKey(r, false) {
		t.Fatal("expected a tunnel-relayed loopback connection to still require the API key")
	}
}

func TestDevGatewayRequireKey_ConfiguredOnAlwaysRequiresKey(t *testing.T) {
	r := httptest.NewRequest(http.MethodPost, "/v1/messages", nil)
	r.RemoteAddr = "127.0.0.1:54321"
	if !devGatewayRequireKey(r, true) {
		t.Fatal("expected the configured toggle=true to always require the key, even for loopback")
	}
}

func TestLastUserMessageText(t *testing.T) {
	cases := []struct {
		name     string
		messages []provider.Message
		want     string
	}{
		{
			name: "simple user message",
			messages: []provider.Message{
				{Role: "system", Content: "You are Claude Code."},
				{Role: "user", Content: "hello"},
			},
			want: "hello",
		},
		{
			name: "picks the LAST user message, not the first",
			messages: []provider.Message{
				{Role: "user", Content: "first"},
				{Role: "assistant", Content: "reply"},
				{Role: "user", Content: "second"},
			},
			want: "second",
		},
		{
			name:     "no user message",
			messages: []provider.Message{{Role: "system", Content: "hi"}},
			want:     "",
		},
		{
			name:     "empty",
			messages: nil,
			want:     "",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := lastUserMessageText(c.messages); got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}
}
