package orchestra

import (
	"context"
	"errors"
	"testing"
)

func TestExtractJSON(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "bare JSON object",
			input: `{"tasks": [{"role": "frontend"}]}`,
			want:  `{"tasks": [{"role": "frontend"}]}`,
		},
		{
			name:  "json code block",
			input: "some text\n```json\n{\"tasks\": []}\n```\nmore text",
			want:  `{"tasks": []}`,
		},
		{
			name:  "bare code block",
			input: "```\n{\"key\": \"value\"}\n```",
			want:  `{"key": "value"}`,
		},
		{
			name:  "JSON array",
			input: `[{"a": 1}, {"b": 2}]`,
			want:  `[{"a": 1}, {"b": 2}]`,
		},
		{
			name:  "nested braces",
			input: `{"a": {"b": "c"}}`,
			want:  `{"a": {"b": "c"}}`,
		},
		{
			name:  "no JSON returns as-is",
			input: "hello world",
			want:  "hello world",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractJSON(tt.input)
			if got != tt.want {
				t.Errorf("extractJSON() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCreatePlan(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock(`{"tasks": [{"role": "general", "context": "test task"}], "parallel": true}`))

	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := newConductor(cfg, f)

	plan, err := c.createPlan(context.Background(), cfg, "test message", nil)
	if err != nil {
		t.Fatalf("createPlan: %v", err)
	}
	if len(plan.Tasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(plan.Tasks))
	}
	if plan.Tasks[0].Role != RoleGeneral {
		t.Errorf("expected role=general, got %s", plan.Tasks[0].Role)
	}
}

func TestCreatePlanFillsModelInfo(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock(`{"tasks": [{"role": "planner", "context": "plan it"}], "parallel": false}`))

	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := newConductor(cfg, f)

	plan, err := c.createPlan(context.Background(), cfg, "test message", nil)
	if err != nil {
		t.Fatalf("createPlan: %v", err)
	}
	if plan.Tasks[0].ModelType == "" {
		t.Error("ModelType should be filled from config")
	}
	if plan.Tasks[0].ModelName == "" {
		t.Error("ModelName should be filled from config")
	}
}

func TestCreatePlanChiefError(t *testing.T) {
	f := newMockFactory()
	f.set("openai", &mockProvider{
		name:    "openai",
		display: "OpenAI",
		chatErr: errors.New("API error"),
	})

	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := newConductor(cfg, f)

	_, err := c.createPlan(context.Background(), cfg, "test", nil)
	if err == nil {
		t.Fatal("expected error from chief")
	}
}

func TestCreatePlanInvalidJSON(t *testing.T) {
	f := newMockFactory()
	f.set("openai", openAIMock("not json at all"))

	cfg := defaultEnabledConfig()
	cfg.ChiefType = "openai"
	cfg.ChiefModel = "gpt-4o"

	c := newConductor(cfg, f)

	_, err := c.createPlan(context.Background(), cfg, "test", nil)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}
