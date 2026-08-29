package app

import (
	"context"
	"testing"

	"memo/internal/config"
)

func TestPlanTaskConfig_DisabledIsNoop(t *testing.T) {
	a := newSelfHealApp(t, "primary")
	a.cfg = &config.AppConfig{TaskLoop: config.TaskLoopConfig{PlanningSelfConfig: false}}
	a.seedTaskRunConfig("L1", "primary")

	if err := a.planTaskConfig(context.Background(), "L1", "chat", []string{"do x"}); err != nil {
		t.Fatalf("planTaskConfig with the feature off should be a no-op, got %v", err)
	}
}

func TestApplyPlanConfig_SwitchesProviderTaskLocal(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfig("L1", "primary")
	enabled := a.enabledProviderConfigs()

	a.applyPlanConfig("L1", trc, enabled, planConfigChoice{Provider: "backup"})

	if trc.providerName != "backup" {
		t.Fatalf("task provider = %q, want backup", trc.providerName)
	}
	if trc.model != "backup-model" {
		t.Fatalf("task model = %q, want backup-model", trc.model)
	}
	if a.activeProviderName != "primary" {
		t.Fatalf("global provider = %q, must be untouched", a.activeProviderName)
	}
}

func TestApplyPlanConfig_UnknownProviderIgnored(t *testing.T) {
	a := newSelfHealApp(t, "primary", "backup")
	trc := a.seedTaskRunConfig("L1", "primary")
	enabled := a.enabledProviderConfigs()

	a.applyPlanConfig("L1", trc, enabled, planConfigChoice{Provider: "does-not-exist"})

	if trc.providerName != "primary" {
		t.Fatalf("task provider = %q, want unchanged primary", trc.providerName)
	}
}

func TestApplyPlanConfig_EffortAndModel(t *testing.T) {
	a := newSelfHealApp(t, "primary")
	trc := a.seedTaskRunConfig("L1", "primary")
	enabled := a.enabledProviderConfigs()

	a.applyPlanConfig("L1", trc, enabled, planConfigChoice{Model: "primary-xl", Effort: "HIGH"})

	if trc.model != "primary-xl" {
		t.Fatalf("model = %q, want primary-xl", trc.model)
	}
	if trc.effortLevel != "high" {
		t.Fatalf("effort = %q, want high", trc.effortLevel)
	}
}
