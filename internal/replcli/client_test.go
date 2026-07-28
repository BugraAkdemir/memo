package replcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"memo/internal/api"
)

func TestClient_Status_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/status" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.Status(context.Background()); err != nil {
		t.Fatalf("Status() error = %v", err)
	}
}

func TestClient_Status_Down(t *testing.T) {
	c := NewClient("http://127.0.0.1:1") // nothing listens here
	if err := c.Status(context.Background()); err == nil {
		t.Fatal("Status() expected error, got nil")
	}
}

func TestClient_NewAgentChat(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/agent/chat" || r.Method != http.MethodPost {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"id": "chat-1"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	id, err := c.NewAgentChat(context.Background(), "/home/user/project")
	if err != nil {
		t.Fatalf("NewAgentChat() error = %v", err)
	}
	if id != "chat-1" {
		t.Errorf("id = %q, want chat-1", id)
	}
	if gotBody["project_path"] != "/home/user/project" {
		t.Errorf("project_path = %q, want /home/user/project", gotBody["project_path"])
	}
}

func TestClient_SwitchChat(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]string{"ok": "true"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.SwitchChat(context.Background(), "chat-1"); err != nil {
		t.Fatalf("SwitchChat() error = %v", err)
	}
	if gotBody["id"] != "chat-1" {
		t.Errorf("id = %q, want chat-1", gotBody["id"])
	}
}

func TestClient_SetAgentEnabled(t *testing.T) {
	var gotMethod string
	var gotBody map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.SetAgentEnabled(context.Background(), true); err != nil {
		t.Fatalf("SetAgentEnabled() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if !gotBody["enabled"] {
		t.Errorf("enabled = %v, want true", gotBody["enabled"])
	}
}

func TestClient_SendPermission(t *testing.T) {
	var gotBody map[string]string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.SendPermission(context.Background(), "req-1", "allow_once"); err != nil {
		t.Fatalf("SendPermission() error = %v", err)
	}
	if gotBody["request_id"] != "req-1" || gotBody["policy"] != "allow_once" {
		t.Errorf("got %+v", gotBody)
	}
}

func TestClient_GetAgentAutoPermission(t *testing.T) {
	var gotMethod string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewEncoder(w).Encode(map[string]bool{"enabled": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	enabled, err := c.GetAgentAutoPermission(context.Background())
	if err != nil {
		t.Fatalf("GetAgentAutoPermission() error = %v", err)
	}
	if !enabled {
		t.Error("enabled = false, want true")
	}
	if gotMethod != http.MethodGet {
		t.Errorf("method = %s, want GET", gotMethod)
	}
}

func TestClient_SetAgentAutoPermission(t *testing.T) {
	var gotMethod string
	var gotBody map[string]bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		json.NewDecoder(r.Body).Decode(&gotBody)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if err := c.SetAgentAutoPermission(context.Background(), true); err != nil {
		t.Fatalf("SetAgentAutoPermission() error = %v", err)
	}
	if gotMethod != http.MethodPut {
		t.Errorf("method = %s, want PUT", gotMethod)
	}
	if !gotBody["enabled"] {
		t.Errorf("enabled = %v, want true", gotBody["enabled"])
	}
}

func TestClient_ErrorStatusCode(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad json", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	if _, err := c.NewAgentChat(context.Background(), "/tmp"); err == nil {
		t.Fatal("expected error on non-200 response, got nil")
	}
}

func TestClient_SendStream(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/send/stream" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		flusher := w.(http.Flusher)
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"content\":\"Mer\",\"done\":false}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"content\":\"haba\",\"done\":false}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"content\":\"\",\"done\":true,\"finish_reason\":\"stop\"}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	var got []string
	err := c.SendStream(context.Background(), "selam", func(chunk api.StreamChunk) error {
		got = append(got, chunk.Content)
		return nil
	})
	if err != nil {
		t.Fatalf("SendStream() error = %v", err)
	}
	if len(got) != 3 || got[0] != "Mer" || got[1] != "haba" || got[2] != "" {
		t.Errorf("got chunks %v", got)
	}
}

func TestClient_SendStream_StopsOnCallbackError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		fmt.Fprint(w, "data: {\"content\":\"a\",\"done\":false}\n\n")
		flusher.Flush()
		fmt.Fprint(w, "data: {\"content\":\"b\",\"done\":false}\n\n")
		flusher.Flush()
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	callCount := 0
	boom := errors.New("boom")
	err := c.SendStream(context.Background(), "selam", func(chunk api.StreamChunk) error {
		callCount++
		return boom
	})
	if !errors.Is(err, boom) {
		t.Fatalf("err = %v, want boom", err)
	}
	if callCount != 1 {
		t.Errorf("callCount = %d, want 1 (should stop after first error)", callCount)
	}
}
