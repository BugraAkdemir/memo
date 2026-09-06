package agent

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
)

type panicProvider struct{}

func (panicProvider) ChatCompletion(ctx context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	panic("boom")
}

// TestRunStream_RecoversPanic guards BUG-H3: a panic anywhere in the agent's
// LLM call loop (e.g. a bad provider response) must not crash the whole
// backend — it must be recovered, logged, and surfaced to the caller as an
// error chunk on the stream instead, the same way taskloop/engine.go's
// run() already protects task-list goroutines.
func TestRunStream_RecoversPanic(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(t.TempDir()))
	pipeline := NewPipeline(registry, permissions, sandbox, panicProvider{}, nil)

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}

	var gotError, gotDone bool
	for chunk := range ch {
		if chunk.Error != "" {
			gotError = true
		}
		if chunk.Done {
			gotDone = true
		}
	}
	if !gotError {
		t.Error("expected an error chunk after the panicking ChatCompletion call")
	}
	if !gotDone {
		t.Error("expected the channel to still be closed with a Done chunk, not just silently dropped")
	}
}

// capturingProvider records the last ChatRequest it received, for
// asserting what Pipeline.RunStream actually sends upstream.
type capturingProvider struct{ gotReq provider.ChatRequest }

func (p *capturingProvider) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	p.gotReq = req
	return &provider.ChatResponse{Content: "ok"}, nil
}

// TestRunStream_SetsEffortLevelOnRequest verifies the Executor->Pipeline
// plumbing: effortLevel set on the pipeline (see executor.go's
// pipeline.effortLevel = effortLevel) ends up on the outgoing
// ChatRequest, the same way agent/orchestra's other request fields do.
func TestRunStream_SetsEffortLevelOnRequest(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(t.TempDir()))
	prov := &capturingProvider{}
	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)
	pipeline.effortLevel = "high"

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	for range ch {
	}

	if prov.gotReq.EffortLevel != "high" {
		t.Errorf("ChatRequest.EffortLevel = %q, want %q", prov.gotReq.EffortLevel, "high")
	}
}

// scriptedProvider returns one canned ChatResponse per call, in order —
// lets a test drive a specific sequence of tool calls without a real LLM.
type scriptedProvider struct {
	responses []provider.ChatResponse
	calls     int
}

func (p *scriptedProvider) ChatCompletion(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	resp := p.responses[p.calls]
	p.calls++
	return &resp, nil
}

func mustToolCall(t *testing.T, id, name string, args any) provider.ToolCall {
	t.Helper()
	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatalf("marshal tool args: %v", err)
	}
	return provider.ToolCall{ID: id, Type: "function", Function: provider.ToolCallFunction{Name: name, Arguments: raw}}
}

