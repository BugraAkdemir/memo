package replcli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// Regression tests for memorySavedSince. It used to identify "the snapshot
// point" by scanning for the last event structurally equal to a snapshotted
// `before` Event, which broke the moment two occurrences of the same event
// (most importantly two memory:saved events, which always carry identical
// empty Data) could exist in the ring at once — see
// TestMemorySavedSince_ConsecutiveSavesLookIdentical below for the concrete
// failure this caused. Fixed by keying off Seq, a monotonically increasing
// counter assigned once per event at push time (internal/app/app.go's
// eventRing) that never repeats, instead of Name+Data equality.
func TestMemorySavedSince_FindsSaveNotAtTheVeryEnd(t *testing.T) {
	events := []Event{
		{Name: "memory:saved", Seq: 1},
		{Name: "chat:done", Seq: 2}, // some other subsystem logs after the save
	}
	if !memorySavedSince(events, 0) {
		t.Error("expected memorySavedSince to find memory:saved even though it's not the last event")
	}
}

func TestMemorySavedSince_IgnoresSaveThatPredatesThisTurn(t *testing.T) {
	events := []Event{
		{Name: "memory:saved", Seq: 1}, // predates this turn
		{Name: "chat:done", Seq: 2},
	}
	if memorySavedSince(events, 1) {
		t.Error("expected memorySavedSince to ignore a memory:saved that was already there before this turn started")
	}
}

func TestMemorySavedSince_FindsNewSaveAfterStaleOne(t *testing.T) {
	events := []Event{
		{Name: "memory:saved", Seq: 1}, // stale — predates this turn
		{Name: "chat:done", Seq: 2},
		{Name: "memory:saved", Seq: 3}, // new
	}
	if !memorySavedSince(events, 1) {
		t.Error("expected memorySavedSince to find the new memory:saved after the stale one, with an unrelated event in between")
	}
}

func TestMemorySavedSince_BeforeEvictedFromRing_AnySaveCounts(t *testing.T) {
	// The snapshotted seq (50) is older than everything still in the ring
	// (evicted by 64+ newer entries since) — any memory:saved now visible
	// must be newer than it.
	events := []Event{
		{Name: "mood:updated", Seq: 120},
		{Name: "memory:saved", Seq: 121},
	}
	if !memorySavedSince(events, 50) {
		t.Error("expected memorySavedSince to treat any memory:saved as new once the snapshot seq fell out of the ring")
	}
}

func TestMemorySavedSince_NoSaveAtAll(t *testing.T) {
	events := []Event{{Name: "chat:done", Seq: 1}, {Name: "mood:updated", Seq: 2}}
	if memorySavedSince(events, 0) {
		t.Error("expected memorySavedSince to return false when there's no memory:saved event")
	}
}

// TestMemorySavedSince_ConsecutiveSavesLookIdentical is the real-world case
// none of the tests above cover: every memory:saved event carries the exact
// same (empty) Data, so once two turns in a row each produce a save — the
// ordinary case for a real back-to-back conversation, since saveMemorySync
// saves nearly every turn — the snapshotted "before" event (the ring's last
// entry at the start of a turn) can itself BE a memory:saved event from the
// *previous* turn. Under the old Name+Data-equality implementation this was
// indistinguishable from the *new* save: scanning for "the last occurrence
// equal to before" landed on the new save itself (content-identical to the
// old one), leaving nothing after it to find. Reproduced live: an automated
// multi-turn CLI probe (Fatih persona, 2026-07-15) got
// `[memory:none-detected]` on every single turn after the first, even
// though the backend log showed a real, fast (<300ms) SaveInteraction
// completing for each one. Seq — unique per event, unlike Name+Data —
// closes this: the previous turn's save is Seq 2, this turn's is Seq 4,
// and 4 > 2 regardless of either event's content.
func TestMemorySavedSince_ConsecutiveSavesLookIdentical(t *testing.T) {
	events := []Event{
		{Name: "chat:done", Seq: 1},
		{Name: "memory:saved", Seq: 2}, // previous turn's save — snapshotted seq
		{Name: "chat:done", Seq: 3},    // this turn unfolds
		{Name: "memory:saved", Seq: 4}, // this turn's NEW save — content-identical to Seq 2's event
	}
	if !memorySavedSince(events, 2) {
		t.Error("expected memorySavedSince to detect this turn's new save even though it's content-identical to the previous turn's save")
	}
}

