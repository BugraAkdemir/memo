package webserver

import (
	"context"
	"encoding/json"
	"errors"
	"memo/internal/autonomy"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/twin"
)

type fakeTwinBridge struct {
	mockBridge
	called    int
	operation string
	err       error
}

func (f *fakeTwinBridge) CallTwin(_ context.Context, op string, _ json.RawMessage) (any, error) {
	f.called++
	f.operation = op
	return map[string]bool{"ok": true}, f.err
}
func TestTwinHTTPMethodsLimitsAndCodes(t *testing.T) {
	f := &fakeTwinBridge{}
	s := New(f)
	for _, tc := range []struct {
		method, path, body string
		status             int
		err                error
	}{
		{"GET", "/api/twin/state", "", 200, nil},
		{"GET", "/api/twin/activity", "", 200, nil},
		{"GET", "/api/v1/twin/activity", "", 200, nil},
		{"POST", "/api/twin/activity", "{}", 405, nil},
		{"GET", "/api/v1/twin/state", "", 200, nil},
		{"POST", "/api/twin/state", "{}", 405, nil},
		{"GET", "/api/twin/contact", "", 405, nil},
		{"POST", "/api/twin/contact", "{", 400, nil},
		{"POST", "/api/twin/contact", strings.Repeat("x", 32769), 400, nil},
		{"POST", "/api/twin/run_command", "{}", 404, nil},
		{"POST", "/api/twin/contact", "{}", 409, twin.ErrAliasConflict},
	} {
		f.err = tc.err
		w := httptest.NewRecorder()
		s.handleTwin(w, httptest.NewRequest(tc.method, tc.path, strings.NewReader(tc.body)))
		if w.Code != tc.status {
			t.Fatalf("%+v %d %s", tc, w.Code, w.Body)
		}
	}
}
func TestTwinHTTPRequiresAdminForReadsAndWrites(t *testing.T) {
	stub := &swarmStubBridge{sessionRole: func(token string) (string, bool) { return "user", token == "user-session" }}
	s := New(stub)
	for _, op := range []string{"state", "activity", "contact", "merge", "preferences", "reply_model", "cancel_action", "send_action", "contact_timeline", "check_availability", "git_project_status"} {
		req := httptest.NewRequest("POST", "/api/twin/"+op, strings.NewReader(`{}`))
		if op == "state" || op == "activity" {
			req.Method = "GET"
		}
		req.Header.Set("X-Memo-Token", "user-session")
		w := httptest.NewRecorder()
		called := false
		s.adminOnly(func(_w http.ResponseWriter, _r *http.Request) { called = true })(w, req)
		if called || w.Code != 403 {
			t.Fatal(op, w.Code)
		}
	}
}

func TestOwnerReplySendHTTPFailuresAreStableAndRedacted(t *testing.T) {
	for _, tc := range []struct {
		err    error
		status int
		code   string
	}{
		{twin.ErrDeliveryConflict, 409, "mail_reply_not_pending"},
		{twin.ErrSentHistory, 409, "mail_reply_sent_history"},
		{twin.ErrDeliveryDenied, 403, "mail_reply_contact_denied"},
		{&autonomy.BlockedError{Code: "secret_private_key"}, 403, "mail_reply_blocked"},
		{errors.New("SMTP private transport details"), 500, "mail_reply_send_uncertain"},
	} {
		f := &fakeTwinBridge{err: tc.err}
		s := New(f)
		w := httptest.NewRecorder()
		s.handleTwin(w, httptest.NewRequest("POST", "/api/twin/send_action", strings.NewReader(`{"id":"draft","subject":"Re: meeting","body":"edited"}`)))
		if w.Code != tc.status || !strings.Contains(w.Body.String(), tc.code) || strings.Contains(w.Body.String(), "private transport") {
			t.Fatalf("%v: %d %s", tc.err, w.Code, w.Body)
		}
	}
}
