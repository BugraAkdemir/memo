package tools

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

type fakePlanSaver struct {
	gotContent string
	returnMsg  string
	err        error
}

func (f *fakePlanSaver) SaveCodePlan(ctx context.Context, content string) (string, error) {
	f.gotContent = content
	if f.err != nil {
		return "", f.err
	}
	return f.returnMsg, nil
}

func TestSaveCodePlan_InvalidArgs(t *testing.T) {
	prev := PlanSaver
	defer func() { PlanSaver = prev }()
	PlanSaver = &fakePlanSaver{}

	_, err := SaveCodePlan(context.Background(), json.RawMessage(`not json`), "", nil)
	if err == nil {
		t.Fatal("expected an error for invalid arguments JSON")
	}
}

func TestSaveCodePlan_NotReady(t *testing.T) {
	prev := PlanSaver
	defer func() { PlanSaver = prev }()
	PlanSaver = nil

	out, err := SaveCodePlan(context.Background(), json.RawMessage(`{"content":"# Plan"}`), "", nil)
	if err != nil {
		t.Fatalf("expected a not-ready message, not an error: %v", err)
	}
	if out == "" {
		t.Fatal("expected a non-empty not-ready message")
	}
}

func TestSaveCodePlan_DelegatesToPlanSaver(t *testing.T) {
	prev := PlanSaver
	defer func() { PlanSaver = prev }()
	fake := &fakePlanSaver{returnMsg: "Plan saved: /tmp/plans/x/plan.md"}
	PlanSaver = fake

	out, err := SaveCodePlan(context.Background(), json.RawMessage(`{"content":"# My Plan\n\n- step 1"}`), "", nil)
	if err != nil {
		t.Fatalf("SaveCodePlan: %v", err)
	}
	if out != fake.returnMsg {
		t.Errorf("out = %q, want %q", out, fake.returnMsg)
	}
	if fake.gotContent != "# My Plan\n\n- step 1" {
		t.Errorf("PlanSaver got content %q", fake.gotContent)
	}
}

func TestSaveCodePlan_PropagatesError(t *testing.T) {
	prev := PlanSaver
	defer func() { PlanSaver = prev }()
	PlanSaver = &fakePlanSaver{err: errors.New("disk full")}

	_, err := SaveCodePlan(context.Background(), json.RawMessage(`{"content":"# Plan"}`), "", nil)
	if err == nil {
		t.Fatal("expected the underlying error to propagate")
	}
}
