package replcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestServer wires up the five endpoints Run() calls, backed by a
// scripted list of SSE lines to emit for every /api/send/stream call and a
// recorder for every /api/agent/permission POST it receives.
func newTestServer(t *testing.T, sseLines []string) (*httptest.Server, *[]map[string]string) {
	t.Helper()
	var permissionCalls []map[string]string

	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/chat", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]string{"id": "chat-1"})
	})
	mux.HandleFunc("/api/chats/switch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/agent/enabled", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/send/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		for _, line := range sseLines {
			fmt.Fprint(w, line+"\n\n")
			flusher.Flush()
		}
	})
	mux.HandleFunc("/api/agent/permission", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		permissionCalls = append(permissionCalls, body)
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})

	return httptest.NewServer(mux), &permissionCalls
}

func TestRun_PlainMessage(t *testing.T) {
	srv, _ := newTestServer(t, []string{
		`data: {"content":"Merhaba","done":false}`,
		`data: {"content":"!","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("selam\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if !strings.Contains(out.String(), "Merhaba!") {
		t.Errorf("output missing streamed reply, got:\n%s", out.String())
	}
}

func TestRun_PermissionRequest_Allow(t *testing.T) {
	event := `{"type":"permission_request","request_id":"req-1","tool":"run_command","args":{"command":"ls"}}`
	srv, calls := newTestServer(t, []string{
		fmt.Sprintf(`data: {"content":%q,"finish_reason":"agent_event"}`, event),
		`data: {"content":"","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	// First line answers the permission prompt ("y"), second line exits.
	in := strings.NewReader("bir komut çalıştır\ny\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("permission calls = %d, want 1", len(*calls))
	}
	if (*calls)[0]["request_id"] != "req-1" || (*calls)[0]["policy"] != "allow_once" {
		t.Errorf("got %+v", (*calls)[0])
	}
}

func TestRun_PermissionRequest_Deny(t *testing.T) {
	event := `{"type":"permission_request","request_id":"req-2","tool":"run_command","args":{"command":"rm -rf /"}}`
	srv, calls := newTestServer(t, []string{
		fmt.Sprintf(`data: {"content":%q,"finish_reason":"agent_event"}`, event),
		`data: {"content":"","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("tehlikeli bir şey yap\nn\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("permission calls = %d, want 1", len(*calls))
	}
	if (*calls)[0]["policy"] != "deny_once" {
		t.Errorf("policy = %q, want deny_once", (*calls)[0]["policy"])
	}
}

func TestRun_ExitsOnEOF(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	defer srv.Close()

	in := strings.NewReader("") // immediate EOF, no /exit needed
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
