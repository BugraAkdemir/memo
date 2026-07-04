package replcli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClient_ListLocalModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/local" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode([]LocalModel{
			{Filename: "chat.gguf", Path: "/models/chat.gguf", IsEmbedding: false},
			{Filename: "embed.gguf", Path: "/models/embed.gguf", IsEmbedding: true},
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	models, err := c.ListLocalModels(context.Background())
	if err != nil {
		t.Fatalf("ListLocalModels() error = %v", err)
	}
	if len(models) != 2 || models[0].Filename != "chat.gguf" || !models[1].IsEmbedding {
		t.Errorf("got %+v", models)
	}
}

func TestClient_ModelStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelStatus{Running: true, ModelName: "chat.gguf", Port: 8081})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	status, err := c.ModelStatus(context.Background())
	if err != nil {
		t.Fatalf("ModelStatus() error = %v", err)
	}
	if !status.Running || status.ModelName != "chat.gguf" {
		t.Errorf("got %+v", status)
	}
}

func TestClient_EmbeddingStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/embedding/status" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewEncoder(w).Encode(ModelStatus{Running: false})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	status, err := c.EmbeddingStatus(context.Background())
	if err != nil {
		t.Fatalf("EmbeddingStatus() error = %v", err)
	}
	if status.Running {
		t.Errorf("got %+v, want Running=false", status)
	}
}

func TestClient_StartModel(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/start" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.StartModel(context.Background(), "/models/chat.gguf", 0, 0, -1); err != nil {
		t.Fatalf("StartModel() error = %v", err)
	}
	if gotBody["path"] != "/models/chat.gguf" {
		t.Errorf("path = %v, want /models/chat.gguf", gotBody["path"])
	}
	if gotBody["gpu_layers"] != float64(-1) {
		t.Errorf("gpu_layers = %v, want -1", gotBody["gpu_layers"])
	}
}

func TestClient_StartEmbedding(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/models/embedding/start" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.StartEmbedding(context.Background(), "/models/embed.gguf", -1); err != nil {
		t.Fatalf("StartEmbedding() error = %v", err)
	}
	if gotBody["path"] != "/models/embed.gguf" {
		t.Errorf("path = %v, want /models/embed.gguf", gotBody["path"])
	}
}

func TestClient_UpdateProvider(t *testing.T) {
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers" || r.Method != http.MethodPut {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	cfg := ProviderConfig{Type: "custom", Name: "cli", BaseURL: "http://x", Model: "m", Enabled: true}
	if err := c.UpdateProvider(context.Background(), cfg); err != nil {
		t.Fatalf("UpdateProvider() error = %v", err)
	}
	if gotBody["base_url"] != "http://x" || gotBody["model"] != "m" {
		t.Errorf("got %+v", gotBody)
	}
}

func TestClient_SetActiveProvider(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/providers/active" || r.Method != http.MethodPut {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.SetActiveProvider(context.Background(), "cli"); err != nil {
		t.Fatalf("SetActiveProvider() error = %v", err)
	}
	if gotBody["provider"] != "cli" {
		t.Errorf("provider = %q, want cli", gotBody["provider"])
	}
}
