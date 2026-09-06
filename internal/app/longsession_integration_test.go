package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
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
	"memo/internal/stats"
)

// This file is the hermetic end-to-end check for the long-session work
// (Phases 0-4): a real App, a real agent.Executor + pipeline, real
// buildMessagesForSession/memory/working-set/compaction code — driven
// against a scripted in-process HTTP provider, never a real model.

// --- fake provider -------------------------------------------------------

type fakeProvider struct {
	srv      *httptest.Server
	mu       sync.Mutex
	requests [][]byte // raw request bodies, in order
	// script returns the OpenAI-shaped chat.completion body for call n
	// (1-based), given the parsed request.
	script func(n int, req map[string]any) map[string]any
}

func newFakeProvider(t *testing.T, script func(n int, req map[string]any) map[string]any) *fakeProvider {
	t.Helper()
	fp := &fakeProvider{script: script}
	fp.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		fp.mu.Lock()
		fp.requests = append(fp.requests, raw)
		n := len(fp.requests)
		fp.mu.Unlock()
		resp := fp.script(n, body)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(resp)
	}))
	t.Cleanup(fp.srv.Close)
	return fp
}

func (fp *fakeProvider) reqBodies() []string {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	out := make([]string, len(fp.requests))
	for i, b := range fp.requests {
		out[i] = string(b)
	}
	return out
}

func (fp *fakeProvider) callCount() int {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	return len(fp.requests)
}

// chat.completion response helpers ------------------------------------------------

func respText(model, content string, promptTok, complTok int) map[string]any {
	return map[string]any{
		"id":     "cmpl-x",
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": "stop",
			"message":       map[string]any{"role": "assistant", "content": content},
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTok,
			"completion_tokens": complTok,
			"total_tokens":      promptTok + complTok,
		},
	}
}

func respToolCall(model, id, name string, args map[string]any, promptTok, complTok int) map[string]any {
	argJSON, _ := json.Marshal(args)
	return map[string]any{
		"id":     "cmpl-x",
		"object": "chat.completion",
		"model":  model,
		"choices": []map[string]any{{
			"index":         0,
			"finish_reason": "tool_calls",
			"message": map[string]any{
				"role": "assistant",
				"tool_calls": []map[string]any{{
					"id":       id,
					"type":     "function",
					"function": map[string]any{"name": name, "arguments": string(argJSON)},
				}},
			},
		}},
		"usage": map[string]any{
			"prompt_tokens":     promptTok,
			"completion_tokens": complTok,
			"total_tokens":      promptTok + complTok,
		},
	}
}

// --- App harness -------------------------------------------------------------

type lsHarness struct {
	a      *App
	sm     *sessions.Manager
	stats  *stats.Store
	fp     *fakeProvider
	chatID string
}

// newLongSessionApp wires a real App around fp: agent mode on, stats store
// live, memory optionally on, a running memory-save worker, config filled
// from Default()+validate() so every Phase 1-4 knob has its shipped value.
func newLongSessionApp(t *testing.T, fp *fakeProvider, withMemory bool) *lsHarness {
	t.Helper()

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	st, err := stats.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("stats.NewStore: %v", err)
	}
	t.Cleanup(func() { st.Close() })

	cfg := config.Default()
	cfg.Memory.MemoryEnabled = withMemory
	// keep the fact-extraction LLM call out of these tests
	cfg.Memory.AutoFactExtraction = false

	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), nil)
	cfgMgr.Set(provider.ProviderConfig{
		Type:          provider.ProviderCustom,
		Name:          "fake",
		BaseURL:       fp.srv.URL,
		Model:         "fake-model",
		Enabled:       true,
		ContextTokens: 200000,
	})
	router := provider.NewRouter(cfgMgr.GetEnabled())
	router.SetActiveProvider("fake")

	a := &App{
		cfg:                cfg,
		identity:           identity.New("Tester", "Memo", "casual", "", false),
		sessions:           sm,
		providerCfgMgr:     cfgMgr,
		providerRouter:     router,
		activeProviderName: "fake",
		statsStore:         st,
		events:             &eventRing{},
		lifecycleCtx:       context.Background(),
		memorySaveCh:       make(chan saveTask, 64),
	}
	a.agentEnabled = true
	a.agentExecutor = agent.NewExecutor(t.TempDir(), router, cfgMgr, sm)
	a.agentExecutor.SetMaxIterations(cfg.AgentMode.MaxIterations)
	a.agentExecutor.SetMaxContinuations(cfg.AgentMode.MaxContinuations)
	go a.memorySaveWorker()

	if withMemory {
		store, err := memory.NewStore(memory.StoreConfig{
			Dir:                 t.TempDir(),
			Dimension:           8,
			EmbeddingFunc:       bagOfWords8,
			RecencyHalfLifeDays: cfg.Memory.RecencyHalfLifeDays,
		})
		if err != nil {
			t.Fatalf("memory.NewStore: %v", err)
		}
		t.Cleanup(func() { store.Close() })
		a.store = store
	}

	h := &lsHarness{a: a, sm: sm, stats: st, fp: fp, chatID: sm.NewChat()}
	return h
}

