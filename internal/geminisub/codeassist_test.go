// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func connectedManager(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "at", Expiry: time.Now().Add(time.Hour)}
	return m
}

func withEndpoint(t *testing.T, baseURL string) {
	t.Helper()
	// Mirror the real endpoint's shape: a path segment before the ":method"
	// suffix, so "<base>:loadCodeAssist" parses as a URL.
	t.Setenv(envEndpoint, baseURL+"/v1internal")
}

func TestEnsureBootstrap_LoadOnly(t *testing.T) {
	var loads int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ":loadCodeAssist") {
			atomic.AddInt32(&loads, 1)
			_, _ = w.Write([]byte(`{"cloudaicompanionProject":"proj-42","currentTier":{"id":"legacy-tier"}}`))
			return
		}
		t.Errorf("unexpected call to %s", r.URL.Path)
		http.Error(w, "no", http.StatusInternalServerError)
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	m := connectedManager(t)
	b, err := m.ensureBootstrap(context.Background())
	if err != nil {
		t.Fatalf("ensureBootstrap: %v", err)
	}
	if b.ProjectID != "proj-42" || b.TierID != "legacy-tier" {
		t.Errorf("bootstrap = %+v", b)
	}

	// Second call is cached — no more HTTP.
	if _, err := m.ensureBootstrap(context.Background()); err != nil {
		t.Fatalf("second ensureBootstrap: %v", err)
	}
	if n := atomic.LoadInt32(&loads); n != 1 {
		t.Errorf("loadCodeAssist called %d times, want 1 (cached)", n)
	}

	m.invalidateBootstrap()
	if _, err := m.ensureBootstrap(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := atomic.LoadInt32(&loads); n != 2 {
		t.Errorf("after invalidate, loadCodeAssist called %d times, want 2", n)
	}
}

func TestEnsureBootstrap_OnboardFlow(t *testing.T) {
	prev := onboardPollInterval
	onboardPollInterval = 10 * time.Millisecond
	defer func() { onboardPollInterval = prev }()

	var onboards, polls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ":loadCodeAssist"):
			// No project yet -> caller must onboard.
			_, _ = w.Write([]byte(`{"allowedTiers":[{"id":"free-tier","isDefault":true}]}`))
		case strings.HasSuffix(r.URL.Path, ":onboardUser"):
			atomic.AddInt32(&onboards, 1)
			_, _ = w.Write([]byte(`{"name":"operations/op-1","done":false}`))
		case r.Method == http.MethodGet && r.URL.Path == "/v1internal/operations/op-1":
			if atomic.AddInt32(&polls, 1) < 2 {
				_, _ = w.Write([]byte(`{"name":"operations/op-1","done":false}`))
				return
			}
			_, _ = w.Write([]byte(`{"done":true,"response":{"cloudaicompanionProject":{"id":"proj-onboarded"}}}`))
		default:
			http.Error(w, "no", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	m := connectedManager(t)
	b, err := m.ensureBootstrap(context.Background())
	if err != nil {
		t.Fatalf("ensureBootstrap: %v", err)
	}
	if b.ProjectID != "proj-onboarded" {
		t.Errorf("project = %q, want proj-onboarded", b.ProjectID)
	}
	if b.TierID != "free-tier" {
		t.Errorf("tier = %q, want free-tier (from default allowedTier)", b.TierID)
	}
	// The regression: onboardUser used to be re-POSTed to "poll", which ends in
	// a 429 RESOURCE_EXHAUSTED. It must be sent once and the operation GET-polled.
	if n := atomic.LoadInt32(&onboards); n != 1 {
		t.Errorf("onboardUser POSTed %d times, want exactly 1", n)
	}
	if n := atomic.LoadInt32(&polls); n < 2 {
		t.Errorf("operation polled %d times, want >= 2", n)
	}
}

func TestEnsureBootstrap_OnboardInline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, ":loadCodeAssist"):
			_, _ = w.Write([]byte(`{"allowedTiers":[{"id":"free-tier","isDefault":true}]}`))
		case strings.HasSuffix(r.URL.Path, ":onboardUser"):
			_, _ = w.Write([]byte(`{"done":true,"response":{"cloudaicompanionProject":{"id":"proj-inline"}}}`))
		default:
			http.Error(w, "no", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	b, err := connectedManager(t).ensureBootstrap(context.Background())
	if err != nil || b.ProjectID != "proj-inline" {
		t.Fatalf("bootstrap = %+v, err = %v", b, err)
	}
}

func TestEnsureBootstrap_NotConnected(t *testing.T) {
	m := newTestManager(t)
	if _, err := m.ensureBootstrap(context.Background()); err != ErrNotConnected {
		t.Errorf("err = %v, want ErrNotConnected", err)
	}
}

func TestEnsureBootstrap_LoadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"PERMISSION_DENIED"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	withEndpoint(t, srv.URL)

	m := connectedManager(t)
	_, err := m.ensureBootstrap(context.Background())
	if err == nil || !strings.Contains(err.Error(), "loadCodeAssist") {
		t.Errorf("err = %v, want a loadCodeAssist failure", err)
	}
}