// Regression tests for eventDataSince: reportMemorySaved/printWelcome used
// to only ever check for memory:saved, so a memory:error the backend had
// already emitted (autoStartEmbeddingModel/startupEmbeddingModel/
// saveMemorySync all emit one with a real, specific reason) was never
// surfaced anywhere in the REPL — silently indistinguishable from a save
// that just hadn't finished yet.
func TestEventDataSince_FindsErrorAnywhereWhenNoBefore(t *testing.T) {
	events := []Event{{Name: "chat:done", Seq: 1}, {Name: "memory:error", Data: "port dolu", Seq: 2}}
	msg, ok := eventDataSince(events, 0, "memory:error")
	if !ok || msg != "port dolu" {
		t.Errorf("eventDataSince = (%q, %v), want (\"port dolu\", true)", msg, ok)
	}
}

func TestEventDataSince_IgnoresErrorThatPredatesThisTurn(t *testing.T) {
	events := []Event{
		{Name: "memory:error", Data: "old", Seq: 1},
		{Name: "chat:done", Seq: 2},
	}
	if _, ok := eventDataSince(events, 1, "memory:error"); ok {
		t.Error("expected eventDataSince to ignore a memory:error that was already there before this turn started")
	}
}

func TestEventDataSince_FindsNewErrorAfterStaleOne(t *testing.T) {
	events := []Event{
		{Name: "memory:error", Data: "old", Seq: 1},
		{Name: "chat:done", Seq: 2},
		{Name: "memory:error", Data: "new", Seq: 3},
	}
	msg, ok := eventDataSince(events, 1, "memory:error")
	if !ok || msg != "new" {
		t.Errorf("eventDataSince = (%q, %v), want (\"new\", true)", msg, ok)
	}
}