// bagOfWords8 is a tiny deterministic embedder so memory retrieval is
// exercised for real without an embedding server.
func bagOfWords8(_ context.Context, text string) ([]float32, error) {
	v := make([]float32, 8)
	for _, w := range strings.Fields(strings.ToLower(text)) {
		var s int
		for _, r := range w {
			s += int(r)
		}
		v[s%8] += 1
	}
	var norm float64
	for _, x := range v {
		norm += float64(x) * float64(x)
	}
	if norm == 0 {
		norm = 1
	}
	for i := range v {
		v[i] = float32(float64(v[i]) / (norm))
	}
	return v, nil
}

// turn runs one full agent turn through the real pipeline and returns the
// assistant text plus the raw request bodies the provider saw during it.
func (h *lsHarness) turn(t *testing.T, userMsg string) (reply string, reqsDuringTurn []string) {
	t.Helper()
	before := h.fp.callCount()

	// Match sendMessageStreamCore's real order and its Code Mode ctx attach.
	ctx := context.Background()
	if h.a.resolveCodeMode(h.chatID) {
		ctx = withCodeMode(ctx)
	}
	msgs := h.a.buildMessagesForSession(ctx, h.chatID, userMsg, nil, nil)
	h.sm.AddMessageToSession(h.chatID, "user", userMsg, "", "")

	out := h.a.callAgentStream(ctx, msgs, userMsg, h.chatID)
	var sb strings.Builder
	deadline := time.After(10 * time.Second)
	for {
		select {
		case chunk, ok := <-out:
			if !ok {
				all := h.fp.reqBodies()
				return sb.String(), all[before:]
			}
			if chunk.FinishReason == "agent_event" {
				continue
			}
			if chunk.Error != "" {
				sb.WriteString(chunk.Error)
			}
			sb.WriteString(chunk.Content)
		case <-deadline:
			t.Fatal("agent turn did not complete within 10s")
		}
	}
}

func (h *lsHarness) usageSummary(t *testing.T) stats.Summary {
	t.Helper()
	s, err := h.stats.Summary(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("stats.Summary: %v", err)
	}
	return s
}

type capturedReq struct {
	Messages []struct {
		Role    string `json:"role"`
		Content any    `json:"content"`
	} `json:"messages"`
	Tools json.RawMessage `json:"tools"`
}

func parseReq(t *testing.T, body string) capturedReq {
	t.Helper()
	var r capturedReq
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}
	return r
}

func (r capturedReq) system() string {
	for _, m := range r.Messages {
		if m.Role == "system" {
			if s, ok := m.Content.(string); ok {
				return s
			}
		}
	}
	return ""
}

// allText concatenates every message's string content — used when a block
// (time context, working set) may ride on the system prompt or on the
// current user message depending on the path.
func (r capturedReq) allText() string {
	var b strings.Builder
	for _, m := range r.Messages {
		if s, ok := m.Content.(string); ok {
			b.WriteString(s)
			b.WriteByte('\n')
		}
	}
	return b.String()
}

func (r capturedReq) isAgent() bool {
	return len(r.Tools) > 0 && string(r.Tools) != "null"
}

// agentReqs keeps only the actual agent pipeline requests (they carry a
// tools array) — dropping the title-generation and compaction-summary calls
// that also hit the fake provider.
func agentReqs(t *testing.T, bodies []string) []capturedReq {
	t.Helper()
	var out []capturedReq
	for _, b := range bodies {
		if r := parseReq(t, b); r.isAgent() {
			out = append(out, r)
		}
	}
	return out
}

