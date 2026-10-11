// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// callbackBridge records what the Subscriptions handler hands to the bridge.
type callbackBridge struct {
	swarmStubBridge
	got string
	err error
}

func (b *callbackBridge) SubmitSubscriptionCallback(u string) error {
	b.got = u
	return b.err
}

func postSubscriptions(s *Server, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/subscriptions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.handleSubscriptions(rec, req)
	return rec
}

// The pasted address-bar URL must reach the bridge verbatim. The handler once
// decoded the request body twice, so the second decode saw EOF and every paste
// was answered "bad json" without the bridge being called — the remote sign-in
// hand-off was dead over HTTP while the Manager-level tests stayed green.
func TestSubscriptions_SubmitCallbackReachesTheBridge(t *testing.T) {
	b := &callbackBridge{}
	s := &Server{fullBridge: b}
	const pasted = "http://localhost:1455/auth/callback?code=REALCODE&state=REALSTATE"

	rec := postSubscriptions(s, `{"action":"submit_callback","callback_url":"`+pasted+`"}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d (%s), want 200", rec.Code, strings.TrimSpace(rec.Body.String()))
	}
	if b.got != pasted {
		t.Errorf("bridge got %q, want %q", b.got, pasted)
	}
}

// A refusal from the bridge (no login in flight, already finished…) is the
// caller's mistake, not a server fault.
func TestSubscriptions_SubmitCallbackSurfacesTheBridgeError(t *testing.T) {
	s := &Server{fullBridge: &callbackBridge{err: errors.New("no login in progress")}}

	rec := postSubscriptions(s, `{"action":"submit_callback","callback_url":"http://localhost:1455/x?code=1"}`)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "no login in progress") {
		t.Errorf("body = %q, want the bridge's reason", rec.Body.String())
	}
}
