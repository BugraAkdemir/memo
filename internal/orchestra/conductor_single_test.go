package orchestra

import (
	"context"
	"errors"
	"testing"
)

// Regression test: callLLM's callers (chat title generation, routine
// parsing, memory summarization, proactive checks) just want one plain-text
// completion — they must not be forced through the plan→execute→synthesize
// pipeline, which imposes a {"tasks":[...]} JSON contract the caller never
// asked for. Reproduced live: with Orchestra enabled, a routine request
// failed with "chief returned no tasks" because Run() made the chief try to
// decompose "her sabah saat 9'da bana günaydın de" into tasks instead of
// just answering it, and a plain chat-title request silently ran the full
// pipeline (plan, one task, synthesis) and took 3+ minutes.
func TestRunSingleReturnsPlainCompletionWithoutTaskJSON(t *testing.T) {
	// The mock's response is plain text, NOT {"tasks":[...]} JSON - if
	// RunSingle went through the plan/execute/synthesize pipeline like Run()
	// does, this would fail to parse as a plan and error out instead of
	// returning it directly.
	c, _ := openAIConductor(defaultEnabledConfig(), "Short Chat Title")

	got, err := c.RunSingle(context.Background(), "You generate chat titles.", "Summarize this conversation")
	if err != nil {
		t.Fatalf("RunSingle: %v", err)
	}
	if got != "Short Chat Title" {
		t.Errorf("RunSingle() = %q, want %q", got, "Short Chat Title")
	}
}

func TestRunSingleDisabled(t *testing.T) {
	c, _ := openAIConductor(DefaultConfig(), "anything") // Enabled defaults to false

	_, err := c.RunSingle(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error when orchestra mode is disabled")
	}
}

func TestRunSingleNoSystemPrompt(t *testing.T) {
	c, f := openAIConductor(defaultEnabledConfig(), "ok")

	got, err := c.RunSingle(context.Background(), "", "just a user prompt")
	if err != nil {
		t.Fatalf("RunSingle: %v", err)
	}
	if got != "ok" {
		t.Errorf("RunSingle() = %q, want %q", got, "ok")
	}
	if f.get("openai") == nil {
		t.Fatal("expected the openai mock provider to have been used")
	}
}

func TestRunSingleProviderError(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{name: "openai", display: "OpenAI", chatErr: errors.New("boom")})

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	_, err := c.RunSingle(context.Background(), "sys", "user")
	if err == nil {
		t.Fatal("expected error to propagate from the provider")
	}
}
