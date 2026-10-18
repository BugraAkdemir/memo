package replcli

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

// optionsBackend records the body of /api/send/stream and every call that
// would change backend-wide state.
func optionsBackend(t *testing.T) (*httptest.Server, func() (map[string]any, []string)) {
	t.Helper()
	var mu sync.Mutex
	var sent map[string]any
	var global []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch r.URL.Path {
		case "/api/send/stream":
			_ = json.NewDecoder(r.Body).Decode(&sent)
			w.Header().Set("Content-Type", "text/event-stream")
			_, _ = w.Write([]byte("data: {\"content\":\"tamam\",\"done\":true,\"finish_reason\":\"stop\"}\n\n"))
		case "/api/chats/switch", "/api/agent/enabled", "/api/websearch", "/api/agent/auto-permission":
			if r.Method != http.MethodGet || r.URL.Path == "/api/chats/switch" {
				global = append(global, r.Method+" "+r.URL.Path)
			}
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	return srv, func() (map[string]any, []string) {
		mu.Lock()
		defer mu.Unlock()
		return sent, append([]string(nil), global...)
	}
}

// What this session wants travels WITH the request; nothing backend-wide is
// written, so a desktop app on the same backend keeps its own settings.
func TestSendMessage_SendsTheSessionsOwnOptionsAndWritesNothingGlobal(t *testing.T) {
	srv, seen := optionsBackend(t)
	defer srv.Close()
	s, _ := newTestSession(t, srv)
	s.chatID = "chat-1"
	s.autoPermission = true
	s.webTools = false

	s.sendMessage("merhaba")

	sent, global := seen()
	if sent == nil {
		t.Fatal("no request reached /api/send/stream")
	}
	if sent["chat_id"] != "chat-1" {
		t.Errorf("chat_id = %v", sent["chat_id"])
	}
	opts, _ := sent["options"].(map[string]any)
	if opts["auto_permission"] != true || opts["web_tools"] != false {
		t.Errorf("options = %v, want auto_permission=true web_tools=false", opts)
	}
	if len(global) != 0 {
		t.Errorf("the REPL wrote backend-wide state: %v", global)
	}
}

// A fresh session: tools on, web on, auto-permission off — coding defaults.
func TestSessionDefaults_WebOnAutoPermissionOff(t *testing.T) {
	srv, seen := optionsBackend(t)
	defer srv.Close()
	s, _ := newTestSession(t, srv)
	s.chatID = "c"
	s.webTools = true // what Run() initialises

	s.sendMessage("x")
	sent, _ := seen()
	opts, _ := sent["options"].(map[string]any)
	if opts["web_tools"] != true || opts["auto_permission"] != false {
		t.Errorf("defaults sent = %v", opts)
	}
}

// Shift+Tab flips the session's own flag and calls no backend endpoint.
func TestToggleAutoPermission_IsLocal(t *testing.T) {
	srv, seen := optionsBackend(t)
	defer srv.Close()
	s, _ := newTestSession(t, srv)

	if got := s.toggleAutoPermission(false); !got || !s.autoPermission {
		t.Fatalf("toggle from off = %v (session %v), want on", got, s.autoPermission)
	}
	if got := s.toggleAutoPermission(true); got || s.autoPermission {
		t.Fatalf("toggle from on = %v (session %v), want off", got, s.autoPermission)
	}
	if _, global := seen(); len(global) != 0 {
		t.Errorf("toggling called the backend: %v", global)
	}
}

// Two sessions on one backend keep their own settings.
func TestTwoSessionsKeepTheirOwnSettings(t *testing.T) {
	srv, _ := optionsBackend(t)
	defer srv.Close()
	a, _ := newTestSession(t, srv)
	b, _ := newTestSession(t, srv)
	a.webTools, b.webTools = true, true

	a.handleCommand("/web off")
	a.toggleAutoPermission(false)

	if !b.webTools || b.autoPermission {
		t.Errorf("session B changed because session A did: web=%v auto=%v", b.webTools, b.autoPermission)
	}
	if a.webTools || !a.autoPermission {
		t.Errorf("session A lost its own settings: web=%v auto=%v", a.webTools, a.autoPermission)
	}
}
