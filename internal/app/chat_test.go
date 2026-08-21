package app

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/memory"
	"memo/internal/provider"
	"memo/internal/sessions"
)

// TestDrainToReply_SkipsStatusMarkerChunks is the regression test for a bug
// found live via the WhatsApp self-chat assistant: every reply arrived with
// a stray leading "9", "10", or "web_searchweb_search" glued onto the real
// text. drainToReply used to allowlist only FinishReason=="agent_event" as
// "not real content" — but "status" (web-search indicator, Content:
// "web_search") and "memory_used" (Content: the memory count as a bare
// number) chunks aren't agent_event either, so their Content leaked
// straight into the concatenated reply. Only a chunk with an empty
// FinishReason is real reply text; everything else is a status/metadata
// marker meant for an SSE consumer to render separately, not to appear in
// the message body.
func TestDrainToReply_SkipsStatusMarkerChunks(t *testing.T) {
	ch := make(chan api.StreamChunk, 8)
	ch <- api.StreamChunk{FinishReason: "memory_used", Content: "9"}
	ch <- api.StreamChunk{FinishReason: "status", Content: "web_search"}
	ch <- api.StreamChunk{Content: "Selam kanka, "}
	ch <- api.StreamChunk{Content: "naber?"}
	ch <- api.StreamChunk{FinishReason: "agent_event", Content: `{"type":"tool_call"}`}
	ch <- api.StreamChunk{FinishReason: "activity", Content: `{"foo":"bar"}`}
	ch <- api.StreamChunk{FinishReason: "usage", Content: `{"tokens":42}`}
	ch <- api.StreamChunk{Done: true, FinishReason: "stop"}
	close(ch)

	got := drainToReply(ch)
	want := "Selam kanka, naber?"
	if got != want {
		t.Errorf("drainToReply() = %q, want %q", got, want)
	}
}

// TestSendMessageStreamTo_UnknownChatID_ReturnsError verifies SendMessageStreamTo
// rejects a chat ID that doesn't exist instead of silently operating on
// whatever chat happens to be globally active — the whole point of taking an
// explicit chatID (docs/plans/PLAN_chatid_refactor.md Faz 3).
func TestSendMessageStreamTo_UnknownChatID_ReturnsError(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	ch := a.SendMessageStreamTo(context.Background(), "does-not-exist", "hello")
	chunk, ok := <-ch
	if !ok {
		t.Fatal("expected an error chunk, channel closed with nothing")
	}
	if chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected a Done error chunk for an unknown chat ID, got %+v", chunk)
	}
	if _, stillOpen := <-ch; stillOpen {
		t.Fatal("expected channel to be closed after the single error chunk")
	}
}

// TestSendMessageStreamToAsAgent_UnknownChatID_ReturnsError mirrors
// TestSendMessageStreamTo_UnknownChatID_ReturnsError for the
// agent-forcing variant — same guard, just reached via a different entry
// point.
func TestSendMessageStreamToAsAgent_UnknownChatID_ReturnsError(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	ch := a.SendMessageStreamToAsAgent(context.Background(), "does-not-exist", "hello")
	chunk, ok := <-ch
	if !ok {
		t.Fatal("expected an error chunk, channel closed with nothing")
	}
	if chunk.Error == "" || !chunk.Done {
		t.Fatalf("expected a Done error chunk for an unknown chat ID, got %+v", chunk)
	}
}

