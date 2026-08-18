package orchestra

import (
	"context"
	"errors"
	"testing"

	"memo/internal/provider"
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

// TestChiefAttemptSetsEffortLevelFromProviderConfig verifies chiefAttempt
// copies the matched provider config's EffortLevel onto the outgoing
// ChatRequest the same way it already copies Model — covering RunSingle,
// createPlan, and synthesize in one place since all three funnel through
// runChiefWithFallback -> chiefAttempt.
func TestChiefAttemptSetsEffortLevelFromProviderConfig(t *testing.T) {
	f := newMockFactory()
	mp := openAIMock("ok")
	f.set("openai", mp)

	getConfigs := func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Name: "test-openai", Type: "openai", Enabled: true, Model: "gpt-4o", EffortLevel: "high"},
		}
	}
	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"
	c := NewConductor(cfg, f.factory, getConfigs)

	if _, err := c.RunSingle(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("RunSingle: %v", err)
	}

	if got := mp.LastRequest().EffortLevel; got != "high" {
		t.Errorf("ChatRequest.EffortLevel = %q, want %q", got, "high")
	}
}

// TestChiefAttemptOmitsEffortLevelWhenUnset guards the other half: a
// provider config with no EffortLevel set must leave the ChatRequest field
// empty too, not some stale/leftover value.
func TestChiefAttemptOmitsEffortLevelWhenUnset(t *testing.T) {
	f := newMockFactory()
	mp := openAIMock("ok")
	f.set("openai", mp)

	getConfigs := func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Name: "test-openai", Type: "openai", Enabled: true, Model: "gpt-4o"},
		}
	}
	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"
	c := NewConductor(cfg, f.factory, getConfigs)

	if _, err := c.RunSingle(context.Background(), "sys", "user"); err != nil {
		t.Fatalf("RunSingle: %v", err)
	}

	if got := mp.LastRequest().EffortLevel; got != "" {
		t.Errorf("ChatRequest.EffortLevel = %q, want empty", got)
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