// TestRunStream_ChangeDirectoryTakesEffectSameTurn is the "look then act in
// one message" guarantee change_directory (internal/agent/tools/changedir.go)
// depends on: Pipeline.RunStream re-reads the sandbox's base path fresh at
// the top of every iteration (see the "Snapshot basePath once per
// iteration" comment below), so a change_directory call in iteration N must
// already be visible to a read_file call in iteration N+1 of the *same*
// RunStream call — not just on the next turn.
func TestRunStream_ChangeDirectoryTakesEffectSameTurn(t *testing.T) {
	oldDir := t.TempDir()
	newDir := t.TempDir()
	marker := filepath.Join(newDir, "marker.txt")
	if err := os.WriteFile(marker, []byte("hello-from-new-dir"), 0644); err != nil {
		t.Fatal(err)
	}

	registry := NewRegistry() // includes change_directory and read_file
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(oldDir))
	backup := NewBackupManager(t.TempDir())

	prov := &scriptedProvider{responses: []provider.ChatResponse{
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "call-1", "change_directory", map[string]string{"path": newDir})}},
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "call-2", "read_file", map[string]string{"path": "marker.txt"})}},
		{Content: "done"},
	}}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, backup)
	// Dangerous-level tools (change_directory included) would otherwise
	// block on a permission prompt — same bypass a real Shift+Tab session
	// uses (executor.go's autoPermission), so this test can run unattended.
	pipeline.autoPermission = true

	var toolResults []string
	var executingArgs []string
	onEvent := func(ev AgentEvent) {
		if ev.Type == EventToolResult {
			toolResults = append(toolResults, ev.Result)
		}
		if ev.Type == EventToolExecuting {
			executingArgs = append(executingArgs, string(ev.Args))
		}
	}

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", onEvent, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	var gotErrorChunk string
	for chunk := range ch {
		if chunk.Error != "" {
			gotErrorChunk = chunk.Error
		}
	}
	if gotErrorChunk != "" {
		t.Fatalf("unexpected error chunk: %s", gotErrorChunk)
	}

	if len(toolResults) != 2 {
		t.Fatalf("got %d tool results, want 2 (change_directory, read_file): %v", len(toolResults), toolResults)
	}
	if !strings.Contains(toolResults[1], "hello-from-new-dir") {
		t.Errorf("read_file's result (run right after change_directory, same turn) = %q, want it to contain the new directory's marker content — the sandbox root did not move in time", toolResults[1])
	}

	// tool_executing must carry Args: the live task card renders it as the
	// "starting" line for slow tools, and shipped printing a bare "Komut …"
	// because this emission (the one the agent loop actually uses) left them
	// out — found by running a real task, not by a test.
	if len(executingArgs) != 2 {
		t.Fatalf("got %d tool_executing events, want 2", len(executingArgs))
	}
	for i, a := range executingArgs {
		if a == "" || a == "null" {
			t.Errorf("tool_executing[%d] carried no Args — the starting line has nothing to show", i)
		}
	}
}