func TestEventDataSince_NoMatch(t *testing.T) {
	events := []Event{{Name: "chat:done", Seq: 1}, {Name: "mood:updated", Seq: 2}}
	if _, ok := eventDataSince(events, 0, "memory:error"); ok {
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

// TestActivateChat_DoesNotTouchGlobalAgentOrWebSearchToggles is the
// regression test for the opposite, later-discovered bug: activateChat
// used to force both agent mode (SetAgentEnabled) and web search
// (SetWebSearchEnabled) on globally on every fresh chat/clear/session
// switch. Both are backend-wide, not per-chat or per-client — a
// concurrently-running Flutter GUI shared the exact same App instance, so
// every replcli launch silently flipped its agent-mode and web-search
// settings on too, even if the GUI user had deliberately turned either off.
// Agent mode no longer needs the global flag at all (every chat replcli
// sends into via chat_id now forces tool execution per-call through
// SendMessageStreamTo/IsAgentChat, see the Faz 4 chat-id work); web search
// has no per-chat equivalent, so it's simply no longer force-enabled — see
// the new /web command for how a user turns it on deliberately instead.
func TestActivateChat_DoesNotTouchGlobalAgentOrWebSearchToggles(t *testing.T) {
	agentCalled, webSearchCalled := false, false

	mux := http.NewServeMux()
	mux.HandleFunc("/api/chats/switch", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/agent/enabled", func(w http.ResponseWriter, r *http.Request) {
		agentCalled = true
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	mux.HandleFunc("/api/websearch", func(w http.ResponseWriter, r *http.Request) {
		webSearchCalled = true
		json.NewEncoder(w).Encode(map[string]bool{"ok": true})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	s := &session{client: NewClient(srv.URL), ctx: context.Background()}
	if err := s.activateChat("chat-1"); err != nil {
		t.Fatalf("activateChat() error = %v", err)
	}
	if agentCalled {
		t.Error("activateChat must not call /api/agent/enabled — it's a global toggle, not per-chat")
	}
	if webSearchCalled {
		t.Error("activateChat must not call /api/websearch — it's a global toggle, not per-chat")
	}
	if s.chatID != "chat-1" {
		t.Errorf("chatID = %q, want chat-1", s.chatID)
	}
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

// TestRun_PermissionRequest_AllowSession is a regression test: the CLI used
// to only ever send allow_once/deny_once, even though the backend
// (internal/agent/permissions.go) and the Flutter GUI
// (permission_dialog.dart) both already support allow_session — so a
// multi-step agent task re-prompted for the identical tool call every time
// it recurred in one CLI session, unlike the GUI. A non-dangerous tool must
// offer, and correctly send, allow_session.
func TestRun_PermissionRequest_AllowSession(t *testing.T) {
	event := `{"type":"permission_request","request_id":"req-3","tool":"write_file","danger_level":"medium","args":{"path":"x.txt"}}`
	srv, calls := newTestServer(t, []string{
		fmt.Sprintf(`data: {"content":%q,"finish_reason":"agent_event"}`, event),
		`data: {"content":"","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("bir dosya yaz\na\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("permission calls = %d, want 1", len(*calls))
	}
	if (*calls)[0]["policy"] != "allow_session" {
		t.Errorf("policy = %q, want allow_session", (*calls)[0]["policy"])
	}
}

// TestRun_PermissionRequest_DangerousToolHasNoSessionOption is a regression
// test: a dangerous tool must not offer "allow for session" at all — typing
// "a" for a dangerous call must NOT be interpreted as allow_session (it
// isn't a recognized answer for a dangerous prompt, so it falls through to
// deny), matching the GUI's PermissionDialog withholding the session-allow
// button specifically for dangerous tools.
func TestRun_PermissionRequest_DangerousToolHasNoSessionOption(t *testing.T) {
	event := `{"type":"permission_request","request_id":"req-4","tool":"run_command","danger_level":"dangerous","args":{"command":"rm -rf /"}}`
	srv, calls := newTestServer(t, []string{
		fmt.Sprintf(`data: {"content":%q,"finish_reason":"agent_event"}`, event),
		`data: {"content":"","done":true,"finish_reason":"stop"}`,
	})
	defer srv.Close()

	in := strings.NewReader("tehlikeli bir şey yap\na\n/exit\n")
	var out bytes.Buffer

	if err := Run(srv.URL, "/tmp/project", in, &out, false); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(*calls) != 1 {
		t.Fatalf("permission calls = %d, want 1", len(*calls))
	}
	if (*calls)[0]["policy"] != "deny_once" {
		t.Errorf("policy = %q, want deny_once (dangerous tools must not honor 'a' as allow_session)", (*calls)[0]["policy"])
	}
}

// TestHandleAgentEvent_UpdatesSpinnerLabelInsteadOfPrintingPermanentLines is
// a regression test: tool_executing/tool_result/tool_error/permission_denied
// used to each Fprintln a permanent line, so a multi-tool-call turn left a
// wall of scrollback for something the user only cares about the end result
// of. They now update the turn's status-line spinner in place instead — the
// terminal output itself gets nothing from these calls directly.
func TestHandleAgentEvent_UpdatesSpinnerLabelInsteadOfPrintingPermanentLines(t *testing.T) {
	var out bytes.Buffer
	sp := newSpinner(&out)
	defer sp.Stop()
	s := &session{out: &out, sp: sp}

	if err := s.handleAgentEvent(AgentEvent{Type: "tool_executing", Tool: "list_directory"}); err != nil {
		t.Fatalf("handleAgentEvent(tool_executing) error = %v", err)
	}
	if !strings.Contains(sp.Label(), "list_directory") {
		t.Errorf("spinner label = %q, want it to mention list_directory", sp.Label())
	}

	if err := s.handleAgentEvent(AgentEvent{Type: "tool_result", Tool: "list_directory"}); err != nil {
		t.Fatalf("handleAgentEvent(tool_result) error = %v", err)
	}
	if !strings.Contains(sp.Label(), "list_directory") {
		t.Errorf("spinner label after tool_result = %q, want it to still mention list_directory", sp.Label())
	}
}

// TestHandleAgentEvent_NoOpWithoutAnInFlightTurn guards the s.sp nil check:
// handleAgentEvent must never panic or attempt to touch a status line when
// called outside of sendMessage's turn (s.sp only set there).
func TestHandleAgentEvent_NoOpWithoutAnInFlightTurn(t *testing.T) {
	var out bytes.Buffer
	s := &session{out: &out}
	if err := s.handleAgentEvent(AgentEvent{Type: "tool_executing", Tool: "list_directory"}); err != nil {
		t.Fatalf("handleAgentEvent() error = %v", err)
	}
}

// TestDescribeToolCall_PrefersBackendPreview is a regression test: the
// permission prompt used to always show a blind character-truncation of the
// raw tool-call args JSON, in whatever key order the model emitted them —
// for a long field ahead of "path", the truncation could end before the
// target path ever appeared, so the user approved a write without ever
// seeing which file it targets. The backend's curated Preview field
// (populated server-side via a tool's PreviewFn) must be preferred when present.
func TestDescribeToolCall_PrefersBackendPreview(t *testing.T) {
	ev := AgentEvent{
		Args:    json.RawMessage(`{"content":"` + strings.Repeat("x", 200) + `","path":"real_target.go"}`),
		Preview: "Yaz: real_target.go",
	}
	got := describeToolCall(ev)
	if got != "Yaz: real_target.go" {
		t.Errorf("describeToolCall() = %q, want the curated Preview, not a truncated raw-args blob", got)
	}
}

func TestDescribeToolCall_FallsBackToRawArgsWhenNoPreview(t *testing.T) {
	ev := AgentEvent{Args: json.RawMessage(`{"command":"ls"}`)}
	got := describeToolCall(ev)
	if got != `{"command":"ls"}` {
		t.Errorf("describeToolCall() = %q, want raw args fallback", got)
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

// TestSendMessage_StopsInterruptWatchBeforeMemorySavedPoll is a regression
// test: stopInterruptWatch used to be deferred to when sendMessage returns,
// which is AFTER reportMemorySaved's up-to-~2.4s poll loop (whenever the
// embedding model is running) — but keys.go's watchInterrupt goroutine reads
// every key from the shared byte channel the instant it arrives and silently
// discards anything that isn't Esc/Ctrl+C. Leaving the watcher attached
// during that whole window meant any key the user typed right after a reply
// finished (or a whole pasted block) was consumed and lost, a real
// contributor to the CLI "randomly" losing input. The watcher must be
// detached as soon as the network stream itself ends, before the
// memory-saved poll runs.
func TestSendMessage_StopsInterruptWatchBeforeMemorySavedPoll(t *testing.T) {
	// eventsCalls counts /api/events hits: the 1st is sendMessage's own
	// "before" snapshot (taken prior to SendStream); the 2nd is the first
	// call inside reportMemorySaved's poll loop, which only happens after a
	// 400ms sleep — by construction, stopInterruptWatch has already run by
	// then (it now runs immediately after SendStream returns, before
	// reportMemorySaved is even called). Signaling eventsHit exactly on that
	// 2nd call, then synchronizing on it via channel receive, gives a
	// race-free happens-before edge to safely read s.watcher afterward —
	// unlike a raw sleep-then-peek, which the race detector correctly flags
	// as a data race against sendMessage's own goroutine.
	var eventsCalls int
	eventsHit := make(chan struct{}, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/models/embedding/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"running": true})
	})
	mux.HandleFunc("/api/events", func(w http.ResponseWriter, r *http.Request) {
		eventsCalls++
		if eventsCalls == 2 {
			select {
			case eventsHit <- struct{}{}:
			default:
			}
		}
		// Never reports memory:saved, forcing reportMemorySaved through its
		// full ~2.4s poll budget instead of returning early.
		json.NewEncoder(w).Encode([]Event{{Name: "chat:done"}})
	})
	mux.HandleFunc("/api/send/stream", func(w http.ResponseWriter, r *http.Request) {
		flusher := w.(http.Flusher)
		fmt.Fprint(w, `data: {"content":"ok","done":true,"finish_reason":"stop"}`+"\n\n")
		flusher.Flush()
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	s := &session{
		client: NewClient(srv.URL),
		ctx:    context.Background(),
		out:    &out,
		keys:   &keySource{ch: make(chan byte, 4), escWait: 50 * time.Millisecond},
	}

	done := make(chan struct{})
	go func() {
		s.sendMessage("selam")
		close(done)
	}()

	<-eventsHit
	if s.watcher != nil {
		t.Fatal("interrupt watcher still attached during the post-reply memory-saved poll — any key typed right now would be silently discarded")
	}

	<-done
}

// ─── Welcome panel: memory hint replaces the old status row ──────────────
//
// These name the *testing.T parameter "tt": this package's l10n helper is a
// function named t(key string) string, and a parameter named t would shadow
// it for the whole test body (same convention as l10n_test.go).

// runWelcomeOnce runs one REPL session that exits immediately, against a
// backend whose embedding status is fixed to embeddingRunning, and returns
// everything the welcome panel printed. Every other endpoint is left
// unregistered on purpose — each of those calls is best-effort, so 404ing
// them also checks the panel still renders when the backend answers nothing.
func runWelcomeOnce(tt *testing.T, embeddingRunning bool) string {
	tt.Helper()
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
	mux.HandleFunc("/api/models/embedding/status", func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]bool{"running": embeddingRunning})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	var out bytes.Buffer
	if err := Run(srv.URL, "/tmp/proj", strings.NewReader("/exit\n"), &out, false); err != nil {
		tt.Fatalf("Run: %v", err)
	}
	return out.String()
}

func TestWelcome_ShowsEmbeddingHintWhileMemoryIsDown(tt *testing.T) {
	got := runWelcomeOnce(tt, false)
	if !strings.Contains(got, t("memory_off_hint")) {
		tt.Errorf("expected the /embedding hint while embedding is down, got:\n%s", got)
	}
}

func TestWelcome_HidesEmbeddingHintOnceMemoryIsUp(tt *testing.T) {
	got := runWelcomeOnce(tt, true)
	if strings.Contains(got, t("memory_off_hint")) {
		tt.Errorf("the /embedding hint should be gone once embedding is running, got:\n%s", got)
	}
}

// The status row was removed for being untrustworthy (the backend can report
// "running" off a bare port ping), so it must not come back in either state.
func TestWelcome_NeverPrintsAMemoryStatusRow(tt *testing.T) {
	for _, running := range []bool{true, false} {
		got := runWelcomeOnce(tt, running)
		for _, label := range []string{"Hafıza:", "Memory:"} {
			if strings.Contains(got, label) {
				tt.Errorf("embeddingRunning=%v: panel still prints a memory status row (%q):\n%s", running, label, got)
			}
		}
	}
}
