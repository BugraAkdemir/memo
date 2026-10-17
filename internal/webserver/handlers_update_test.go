// SPDX-License-Identifier: AGPL-3.0-or-later

package webserver

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"

	"memo/internal/config"
	"memo/internal/updater"
)

// updateBridge records what the update endpoints ask the app to do.
type updateBridge struct {
	*swarmStubBridge
	mu        sync.Mutex
	forced    []bool
	applied   [][2]string
	applyErr  error
	info      *updater.Info
	statusVal updater.Status
}

func (b *updateBridge) CheckForUpdate(_ context.Context, force bool) (*updater.Info, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.forced = append(b.forced, force)
	return b.info, nil
}

func (b *updateBridge) ApplyUpdate(kind, component string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.applied = append(b.applied, [2]string{kind, component})
	return b.applyErr
}

func (b *updateBridge) UpdateStatus() updater.Status { return b.statusVal }

func startUpdateServer(t *testing.T, b *updateBridge) string {
	t.Helper()
	b.swarmStubBridge = &swarmStubBridge{
		sessionRole: func(token string) (string, bool) {
			if token == "admin-session" {
				return "admin", true
			}
			return "user", true
		},
		sessionPermissions: func(token string) (config.AccountPermissions, bool) {
			return config.AccountPermissions{}, true
		},
	}
	port := freePort(t)
	s := New(b)
	if err := s.StartHTTPWithAddr(port, "127.0.0.1"); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Stop() })
	waitForListening(t, port)
	return fmt.Sprintf("http://127.0.0.1:%d", port)
}

func TestUpdateEndpoints_CheckAndStatusAreReadableByAnyone(t *testing.T) {
	b := &updateBridge{
		info:      &updater.Info{Current: "V4.6.0", Latest: "V4.7.0", Available: true, Mode: updater.ModeAuto, Components: []updater.ComponentInfo{}},
		statusVal: updater.Status{State: updater.StateDownloading, Done: 5, Total: 10},
	}
	base := startUpdateServer(t, b)

	st, body := do(t, "GET", base+"/api/update/check", "user-session", "")
	if st != http.StatusOK {
		t.Fatalf("check: %d %s", st, body)
	}
	var info updater.Info
	if err := json.Unmarshal([]byte(body), &info); err != nil || !info.Available || info.Latest != "V4.7.0" || info.Mode != updater.ModeAuto {
		t.Fatalf("check body = %s (%v)", body, err)
	}
	if len(b.forced) != 1 || b.forced[0] {
		t.Errorf("forced = %v; a plain check must not force", b.forced)
	}
	if st, _ := do(t, "GET", base+"/api/update/check?force=1", "user-session", ""); st != http.StatusOK || !b.forced[len(b.forced)-1] {
		t.Errorf("?force=1 did not reach the app: %v", b.forced)
	}

	st, body = do(t, "GET", base+"/api/update/status", "user-session", "")
	var status updater.Status
	if st != http.StatusOK || json.Unmarshal([]byte(body), &status) != nil || status.State != updater.StateDownloading || status.Done != 5 {
		t.Fatalf("status = %d %s", st, body)
	}
	if st, _ := do(t, "POST", base+"/api/update/check", "user-session", "{}"); st != http.StatusMethodNotAllowed {
		t.Errorf("POST /check: %d, want 405", st)
	}
}

func TestUpdateEndpoints_ApplyIsAdminOnlyAndStartsOnce(t *testing.T) {
	b := &updateBridge{}
	base := startUpdateServer(t, b)

	// A restricted account cannot start an update.
	if st, _ := do(t, "POST", base+"/api/update/apply", "user-session", `{"kind":"app"}`); st != http.StatusForbidden {
		t.Fatalf("restricted user: %d, want 403", st)
	}
	if len(b.applied) != 0 {
		t.Fatalf("the app was asked to update on behalf of a restricted user: %v", b.applied)
	}
	// The admin can; it answers 202 at once.
	st, body := do(t, "POST", base+"/api/update/apply", "admin-session", `{"kind":"component","component":"llama"}`)
	if st != http.StatusAccepted || !strings.Contains(body, "started") {
		t.Fatalf("admin: %d %s", st, body)
	}
	if len(b.applied) != 1 || b.applied[0] != [2]string{"component", "llama"} {
		t.Fatalf("applied = %v", b.applied)
	}
}

func TestUpdateEndpoints_ApplyErrorsMapToStatusCodes(t *testing.T) {
	b := &updateBridge{applyErr: updater.ErrBusy}
	base := startUpdateServer(t, b)
	if st, _ := do(t, "POST", base+"/api/update/apply", "admin-session", `{"kind":"app"}`); st != http.StatusConflict {
		t.Errorf("busy: %d, want 409", st)
	}
	b.applyErr = fmt.Errorf("unknown update kind %q", "x")
	if st, _ := do(t, "POST", base+"/api/update/apply", "admin-session", `{"kind":"x"}`); st != http.StatusBadRequest {
		t.Errorf("bad request: %d, want 400", st)
	}
	if st, _ := do(t, "POST", base+"/api/update/apply", "admin-session", `not json`); st != http.StatusBadRequest {
		t.Errorf("bad json: %d, want 400", st)
	}
	if st, _ := do(t, "GET", base+"/api/update/apply", "admin-session", ""); st != http.StatusMethodNotAllowed {
		t.Errorf("GET apply: %d, want 405", st)
	}
}
