package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/config"
)

func TestCodeSubModeDirective_DefaultsPerSubMode(t *testing.T) {
	var cfg config.AgentModeConfig
	cases := []struct {
		subMode string
		want    string
	}{
		{"plan", codePlanDirective},
		{"auto", codingDirective},
		{"build", codeBuildDirective},
		{"", codingDirective}, // "" treated the same as "auto"
	}
	for _, c := range cases {
		if got := codeSubModeDirective(cfg, c.subMode); got != c.want {
			t.Errorf("codeSubModeDirective(%q) = %q, want %q", c.subMode, got, c.want)
		}
	}
}

func TestCodeSubModeDirective_ConfigOverrideWins(t *testing.T) {
	cfg := config.AgentModeConfig{
		CodePlanPrompt:  "custom plan prompt",
		CodeAutoPrompt:  "custom auto prompt",
		CodeBuildPrompt: "custom build prompt",
	}
	cases := []struct {
		subMode string
		want    string
	}{
		{"plan", "custom plan prompt"},
		{"auto", "custom auto prompt"},
		{"build", "custom build prompt"},
		{"", "custom auto prompt"},
	}
	for _, c := range cases {
		if got := codeSubModeDirective(cfg, c.subMode); got != c.want {
			t.Errorf("codeSubModeDirective(%q) = %q, want %q", c.subMode, got, c.want)
		}
	}
}

// TestCodeModeDirectives_MentionTaskStatusTools is a live-testing regression:
// get_task_status/pause_task/resume_task (internal/agent/tools.go) were
// always registered and reachable from a Code Mode turn, but none of the
// three built-in directives ever told the model they existed — a project/
// agent chat (Code Mode's default) is exactly where a Self-Driving task list
// gets bound, so the model asked about a running task's progress had no
// prompted reason to call the tool designed to answer that (BUG-PLAN10)
// instead of guessing from its own turn's context.
func TestCodeModeDirectives_MentionTaskStatusTools(t *testing.T) {
	for _, d := range []struct {
		name string
		text string
	}{
		{"codingDirective", codingDirective},
		{"codePlanDirective", codePlanDirective},
		{"codeBuildDirective", codeBuildDirective},
	} {
		for _, tool := range []string{"get_task_status", "pause_task", "resume_task"} {
			if !strings.Contains(d.text, tool) {
				t.Errorf("%s does not mention %q — the model has no prompted reason to call it", d.name, tool)
			}
		}
	}
}

func TestCodeSubModeCtx_RoundTrips(t *testing.T) {
	ctx := context.Background()
	if got := codeSubModeFromCtx(ctx); got != "" {
		t.Errorf("codeSubModeFromCtx on bare ctx = %q, want \"\"", got)
	}
	ctx = withCodeSubMode(ctx, "build")
	if got := codeSubModeFromCtx(ctx); got != "build" {
		t.Errorf("codeSubModeFromCtx = %q, want \"build\"", got)
	}
}
