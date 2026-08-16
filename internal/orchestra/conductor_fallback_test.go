package orchestra

import (
	"context"
	"errors"
	"testing"

	"memo/internal/provider"
)

// twoProviderConfigs returns openai (priority 10) and claude (priority 5) —
// both enabled, each carrying its own distinct, realistic Model — for
// fallback tests that need a genuine second provider to fall back to.
func twoProviderConfigs() []provider.ProviderConfig {
	return []provider.ProviderConfig{
		{Name: "openai", Type: "openai", Enabled: true, Model: "gpt-4o", Priority: 10},
		{Name: "claude", Type: "claude", Enabled: true, Model: "claude-3-5-sonnet-20241022", Priority: 5},
	}
}

// TestTryFallbackProviders_UsesOwnModelNotFailedProvidersModel is the
// regression test for BUG-H3: the fallback provider's own configured Model
// must be used, not the failed primary provider's ModelName — a different
// vendor's model IDs (e.g. "gpt-4o") are meaningless, often outright
// invalid, to another vendor's API.
func TestTryFallbackProviders_UsesOwnModelNotFailedProvidersModel(t *testing.T) {
	f := newMockFactory()
	f.set("claude", openAIMock("fallback response")) // name is arbitrary; Name() isn't checked here

	c := NewConductor(DefaultConfig(), f.factory, twoProviderConfigs)

	task := OrchestraTask{
		Role:      RoleGeneral,
		ModelType: "openai", // the primary that already failed
		ModelName: "gpt-4o",
	}
	req := provider.ChatRequest{Model: task.ModelName}

	resp, err := c.tryFallbackProviders(context.Background(), task, req, 0, nil)
	if err != nil {
		t.Fatalf("tryFallbackProviders() error = %v", err)
	}
	if resp.Content != "fallback response" {
		t.Errorf("resp.Content = %q, want %q", resp.Content, "fallback response")
	}

	gotCfg, ok := f.lastConfigFor("claude")
	if !ok {
		t.Fatal("expected the claude provider to have been created")
	}
	if gotCfg.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("fallback provider created with Model = %q, want its own configured %q (not the failed provider's %q)",
			gotCfg.Model, "claude-3-5-sonnet-20241022", task.ModelName)
	}
}

// TestCreateProviderForType_FallbackKeepsOwnModel is CreateProviderForType's
// counterpart to the test above — its "requested type not found, fall back
// to any enabled provider" branch had the identical BUG-H3 bug.
func TestCreateProviderForType_FallbackKeepsOwnModel(t *testing.T) {
	f := newMockFactory()
	f.set("claude", openAIMock("irrelevant"))

	c := NewConductor(DefaultConfig(), f.factory, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Name: "claude", Type: "claude", Enabled: true, Model: "claude-3-5-sonnet-20241022"},
		}
	})

	if _, err := c.CreateProviderForType("nonexistent-type", "gpt-4o"); err != nil {
		t.Fatalf("CreateProviderForType() error = %v", err)
	}

	gotCfg, ok := f.lastConfigFor("claude")
	if !ok {
		t.Fatal("expected the claude provider to have been created")
	}
	if gotCfg.Model != "claude-3-5-sonnet-20241022" {
		t.Errorf("fallback provider created with Model = %q, want its own configured %q (not the requested %q)",
			gotCfg.Model, "claude-3-5-sonnet-20241022", "gpt-4o")
	}
}

// TestCreatePlan_FallsBackToSecondProviderOnChiefFailure is the regression
// test for BUG-H4's createPlan half: previously, any chief-provider failure
// (here, the primary "openai" provider erroring) aborted planning entirely
// with zero fallback attempt, even with another enabled, healthy provider
// (claude) available — unlike ordinary task execution, which already
// retries other providers via tryFallbackProviders.
func TestCreatePlan_FallsBackToSecondProviderOnChiefFailure(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{
		name:    "openai",
		display: "OpenAI",
		chatErr: errors.New("primary provider unavailable"),
	})
	f.set("claude", openAIMock(`{"tasks": [{"role": "general", "context": "test task"}], "parallel": true}`))

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := NewConductor(cfg, f.factory, twoProviderConfigs)

	plan, err := c.createPlan(context.Background(), cfg, "test message", nil)
	if err != nil {
		t.Fatalf("createPlan() error = %v, want success via fallback to claude", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("len(plan.Tasks) = %d, want 1", len(plan.Tasks))
	}
}

// TestSynthesize_FallsBackToSecondProviderOnChiefFailure is synthesize's
// counterpart to the createPlan test above.
func TestSynthesize_FallsBackToSecondProviderOnChiefFailure(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{
		name:    "openai",
		display: "OpenAI",
		chatErr: errors.New("primary provider unavailable"),
	})
	f.set("claude", openAIMock("synthesized answer"))

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := NewConductor(cfg, f.factory, twoProviderConfigs)

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "task", ModelType: "openai", ModelName: "gpt-4o"},
	}
	results := []OrchestraResult{
		{TaskIndex: 0, Role: RoleGeneral, Content: "task result"},
	}

	resp, err := c.synthesize(context.Background(), cfg, "user message", tasks, results, nil)
	if err != nil {
		t.Fatalf("synthesize() error = %v, want success via fallback to claude", err)
	}
	if resp != "synthesized answer" {
		t.Errorf("resp = %q, want %q", resp, "synthesized answer")
	}
}

// TestCreatePlan_AllProvidersFailReturnsError confirms the fallback loop
// still surfaces a genuine error when every candidate fails — fallback
// coverage must not turn a real, total outage into a silent/incorrect
// success.
func TestCreatePlan_AllProvidersFailReturnsError(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{name: "openai", display: "OpenAI", chatErr: errors.New("down")})
	f.set("claude", &mockProvider{name: "claude", display: "Claude", chatErr: errors.New("also down")})

	cfg := DefaultConfig()
	cfg.Enabled = true
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := NewConductor(cfg, f.factory, twoProviderConfigs)

	if _, err := c.createPlan(context.Background(), cfg, "test message", nil); err == nil {
		t.Fatal("createPlan() error = nil, want an error when every candidate provider fails")
	}
}

// TestExecuteSingleTask_FallsBackAfterMidStreamError is the regression test
// for BUG-L3: executeSingleTask's stream-chunk-error branch used to return
// immediately with no retry/fallback attempt, unlike its two sibling
// failure paths in the same function (an immediate stream-open failure
// falls through to non-streaming retry; a non-streaming ChatCompletion
// failure tries fallback providers) — a task whose stream opened fine but
// broke partway through got no second chance at all.
func TestExecuteSingleTask_FallsBackAfterMidStreamError(t *testing.T) {
	f := newMockFactory()

	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Content: "partial "}
	ch <- provider.StreamChunk{Error: "connection reset mid-stream"}
	close(ch)
	f.set("openai", &mockProvider{name: "openai", display: "OpenAI", streamCh: ch})
	f.set("claude", openAIMock("fallback answer"))

	cfg := DefaultConfig()
	cfg.Enabled = true
	c := NewConductor(cfg, f.factory, twoProviderConfigs)

	task := OrchestraTask{
		Role:      RoleGeneral,
		Context:   "task",
		ModelType: "openai",
		ModelName: "gpt-4o",
	}

	result := c.executeSingleTask(context.Background(), cfg, task, 0, func(up ProgressUpdate) {}, true, nil)
	if result.Error != "" {
		t.Fatalf("executeSingleTask() Error = %q, want success via fallback after the mid-stream error", result.Error)
	}
	if result.Content != "fallback answer" {
		t.Errorf("Content = %q, want %q", result.Content, "fallback answer")
	}
}
