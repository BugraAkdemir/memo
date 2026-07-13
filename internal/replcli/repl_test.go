package replcli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Regression tests for memorySavedSince: reportMemorySaved used to check
// only events[len(events)-1], so a save that genuinely happened was missed
// the moment any other subsystem (mood, learning/calendar intent, proactive
// suggestions, sync) logged a *different* event afterward and became the
// new final entry in the shared ring buffer. A real user hit this
// reliably: the first message after a fresh CLI start (nothing else had
// fired yet) always showed "✓ hafıza kaydedildi", later messages in the
// same session increasingly didn't, even though the save had genuinely
// happened (confirmed separately via the Settings memory-debug search).
func TestMemorySavedSince_FindsSaveNotAtTheVeryEnd(t *testing.T) {
	events := []Event{
		{Name: "memory:saved"},
		{Name: "chat:done"}, // some other subsystem logs after the save
	}
	if !memorySavedSince(events, Event{}, false) {
		t.Error("expected memorySavedSince to find memory:saved even though it's not the last event")
	}
}

func TestMemorySavedSince_IgnoresSaveThatPredatesThisTurn(t *testing.T) {
	stale := Event{Name: "memory:saved", Data: "old"}
	events := []Event{stale, {Name: "chat:done"}}
	if memorySavedSince(events, stale, true) {
		t.Error("expected memorySavedSince to ignore a memory:saved that was already there before this turn started")
	}
}

func TestMemorySavedSince_FindsNewSaveAfterStaleOne(t *testing.T) {
	stale := Event{Name: "memory:saved", Data: "old"}
	events := []Event{stale, {Name: "chat:done"}, {Name: "memory:saved", Data: "new"}}
	if !memorySavedSince(events, stale, true) {
		t.Error("expected memorySavedSince to find the new memory:saved after the stale one, with an unrelated event in between")
	}
}

func TestMemorySavedSince_BeforeEvictedFromRing_AnySaveCounts(t *testing.T) {
	// `before` is no longer present at all — evicted by 64+ newer ring
	// entries since it was captured — so any memory:saved now visible must
	// be newer than it.
	before := Event{Name: "chat:done", Data: "long-gone"}
	events := []Event{{Name: "mood:updated"}, {Name: "memory:saved"}}
	if !memorySavedSince(events, before, true) {
		t.Error("expected memorySavedSince to treat any memory:saved as new once `before` fell out of the ring")
	}
}

func TestMemorySavedSince_NoSaveAtAll(t *testing.T) {
	events := []Event{{Name: "chat:done"}, {Name: "mood:updated"}}
	if memorySavedSince(events, Event{}, false) {
		t.Error("expected memorySavedSince to return false when there's no memory:saved event")
	}
}

// Regression tests for eventDataSince: reportMemorySaved/printWelcome used
// to only ever check for memory:saved, so a memory:error the backend had
// already emitted (autoStartEmbeddingModel/startupEmbeddingModel/
// saveMemorySync all emit one with a real, specific reason) was never
// surfaced anywhere in the REPL — silently indistinguishable from a save
// that just hadn't finished yet.
func TestEventDataSince_FindsErrorAnywhereWhenNoBefore(t *testing.T) {
	events := []Event{{Name: "chat:done"}, {Name: "memory:error", Data: "port dolu"}}
	msg, ok := eventDataSince(events, Event{}, false, "memory:error")
	if !ok || msg != "port dolu" {
		t.Errorf("eventDataSince = (%q, %v), want (\"port dolu\", true)", msg, ok)
	}
}

func TestEventDataSince_IgnoresErrorThatPredatesThisTurn(t *testing.T) {
	stale := Event{Name: "memory:error", Data: "old"}
	events := []Event{stale, {Name: "chat:done"}}
	if _, ok := eventDataSince(events, stale, true, "memory:error"); ok {
		t.Error("expected eventDataSince to ignore a memory:error that was already there before this turn started")
	}
}

