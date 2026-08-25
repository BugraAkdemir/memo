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
	onEvent := func(ev AgentEvent) {
		if ev.Type == EventToolResult {
			toolResults = append(toolResults, ev.Result)
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