// TestSendMessageStreamToAsAgent_WorksOnANonAgentBackgroundChat is the
// actual regression test for the reported bug: a session created via
// sessions.Manager.NewBackgroundChat (WhatsApp/Telegram self-chat's own
// session, see initWhatsApp/handleWhatsAppSelfChatMessage) is never an
// "agent chat" per IsAgentChat (NewBackgroundChat never sets ProjectPath) —
// plain SendMessageStreamTo would fall back to the global agentEnabled flag
// for such a session, which defaults to false. This only proves
// SendMessageStreamToAsAgent doesn't reject/short-circuit a background
// chat any differently than SendMessageStreamTo would (both reach the same
// "session exists, proceed" point) — the actual forceAgent=true threading
// into routeStream's agentActive decision is a straight-line, one-argument
// change verified by reading sendMessageStreamInnerTo's call site, not
// independently re-provable here without a live provider/tool-calling mock.
func TestSendMessageStreamToAsAgent_WorksOnANonAgentBackgroundChat(t *testing.T) {
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	chatID := sm.NewBackgroundChat("Self-Chat")
	if sm.IsAgentChat(chatID) {
		t.Fatal("test assumption violated: NewBackgroundChat should never produce an agent chat")
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: identity.New("Test", "Memo", "casual", "", false),
		sessions: sm,
	}

	ch := a.SendMessageStreamToAsAgent(context.Background(), chatID, "hello")
	chunk, ok := <-ch
	if !ok {
		t.Fatal("channel closed with no chunk at all")
	}
	// No provider/local model is configured in this test, so generation
	// itself fails downstream either way — what matters is that failure is
	// NOT the "sohbet bulunamadı" (session not found) error the unknown-ID
	// test above produces, proving the valid background chat was accepted.
	if chunk.Error == "sohbet bulunamadı: "+chatID {
		t.Error("a valid background chat was incorrectly rejected as if it didn't exist")
	}
}

// TestSendMessageStreamTo_TargetsGivenChatID_NotGloballyActiveChat is the
// regression test for BUG_REPORT.md's TD-1 / PLAN_chatid_refactor.md Faz 3:
// the task loop used to SwitchChat(chatID) before every call so
// SendMessageStream would act on the right chat — clobbering whatever chat
// the user had open in the GUI for the call's duration. SendMessageStreamTo
// must write the user's message into chatA's own history without touching
// which chat is globally active, even while chatB is the one currently
// active.
func TestSendMessageStreamTo_TargetsGivenChatID_NotGloballyActiveChat(t *testing.T) {
	id := identity.New("Test", "Memo", "casual", "", false)
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	chatA := sm.NewAgentChat(t.TempDir())
	chatB := sm.NewChat()
	if err := sm.SwitchChat(chatB); err != nil {
		t.Fatalf("SwitchChat(chatB) error = %v", err)
	}

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: id,
		sessions: sm,
	}

	ch := a.SendMessageStreamTo(context.Background(), chatA, "task loop message")

	// Drain fully (no working client/provider is configured, so this will
	// resolve to an error chunk quickly) so streamMu is released and the
	// background goroutine doesn't leak past the test.
	done := make(chan struct{})
	go func() {
		for range ch {
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("SendMessageStreamTo did not finish streaming in time")
	}

	if got := sm.GetActiveID(); got != chatB {
		t.Fatalf("GetActiveID() = %q, want %q — SendMessageStreamTo must not change the globally active chat", got, chatB)
	}

	historyA := sm.GetHistoryForAPIForSession(chatA, 100)
	var foundInA bool
	for _, m := range historyA {
		if m["content"] == "task loop message" {
			foundInA = true
		}
	}
	if !foundInA {
		t.Fatal("chat A's history does not contain the message sent via SendMessageStreamTo(chatA, ...)")
	}

	historyB := sm.GetHistoryForAPIForSession(chatB, 100)
	for _, m := range historyB {
		if m["content"] == "task loop message" {
			t.Fatal("message sent via SendMessageStreamTo(chatA, ...) leaked into chat B's history")
		}
	}
}

