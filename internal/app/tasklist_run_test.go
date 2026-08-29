package app

import (
	"context"
	"testing"
)

func TestTaskRunConfigFromCtx_NilWhenAbsent(t *testing.T) {
	if got := taskRunConfigFromCtx(context.Background()); got != nil {
		t.Fatalf("taskRunConfigFromCtx on a bare context = %v, want nil", got)
	}
}

func TestTaskRunConfigCtx_RoundTrip(t *testing.T) {
	want := &taskRunConfig{providerName: "openai", model: "gpt-x", effortLevel: "high"}
	ctx := withTaskRunConfig(context.Background(), want)
	got := taskRunConfigFromCtx(ctx)
	if got != want {
		t.Fatalf("round-trip returned %v, want %v", got, want)
	}
	if got.model != "gpt-x" || got.providerName != "openai" {
		t.Fatalf("fields not preserved: %+v", got)
	}
}

func TestClearTaskRunConfig(t *testing.T) {
	a := &App{taskRunCfgs: map[string]*taskRunConfig{"L1": {model: "m"}}}
	a.clearTaskRunConfig("L1")
	a.taskRunMu.RLock()
	_, still := a.taskRunCfgs["L1"]
	a.taskRunMu.RUnlock()
	if still {
		t.Fatal("clearTaskRunConfig left the entry in place")
	}
}
