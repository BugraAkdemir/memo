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

func TestHandleCommand_Exit_ReturnsFalseForNormalCommands(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, _ := newTestSession(t, srv)

	if exit := s.handleCommand("/help"); exit {
		t.Error("/help should never signal exit")
	}
}

func TestHandleCommand_Gui_MissingBinary(t *testing.T) {
	srv, _ := newModelsTestServer(t)
	defer srv.Close()
	s, out := newTestSession(t, srv)

	// No memo_flutter binary sits next to the test binary, so this must
	// report a clear error instead of panicking or hanging.
	s.handleCommand("/gui")

	if !strings.Contains(out.String(), "GUI bulunamadı") {
		t.Errorf("expected GUI-not-found message, got:\n%s", out.String())
	}
}

func TestHandleCommand_Clear(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/chat", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "chat-new"})
	})
	mux.HandleFunc("/api/chats/switch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/agent/enabled", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.projectPath = "/tmp/project"
	s.chatID = "chat-old"

	s.handleCommand("/clear")

	if s.chatID != "chat-new" {
		t.Errorf("chatID = %q, want chat-new", s.chatID)
	}
	if !strings.Contains(out.String(), "temizlendi") {
		t.Errorf("expected confirmation message, got:\n%s", out.String())
	}
}

func TestHandleCommand_Session_List_FiltersByProject(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]SessionInfo{
			{ID: "chat-a", Title: "Sohbet A", ProjectPath: "/tmp/project", UpdatedAt: "2026-01-02 10:00", MsgCount: 4},
			{ID: "chat-b", Title: "Sohbet B", ProjectPath: "/tmp/other", UpdatedAt: "2026-01-01 10:00", MsgCount: 1},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.projectPath = "/tmp/project"

	s.handleCommand("/session list")

	got := out.String()
	if !strings.Contains(got, "Sohbet A") {
		t.Errorf("expected Sohbet A in list, got:\n%s", got)
	}
	if strings.Contains(got, "Sohbet B") {
		t.Errorf("expected chats from other projects to be filtered out, got:\n%s", got)
	}
}

func TestHandleCommand_Session_SwitchByNumber(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]SessionInfo{
			{ID: "chat-a", Title: "Sohbet A", ProjectPath: "/tmp/project"},
		})
	})
	mux.HandleFunc("/api/chats/switch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/agent/enabled", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ChatMessage{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, _ := newTestSession(t, srv)
	s.projectPath = "/tmp/project"

	s.handleCommand("/session 1")

	if s.chatID != "chat-a" {
		t.Errorf("chatID = %q, want chat-a", s.chatID)
	}
}

func TestHandleCommand_Session_SwitchByName_NoMatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]SessionInfo{
			{ID: "chat-a", Title: "Sohbet A", ProjectPath: "/tmp/project"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.projectPath = "/tmp/project"

	s.handleCommand("/session bilinmeyen-sohbet")

	if !strings.Contains(out.String(), "bulunamadı") {
		t.Errorf("expected not-found message, got:\n%s", out.String())
	}
	if s.chatID != "" {
		t.Errorf("chatID should remain unset, got %q", s.chatID)
	}
}

func TestHandleCommand_Remote_AlreadyRunning(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/remote-access", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(RemoteStatus{
			Running: true, NgrokMode: true, NgrokToken: "tok", NgrokURL: "https://abc123.ngrok.io",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.handleCommand("/remote")

	got := out.String()
	if !strings.Contains(got, "https://abc123.ngrok.io") {
		t.Errorf("expected existing ngrok URL, got:\n%s", got)
	}
	if !strings.Contains(got, "erişebilir") {
		t.Errorf("expected exposure warning, got:\n%s", got)
	}
}

func TestHandleCommand_Remote_StartsWithExistingToken(t *testing.T) {
	var putBody map[string]any
	var getCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/remote-access", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			json.NewDecoder(r.Body).Decode(&putBody)
			json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
			return
		}
		getCalls++
		if getCalls == 1 {
			// Initial status check: not running yet, but token already saved.
			json.NewEncoder(w).Encode(RemoteStatus{NgrokToken: "saved-token"})
			return
		}
		// First poll after StartNgrok already reports the tunnel as live.
		json.NewEncoder(w).Encode(RemoteStatus{
			Running: true, NgrokMode: true, NgrokToken: "saved-token", NgrokURL: "https://xyz789.ngrok.io",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.handleCommand("/remote")

	got := out.String()
	if !strings.Contains(got, "https://xyz789.ngrok.io") {
		t.Errorf("expected new ngrok URL, got:\n%s", got)
	}
	if putBody["ngrok_token"] != "saved-token" {
		t.Errorf("expected saved token to be reused without prompting, got PUT body %+v", putBody)
	}
	if putBody["ngrok_mode"] != true {
		t.Errorf("expected ngrok_mode true, got %+v", putBody)
	}
}

func TestHandleCommand_Remote_PromptsForMissingToken(t *testing.T) {
	var putBody map[string]any
	var getCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/remote-access", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut {
			json.NewDecoder(r.Body).Decode(&putBody)
			json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
			return
		}
		getCalls++
		if getCalls == 1 {
			json.NewEncoder(w).Encode(RemoteStatus{}) // no token configured yet
			return
		}
		json.NewEncoder(w).Encode(RemoteStatus{
			Running: true, NgrokMode: true, NgrokURL: "https://fresh.ngrok.io",
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	s := &session{
		client:  NewClient(srv.URL),
		ctx:     context.Background(),
		out:     &out,
		scanner: bufio.NewScanner(strings.NewReader("typed-token-123\n")),
	}
	s.handleCommand("/remote")

	got := out.String()
	if !strings.Contains(got, "https://fresh.ngrok.io") {
		t.Errorf("expected new ngrok URL, got:\n%s", got)
	}
	if putBody["ngrok_token"] != "typed-token-123" {
		t.Errorf("expected typed token to be sent, got PUT body %+v", putBody)
	}
}

func TestHandleCommand_ModelDownload_NoResults(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/search", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]HFModelResult{})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s, out := newTestSession(t, srv)
	s.handleCommand("/model-download nonexistent-model-xyz")

	if !strings.Contains(out.String(), "Sonuç bulunamadı") {
		t.Errorf("expected no-results message, got:\n%s", out.String())
	}
}