// TestSendMessageStreamTo_MemoryUsed_AnnotatesSavedReply is the regression
// test for the memory-usage badge: when buildMessagesForSession retrieves
// N memories for a turn, the resulting assistant message must end up with
// ChatMessage.MemoryUsed == N once the stream finishes — the whole point of
// threading the count through memoryUsedCtxKey (chat.go) to finishStream
// (llm.go) without a parameter on every function in between.
func TestSendMessageStreamTo_MemoryUsed_AnnotatesSavedReply(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("memory.NewStore() error = %v", err)
	}
	defer store.Close()
	if err := store.SaveExplicit(context.Background(), "kullanicinin adi Ahmet", "profile"); err != nil {
		t.Fatalf("SaveExplicit() error = %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"merhaba Ahmet\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	router := provider.NewRouter([]provider.ProviderConfig{{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	}})

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager() error = %v", err)
	}
	chatID := sm.NewChat()

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0},
		},
		identity:           identity.New("Test", "Memo", "casual", "", false),
		sessions:           sm,
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		events:             &eventRing{},
	}

	ch := a.SendMessageStreamTo(context.Background(), chatID, "adimi hatirliyor musun")
	sawLiveMemoryUsedChunk := false
	for chunk := range ch {
		if chunk.FinishReason == "memory_used" {
			sawLiveMemoryUsedChunk = true
			if chunk.Content != "1" {
				t.Errorf("live memory_used chunk Content = %q, want %q", chunk.Content, "1")
			}
		}
	}
	if !sawLiveMemoryUsedChunk {
		t.Error("stream never carried a finishReason==\"memory_used\" chunk — the badge would only ever appear after a reload, not live like agent_event badges do")
	}

	msgs := sm.GetActiveMessagesForSession(chatID)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (user + assistant)", len(msgs))
	}
	assistant := msgs[1]
	if assistant.Role != "assistant" {
		t.Fatalf("msgs[1].Role = %q, want assistant", assistant.Role)
	}
	if assistant.MemoryUsed != 1 {
		t.Fatalf("assistant.MemoryUsed = %d, want 1 (the pinned fact SaveExplicit saved)", assistant.MemoryUsed)
	}
}

// TestSendMessageStreamTo_NoMemoryRetrieved_LeavesMemoryUsedZero guards the
// other half: a turn where memory is enabled but nothing relevant comes
// back (empty store) must not stamp a stray MemoryUsed value on the saved
// reply — the frontend badge is only supposed to render when it's >0.
func TestSendMessageStreamTo_NoMemoryRetrieved_LeavesMemoryUsedZero(t *testing.T) {
	store, err := memory.NewStore(memory.StoreConfig{
		Dir:       t.TempDir(),
		Dimension: 4,
		EmbeddingFunc: func(_ context.Context, _ string) ([]float32, error) {
			return []float32{1, 0, 0, 0}, nil
		},
	})
	if err != nil {
		t.Fatalf("memory.NewStore() error = %v", err)
	}
	defer store.Close()
	// Deliberately empty — no SaveExplicit/SaveInteraction calls.

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"selam\"}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	router := provider.NewRouter([]provider.ProviderConfig{{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	}})

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager() error = %v", err)
	}
	chatID := sm.NewChat()

	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: true, TopK: 5, MinSimilarity: 0},
		},
		identity:           identity.New("Test", "Memo", "casual", "", false),
		sessions:           sm,
		store:              store,
		providerRouter:     router,
		activeProviderName: "test",
		events:             &eventRing{},
	}

	ch := a.SendMessageStreamTo(context.Background(), chatID, "selam")
	for chunk := range ch {
		if chunk.FinishReason == "memory_used" {
			t.Errorf("got a live memory_used chunk (content %q) when nothing was retrieved — should never fire for a zero count", chunk.Content)
		}
	}

	msgs := sm.GetActiveMessagesForSession(chatID)
	if len(msgs) != 2 {
		t.Fatalf("got %d messages, want 2 (user + assistant)", len(msgs))
	}
	if got := msgs[1].MemoryUsed; got != 0 {
		t.Fatalf("assistant.MemoryUsed = %d, want 0 (empty memory store, nothing retrieved)", got)
	}
}