// firstAgentReq returns the first captured body that is an actual agent
// pipeline request.
func firstAgentReq(t *testing.T, bodies []string) capturedReq {
	t.Helper()
	rs := agentReqs(t, bodies)
	if len(rs) == 0 {
		t.Fatalf("no agent (tools-carrying) request among %d bodies", len(bodies))
	}
	return rs[0]
}

// waitUsageCategory polls the stats store until a row with the given
// category exists (async fire-and-forget write from finishStream).
func (h *lsHarness) waitUsageCategory(t *testing.T, category string) stats.CategoryUsage {
	t.Helper()
	deadline := time.After(3 * time.Second)
	for {
		sum := h.usageSummary(t)
		for _, c := range sum.CategoryBreakdown {
			if c.Category == category {
				return c
			}
		}
		select {
		case <-deadline:
			t.Fatalf("no %q usage row after 3s; breakdown=%+v", category, sum.CategoryBreakdown)
		case <-time.After(20 * time.Millisecond):
		}
	}
}

// ===========================================================================
// Phase 0 — real token usage is recorded for agent turns
// ===========================================================================

func TestIntegration_AgentTurnRecordsRealUsage(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		switch n {
		case 1:
			return respToolCall("fake-model", "c1", "read_file", map[string]any{"path": "go.mod"}, 1500, 30)
		default:
			return respText("fake-model", "all done", 1800, 42)
		}
	})
	h := newLongSessionApp(t, fp, false)

	reply, _ := h.turn(t, "read go.mod and summarise")
	if !strings.Contains(reply, "all done") {
		t.Fatalf("reply = %q, want it to contain the final content", reply)
	}

	// The agent turn's row must carry the provider's REAL usage summed across
	// both iterations (prompt 1500+1800, completion 30+42) — not a word-count
	// estimate of the seed messages.
	agentRow := h.waitUsageCategory(t, categoryAgent)
	if agentRow.PromptTokens != 3300 {
		t.Errorf("agent prompt tokens = %d, want 3300 (1500+1800 from the fake provider's usage)", agentRow.PromptTokens)
	}
	if agentRow.CompletionTokens != 72 {
		t.Errorf("agent completion tokens = %d, want 72 (30+42)", agentRow.CompletionTokens)
	}
	if agentRow.Requests != 1 {
		t.Errorf("agent request count = %d, want 1", agentRow.Requests)
	}
}

// ===========================================================================
// Phase 2 (A) — the working set stops the model re-reading a known file
// ===========================================================================

func TestIntegration_WorkingSetInjectedNextTurn(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		// turn 1: one read_file then finish. turn 2: just finish.
		switch n {
		case 1:
			return respToolCall("fake-model", "c1", "read_file",
				map[string]any{"path": "internal/app/llm.go"}, 900, 10)
		default:
			return respText("fake-model", "ok", 900, 10)
		}
	})
	h := newLongSessionApp(t, fp, false)

	h.turn(t, "look at internal/app/llm.go")
	_, reqs := h.turn(t, "now what does it do")

	txt := firstAgentReq(t, reqs).allText()
	if !strings.Contains(txt, "[Working set") {
		t.Fatalf("turn 2 request has no working-set block:\n%s", txt)
	}
	if !strings.Contains(txt, "internal/app/llm.go") {
		t.Errorf("working set should name the file read in turn 1:\n%s", txt)
	}
}

// ===========================================================================
// Phase 2 (B) — a long history is condensed, not silently dropped
// ===========================================================================

