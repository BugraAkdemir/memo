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

func TestAgentStatusCmd_PrintsBothFlags(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/agent/enabled":
			json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
		case "/api/agent/auto-permission":
			json.NewEncoder(w).Encode(map[string]bool{"enabled": false})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := agentStatusCmd(ctx, client); code != 0 {
		t.Fatalf("agentStatusCmd returned %d, want 0", code)
	}
}

func TestAgentStatusCmd_FailsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := agentStatusCmd(ctx, client); code == 0 {
		t.Fatal("expected a non-zero exit code on a 401 response")
	}
}

func TestAgentSetEnabledCmd_SendsExpectedBody(t *testing.T) {
	var body map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/agent/enabled" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := agentSetEnabledCmd(ctx, client, true); code != 0 {
		t.Fatalf("agentSetEnabledCmd returned %d, want 0", code)
	}
	if !body["enabled"] {
		t.Errorf("unexpected request body: %+v, want enabled=true", body)
	}
}

func TestAgentSetAutoPermissionCmd_SendsExpectedBody(t *testing.T) {
	var body map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/agent/auto-permission" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := agentSetAutoPermissionCmd(ctx, client, false); code != 0 {
		t.Fatalf("agentSetAutoPermissionCmd returned %d, want 0", code)
	}
	if body["enabled"] {
		t.Errorf("unexpected request body: %+v, want enabled=false", body)
	}
}
