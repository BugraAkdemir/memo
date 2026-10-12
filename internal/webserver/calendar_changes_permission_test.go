package webserver

import (
	"memo/internal/config"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCalendarChangesDeniesRestrictedRead(t *testing.T) {
	stub := &swarmStubBridge{sessionPermissions: func(string) (config.AccountPermissions, bool) {
		return config.AccountPermissions{Calendar: false}, true
	}}
	s := New(stub)
	handler := s.requirePermissionStrict(s.handleCalendarChanges, hasCalendarPerm)
	req := httptest.NewRequest(http.MethodGet, "/api/calendar/changes", nil)
	req.Header.Set("X-Memo-Token", "user-session")
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("restricted SSE read=%d", w.Code)
	}
}
