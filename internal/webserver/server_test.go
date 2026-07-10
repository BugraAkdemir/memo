package webserver

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var errChatNotFound = errors.New("chat not found")
var errTranscribeFail = errors.New("transcription failed")

// ─── Mock Bridge ──────────────────────────────────────────────────

type mockBridge struct {
	sendMsg     func(msg string) string
	chats       func() interface{}
	newChat     func() string
	switchChat  func(id string) error
	deleteChat  func(id string) error
	renameChat  func(id, title string) error
	activeChat  func() string
	messages    func() interface{}
	updateMsg   func(index int, content string) error
	deleteMsg   func(index int) error
	status      func() interface{}
	toggleIncog func(enabled bool)
	incog       bool
	memoryCount func() int
	transcribe  func(audio []byte) (string, error)

	registerClient   func() string
	heartbeatClient  func(clientID string) error
	unregisterClient func(clientID string)
}

func (m *mockBridge) SendMessage(userMsg string) string {
	if m.sendMsg != nil {
		return m.sendMsg(userMsg)
	}
	return "mock reply"
}
func (m *mockBridge) SendMessageWithImage(userMsg, imagePath string) string {
	return m.SendMessage(userMsg)
}
func (m *mockBridge) SendMessageWithFile(userMsg, filePath string) string {
	return m.SendMessage(userMsg)
}
func (m *mockBridge) NewChat() string                   { return m.newChat() }
func (m *mockBridge) WebListChats() interface{}         { return m.chats() }
func (m *mockBridge) SwitchChat(id string) error        { return m.switchChat(id) }
func (m *mockBridge) DeleteChat(id string) error        { return m.deleteChat(id) }
func (m *mockBridge) RenameChat(id, title string) error { return m.renameChat(id, title) }
func (m *mockBridge) WebGetActiveMessages() interface{} { return m.messages() }
func (m *mockBridge) GetActiveChatID() string           { return m.activeChat() }
func (m *mockBridge) WebCheckConnection() interface{}   { return m.status() }
func (m *mockBridge) GetMemoryCount() int               { return m.memoryCount() }
func (m *mockBridge) ToggleIncognito(enabled bool)      { m.toggleIncog(enabled) }
func (m *mockBridge) GetIncognito() bool                { return m.incog }
func (m *mockBridge) TranscribeAudio(audioData []byte) (string, error) {
	return m.transcribe(audioData)
}
func (m *mockBridge) UpdateMessage(index int, content string) error {
	return m.updateMsg(index, content)
}
func (m *mockBridge) DeleteMessage(index int) error { return m.deleteMsg(index) }
func (m *mockBridge) RegisterClient() string {
	if m.registerClient != nil {
		return m.registerClient()
	}
	return "mock-client"
}
func (m *mockBridge) HeartbeatClient(clientID string) error {
	if m.heartbeatClient != nil {
		return m.heartbeatClient(clientID)
	}
	return nil
}
func (m *mockBridge) UnregisterClient(clientID string) {
	if m.unregisterClient != nil {
		m.unregisterClient(clientID)
	}
}

func newMockServer(m *mockBridge) *Server {
	s := New(m)
	s.port = 8090
	s.listenAddr = "127.0.0.1"
	return s
}

// ─── Handler Tests ────────────────────────────────────────────────

func TestHandleSend(t *testing.T) {
	m := &mockBridge{sendMsg: func(msg string) string { return "hello " + msg }}
	s := newMockServer(m)

	body := `{"message":"world"}`
	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	s.handleSend(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp["reply"] != "hello world" {
		t.Errorf("got reply %q, want %q", resp["reply"], "hello world")
	}
}

func TestHandleSend_MethodNotAllowed(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodGet, "/api/send", nil)
	w := httptest.NewRecorder()
	s.handleSend(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleSend_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/send", strings.NewReader(`not json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSend(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleChats(t *testing.T) {
	expected := []map[string]string{{"id": "abc", "title": "Test"}}
	m := &mockBridge{chats: func() interface{} { return expected }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodGet, "/api/chats", nil)
	w := httptest.NewRecorder()
	s.handleChats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var got []map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got) != 1 || got[0]["id"] != "abc" {
		t.Errorf("got %+v, want %+v", got, expected)
	}
}

func TestHandleNewChat(t *testing.T) {
	m := &mockBridge{newChat: func() string { return "chat-42" }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/chats/new", nil)
	w := httptest.NewRecorder()
	s.handleNewChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "chat-42" {
		t.Errorf("got id %q, want %q", resp["id"], "chat-42")
	}
}

func TestHandleNewChat_MethodNotAllowed(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodGet, "/api/chats/new", nil)
	w := httptest.NewRecorder()
	s.handleNewChat(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got status %d, want %d", w.Code, http.StatusMethodNotAllowed)
	}
}

