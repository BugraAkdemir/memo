package replcli

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newModelsTestServer wires up the model/embedding/provider endpoints
// commands.go calls, backed by a fixed model list.
func newModelsTestServer(t *testing.T) (*httptest.Server, *[]map[string]any) {
	t.Helper()
	var starts []map[string]any

	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/local", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]LocalModel{
			{Filename: "llama-chat.gguf", Path: "/models/llama-chat.gguf", IsEmbedding: false},
			{Filename: "bge-embed.gguf", Path: "/models/bge-embed.gguf", IsEmbedding: true},
		})
	})
	mux.HandleFunc("/api/models/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelStatus{Running: false})
	})
	mux.HandleFunc("/api/models/embedding/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelStatus{Running: false})
	})
	mux.HandleFunc("/api/models/start", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["_endpoint"] = "start"
		starts = append(starts, body)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})
	mux.HandleFunc("/api/models/embedding/start", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		body["_endpoint"] = "embedding_start"
		starts = append(starts, body)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode([]ProviderConfig{})
			return
		}
		body := map[string]any{}
		json.NewDecoder(r.Body).Decode(&body)
		body["_endpoint"] = "providers"
		starts = append(starts, body)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})
	mux.HandleFunc("/api/providers/active", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			json.NewEncoder(w).Encode(map[string]string{"provider": ""})
			return
		}
		body := map[string]any{}
		json.NewDecoder(r.Body).Decode(&body)
		body["_endpoint"] = "providers_active"
		starts = append(starts, body)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	})

	return httptest.NewServer(mux), &starts
}

func newTestSession(t *testing.T, srv *httptest.Server) (*session, *bytes.Buffer) {
	t.Helper()
	var out bytes.Buffer
	s := &session{
		client:  NewClient(srv.URL),
		ctx:     context.Background(),
		out:     &out,
		scanner: bufio.NewScanner(strings.NewReader("")),
	}
	return s, &out
}

func TestHandleCommand_Help(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/help")

	if !strings.Contains(out.String(), "/models") || !strings.Contains(out.String(), "/connect") {
		t.Errorf("help text missing expected commands, got:\n%s", out.String())
	}
}

func TestHandleCommand_BareSlash_AliasesHelp(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/")

	if !strings.Contains(out.String(), "/models") || !strings.Contains(out.String(), "/connect") {
		t.Errorf("bare / should show the command list, got:\n%s", out.String())
	}
}

func TestHandleCommand_Models(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/models")

	got := out.String()
	if !strings.Contains(got, "llama-chat.gguf") || !strings.Contains(got, "bge-embed.gguf") {
		t.Errorf("models list missing local entries, got:\n%s", got)
	}
	if !strings.Contains(got, "Hiç sağlayıcı yapılandırılmamış") {
		t.Errorf("expected empty-providers message, got:\n%s", got)
	}
}

func TestHandleCommand_Models_ShowsProviders(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/local", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]LocalModel{})
	})
	mux.HandleFunc("/api/models/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelStatus{})
	})
	mux.HandleFunc("/api/models/embedding/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(ModelStatus{})
	})
	mux.HandleFunc("/api/providers", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ProviderConfig{
			{Type: "openai", Name: "openai", Model: "gpt-4o", Enabled: true},
			{Type: "custom", Name: "cli", Model: "llama-3", Enabled: true},
		})
	})
	mux.HandleFunc("/api/providers/active", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"provider": "cli"})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.handleCommand("/models")

	got := out.String()
	if !strings.Contains(got, "openai") || !strings.Contains(got, "gpt-4o") {
		t.Errorf("missing openai provider, got:\n%s", got)
	}
	if !strings.Contains(got, "cli") || !strings.Contains(got, "llama-3") {
		t.Errorf("missing cli provider, got:\n%s", got)
	}
}

func TestHandleCommand_Model_StartsMatchingChatModel(t *testing.T) {
	srv, starts := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/model chat")

	if !strings.Contains(out.String(), "başlatıldı") {
		t.Errorf("expected success message, got:\n%s", out.String())
	}
	if len(*starts) != 1 || (*starts)[0]["path"] != "/models/llama-chat.gguf" {
		t.Errorf("got starts %+v", *starts)
	}
}

func TestHandleCommand_Model_NoMatch(t *testing.T) {
	srv, starts := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/model nonexistent")

	if !strings.Contains(out.String(), "bulunamadı") {
		t.Errorf("expected not-found message, got:\n%s", out.String())
	}
	if len(*starts) != 0 {
		t.Errorf("expected no start call, got %+v", *starts)
	}
}

func TestHandleCommand_Embedding_DefaultsToFirstEmbeddingModel(t *testing.T) {
	srv, starts := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/embedding")

	if !strings.Contains(out.String(), "başlatıldı") {
		t.Errorf("expected success message, got:\n%s", out.String())
	}
	if len(*starts) != 1 || (*starts)[0]["path"] != "/models/bge-embed.gguf" {
		t.Errorf("got starts %+v", *starts)
	}
}

func TestHandleCommand_Connect(t *testing.T) {
	srv, starts := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/connect http://example.com/v1 sk-test gpt-4")

	if !strings.Contains(out.String(), "bağlanıldı") {
		t.Errorf("expected success message, got:\n%s", out.String())
	}
	if len(*starts) != 2 {
		t.Fatalf("expected 2 calls (providers + providers/active), got %d: %+v", len(*starts), *starts)
	}
	if (*starts)[0]["base_url"] != "http://example.com/v1" || (*starts)[0]["model"] != "gpt-4" {
		t.Errorf("providers call got %+v", (*starts)[0])
	}
	if (*starts)[1]["provider"] != "cli" {
		t.Errorf("providers/active call got %+v", (*starts)[1])
	}
}

func TestHandleCommand_Connect_MissingArgs(t *testing.T) {
	srv, starts := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/connect http://example.com/v1")

	if !strings.Contains(out.String(), "Kullanım") {
		t.Errorf("expected usage message, got:\n%s", out.String())
	}
	if len(*starts) != 0 {
		t.Errorf("expected no calls, got %+v", *starts)
	}
}

func TestHandleCommand_Unknown(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	s.handleCommand("/frobnicate")

	if !strings.Contains(out.String(), "Bilinmeyen komut") {
		t.Errorf("expected unknown-command message, got:\n%s", out.String())
	}
}
