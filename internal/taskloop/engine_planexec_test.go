package taskloop

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// newPlanExecEngine builds an engine with stub worker/chief plus a stub
// planner and step runner. plannerCalls / stepCalls count invocations.
func newPlanExecEngine(t *testing.T, store *Store, plan Plan, opts ...EngineOption) (*Engine, *int32, *int32) {
	t.Helper()
	var plannerCalls, stepCalls int32
	base := []EngineOption{
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			atomic.AddInt32(&plannerCalls, 1)
			p := plan
			p.ListID = listID
			return &p, nil
		}),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			atomic.AddInt32(&stepCalls, 1)
			return "did " + step.ID, nil
		}),
	}
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		append(base, opts...)...,
	)
	return eng, &plannerCalls, &stepCalls
}

func TestEngine_PlanExec_ApprovalGateThenRun(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "Task.md")
	os.WriteFile(md, []byte("- [ ] build page\n- [ ] add js\n"), 0644)

	store, _ := NewStore(t.TempDir())
	tl, _ := store.CreateWithMeta("c1", "T", []string{"build page", "add js"}, NotifyImportant, md)
	_ = store.SetItemLine(tl.ID, tl.Items[0].ID, 1)
	_ = store.SetItemLine(tl.ID, tl.Items[1].ID, 2)
	_ = store.SetMode(tl.ID, ModePlanner)

	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "html"},
		{ID: "S2", ItemID: "1", Text: "css", DependsOn: []string{"S1"}},
		{ID: "S3", ItemID: "2", Text: "js", DependsOn: []string{"S1"}},
	}}
	eng, planned, stepped := newPlanExecEngine(t, store, plan)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListAwaitingPlan, 3*time.Second)

	if atomic.LoadInt32(planned) != 1 {
		t.Fatalf("planner calls = %d, want 1", *planned)
	}
	if atomic.LoadInt32(stepped) != 0 {
		t.Fatal("steps ran before approval")
	}
	// Plan.md written next to Task.md.
	if _, err := os.Stat(PlanMdPath(md)); err != nil {
		t.Fatalf("Plan.md not written: %v", err)
	}
	if !store.HasPlan(tl.ID) {
		t.Fatal("plan not persisted")
	}

	if err := eng.ApprovePlan(ctx, tl.ID); err != nil {
		t.Fatalf("ApprovePlan: %v", err)
	}
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	if atomic.LoadInt32(planned) != 1 {
		t.Fatalf("planner re-ran on approval: %d", *planned)
	}
	if atomic.LoadInt32(stepped) != 3 {
		t.Fatalf("step runs = %d, want 3", *stepped)
	}
	got, _ := os.ReadFile(md)
	if string(got) != "- [x] build page\n- [x] add js\n" {
		t.Fatalf("Task.md not mirrored:\n%q", string(got))
	}
}

func TestEngine_PlanExec_AutoApproveSkipsGate(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"only item"})
	_ = store.SetMode(tl.ID, ModePlanner)

	plan := Plan{Steps: []PlanStep{{ID: "S1", ItemID: "1", Text: "do it"}}}
	eng, _, stepped := newPlanExecEngine(t, store, plan, WithAutoApprovePlan(true))

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)
	if atomic.LoadInt32(stepped) != 1 {
		t.Fatalf("step runs = %d, want 1", *stepped)
	}
}

func TestEngine_PlanExec_FailedAcceptanceMarksItemStuck(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"item a", "item b"})
	_ = store.SetMode(tl.ID, ModePlanner)

	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "a"},
		{ID: "S2", ItemID: "2", Text: "b"},
	}}
	eng, _, _ := newPlanExecEngine(t, store, plan,
		WithAutoApprovePlan(true),
		WithAcceptanceChecker(func(ctx context.Context, listID string, step PlanStep) (bool, string, error) {
			return step.ID == "S1", "S2 fails its check", nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second) // any item done -> list "done"

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item a status = %q, want done", got.Items[0].Status)
	}
	if got.Items[1].Status != "stuck" {
		t.Fatalf("item b status = %q, want stuck", got.Items[1].Status)
	}
	p, _ := store.GetPlan(tl.ID)
	if p.Steps[0].Status != "done" || p.Steps[1].Status != "stuck" {
		t.Fatalf("step statuses = %q/%q", p.Steps[0].Status, p.Steps[1].Status)
	}
}

func TestEngine_PlanExec_NoStepRunnerFails(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"x"})
	_ = store.SetMode(tl.ID, ModePlanner)

	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			return &Plan{Steps: []PlanStep{{ID: "S1", ItemID: "1", Text: "x"}}}, nil
		}),
		WithAutoApprovePlan(true),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListFailed, 3*time.Second)
}
