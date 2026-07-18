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