func TestIntegration_LongHistoryGetsCompacted(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		msgs, _ := req["messages"].([]any)
		for _, mi := range msgs {
			m, _ := mi.(map[string]any)
			if c, _ := m["content"].(string); strings.Contains(c, "Condense this earlier stretch") {
				return respText("fake-model", "- user did A\n- decided B\n- open: C", 50, 20)
			}
		}
		return respText("fake-model", "answer", 100, 10)
	})
	h := newLongSessionApp(t, fp, false)

	// A budget big enough that many turns survive the token-aware fetch, but
	// small enough that their combined size crosses CompactThresholdPct (60%).
	h.a.cfgMu.Lock()
	h.a.cfg.Llama.MaxContextTokens = 3500
	h.a.cfgMu.Unlock()

	filler := strings.Repeat("word ", 60) // ~100 tok/message
	for i := 0; i < 30; i++ {
		h.turn(t, fmt.Sprintf("%s question %d", filler, i))
	}
	_, reqs := h.turn(t, "final question after a long chat")

	req := firstAgentReq(t, reqs)
	foundSummary := false
	for _, m := range req.Messages {
		s, _ := m.Content.(string)
		if m.Role == "system" && strings.Contains(s, "Earlier conversation summary") {
			foundSummary = true
			if !strings.Contains(s, "decided B") {
				t.Errorf("summary system message missing the condensed content: %q", s)
			}
		}
	}
	if !foundSummary {
		t.Fatalf("a long history was not compacted into a summary system message (%d messages in the final request)", len(req.Messages))
	}
	// The summarization LLM call must actually have fired (once).
	summaries := 0
	for _, b := range h.fp.reqBodies() {
		if strings.Contains(b, "Condense this earlier stretch") {
			summaries++
		}
	}
	if summaries == 0 {
		t.Error("compaction summary LLM call never fired")
	}
	if summaries > 8 {
		t.Errorf("compaction re-ran %d times over 30 turns — the prefix cache isn't holding", summaries)
	}
}

// ===========================================================================
// Code Mode — coding preset: coding directive only, working set kept, zero
// Memo-initiated side LLM calls (no title generation)
// ===========================================================================

func TestIntegration_CodeMode(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		// turn 1: read a file then finish; turn 2: finish.
		if n == 1 {
			return respToolCall("fake-model", "c1", "read_file",
				map[string]any{"path": "cmd/main.go"}, 800, 10)
		}
		return respText("fake-model", "done", 800, 10)
	})
	h := newLongSessionApp(t, fp, true) // memory ON — Code Mode must suppress it anyway
	h.chatID = h.sm.NewAgentChat("/tmp/proj") // project chat → Code Mode on by default
	if !h.a.resolveCodeMode(h.chatID) {
		t.Fatal("a project chat should resolve to Code Mode on")
	}

	h.turn(t, "read cmd/main.go")
	_, reqs := h.turn(t, "now add a flag")

	r := firstAgentReq(t, reqs)
	sys := r.system()
	if !strings.HasPrefix(sys, "You are a coding agent") {
		t.Fatalf("Code Mode system prompt should be the coding directive, got: %q", sys)
	}
	full := r.allText()
	for _, banned := range []string{"You are Memo", "AI friend", "[Time context]", "RELEVANT MEMORIES", "Communication Style"} {
		if strings.Contains(full, banned) {
			t.Errorf("Code Mode leaked chat content: %q", banned)
		}
	}
	// working set is coding infra — it stays.
	if !strings.Contains(full, "[Working set") || !strings.Contains(full, "cmd/main.go") {
		t.Errorf("Code Mode should keep the working-set digest; request text:\n%s", full)
	}

	// finishStream suppressions: no chat-title generation call for a Code Mode
	// chat, so no `title` usage row — only `agent`.
	agentRow := h.waitUsageCategory(t, categoryAgent)
	if agentRow.Requests != 2 {
		t.Errorf("agent request count = %d, want 2", agentRow.Requests)
	}
	sum := h.usageSummary(t)
	for _, c := range sum.CategoryBreakdown {
		if c.Category == categoryTitle {
			t.Errorf("Code Mode should not run chat-title generation, found %d %q rows", c.Requests, c.Category)
		}
	}
	// no title-generation HTTP request either
	for _, b := range h.fp.reqBodies() {
		if strings.Contains(b, "generate a very short chat title") {
			t.Error("Code Mode fired a chat-title generation request")
		}
	}
}

