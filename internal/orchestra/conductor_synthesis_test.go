package orchestra

import (
	"context"
	"testing"
)

func TestSynthesize(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("final answer"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "task", ModelType: "openai", ModelName: "gpt-4o"},
	}
	results := []OrchestraResult{
		{TaskIndex: 0, Role: RoleGeneral, Content: "task result"},
	}

	resp, err := c.synthesize(context.Background(), cfg, "user message", tasks, results, nil)
	if err != nil {
		t.Fatalf("synthesize: %v", err)
	}
	if resp == "" {
		t.Error("expected non-empty response")
	}
}

func TestSynthesizeAllFailed(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("should not be called"))

	cfg := defaultEnabledConfig()
	c := newConductor(cfg, f)

	tasks := []OrchestraTask{
		{Role: RoleGeneral, Context: "task", ModelType: "openai", ModelName: "gpt-4o"},
	}
	results := []OrchestraResult{
		{TaskIndex: 0, Role: RoleGeneral, Error: "failed", Content: ""},
	}

	resp, err := c.synthesize(context.Background(), cfg, "user message", tasks, results, nil)
	if err != nil {
		t.Fatalf("synthesize should not return error when all tasks fail: %v", err)
	}
	if resp == "" {
		t.Error("expected error summary response")
	}
}