// capturedRequests collects outbound request bodies the fake provider
// server saw, safe for concurrent access. A single successful SendMessage
// call is not the only thing that can hit the mocked provider: routeStream
// also kicks off fire-and-forget background calls (e.g. processMessageIntent)
// against the same router, on their own goroutines, with no signal the test
// can wait on — an earlier version of this test captured into a bare
// *string with no synchronization at all and raced (caught by CI's -race,
// not reproducible locally under normal load): the background goroutine's
// write and the test's own read of the same string were unsynchronized.
// Aggregating into a mutex-guarded slice and checking "did any captured
// request contain X" instead of "did the last one" fixes the race and stays
// correct regardless of how many extra background calls happen to fire —
// only the real agent-routed call will ever carry tool definitions, so the
// containsAny checks below are unaffected by unrelated background traffic.
type capturedRequests struct {
	mu     sync.Mutex
	bodies []string
}

func (c *capturedRequests) add(body string) {
	c.mu.Lock()
	c.bodies = append(c.bodies, body)
	c.mu.Unlock()
}

func (c *capturedRequests) containsAny(substr string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for _, b := range c.bodies {
		if strings.Contains(b, substr) {
			return true
		}
	}
	return false
}

func (c *capturedRequests) String() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return strings.Join(c.bodies, "\n---\n")
}

// newSendMessageTestApp builds a real App wired to a fake OpenAI-compatible
// HTTP server standing in for the active provider, so SendMessage's actual
// outbound request(s) can be inspected via reqs.
func newSendMessageTestApp(t *testing.T, reqs *capturedRequests) *App {
	t.Helper()
	t.Setenv("MEMO_DATA_DIR", t.TempDir())

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		reqs.add(string(body))
		if strings.Contains(string(body), `"stream":true`) {
			// The agent pipeline's ChatCompletion (non-streaming, used to
			// send tool definitions) is a plain JSON POST/response — but
			// routeStream's non-agent fallback goes through
			// ChatCompletionStream, which requests SSE. Both must be served
			// correctly since which one a given test exercises depends on
			// agent mode being on or off.
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"choices\":[{\"delta\":{\"content\":\"fake agent reply\"}}]}\n\n")
			fmt.Fprint(w, "data: [DONE]\n\n")
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"1","object":"chat.completion","created":1,"model":"test-model",`+
			`"choices":[{"index":0,"message":{"role":"assistant","content":"fake agent reply"},"finish_reason":"stop"}],`+
			`"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`)
	}))
	t.Cleanup(srv.Close)

	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	cfgMgr.Set(provider.ProviderConfig{
		Type:    provider.ProviderCustom,
		Name:    "test",
		BaseURL: srv.URL,
		Model:   "test-model",
		Enabled: true,
	})

	router := provider.NewRouter(cfgMgr.GetEnabled())
	router.SetActiveProvider("test")

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}

	agentExecutor := agent.NewExecutor(t.TempDir(), nil, nil)

	return &App{
		cfg:                &config.AppConfig{Memory: config.MemoryConfig{MemoryEnabled: false}},
		identity:           identity.New("Test", "Memo", "casual", "", false),
		sessions:           sm,
		providerRouter:     router,
		providerCfgMgr:     cfgMgr,
		activeProviderName: "test",
		agentExecutor:      agentExecutor,
		webSearchExecutor:  agent.NewWebSearchExecutor(agentExecutor),
		events:             &eventRing{},
	}
}

// TestSendMessage_AgentModeOn_SendsToolDefinitions is the regression test for
// the POST /api/send agent-skipping bug: App.SendMessage used to call
// callLLM directly, which builds a plain message list with no tool
// definitions and no agent system prompt — bypassing routeStream (and thus
// agent mode) entirely, regardless of the global agent-mode toggle. A
// tool-requiring message sent through this path always got a plain,
// tool-less reply (the model just claiming it had no tools available), with
// no error or indication anything was skipped.
//
// SendMessage now routes through sendMessageStreamCore/routeStream exactly
// like SendMessageStream does, so with agent mode on, the outbound request
// to the provider must carry tool definitions. Confirmed live against a real
// self-hosted install before this fix: the same message sent via
// POST /api/send got a plain refusal ("I don't have terminal access"), while
// POST /api/send/stream (already routed through routeStream) correctly
// triggered a run_command tool call.
func TestSendMessage_AgentModeOn_SendsToolDefinitions(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	if err := a.SetAgentEnabled(true); err != nil {
		t.Fatalf("SetAgentEnabled(true): %v", err)
	}

	reply := a.SendMessage("list the files in the current directory")

	if !reqs.containsAny(`"tools"`) {
		t.Fatalf("agent mode on: outbound provider request had no tool definitions — agent routing was skipped (the bug this test guards against). requests=%s", reqs)
	}
	if reply != "fake agent reply" {
		t.Fatalf("SendMessage() = %q, want %q", reply, "fake agent reply")
	}
}

