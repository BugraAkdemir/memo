// SPDX-License-Identifier: AGPL-3.0-or-later

package geminisub

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

func modelsServer(t *testing.T, pages ...string) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&hits, 1)) - 1
		if n >= len(pages) {
			n = len(pages) - 1
		}
		_, _ = w.Write([]byte(pages[n]))
	}))
	prev := modelsEndpoint
	modelsEndpoint = srv.URL
	t.Cleanup(func() {
		modelsEndpoint = prev
		srv.Close()
	})
	return srv, &hits
}

func connectedMgr(t *testing.T) *Manager {
	t.Helper()
	m := newTestManager(t)
	m.token = &oauth2.Token{AccessToken: "at", Expiry: time.Now().Add(time.Hour)}
	return m
}

func TestFetchModels_FilterAndPaginate(t *testing.T) {
	page1 := `{"models":[
		{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent","countTokens"]},
		{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]},
		{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]},
		{"name":"models/gemini-embedding-001","supportedGenerationMethods":["generateContent","embedContent"]},
		{"name":"models/aqa","supportedGenerationMethods":["generateAnswer"]}
	],"nextPageToken":"p2"}`
	page2 := `{"models":[
		{"name":"models/gemini-2.0-flash","supportedGenerationMethods":["generateContent"]},
		{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]}
	]}`
	_, hits := modelsServer(t, page1, page2)

	got, err := connectedMgr(t).fetchModels(context.Background())
	if err != nil {
		t.Fatalf("fetchModels: %v", err)
	}
	want := []string{"gemini-2.0-flash", "gemini-2.5-flash", "gemini-2.5-pro"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("fetchModels = %v, want %v", got, want)
	}
	if *hits != 2 {
		t.Errorf("expected 2 page requests, got %d", *hits)
	}
}

func TestModels_CachesAndRefreshes(t *testing.T) {
	body := `{"models":[{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent"]}]}`
	_, hits := modelsServer(t, body)

	prevTTL := modelsCacheTTL
	modelsCacheTTL = 40 * time.Millisecond
	defer func() { modelsCacheTTL = prevTTL }()

	m := connectedMgr(t)
	if _, err := m.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *hits != 1 {
		t.Fatalf("second call within TTL hit the network %d times, want 1", *hits)
	}
	time.Sleep(60 * time.Millisecond)
	if _, err := m.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *hits != 2 {
		t.Errorf("call after TTL did not refetch (hits=%d)", *hits)
	}
}

func TestModels_FallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":"SERVICE_DISABLED"}`, http.StatusForbidden)
	}))
	defer srv.Close()
	prev := modelsEndpoint
	modelsEndpoint = srv.URL
	defer func() { modelsEndpoint = prev }()

	got, err := connectedMgr(t).Models(context.Background())
	if err != nil {
		t.Fatalf("Models returned an error instead of falling back: %v", err)
	}
	if !reflect.DeepEqual(got, fallbackModels) {
		t.Errorf("fallback = %v, want %v", got, fallbackModels)
	}
}

func TestModels_NotConnected(t *testing.T) {
	if _, err := newTestManager(t).Models(context.Background()); err != ErrNotConnected {
		t.Errorf("err = %v, want ErrNotConnected", err)
	}
}

func TestCachedModelsAndInvalidate(t *testing.T) {
	body := `{"models":[{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]}]}`
	modelsServer(t, body)
	m := connectedMgr(t)

	if got := m.CachedModels(); got != nil {
		t.Errorf("CachedModels before any fetch = %v, want nil", got)
	}
	if _, err := m.Models(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := m.CachedModels(); !reflect.DeepEqual(got, []string{"gemini-2.5-flash"}) {
		t.Errorf("CachedModels after fetch = %v", got)
	}
	m.invalidateModels()
	if got := m.CachedModels(); got != nil {
		t.Errorf("CachedModels after invalidate = %v, want nil", got)
	}
}