func TestStripHallucinatedToolSyntax(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"no hallucinated syntax, untouched",
			"Merhaba! Nasıl yardımcı olabilirim?",
			"Merhaba! Nasıl yardımcı olabilirim?",
		},
		{
			"strips a full function_calls block",
			`Selam kanka!<function_calls><invoke name="read_env"></invoke></function_calls>`,
			"Selam kanka!",
		},
		{
			"strips an unclosed function_calls block to end of string",
			`Cevap burada.<function_calls><invoke name="list_directory">`,
			"Cevap burada.",
		},
		{
			"only hallucinated syntax, nothing else",
			`<function_calls><invoke name="read_env"></invoke></function_calls>`,
			"",
		},
		{
			"unrelated content mentioning function words is untouched",
			"function_calls is a term used in some APIs, not a Memo feature.",
			"function_calls is a term used in some APIs, not a Memo feature.",
		},
		{
			"strips an opencode-zen <tool_call:id> block, keeps surrounding prose",
			"Merhaba! Hafızama bakayım.<tool_calls:6124c78e> <tool_call:6124c78e>Bash command`> ls -la ~/.claude/ description`> list</tool_calls:6124c78e> Tamam.",
			"Merhaba! Hafızama bakayım. Tamam.",
		},
		{
			"strips an unclosed <tool_call:id> block to end of string",
			"Bakıyorum.<tool_call:abc123>Bash command`> echo hi",
			"Bakıyorum.",
		},
		{
			"strips a multi-opener <tool_calls:id> leak entirely",
			"<tool_calls:6124c78e> <tool_call:6124c78e>Bash x <tool_call:6124c78e>Bash y </tool_calls:6124c78e>",
			"",
		},
		{
			"plain <tool_calls> with no id is still stripped",
			"Cevap.<tool_calls>stuff</tool_calls>",
			"Cevap.",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := stripHallucinatedToolSyntax(tc.in)
			if got != tc.want {
				t.Errorf("stripHallucinatedToolSyntax(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// fakeContentProvider returns a fixed ChatResponse with no ToolCalls,
// simulating a model that free-associated pseudo-tool-call text into its
// content instead of making a real structured call.
type fakeContentProvider struct{ content string }

func (p fakeContentProvider) ChatCompletion(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	return &provider.ChatResponse{Content: p.content}, nil
}

// TestRunStream_StripsHallucinatedToolCallFromFinalContent is the direct
// regression test for the reported bug: a plain question got a reply with
// raw "<function_calls><invoke name=\"read_env\">..." XML leaking into the
// visible chat text, from a model that doesn't reliably use real
// tool_calls. Since resp.ToolCalls is empty, this pipeline treats
// resp.Content as the final answer — it must not pass hallucinated
// tool-call syntax through to the user verbatim.
func TestRunStream_StripsHallucinatedToolCallFromFinalContent(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(t.TempDir()))
	prov := fakeContentProvider{content: `Selam kanka!<function_calls><invoke name="read_env"></invoke></function_calls>`}
	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}

	var gotContent string
	for chunk := range ch {
		gotContent += chunk.Content
	}
	if strings.Contains(gotContent, "<function_calls>") {
		t.Fatalf("hallucinated tool-call XML leaked into final content: %q", gotContent)
	}
	if gotContent != "Selam kanka!" {
		t.Errorf("final content = %q, want %q", gotContent, "Selam kanka!")
	}
}

// TestRunStream_AccumulatesUsageOntoTerminalChunk is the Phase 0 measurement
// guarantee: the pipeline runs one non-streaming ChatCompletion per
// iteration, each separately billed, so the terminal chunk must carry the
// sum of every iteration's prompt AND completion tokens — that is what lets
// callAgentStream record a real per-turn cost instead of a word-count
// estimate of the seed messages alone.
func TestRunStream_AccumulatesUsageOntoTerminalChunk(t *testing.T) {
	registry := NewRegistry() // has read_file
	permissions := NewPermissionManager(t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hi"), 0644); err != nil {
		t.Fatal(err)
	}
	sandbox := NewSandbox(DefaultSandboxConfig(dir))

	prov := &scriptedProvider{responses: []provider.ChatResponse{
		{
			ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "a.txt"})},
			Usage:     &provider.Usage{PromptTokens: 1000, CompletionTokens: 40, TotalTokens: 1040},
		},
		{
			Content: "done",
			Usage:   &provider.Usage{PromptTokens: 1200, CompletionTokens: 15, TotalTokens: 1215},
		},
	}}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)
	pipeline.autoPermission = true

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}

	var terminal *provider.StreamChunk
	for chunk := range ch {
		c := chunk
		if c.Done {
			terminal = &c
		}
	}
	if terminal == nil {
		t.Fatal("no terminal (Done) chunk received")
	}
	if terminal.Usage == nil {
		t.Fatal("terminal chunk carried no Usage")
	}
	if terminal.Usage.PromptTokens != 2200 {
		t.Errorf("PromptTokens = %d, want 2200 (1000+1200)", terminal.Usage.PromptTokens)
	}
	if terminal.Usage.CompletionTokens != 55 {
		t.Errorf("CompletionTokens = %d, want 55 (40+15)", terminal.Usage.CompletionTokens)
	}
}

// TestRunStream_NoUsageLeavesTerminalChunkUsageNil confirms the fallback
// path stays intact: a provider that reports no usage must not cause a
// zero-valued Usage to be attached (which callAgentStream would misread as
// "real, and zero"), it must stay nil so the word-count estimate is kept.
func TestRunStream_NoUsageLeavesTerminalChunkUsageNil(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(t.TempDir()))
	prov := fakeContentProvider{content: "hello"}
	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)

	ch, err := pipeline.RunStream(context.Background(), nil, "test-model", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	for chunk := range ch {
		if chunk.Done && chunk.Usage != nil {
			t.Fatalf("expected nil Usage on terminal chunk when provider reported none, got %+v", *chunk.Usage)
		}
	}
}

// recordingProvider keeps every request it was sent, and replays a script of
// responses, so a test can inspect what the pipeline sent on a later
// iteration.
type recordingProvider struct {
	responses []provider.ChatResponse
	calls     int
	seen      [][]provider.Message
}

