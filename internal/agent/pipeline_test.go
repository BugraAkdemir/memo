package agent

import (
	"context"
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