// TestIntegration_CodeModeLoopBoundsAndEdits: Code Mode uses its own
// (higher) loop ceilings and lets Medium-danger edits run without a
// permission prompt.
func TestIntegration_CodeModeLoopBoundsAndEdits(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		// n=1: a write_file (Medium). Then keep asking for tool calls so the
		// loop runs to its Code Mode ceiling.
		if n == 1 {
			return respToolCall("fake-model", "w1", "write_file",
				map[string]any{"path": "gen.txt", "content": "generated"}, 100, 5)
		}
		return respToolCall("fake-model", fmt.Sprintf("r%d", n), "read_file",
			map[string]any{"path": "gen.txt"}, 100, 5)
	})
	h := newLongSessionApp(t, fp, false)
	h.chatID = h.sm.NewAgentChat(t.TempDir())
	// small Code Mode ceilings so the test is quick; the non-Code values stay
	// at the Default() 40 / 2.
	h.a.cfgMu.Lock()
	h.a.cfg.AgentMode.CodeModeMaxIterations = 2
	h.a.cfg.AgentMode.CodeModeMaxContinuations = 1
	h.a.cfgMu.Unlock()

	reply, reqs := h.turn(t, "generate gen.txt then keep reading it")

	// 2 iters x (1 + 1 continuation) = 4 model calls, then the bounded stop.
	if n := len(agentReqs(t, reqs)); n != 4 {
		t.Errorf("Code Mode agent calls = %d, want 4 (CodeModeMaxIterations 2 x (1+1))", n)
	}
	if !strings.Contains(reply, "maximum number of tool calls") {
		t.Errorf("expected the bounded hard-stop, got %q", reply)
	}
	// write_file (Medium) must have run despite there being no permission
	// prompt path in this harness.
	proj := h.sm.GetProjectPath(h.chatID)
	if b, err := os.ReadFile(filepath.Join(proj, "gen.txt")); err != nil || string(b) != "generated" {
		t.Errorf("write_file did not run under Code Mode auto-approve: err=%v content=%q", err, b)
	}
}

// ===========================================================================
// Minimal Mode — truly zero injection, so the local KV cache prefix is stable
// ===========================================================================

func TestIntegration_MinimalModeStablePrefix(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		return respText("fake-model", fmt.Sprintf("reply %d", n), 100, 5)
	})
	h := newLongSessionApp(t, fp, false)
	h.a.identity.SetMinimalMode(true)

	// Two turns.
	h.turn(t, "first question")
	h.turn(t, "second question")

	rs := agentReqs(t, h.fp.reqBodies())
	// (no tools in a plain minimal chat — the fake never asked for a tool
	// call, so these are plain chat requests; grab them directly)
	all := parseAll(t, h.fp.reqBodies())
	if len(all) < 2 {
		t.Fatalf("want >=2 requests, got %d", len(all))
	}
	_ = rs

	// The 2nd request's message list must START with the exact messages the
	// 1st request sent (append-only), with NO Memo-injected preamble that
	// changed between them — that byte-stable prefix is what lets
	// llama-server reuse its KV cache instead of re-prefilling everything.
	r1, r2 := all[len(all)-2], all[len(all)-1]
	if len(r2.Messages) <= len(r1.Messages) {
		t.Fatalf("turn 2 should have more messages than turn 1 (append-only); got %d vs %d", len(r2.Messages), len(r1.Messages))
	}
	for i := range r1.Messages {
		c1, _ := r1.Messages[i].Content.(string)
		c2, _ := r2.Messages[i].Content.(string)
		if r1.Messages[i].Role != r2.Messages[i].Role || c1 != c2 {
			t.Fatalf("minimal-mode prefix diverged at message %d:\n  turn1: [%s] %q\n  turn2: [%s] %q",
				i, r1.Messages[i].Role, c1, r2.Messages[i].Role, c2)
		}
	}
	// And no system message / no [Time context] anywhere.
	for _, m := range r2.Messages {
		s, _ := m.Content.(string)
		if m.Role == "system" {
			t.Errorf("minimal mode emitted a system message: %q", s)
		}
		if strings.Contains(s, "[Time context]") || strings.Contains(s, "You are Memo") {
			t.Errorf("minimal mode leaked injected content: %q", s)
		}
	}
}

