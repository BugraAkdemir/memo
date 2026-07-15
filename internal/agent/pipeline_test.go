package agent

import (
	"context"
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
