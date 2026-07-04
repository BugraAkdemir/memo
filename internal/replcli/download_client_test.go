package replcli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_SearchModels(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/search" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode([]HFModelResult{
			{ID: "TheBloke/Llama-2-7B-GGUF", Downloads: 100000, Likes: 500},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	results, err := c.SearchModels(context.Background(), "llama")
	if err != nil {
		t.Fatalf("SearchModels() error = %v", err)
	}
	if len(results) != 1 || results[0].ID != "TheBloke/Llama-2-7B-GGUF" {
		t.Errorf("got %+v", results)
	}
	if gotBody["query"] != "llama" {
		t.Errorf("query = %q, want llama", gotBody["query"])
	}
}

func TestClient_ModelFiles(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/files" || r.URL.Query().Get("repo") != "TheBloke/Llama-2-7B-GGUF" {
			t.Errorf("unexpected request %s?%s", r.URL.Path, r.URL.RawQuery)
		}
		json.NewEncoder(w).Encode([]GGUFFile{
			{Filename: "llama-2-7b.Q4_K_M.gguf", Size: 4_000_000_000},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	files, err := c.ModelFiles(context.Background(), "TheBloke/Llama-2-7B-GGUF")
	if err != nil {
		t.Fatalf("ModelFiles() error = %v", err)
	}
	if len(files) != 1 || files[0].Filename != "llama-2-7b.Q4_K_M.gguf" {
		t.Errorf("got %+v", files)
	}
}

func TestClient_DownloadModel(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/download" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.DownloadModel(context.Background(), "TheBloke/Llama-2-7B-GGUF", "llama-2-7b.Q4_K_M.gguf", 4_000_000_000); err != nil {
		t.Fatalf("DownloadModel() error = %v", err)
	}
	if gotBody["repo_id"] != "TheBloke/Llama-2-7B-GGUF" || gotBody["filename"] != "llama-2-7b.Q4_K_M.gguf" {
		t.Errorf("got %+v", gotBody)
	}
}

func TestClient_DownloadProgress(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(DownloadProgress{Active: true, Percent: 42.5, Speed: "3.1 MB/s"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	p, err := c.DownloadProgress(context.Background())
	if err != nil {
		t.Fatalf("DownloadProgress() error = %v", err)
	}
	if !p.Active || p.Percent != 42.5 {
		t.Errorf("got %+v", p)
	}
}
