package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"memo/internal/replcli"
)

func TestRemoteStatusCmd_PrintsWarningOnAuthDisabled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"enabled":      true,
			"running":      true,
			"auth_mode":    "none",
			"auth_warning": "AUTH DISABLED",
			"addresses":    []string{"192.168.1.5:8090"},
		})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteStatusCmd(ctx, client); code != 0 {
		t.Fatalf("remoteStatusCmd returned %d, want 0", code)
	}
}

func TestHintIfUnauthorized_PrintsJournalHintOn401(t *testing.T) {
	got := captureStderr(t, func() {
		hintIfUnauthorized(fmt.Errorf("GET /api/remote-access: 401 Unauthorized (unauthorized)"))
	})
	if !strings.Contains(got, "journalctl --user -u memo.service") {
		t.Errorf("expected the journalctl hint for a 401 error, got: %q", got)
	}
}

func TestHintIfUnauthorized_SilentOnOtherErrors(t *testing.T) {
	got := captureStderr(t, func() {
		hintIfUnauthorized(fmt.Errorf("connection refused"))
	})
	if got != "" {
		t.Errorf("expected no output for a non-401 error, got: %q", got)
	}
}

func TestHintIfUnauthorized_NilErrorIsNoOp(t *testing.T) {
	got := captureStderr(t, func() {
		hintIfUnauthorized(nil)
	})
	if got != "" {
		t.Errorf("expected no output for a nil error, got: %q", got)
	}
}

func captureStderr(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stderr = w
	defer func() { os.Stderr = old }()

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestRemoteStatusCmd_FailsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteStatusCmd(ctx, client); code == 0 {
		t.Fatal("expected a non-zero exit code on a 401 response")
	}
}

func TestRemoteAddDeviceCmd_SendsTokenViaHeaderWhenSet(t *testing.T) {
	var gotToken string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotToken = r.Header.Get("X-Memo-Token")
		json.NewEncoder(w).Encode(map[string]string{"token": "memo-newdevicetoken"})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	client.SetToken("memo-existingtoken")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteAddDeviceCmd(ctx, client, "Laptop"); code != 0 {
		t.Fatalf("remoteAddDeviceCmd returned %d, want 0", code)
	}
	if gotToken != "memo-existingtoken" {
		t.Errorf("X-Memo-Token header = %q, want %q", gotToken, "memo-existingtoken")
	}
}

func TestRemoteListDevicesCmd_EmptyListSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteListDevicesCmd(ctx, client); code != 0 {
		t.Fatalf("remoteListDevicesCmd returned %d, want 0", code)
	}
}

func TestRemoteRotateTokenCmd_RevokesAndReAddsSameName(t *testing.T) {
	var revokedID, addedName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/remote-access/devices":
			json.NewEncoder(w).Encode([]replcli.RemoteDevice{{ID: "dev-1", Name: "Laptop"}})
		case r.Method == http.MethodDelete && strings.HasPrefix(r.URL.Path, "/api/remote-access/devices/"):
			revokedID = strings.TrimPrefix(r.URL.Path, "/api/remote-access/devices/")
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPost && r.URL.Path == "/api/remote-access/devices":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			addedName = body["name"]
			json.NewEncoder(w).Encode(map[string]string{"token": "memo-newtoken"})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteRotateTokenCmd(ctx, client, "dev-1"); code != 0 {
		t.Fatalf("remoteRotateTokenCmd returned %d, want 0", code)
	}
	if revokedID != "dev-1" {
		t.Errorf("revoked device ID = %q, want %q", revokedID, "dev-1")
	}
	if addedName != "Laptop" {
		t.Errorf("re-added device name = %q, want %q (must preserve the original name)", addedName, "Laptop")
	}
}

func TestRemoteRotateTokenCmd_UnknownDeviceIDFailsWithoutRevoking(t *testing.T) {
	revoked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/remote-access/devices":
			json.NewEncoder(w).Encode([]replcli.RemoteDevice{{ID: "dev-1", Name: "Laptop"}})
		case r.Method == http.MethodDelete:
			revoked = true
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteRotateTokenCmd(ctx, client, "does-not-exist"); code == 0 {
		t.Fatal("expected a non-zero exit code for an unknown device ID")
	}
	if revoked {
		t.Error("must not revoke anything when the given device ID was never found")
	}
}

func TestRemoteSetModeCmd_SendsExpectedBody(t *testing.T) {
	var body map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteSetModeCmd(ctx, client, "password", "admin", "hunter2"); code != 0 {
		t.Fatalf("remoteSetModeCmd returned %d, want 0", code)
	}
	if body["auth_mode"] != "password" || body["username"] != "admin" || body["password"] != "hunter2" {
		t.Errorf("unexpected request body: %+v", body)
	}
}