func parseAll(t *testing.T, bodies []string) []capturedReq {
	t.Helper()
	out := make([]capturedReq, 0, len(bodies))
	for _, b := range bodies {
		// skip the title-generation call (single user message, no history)
		r := parseReq(t, b)
		if len(r.Messages) == 1 {
			if s, _ := r.Messages[0].Content.(string); strings.Contains(s, "generate a very short chat title") {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// ===========================================================================
// Phase 1 — memory block is capped and carries the user's words only
// ===========================================================================

func TestIntegration_MemoryBlock_CappedAndUserOnly(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		return respText("fake-model", "ok", 100, 5)
	})
	h := newLongSessionApp(t, fp, true) // withMemory

	// Toy 8-dim embedder — take the similarity gate out of the way so
	// retrieval definitely returns rows; the cap and strip are what we check.
	h.a.cfgMu.Lock()
	h.a.cfg.Memory.MinSimilarity = 0
	h.a.cfgMu.Unlock()

	ctx := context.Background()
	for i := 0; i < 15; i++ {
		if err := h.a.store.SaveExplicit(ctx, fmt.Sprintf("kullanici hakkinda kalici gercek numara %d", i), "profile"); err != nil {
			t.Fatalf("SaveExplicit: %v", err)
		}
	}
	for i := 0; i < 3; i++ {
		if err := h.a.store.SaveInteraction(ctx,
			fmt.Sprintf("kullanicidan gelen mesaj %d biraz uzun olsun ki kaydedilsin", i),
			fmt.Sprintf("MEMONUN CEVABI %d bu satir promptta gorunmemeli", i)); err != nil {
			t.Fatalf("SaveInteraction: %v", err)
		}
	}

	_, reqs := h.turn(t, "kullanici hakkinda ne biliyorsun kalici gercek")
	sys := firstAgentReq(t, reqs).system()

	if !strings.Contains(sys, "RELEVANT MEMORIES") {
		t.Fatalf("no memory block in the system prompt:\n%s", sys)
	}
	if pinned := strings.Count(sys, "importance=pinned"); pinned > 10 {
		t.Errorf("pinned facts in prompt = %d, want <= 10 (pinned_facts_per_turn)", pinned)
	}
	// stripAssistant: none of Memo's stored replies may appear.
	if strings.Contains(sys, "MEMONUN CEVABI") {
		t.Errorf("assistant reply text leaked into the memory block despite stripAssistant:\n%s", sys)
	}
}

// ===========================================================================
// Phase 3 — configurable maxIters and bounded auto-continue
// ===========================================================================

func TestIntegration_MaxItersAndAutoContinue(t *testing.T) {
	// Always ask for another tool call — the loop only stops on the bound.
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		return respToolCall("fake-model", fmt.Sprintf("c%d", n), "read_file",
			map[string]any{"path": "go.mod"}, 200, 5)
	})
	h := newLongSessionApp(t, fp, false)
	h.a.agentExecutor.SetMaxIterations(3)
	h.a.agentExecutor.SetMaxContinuations(2)

	reply, reqs := h.turn(t, "keep going forever")

	// 3 iters x (1 + 2 continuations) = 9 model calls, then a hard stop.
	if n := len(agentReqs(t, reqs)); n != 9 {
		t.Errorf("agent provider calls this turn = %d, want 9 (maxIters 3 x (1+2))", n)
	}
	if !strings.Contains(reply, "maximum number of tool calls") {
		t.Errorf("expected the bounded hard-stop message, got %q", reply)
	}
	// The auto-continue nudge must have reached the model.
	sawNudge := false
	for _, b := range reqs {
		if strings.Contains(b, "auto-continue") {
			sawNudge = true
		}
	}
	if !sawNudge {
		t.Errorf("auto-continue nudge never sent upstream")
	}
}

// ===========================================================================
// Phase 4 — tools are byte-stable and (for anthropic) cache-controlled
// ===========================================================================

func TestIntegration_ToolScheduleByteStableAcrossIterations(t *testing.T) {
	fp := newFakeProvider(t, func(n int, req map[string]any) map[string]any {
		if n < 3 {
			return respToolCall("fake-model", fmt.Sprintf("c%d", n), "read_file",
				map[string]any{"path": "go.mod"}, 100, 5)
		}
		return respText("fake-model", "done", 100, 5)
	})
	h := newLongSessionApp(t, fp, false)

	_, reqs := h.turn(t, "do a few things")
	rs := agentReqs(t, reqs)
	if len(rs) < 3 {
		t.Fatalf("want >=3 agent iterations, got %d", len(rs))
	}
	first := string(rs[0].Tools)
	for i, r := range rs {
		if string(r.Tools) != first {
			t.Fatalf("tools array changed between iterations (call %d) — breaks prompt caching", i)
		}
	}
}

// small compile-time guard: the harness uses api.Message
var _ = api.NewTextMessage
