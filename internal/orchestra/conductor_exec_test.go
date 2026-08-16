package orchestra

import (
	"context"
	"errors"
	"sync"
	"testing"

	"memo/internal/provider"
)

func TestExecuteSequential(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("task output"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "task 1", ModelType: "openai", ModelName: "gpt-4o"},
		{Role: RoleGeneral, Context: "task 2", ModelType: "openai", ModelName: "gpt-4o"},
	}
	results := make([]OrchestraResult, 2)
	c.executeSequential(context.Background(), cfg, tasks, results, nil, nil)

	for i, r := range results {
		if r.Error != "" {
			t.Errorf("task %d error: %s", i, r.Error)
		}
		if r.Content != "task output" {
			t.Errorf("task %d content mismatch", i)
		}
	}
}

func TestExecuteSequentialDeadlock(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("output"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "task A", ModelType: "openai", ModelName: "gpt-4o", DependsOn: []string{"general"}},
	}
	results := make([]OrchestraResult, 1)
	c.executeSequential(context.Background(), cfg, tasks, results, nil, nil)

	if results[0].Error == "" {
		t.Error("expected deadlock error")
	}
}

func TestExecuteSequentialDependencyOrder(t *testing.T) {
	f := newMockFactory()
	executed := make([]string, 0)
	execMu := sync.Mutex{}

	f.set("openai", &mockProvider{
		name:    "openai",
		display: "OpenAI",
	})
	f.set("openai-2", &mockProvider{
		name:     "openai",
		display:  "OpenAI",
		chatResp: "output",
	})

	execCount := 0
	cfg := DefaultConfig()
	c := NewConductor(cfg, func(cfg provider.ProviderConfig) (provider.Provider, error) {
		execMu.Lock()
		executed = append(executed, cfg.Model)
		execMu.Unlock()
		return f.get("openai-2"), nil
	}, testGetConfigs)

	_ = execCount
	_ = executed

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "dependency", ModelType: "openai", ModelName: "gpt-4o"},
		{Role: RolePlanner, Context: "depends on general", ModelType: "openai", ModelName: "gpt-4o", DependsOn: []string{"general"}},
	}
	results := make([]OrchestraResult, 2)
	c.executeSequential(context.Background(), DefaultConfig(), tasks, results, nil, nil)

	if results[0].Error != "" {
		t.Errorf("task 0 should succeed: %s", results[0].Error)
	}
	if results[1].Error != "" {
		t.Errorf("task 1 should succeed: %s", results[1].Error)
	}
}

func TestExecuteParallel(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("parallel output"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	plan := OrchestraPlan{
		Tasks: []OrchestraTask{
			{Role: RoleGeneral, Context: "task A", ModelType: "openai", ModelName: "gpt-4o"},
			{Role: RoleFrontend, Context: "task B", ModelType: "openai", ModelName: "gpt-4o"},
		},
		Parallel: true,
	}

	results := make([]OrchestraResult, 2)
	c.executeParallel(context.Background(), cfg, plan.Tasks, results, nil, nil)

	for i, r := range results {
		if r.Error != "" {
			t.Errorf("task %d error: %s", i, r.Error)
		}
		if r.Content != "parallel output" {
			t.Errorf("task %d content mismatch", i)
		}
	}
}

func TestExecuteSingleTask(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("single task output"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	task := OrchestraTask{
		Role:      RoleGeneral,
		Context:   "test task",
		ModelType: "openai",
		ModelName: "gpt-4o",
	}

	result := c.executeSingleTask(context.Background(), cfg, task, 0, nil, false, nil)
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Content != "single task output" {
		t.Errorf("expected 'single task output', got %q", result.Content)
	}
	if result.DurationMs < 0 {
		t.Error("expected non-negative duration")
	}
}

func TestExecuteSingleTaskStreaming(t *testing.T) {
	f := newMockFactory()
	ch := make(chan provider.StreamChunk, 2)
	ch <- provider.StreamChunk{Content: "hello "}
	ch <- provider.StreamChunk{Content: "world"}
	close(ch)

	f.set("openai", &mockProvider{
		name:     "openai",
		display:  "OpenAI",
		streamCh: ch,
	})

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	progressUpdates := make([]ProgressUpdate, 0)
	onProgress := func(up ProgressUpdate) {
		progressUpdates = append(progressUpdates, up)
	}

	task := OrchestraTask{
		Role:      RoleGeneral,
		Context:   "stream test",
		ModelType: "openai",
		ModelName: "gpt-4o",
	}

	result := c.executeSingleTask(context.Background(), cfg, task, 0, onProgress, true, nil)
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Content != "hello world" {
		t.Errorf("expected 'hello world', got %q", result.Content)
	}
	if len(progressUpdates) < 2 {
		t.Errorf("expected at least 2 progress updates, got %d", len(progressUpdates))
	}
}

func TestExecuteSingleTaskProviderError(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{
		name:    "openai",
		display: "OpenAI",
		chatErr: errors.New("provider unavailable"),
	})

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	task := OrchestraTask{
		Role:      RoleGeneral,
		Context:   "fail test",
		ModelType: "openai",
		ModelName: "gpt-4o",
	}

	result := c.executeSingleTask(context.Background(), cfg, task, 0, nil, false, nil)
	if result.Error == "" {
		t.Fatal("expected error")
	}
}

func TestExecuteSingleTaskFallbackToNonStreaming(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{
		name:      "openai",
		display:   "OpenAI",
		streamErr: errors.New("streaming not supported"),
		chatResp:  "fallback response",
	})

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	task := OrchestraTask{
		Role:      RoleGeneral,
		Context:   "fallback test",
		ModelType: "openai",
		ModelName: "gpt-4o",
	}

	result := c.executeSingleTask(context.Background(), cfg, task, 0, func(up ProgressUpdate) {}, true, nil)
	if result.Error != "" {
		t.Errorf("unexpected error: %s", result.Error)
	}
	if result.Content != "fallback response" {
		t.Errorf("expected 'fallback response', got %q", result.Content)
	}
}
