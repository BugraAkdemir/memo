package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"memo/internal/replcli"
)

func TestProviderListCmd_MarksActiveProvider(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/providers":
			json.NewEncoder(w).Encode([]replcli.ProviderConfig{
				{Type: "openai", Name: "OpenAI", Model: "gpt-4o", Enabled: false},
				{Type: "opencode-zen", Name: "OpenCode Zen", Model: "hy3-free", Enabled: true, Connected: true, Priority: 1},
			})
		case "/api/providers/active":
			json.NewEncoder(w).Encode(map[string]string{"provider": "OpenCode Zen"})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := providerListCmd(ctx, client); code != 0 {
		t.Fatalf("providerListCmd returned %d, want 0", code)
	}
}

func TestProviderAddCmd_SendsExpectedBody(t *testing.T) {
	var got replcli.ProviderConfig
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/api/providers" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&got)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	code := providerAddCmd(ctx, client, "opencode-zen", "OpenCode Zen", "hy3-free", "", "sk-secret", 1, true, false)
	if code != 0 {
		t.Fatalf("providerAddCmd returned %d, want 0", code)
	}
	if got.Type != "opencode-zen" || got.Name != "OpenCode Zen" || got.Model != "hy3-free" || got.APIKey != "sk-secret" || got.Priority != 1 || !got.Enabled {
		t.Errorf("unexpected request body: %+v", got)
	}
}

func TestProviderAddCmd_ActivateAlsoCallsSetActive(t *testing.T) {
	var activatedName string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPut && r.URL.Path == "/api/providers":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodPut && r.URL.Path == "/api/providers/active":
			var body map[string]string
			json.NewDecoder(r.Body).Decode(&body)
			activatedName = body["provider"]
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	code := providerAddCmd(ctx, client, "openai", "My OpenAI", "gpt-4o", "", "sk-x", 0, true, true)
	if code != 0 {
		t.Fatalf("providerAddCmd returned %d, want 0", code)
	}
	if activatedName != "My OpenAI" {
		t.Errorf("expected /api/providers/active to be called with %q, got %q", "My OpenAI", activatedName)
	}
}

func TestProviderActiveCmd_PrintsFriendlyMessageWhenNoneActive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"provider": ""})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	got := captureStdout(t, func() {
		if code := providerActiveCmd(ctx, client); code != 0 {
			t.Fatalf("providerActiveCmd returned %d, want 0", code)
		}
	})
	if !strings.Contains(got, "No active external provider") && !strings.Contains(got, "Aktif harici sağlayıcı yok") {
		t.Errorf("expected a friendly no-active-provider message, got: %q", got)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = old }()

	fn()

	w.Close()
	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}
