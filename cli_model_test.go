package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/replcli"
)

func TestModelListCmd_PrintsLocalModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]replcli.LocalModel{
			{Filename: "nomic.gguf", Size: 84106624, IsEmbedding: true, Path: "/x/nomic.gguf"},
		})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := modelListCmd(ctx, client); code != 0 {
		t.Fatalf("modelListCmd returned %d, want 0", code)
	}
}

func TestModelStatusCmd_ReportsBothChatAndEmbedding(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/models/status":
			json.NewEncoder(w).Encode(replcli.ModelStatus{Running: true, ModelName: "chat-model", Port: 8081})
		case "/api/models/embedding/status":
			json.NewEncoder(w).Encode(replcli.ModelStatus{Running: false})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := modelStatusCmd(ctx, client); code != 0 {
		t.Fatalf("modelStatusCmd returned %d, want 0", code)
	}
}

func TestModelSearchCmd_FailsOnUnauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := modelSearchCmd(ctx, client, "nomic"); code == 0 {
		t.Fatal("expected a non-zero exit code on a 401 response")
	}
}

func TestModelFilesCmd_ListsGGUFFiles(t *testing.T) {
	var gotRepo string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotRepo = r.URL.Query().Get("repo")
		json.NewEncoder(w).Encode([]replcli.GGUFFile{{Filename: "a.gguf", Size: 123}})
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := modelFilesCmd(ctx, client, "nomic-ai/nomic-embed-text-v1.5-GGUF"); code != 0 {
		t.Fatalf("modelFilesCmd returned %d, want 0", code)
	}
	if gotRepo != "nomic-ai/nomic-embed-text-v1.5-GGUF" {
		t.Errorf("repo query param = %q, want the full repo id", gotRepo)
	}
}

// TestModelDownloadCmd_PollsUntilNoLongerActive drives modelDownloadCmd
// against a fake server that reports the download as active for the first
// couple of progress polls, then stops listing it (the real backend's
// signal that a download finished — see modelDownloadCmd's "no longer in
// the active list" branch).
func TestModelDownloadCmd_PollsUntilNoLongerActive(t *testing.T) {
	var pollCount atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/models/download":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/models/download/progress":
			n := pollCount.Add(1)
			if n <= 2 {
				json.NewEncoder(w).Encode([]replcli.ModelDownloadProgress{
					{Active: true, RepoID: "r", Filename: "f.gguf", Percent: float64(n) * 40},
				})
				return
			}
			json.NewEncoder(w).Encode([]replcli.ModelDownloadProgress{})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if code := modelDownloadCmd(ctx, client, "r", "f.gguf", 1000); code != 0 {
		t.Fatalf("modelDownloadCmd returned %d, want 0", code)
	}
	if pollCount.Load() < 3 {
		t.Errorf("expected at least 3 progress polls before the download dropped off the active list, got %d", pollCount.Load())
	}
}

func TestModelDownloadCmd_ReportsDownloadError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/models/download":
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/api/models/download/progress":
			json.NewEncoder(w).Encode([]replcli.ModelDownloadProgress{
				{Active: true, RepoID: "r", Filename: "f.gguf", Error: "disk full"},
			})
		}
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if code := modelDownloadCmd(ctx, client, "r", "f.gguf", 1000); code == 0 {
		t.Fatal("expected a non-zero exit code when the backend reports a download error")
	}
}

func TestModelStartEmbeddingCmd_SendsExpectedBody(t *testing.T) {
	var body map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/models/embedding/start" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&body)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	client := replcli.NewClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	if code := modelStartEmbeddingCmd(ctx, client, "/x/model.gguf", -1); code != 0 {
		t.Fatalf("modelStartEmbeddingCmd returned %d, want 0", code)
	}
	if body["path"] != "/x/model.gguf" {
		t.Errorf("unexpected request body: %+v", body)
	}
}
