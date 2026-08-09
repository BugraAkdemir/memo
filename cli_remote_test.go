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
