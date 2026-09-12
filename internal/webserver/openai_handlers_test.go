// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestHandleOpenAIModels_NonLoopbackWithConfigOffRequiresKey and its
// _ChatCompletions_ sibling guard the security-audit finding: unlike
// handleAnthropicMessages (/v1/messages), these two handlers passed the raw
// "Require API Key" config value straight to devGatewayAuthOK instead of
// wrapping it in devGatewayRequireKey(r, ...) first — so a non-loopback
// caller (LAN, tunnel) could reach the OpenAI-compatible gateway with zero
// key whenever the toggle was left off (its default), even though the
// Anthropic-compatible sibling already closed this exact hole. Both must
// reject a spoofed-remote, no-key request the same way /v1/messages does.
func TestHandleOpenAIModels_NonLoopbackWithConfigOffRequiresKey(t *testing.T) {
	stub := &swarmStubBridge{devGatewayRequireAPIKey: false, devGatewayTokenValue: "memo-sometoken"}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.RemoteAddr = "192.0.2.55:54321" // TEST-NET-1, definitely non-loopback
	w := httptest.NewRecorder()
	s.handleOpenAIModels(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d (non-loopback caller must require a key even with the toggle off), body: %s",
			w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

func TestHandleOpenAIModels_LoopbackWithConfigOffSkipsKey(t *testing.T) {
	stub := &swarmStubBridge{devGatewayRequireAPIKey: false}
	s := New(stub)
	s.fullBridge = stub

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	req.RemoteAddr = "127.0.0.1:54321"
	w := httptest.NewRecorder()
	s.handleOpenAIModels(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("got status %d, want %d (genuine loopback caller should still skip the key when the toggle is off), body: %s",
			w.Code, http.StatusOK, w.Body.String())
	}
}

func TestHandleOpenAIChatCompletions_NonLoopbackWithConfigOffRequiresKey(t *testing.T) {
	stub := &swarmStubBridge{devGatewayRequireAPIKey: false, devGatewayTokenValue: "memo-sometoken"}
	s := New(stub)
	s.fullBridge = stub

	body := strings.NewReader(`{"model":"local/qwen2.5","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.RemoteAddr = "192.0.2.55:54321"
	w := httptest.NewRecorder()
	s.handleOpenAIChatCompletions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d (non-loopback caller must require a key even with the toggle off), body: %s",
			w.Code, http.StatusUnauthorized, w.Body.String())
	}
}

// TestHandleOpenAIChatCompletions_LoopbackButForwardedStillRequiresKey
// guards the tunnel case (cloudflared/ngrok/nginx relaying to 127.0.0.1):
// RemoteAddr looks loopback at the TCP level even though the real caller is
// remote, so the forwarded-header signal must still force the key check.
func TestHandleOpenAIChatCompletions_LoopbackButForwardedStillRequiresKey(t *testing.T) {
	stub := &swarmStubBridge{devGatewayRequireAPIKey: false, devGatewayTokenValue: "memo-sometoken"}
	s := New(stub)
	s.fullBridge = stub

	body := strings.NewReader(`{"model":"local/qwen2.5","messages":[{"role":"user","content":"hi"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", body)
	req.RemoteAddr = "127.0.0.1:54321"
	req.Header.Set("X-Forwarded-For", "203.0.113.9")
	w := httptest.NewRecorder()
	s.handleOpenAIChatCompletions(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("got status %d, want %d (tunnel-relayed loopback connection must still require the key), body: %s",
			w.Code, http.StatusUnauthorized, w.Body.String())
	}
}