// ─── Faz 5.1.1 (yapacam.md): `memo remote list-accounts/add-account/delete-account` ───

func TestParsePermissions_EmptyStringIsAllFalse(t *testing.T) {
	got, err := parsePermissions("")
	if err != nil {
		t.Fatalf("parsePermissions(\"\"): %v", err)
	}
	if got != (replcli.AccountPermissions{}) {
		t.Errorf("parsePermissions(\"\") = %+v, want zero value", got)
	}
}

func TestParsePermissions_ParsesKnownNames(t *testing.T) {
	got, err := parsePermissions("models, memory,routines")
	if err != nil {
		t.Fatalf("parsePermissions: %v", err)
	}
	want := replcli.AccountPermissions{Models: true, Memory: true, Routines: true}
	if got != want {
		t.Errorf("parsePermissions(\"models, memory,routines\") = %+v, want %+v", got, want)
	}
}

// TestParsePermissions_RejectsUnknownName is the actual point of validating
// eagerly rather than silently ignoring: a typo'd permission name must
// fail loudly, not quietly grant fewer permissions than the operator
// thought they asked for.
func TestParsePermissions_RejectsUnknownName(t *testing.T) {
	if _, err := parsePermissions("models,modle"); err == nil {
		t.Fatal("expected an error for an unknown permission name")
	}
}

func TestRemoteAddAccountCmd_SendsExpectedBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	perms := replcli.AccountPermissions{Models: true, WhatsApp: true}
	if code := remoteAddAccountCmd(ctx, client, "kaya", "pw", "user", perms); code != 0 {
		t.Fatalf("remoteAddAccountCmd returned %d, want 0", code)
	}
	if body["username"] != "kaya" || body["password"] != "pw" || body["role"] != "user" {
		t.Errorf("unexpected request body: %+v", body)
	}
	sentPerms, _ := body["permissions"].(map[string]any)
	if sentPerms["models"] != true || sentPerms["whatsapp"] != true || sentPerms["memory"] != false {
		t.Errorf("unexpected permissions in request body: %+v", sentPerms)
	}
}

func TestRemoteAddAccountCmd_FailsOnServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "an account named \"kaya\" already exists", http.StatusBadRequest)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteAddAccountCmd(ctx, client, "kaya", "pw", "user", replcli.AccountPermissions{}); code == 0 {
		t.Fatal("expected a non-zero exit code when the server rejects account creation")
	}
}

func TestRemoteListAccountsCmd_EmptyListSucceeds(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteListAccountsCmd(ctx, client); code != 0 {
		t.Fatalf("remoteListAccountsCmd returned %d, want 0", code)
	}
}

func TestRemoteDeleteAccountCmd_SendsIDInPath(t *testing.T) {
	var gotPath, gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMethod = r.Method
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := remoteDeleteAccountCmd(ctx, client, "acc-1"); code != 0 {
		t.Fatalf("remoteDeleteAccountCmd returned %d, want 0", code)
	}
	if gotMethod != http.MethodDelete || gotPath != "/api/accounts/acc-1" {
		t.Errorf("got %s %s, want DELETE /api/accounts/acc-1", gotMethod, gotPath)
	}
}

// TestPermissionsSummary_AdminIgnoresStoredValue matches CreateAccount's
// own "perms discarded for admin" behavior — display must not claim an
// admin account has a partial/empty permission set just because that's
// what happens to be stored (never consulted) for it.
func TestPermissionsSummary_AdminIgnoresStoredValue(t *testing.T) {
	got := permissionsSummary("admin", replcli.AccountPermissions{})
	if !strings.Contains(got, "all") && !strings.Contains(got, "hepsi") {
		t.Errorf("permissionsSummary(admin, {}) = %q, want it to indicate full access", got)
	}
}

func TestPermissionsSummary_UserWithNothingCheckedSaysChatOnly(t *testing.T) {
	got := permissionsSummary("user", replcli.AccountPermissions{})
	if !strings.Contains(got, "chat") && !strings.Contains(got, "sohbet") {
		t.Errorf("permissionsSummary(user, {}) = %q, want it to indicate chat-only", got)
	}
}

func TestPermissionsSummary_UserListsGrantedNames(t *testing.T) {
	got := permissionsSummary("user", replcli.AccountPermissions{Models: true, Routines: true})
	if !strings.Contains(got, "models") || !strings.Contains(got, "routines") {
		t.Errorf("permissionsSummary = %q, want it to list both granted permissions", got)
	}
	if strings.Contains(got, "memory") {
		t.Errorf("permissionsSummary = %q, must not list a permission that wasn't granted", got)
	}
}