func (p *recordingProvider) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	msgs := make([]provider.Message, len(req.Messages))
	copy(msgs, req.Messages)
	p.seen = append(p.seen, msgs)
	r := p.responses[p.calls]
	p.calls++
	return &r, nil
}

// TestRunStream_EvictedToolOutputLeavesMarker: when intra-turn truncation
// drops an earlier tool result to fit the budget, the model must get a
// one-line [context-trim] marker on the next request instead of silence, so
// it re-runs a tool deliberately rather than assuming it never ran.
func TestRunStream_EvictedToolOutputLeavesMarker(t *testing.T) {
	dir := t.TempDir()
	big := strings.Repeat("x ", 40000) // ~27k len/3 tokens — dwarfs the tiny budget below
	if err := os.WriteFile(filepath.Join(dir, "big.txt"), []byte(big), 0644); err != nil {
		t.Fatal(err)
	}
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(dir))

	prov := &recordingProvider{responses: []provider.ChatResponse{
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "c1", "read_file", map[string]string{"path": "big.txt"})}},
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "c2", "list_directory", map[string]string{"path": "."})}},
		{Content: "done"},
	}}

	// Budget small enough that iteration 1's ~27k-token tool result cannot be
	// kept alongside the rest.
	pipeline := NewPipelineWithBudget(registry, permissions, sandbox, prov, nil, 4000)
	pipeline.autoPermission = true

	ch, err := pipeline.RunStream(context.Background(), []provider.Message{{Role: "system", Content: "sys"}}, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	for range ch {
	}

	if len(prov.seen) < 2 {
		t.Fatalf("expected at least 2 upstream requests, got %d", len(prov.seen))
	}
	last := prov.seen[len(prov.seen)-1]
	found := false
	for _, m := range last {
		if s, ok := m.Content.(string); ok && strings.HasPrefix(s, contextTrimMarker) {
			found = true
			if !strings.Contains(s, "read_file") {
				t.Errorf("marker should name the evicted tool, got %q", s)
			}
		}
	}
	if !found {
		t.Errorf("no %s marker in the final request after an eviction; messages=%+v", contextTrimMarker, last)
	}
}

// TestRunStream_MaxItersOverride confirms pipeline.maxIters (set by
// Executor from config.AgentMode.MaxIterations) actually bounds the loop
// and is reflected in the ceiling message.
func TestRunStream_MaxItersOverride(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	sandbox := NewSandbox(DefaultSandboxConfig(dir))

	// Always ask for another tool call — the loop only ends on the cap.
	resp := provider.ChatResponse{ToolCalls: []provider.ToolCall{mustToolCall(t, "c", "read_file", map[string]string{"path": "a.txt"})}}
	prov := &scriptedProvider{responses: []provider.ChatResponse{resp, resp, resp, resp, resp, resp, resp, resp}}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)
	pipeline.autoPermission = true
	pipeline.maxIters = 3

	ch, err := pipeline.RunStream(context.Background(), nil, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var text string
	for c := range ch {
		text += c.Content
	}
	if prov.calls != 3 {
		t.Errorf("ChatCompletion calls = %d, want 3 (the maxIters cap)", prov.calls)
	}
	if !strings.Contains(text, "(3)") {
		t.Errorf("ceiling message should name the configured cap, got %q", text)
	}
}

