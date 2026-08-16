package orchestra

import (
	"context"
	"errors"
	"sync"
	"testing"

	"memo/internal/provider"
)

// sequenceProvider returns one canned ChatResponse per call, in order —
// needed because a single RunAgentTasks call makes two distinct chief calls
// (plan, then synthesis) that must return different shapes ({"tasks":...}
// JSON vs a plain final answer), unlike the single-response mockProvider
// used elsewhere.
type sequenceProvider struct {
	mu        sync.Mutex
	responses []string
	calls     int
}

func (p *sequenceProvider) Name() provider.ProviderType { return "openai" }
func (p *sequenceProvider) DisplayName() string          { return "OpenAI" }
func (p *sequenceProvider) ListModels(_ context.Context) ([]string, error) { return nil, nil }

func (p *sequenceProvider) ChatCompletion(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.calls >= len(p.responses) {
		return nil, errors.New("sequenceProvider: no more canned responses")
	}
	resp := p.responses[p.calls]
	p.calls++
	return &provider.ChatResponse{Content: resp}, nil
}

func (p *sequenceProvider) ChatCompletionStream(_ context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	resp, err := p.ChatCompletion(context.Background(), req)
	if err != nil {
		return nil, err
	}
	ch := make(chan provider.StreamChunk, 1)
	ch <- provider.StreamChunk{Content: resp.Content}
	close(ch)
	return ch, nil
}

var _ provider.Provider = (*sequenceProvider)(nil)

// Regression test: giving RunAgentTasks an agentRunner must route every
// task's actual work through it instead of the plain
// CreateProviderForType/ChatCompletion path — reproduced live (twice): a
// task executed as a plain completion has no way to run a real command, so
// it could only narrate "couldn't actually call run_command, simulated it",
// and that fabricated admission then got fed back into the conversation as
// a fake assistant turn, priming any later real agent pass to skip the tool
// call too (self-fulfilling prophecy). This asserts the task's role model
// (sequenceProvider, standing in for CreateProviderForType's target) is
// never invoked for task execution when agentRunner is provided — only for
// the chief's own plan/synthesis calls.
func TestRunAgentTasksUsesAgentRunnerNotPlainCompletion(t *testing.T) {
	taskProvider := &sequenceProvider{responses: []string{
		"THIS SHOULD NEVER BE CALLED — task execution must use agentRunner",
	}}
	chief := &sequenceProvider{responses: []string{
		`{"tasks": [{"role": "general", "context": "run echo combo-test", "depends_on": []}], "parallel": false}`,
		"The command ran and printed: combo-test",
	}}

	f := newMockFactory()
	f.providers["openai"] = &mockProvider{name: "openai"} // placeholder, overridden per-call below
	cfg := defaultEnabledConfig()
	cfg.ChiefType = "chief-provider"
	cfg.ChiefModel = "chief-model"

	// Two distinct provider "types" so the chief's calls (chief-provider)
	// never reach the same provider instance as a task's would-be plain
	// completion (openai, matching the default role's ModelType).
	c := NewConductor(cfg, func(pc provider.ProviderConfig) (provider.Provider, error) {
		switch string(pc.Type) {
		case "chief-provider":
			return chief, nil
		case "openai":
			return taskProvider, nil
		}
		return nil, errors.New("unknown provider type")
	}, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{
			{Name: "chief", Type: "chief-provider", Enabled: true, Model: "chief-model"},
			{Name: "task", Type: "openai", Enabled: true, Model: "gpt-4o"},
		}
	})

	var agentRunnerCalls []string
	agentRunner := func(_ context.Context, prompt string, onEvent func(string)) (string, error) {
		agentRunnerCalls = append(agentRunnerCalls, prompt)
		onEvent(`{"type":"tool_executing","tool_name":"run_command"}`)
		return "combo-test", nil
	}

	result, results, err := c.RunAgentTasks(context.Background(), "run echo combo-test", nil, agentRunner)
	if err != nil {
		t.Fatalf("RunAgentTasks: %v", err)
	}

	if len(agentRunnerCalls) != 1 {
		t.Fatalf("expected agentRunner to be called exactly once, got %d calls", len(agentRunnerCalls))
	}
	if agentRunnerCalls[0] != "run echo combo-test" {
		t.Errorf("agentRunner got prompt %q, want the task's context", agentRunnerCalls[0])
	}
	if taskProvider.calls != 0 {
		t.Errorf("task's plain-completion provider was called %d times, want 0 — task execution must go through agentRunner, not CreateProviderForType", taskProvider.calls)
	}
	if len(results) != 1 || results[0].Content != "combo-test" {
		t.Fatalf("expected the task result to carry agentRunner's real output, got %+v", results)
	}
	if result == "" {
		t.Error("expected a non-empty synthesized final response")
	}
}

// Regression test: an agentRunner error must surface as the task's error
// (so the chief's synthesis can report it honestly) rather than silently
// falling back to a plain completion.
func TestRunAgentTasksPropagatesAgentRunnerError(t *testing.T) {
	chief := &sequenceProvider{responses: []string{
		`{"tasks": [{"role": "general", "context": "do the thing", "depends_on": []}], "parallel": false}`,
		"The task failed: permission denied.",
	}}
	cfg := defaultEnabledConfig()
	cfg.ChiefType = "chief-provider"
	cfg.ChiefModel = "chief-model"

	c := NewConductor(cfg, func(pc provider.ProviderConfig) (provider.Provider, error) {
		if string(pc.Type) == "chief-provider" {
			return chief, nil
		}
		return nil, errors.New("unexpected provider type in this test")
	}, func() []provider.ProviderConfig {
		return []provider.ProviderConfig{{Name: "chief", Type: "chief-provider", Enabled: true, Model: "chief-model"}}
	})

	agentRunner := func(_ context.Context, _ string, _ func(string)) (string, error) {
		return "", errors.New("permission denied")
	}

	_, results, err := c.RunAgentTasks(context.Background(), "do the thing", nil, agentRunner)
	if err != nil {
		t.Fatalf("RunAgentTasks: %v (synthesis should still run over the failed task)", err)
	}
	if len(results) != 1 || results[0].Error == "" {
		t.Fatalf("expected the task result to carry agentRunner's error, got %+v", results)
	}
}