func TestEventDataSince_FindsNewErrorAfterStaleOne(t *testing.T) {
	stale := Event{Name: "memory:error", Data: "old"}
	events := []Event{stale, {Name: "chat:done"}, {Name: "memory:error", Data: "new"}}
	msg, ok := eventDataSince(events, stale, true, "memory:error")
	if !ok || msg != "new" {
		t.Errorf("eventDataSince = (%q, %v), want (\"new\", true)", msg, ok)
	}
}

func TestEventDataSince_NoMatch(t *testing.T) {
	events := []Event{{Name: "chat:done"}, {Name: "mood:updated"}}
	if _, ok := eventDataSince(events, Event{}, false, "memory:error"); ok {
		t.Error("expected eventDataSince to return false when there's no matching event")
	}
}

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

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
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

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
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

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("permission calls = %d, want 1", len(*calls))
	}
	if (*calls)[0]["policy"] != "deny_once" {
		t.Errorf("policy = %q, want deny_once", (*calls)[0]["policy"])
	}
}

// TestRun_ZeroChunkResponse_DoesNotHangOrLeakSpinner is a regression test:
// if a turn's response stream closes with literally no SSE lines (a genuine
// possibility — an upstream hiccup, a model erroring before emitting
// anything), the spinner must still be stopped. Before the fix it was only
// ever stopped from inside the chunk callback, so a zero-chunk turn left
// its goroutine running forever, racing later prompts for the same stdout
// and garbling output — this looked exactly like "the first few messages
// get no visible response".
func TestRun_ZeroChunkResponse_DoesNotHangOrLeakSpinner(t *testing.T) {
	srv, _ := newTestServer(t, nil) // empty SSE body for every /api/send/stream call
	defer srv.Close()

	in := strings.NewReader("selam\n/exit\n")
	var out bytes.Buffer

	done := make(chan error, 1)
	go func() { done <- Run(srv.URL, "/tmp/project", in, &out, false) }()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run() error = %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("Run() did not return in time — spinner goroutine likely leaked/hung")
	}
}

// TestRun_ResumesExistingAgentChat is a regression test: a `memo` run in a
// project that already has an agent chat must resume it (replaying its
// history) instead of always creating a brand-new, empty one.
// TestRun_AlwaysStartsFreshChat asserts every `memo` launch starts a brand
// new chat instead of auto-resuming an old one — even when an existing chat
// is on record for the same project path — so the terminal's context always
// starts clean. Resuming an old chat is left entirely to /session.
func TestRun_AlwaysStartsFreshChat(t *testing.T) {
	var newChatCalls int
	mux := http.NewServeMux()
	mux.HandleFunc("/api/agent/chat", func(w http.ResponseWriter, r *http.Request) {
		newChatCalls++
		json.NewEncoder(w).Encode(map[string]string{"id": "chat-new"})
	})
	mux.HandleFunc("/api/chats", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]SessionInfo{
			{ID: "chat-old", Title: "Eski Sohbet", ProjectPath: "/tmp/project", UpdatedAt: "2026-01-01 10:00", MsgCount: 2},
		})
	})
	mux.HandleFunc("/api/chats/switch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/agent/enabled", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/messages", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]ChatMessage{
			{Role: "user", Content: "merhaba"},
			{Role: "assistant", Content: "selam!"},
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	in := strings.NewReader("/exit\n")
	var out bytes.Buffer
	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if newChatCalls != 1 {
		t.Errorf("expected a new chat to be created on startup, got %d /api/agent/chat calls", newChatCalls)
	}
	got := out.String()
	if strings.Contains(got, "merhaba") || strings.Contains(got, "selam!") {
		t.Errorf("expected no history replay on startup (fresh chat), got:\n%s", got)
	}
}

func TestRun_ExitsOnEOF(t *testing.T) {
	srv, _ := newTestServer(t, nil)
	defer srv.Close()

	in := strings.NewReader("") // immediate EOF, no /exit needed
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
}