func TestHandleSwitchChat(t *testing.T) {
	var switched string
	m := &mockBridge{switchChat: func(id string) error { switched = id; return nil }}
	s := newMockServer(m)

	body := `{"id":"chat-99"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSwitchChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if switched != "chat-99" {
		t.Errorf("got switched %q, want %q", switched, "chat-99")
	}
}

func TestHandleSwitchChat_Error(t *testing.T) {
	m := &mockBridge{switchChat: func(id string) error { return errChatNotFound }}
	s := newMockServer(m)

	body := `{"id":"missing"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats/switch", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSwitchChat(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleDeleteChat(t *testing.T) {
	var deleted string
	m := &mockBridge{deleteChat: func(id string) error { deleted = id; return nil }}
	s := newMockServer(m)

	body := `{"id":"chat-77"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if deleted != "chat-77" {
		t.Errorf("got deleted %q, want %q", deleted, "chat-77")
	}
}

func TestHandleDeleteChat_Error(t *testing.T) {
	m := &mockBridge{deleteChat: func(id string) error { return errChatNotFound }}
	s := newMockServer(m)
	body := `{"id":"missing"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteChat(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

func TestHandleRenameChat(t *testing.T) {
	var renamedID, renamedTitle string
	m := &mockBridge{renameChat: func(id, title string) error {
		renamedID = id
		renamedTitle = title
		return nil
	}}
	s := newMockServer(m)

	body := `{"id":"c1","title":"New Title"}`
	req := httptest.NewRequest(http.MethodPost, "/api/chats/rename", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleRenameChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if renamedID != "c1" || renamedTitle != "New Title" {
		t.Errorf("got (%q, %q), want (c1, New Title)", renamedID, renamedTitle)
	}
}

func TestHandleActiveChat(t *testing.T) {
	m := &mockBridge{activeChat: func() string { return "chat-1" }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodGet, "/api/chats/active", nil)
	w := httptest.NewRecorder()
	s.handleActiveChat(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["id"] != "chat-1" {
		t.Errorf("got id %q, want %q", resp["id"], "chat-1")
	}
}

func TestHandleMessages(t *testing.T) {
	expected := []map[string]string{{"role": "user", "content": "hi"}}
	m := &mockBridge{messages: func() interface{} { return expected }}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodGet, "/api/messages", nil)
	w := httptest.NewRecorder()
	s.handleMessages(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var got []map[string]string
	json.NewDecoder(w.Body).Decode(&got)
	if len(got) != 1 || got[0]["role"] != "user" {
		t.Errorf("got %+v, want %+v", got, expected)
	}
}

func TestHandleUpdateMessage(t *testing.T) {
	var updatedIdx int
	var updatedContent string
	m := &mockBridge{updateMsg: func(index int, content string) error {
		updatedIdx = index
		updatedContent = content
		return nil
	}}
	s := newMockServer(m)

	body := `{"index":2,"content":"edited"}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages/update", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleUpdateMessage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if updatedIdx != 2 || updatedContent != "edited" {
		t.Errorf("got (%d, %q), want (2, edited)", updatedIdx, updatedContent)
	}
}

func TestHandleDeleteMessage(t *testing.T) {
	var deletedIdx int
	m := &mockBridge{deleteMsg: func(index int) error { deletedIdx = index; return nil }}
	s := newMockServer(m)

	body := `{"index":3}`
	req := httptest.NewRequest(http.MethodPost, "/api/messages/delete", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteMessage(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if deletedIdx != 3 {
		t.Errorf("got index %d, want %d", deletedIdx, 3)
	}
}

func TestHandleStatus(t *testing.T) {
	m := &mockBridge{
		status:      func() interface{} { return map[string]string{"ok": "true"} },
		memoryCount: func() int { return 42 },
	}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()
	s.handleStatus(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["memory_count"] != float64(42) {
		t.Errorf("got memory_count %v, want 42", resp["memory_count"])
	}
	if resp["port"] != float64(8090) {
		t.Errorf("got port %v, want 8090", resp["port"])
	}
}

func TestHandleIncognito(t *testing.T) {
	var toggled bool
	m := &mockBridge{toggleIncog: func(enabled bool) { toggled = enabled }}
	s := newMockServer(m)

	body := `{"enabled":true}`
	req := httptest.NewRequest(http.MethodPost, "/api/incognito", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIncognito(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if !toggled {
		t.Error("expected incognito to be toggled on")
	}
}

func TestHandleIncognito_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/incognito", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleIncognito(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleTranscribe(t *testing.T) {
	m := &mockBridge{transcribe: func(audio []byte) (string, error) {
		return "transcribed text", nil
	}}
	s := newMockServer(m)

	body := strings.NewReader("fake-audio-data")
	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", body)
	w := httptest.NewRecorder()
	s.handleTranscribe(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["text"] != "transcribed text" {
		t.Errorf("got text %q, want %q", resp["text"], "transcribed text")
	}
}

func TestHandleTranscribe_Error(t *testing.T) {
	m := &mockBridge{transcribe: func(audio []byte) (string, error) {
		return "", errTranscribeFail
	}}
	s := newMockServer(m)

	req := httptest.NewRequest(http.MethodPost, "/api/transcribe", strings.NewReader("data"))
	w := httptest.NewRecorder()
	s.handleTranscribe(w, req)
	if w.Code != http.StatusInternalServerError {
		t.Errorf("got status %d, want %d", w.Code, http.StatusInternalServerError)
	}
}

// ─── Middleware Tests ─────────────────────────────────────────────

func TestCorsMiddleware_LoopbackOrigins(t *testing.T) {
	tests := []struct {
		origin  string
		allowed bool
	}{
		{"http://localhost:8090", true},
		{"http://127.0.0.1:8090", true},
		{"http://[::1]:8090", true},
		{"http://localhost", true},
		{"", true},
		{"http://evil.com", false},
		{"https://malicious.site", false},
	}

	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			w := httptest.NewRecorder()
			handler.ServeHTTP(w, req)

			origin := w.Header().Get("Access-Control-Allow-Origin")
			if tt.allowed && origin == "" {
				t.Error("expected CORS origin header, got empty")
			}
			if !tt.allowed && origin != "" {
				t.Errorf("expected no CORS origin header, got %q", origin)
			}
		})
	}
}

func TestCorsMiddleware_Methods(t *testing.T) {
	handler := corsMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://localhost")
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("OPTIONS got status %d, want %d", w.Code, http.StatusOK)
	}
	methods := w.Header().Get("Access-Control-Allow-Methods")
	if methods == "" {
		t.Error("expected Access-Control-Allow-Methods header")
	}
}

func TestLimitBodyMiddleware(t *testing.T) {
	t.Run("small_body_passes", func(t *testing.T) {
		handler := limitBodyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			data, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			w.Write(data)
		}), 50)

		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader("small"))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
		}
	})

	t.Run("large_body_errors", func(t *testing.T) {
		handler := limitBodyMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, "body too large", http.StatusRequestEntityTooLarge)
				return
			}
			w.WriteHeader(http.StatusOK)
		}), 10)

		bigBody := strings.Repeat("x", 20)
		req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(bigBody))
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != http.StatusRequestEntityTooLarge {
			t.Errorf("got status %d, want %d", w.Code, http.StatusRequestEntityTooLarge)
		}
	})
}

// ─── Utility Tests ────────────────────────────────────────────────

func TestWriteJSON(t *testing.T) {
	w := httptest.NewRecorder()
	data := map[string]string{"key": "value"}
	writeJSON(w, data)

	if w.Code != http.StatusOK {
		t.Errorf("got status %d, want %d", w.Code, http.StatusOK)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("got Content-Type %q, want application/json", ct)
	}
	var decoded map[string]string
	if err := json.NewDecoder(w.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if decoded["key"] != "value" {
		t.Errorf("got key=%q, want value", decoded["key"])
	}
}

func TestHandleRenameChat_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/chats/rename", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleRenameChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleSwitchChat_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/chats/switch", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleSwitchChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleDeleteChat_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/chats/delete", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteChat(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleUpdateMessage_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/messages/update", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleUpdateMessage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

func TestHandleDeleteMessage_BadJSON(t *testing.T) {
	s := newMockServer(&mockBridge{})
	req := httptest.NewRequest(http.MethodPost, "/api/messages/delete", strings.NewReader(`bad`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.handleDeleteMessage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("got status %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// TestHandleShutdown_MethodNotAllowed is a regression test for BUG-QH1:
// handleShutdown had no method check at all, so any HTTP verb (GET, DELETE,
// OPTIONS) from anyone reachable on the LAN (e.g. remote_access in "lan"
// tunnel mode binds 0.0.0.0) shut the whole app down. Deliberately does not
// exercise the POST success path here: handleShutdown sends a real SIGINT
// to the current process, which would kill this test binary.
func TestHandleShutdown_MethodNotAllowed(t *testing.T) {
	s := newMockServer(&mockBridge{})
	for _, method := range []string{http.MethodGet, http.MethodDelete, http.MethodOptions, http.MethodPut} {
		req := httptest.NewRequest(method, "/api/shutdown", nil)
		w := httptest.NewRecorder()
		s.handleShutdown(w, req)
		if w.Code != http.StatusMethodNotAllowed {
			t.Errorf("method %s: status = %d, want %d", method, w.Code, http.StatusMethodNotAllowed)
		}
	}
}
