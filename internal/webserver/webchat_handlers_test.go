// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/beta"
	"memo/internal/config"
	"memo/internal/webchat"
)

func webChatCall(t *testing.T, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(method, "/api/webchat", strings.NewReader(body))
	(&Server{}).handleWebChat(rec, req)
	return rec
}

func TestWebChatHandler_ConsentNeedsBeta(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	t.Cleanup(func() { beta.SetGate(nil) })

	beta.SetGate(func() bool { return false })
	if rec := webChatCall(t, http.MethodPost, `{"action":"consent","site":"chatgpt"}`); rec.Code != http.StatusConflict {
		t.Fatalf("beta off: status %d, want 409", rec.Code)
	}
	if webchat.Consented("chatgpt") {
		t.Fatal("consent was recorded while Beta was off")
	}

	beta.SetGate(func() bool { return true })
	if rec := webChatCall(t, http.MethodPost, `{"action":"consent","site":"chatgpt"}`); rec.Code != http.StatusOK {
		t.Fatalf("beta on: status %d body %s", rec.Code, rec.Body)
	}
	if !webchat.Consented("chatgpt") {
		t.Fatal("consent not recorded")
	}
	if !strings.Contains(webChatCall(t, http.MethodGet, "").Body.String(), `"consented":true`) {
		t.Fatal("state does not report the confirmation")
	}
	if rec := webChatCall(t, http.MethodPost, `{"action":"revoke","site":"chatgpt"}`); rec.Code != http.StatusOK || webchat.Consented("chatgpt") {
		t.Fatalf("revoke failed: %d consented=%v", rec.Code, webchat.Consented("chatgpt"))
	}
}

func TestWebChatHandler_RefusesConnectWithoutNotice(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	beta.SetGate(func() bool { return true })
	t.Cleanup(func() { beta.SetGate(nil) })

	rec := webChatCall(t, http.MethodPost, `{"action":"connect","site":"claude"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("connect without the notice: status %d, want 409 (no browser may open)", rec.Code)
	}
}

func TestWebChatHandler_RejectsUnknownSiteAndAction(t *testing.T) {
	if rec := webChatCall(t, http.MethodPost, `{"action":"consent","site":"nope"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown site: %d", rec.Code)
	}
	if rec := webChatCall(t, http.MethodPost, `{"action":"explode","site":"chatgpt"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown action: %d", rec.Code)
	}
	if rec := webChatCall(t, http.MethodPut, ``); rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("PUT: %d", rec.Code)
	}
}