// TestSendMessage_AgentModeOff_NoToolDefinitions is the mirror of the test
// above: with agent mode off, SendMessage must still behave like plain chat
// — no tool definitions sent, same as before this fix.
func TestSendMessage_AgentModeOff_NoToolDefinitions(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)

	reply := a.SendMessage("hello")

	if reqs.containsAny(`"tools"`) {
		t.Fatalf("agent mode off: outbound provider request unexpectedly carried tool definitions. requests=%s", reqs)
	}
	if reply != "fake agent reply" {
		t.Fatalf("SendMessage() = %q, want %q", reply, "fake agent reply")
	}
}

// TestSendMessage_WebSearchOnMinimalModeOn_NoToolDefinitions guards a gap
// the web-search tool-calling redesign (see the test above) introduced and
// this same session caught and fixed: the old blind-injection design lived
// inside buildMessagesForSession, gated behind
// `!a.identity.GetMinimalMode()` — Minimal Mode's whole promise is "zero
// injection beyond memory". Moving the decision into routeStream
// (callWebSearchAgentStream) initially dropped that gate entirely, so a
// tool definition would ride along on every request even under Minimal
// Mode. This asserts Minimal Mode still suppresses it.
func TestSendMessage_WebSearchOnMinimalModeOn_NoToolDefinitions(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	a.cfg.WebSearch.Enabled = true
	a.identity.SetMinimalMode(true)

	reply := a.SendMessage("naber")

	if reqs.containsAny(`"tools"`) {
		t.Fatalf("web search on but Minimal Mode on: outbound request unexpectedly carried tool definitions. requests=%s", reqs)
	}
	if reply != "fake agent reply" {
		t.Fatalf("SendMessage() = %q, want %q", reply, "fake agent reply")
	}
}

// TestSendMessage_WebSearchOnAgentOff_SendsOnlyWebSearchTool is the
// regression test for the redesign that replaced blind web-search
// injection: with agent mode off and the web-search toggle on,
// routeStream must route through callWebSearchAgentStream — a native
// tool-calling request carrying exactly the web_search tool definition, not
// the full agent toolset (read_file, run_command, etc.) and not the old
// blind-injected "Web Search Results" text in the system prompt. This is
// what lets the model decide per message, at zero extra cost when it
// decides not to search, instead of a separate query-extraction call or an
// unconditional search on every message.
func TestSendMessage_WebSearchOnAgentOff_SendsOnlyWebSearchTool(t *testing.T) {
	reqs := &capturedRequests{}
	a := newSendMessageTestApp(t, reqs)
	a.cfg.WebSearch.Enabled = true

	reply := a.SendMessage("naber")

	if !reqs.containsAny(`"web_search"`) {
		t.Fatalf("web search on, agent off: outbound request had no web_search tool definition. requests=%s", reqs)
	}
	if reqs.containsAny(`"read_file"`) || reqs.containsAny(`"run_command"`) {
		t.Fatalf("web search on, agent off: outbound request carried the full agent toolset, not just web_search. requests=%s", reqs)
	}
	if reqs.containsAny("Web Search Results") {
		t.Fatalf("web search on, agent off: request still carries the old blind-injected search-results text. requests=%s", reqs)
	}
	if reply != "fake agent reply" {
		t.Fatalf("SendMessage() = %q, want %q", reply, "fake agent reply")
	}
}
