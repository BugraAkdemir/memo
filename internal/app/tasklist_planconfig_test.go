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

func TestPlanTaskConfig_ProviderLockIsNoop(t *testing.T) {
	a := newSelfHealApp(t, "primary")
	// Feature ON, but the provider lock is the default (no store / no roaming
	// header / config default false) → planning must not self-configure.
	a.cfg = &config.AppConfig{TaskLoop: config.TaskLoopConfig{PlanningSelfConfig: true}}
	a.seedTaskRunConfig("L1", "primary")

	if err := a.planTaskConfig(context.Background(), "L1", "chat", []string{"do x"}); err != nil {
		t.Fatalf("planTaskConfig under the provider lock should be a silent no-op, got %v", err)
	}
}

func TestPlanTaskConfig_RoamingRunsSelfConfig(t *testing.T) {
	a := newSelfHealApp(t, "primary")
	a.cfg = &config.AppConfig{TaskLoop: config.TaskLoopConfig{
		PlanningSelfConfig: true,
		ProviderRoaming:    true, // global opt-in past the lock
	}}
	a.seedTaskRunConfig("L1", "primary")

	// With roaming on it gets past the gate and reaches the LLM call, which
	// fails against the fake provider — a non-nil error proves the gate opened.
	if err := a.planTaskConfig(context.Background(), "L1", "chat", []string{"do x"}); err == nil {
		t.Fatal("expected planTaskConfig to reach (and fail) the LLM call with roaming on")
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