// TestRunStream_AutoContinue: when the loop hits maxIters while still making
// tool calls, maxContinuations>0 restarts it with a nudge instead of
// hard-stopping — bounded, so a genuinely long task can finish.
func TestRunStream_AutoContinue(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	sandbox := NewSandbox(DefaultSandboxConfig(dir))

	toolResp := provider.ChatResponse{ToolCalls: []provider.ToolCall{mustToolCall(t, "c", "read_file", map[string]string{"path": "a.txt"})}}
	// 2 iters, then a continuation of 2 more, then it "finishes".
	prov := &recordingProvider{responses: []provider.ChatResponse{
		toolResp, toolResp, // pass 1 (maxIters=2)
		toolResp, {Content: "finally done"}, // pass 2 after auto-continue
	}}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)
	pipeline.autoPermission = true
	pipeline.maxIters = 2
	pipeline.maxContinuations = 1

	ch, err := pipeline.RunStream(context.Background(), []provider.Message{{Role: "system", Content: "s"}}, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var text string
	for c := range ch {
		text += c.Content
	}
	if !strings.Contains(text, "finally done") {
		t.Errorf("expected the turn to finish after the continuation, got %q", text)
	}
	if strings.Contains(text, "maximum number of tool calls") {
		t.Errorf("should not have hit the hard ceiling, got %q", text)
	}
	// The continuation nudge must have been sent upstream.
	sawNudge := false
	for _, req := range prov.seen {
		for _, m := range req {
			if s, ok := m.Content.(string); ok && strings.Contains(s, "[auto-continue 1/1]") {
				sawNudge = true
			}
		}
	}
	if !sawNudge {
		t.Errorf("auto-continue nudge was never sent to the model")
	}
}

// TestRunStream_AutoContinueExhausted: with continuations used up, it still
// hard-stops (bounded).
func TestRunStream_AutoContinueExhausted(t *testing.T) {
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	sandbox := NewSandbox(DefaultSandboxConfig(dir))
	toolResp := provider.ChatResponse{ToolCalls: []provider.ToolCall{mustToolCall(t, "c", "read_file", map[string]string{"path": "a.txt"})}}
	rs := make([]provider.ChatResponse, 20)
	for i := range rs {
		rs[i] = toolResp
	}
	prov := &scriptedProvider{responses: rs}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)
	pipeline.autoPermission = true
	pipeline.maxIters = 2
	pipeline.maxContinuations = 1

	ch, _ := pipeline.RunStream(context.Background(), nil, "m", func(AgentEvent) {}, nil)
	var text string
	for c := range ch {
		text += c.Content
	}
	if !strings.Contains(text, "maximum number of tool calls") {
		t.Errorf("expected hard stop once continuations are exhausted, got %q", text)
	}
	// 2 (pass 1) + 2 (pass 2) = 4 model calls, then stop.
	if prov.calls != 4 {
		t.Errorf("ChatCompletion calls = %d, want 4 (maxIters 2 x (1 + 1 continuation))", prov.calls)
	}
}

// TestRunStream_AutoApproveMedium: with autoApproveMedium set (Code Mode), a
// Medium-danger tool like write_file executes without a permission prompt —
// no permissionWaitFn is provided and the pipeline must not block or panic
// waiting for one.
func TestRunStream_AutoApproveMedium(t *testing.T) {
	dir := t.TempDir()
	registry := NewRegistry()
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(dir))
	backup := NewBackupManager(t.TempDir())

	prov := &scriptedProvider{responses: []provider.ChatResponse{
		{ToolCalls: []provider.ToolCall{mustToolCall(t, "w1", "write_file", map[string]string{
			"path": "out.txt", "content": "hello from code mode",
		})}},
		{Content: "wrote it"},
	}}

	pipeline := NewPipeline(registry, permissions, sandbox, prov, backup)
	pipeline.autoApproveMedium = true // no autoPermission, no bypass

	ch, err := pipeline.RunStream(context.Background(), nil, "m", func(AgentEvent) {}, nil)
	if err != nil {
		t.Fatalf("RunStream: %v", err)
	}
	var text string
	sawDenied := false
	for c := range ch {
		text += c.Content
		if c.Error != "" {
			text += c.Error
		}
	}
	_ = sawDenied
	if !strings.Contains(text, "wrote it") {
		t.Fatalf("turn did not complete cleanly: %q", text)
	}
	data, err := os.ReadFile(filepath.Join(dir, "out.txt"))
	if err != nil || string(data) != "hello from code mode" {
		t.Fatalf("write_file did not run under autoApproveMedium: err=%v data=%q", err, data)
	}
}
