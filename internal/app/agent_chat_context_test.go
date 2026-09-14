package app

import (
	"context"
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
