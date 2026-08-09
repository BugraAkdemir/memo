package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
