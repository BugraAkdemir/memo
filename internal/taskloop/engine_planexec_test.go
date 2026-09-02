package taskloop

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
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
		WithMaxExecutorAttempts(1), // fail once -> stuck, no retry loop
		WithAcceptanceChecker(func(ctx context.Context, listID string, step PlanStep) (bool, string, error) {
			return step.ID == "S1", "S2 fails its check", nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 4*time.Second) // any item done -> list "done"

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

// TestEngine_PlanExec_ItemsCompleteIncrementally is the BUG-PLAN11 regression:
// an item whose steps all finish in an early wave must be marked done (and its
// checkbox ticked, and a tasklist:item_done event fired) right then — not only
// in finishPlanItems after the whole plan ends. The plan is chained so item 1
// completes a full wave before item 2's step even runs, and the test asserts
// item 1's item_done event lands before item 2's step_done.
func TestEngine_PlanExec_ItemsCompleteIncrementally(t *testing.T) {
	dir := t.TempDir()
	md := filepath.Join(dir, "Task.md")
	os.WriteFile(md, []byte("- [ ] first\n- [ ] second\n"), 0644)

	store, _ := NewStore(t.TempDir())
	tl, _ := store.CreateWithMeta("c1", "T", []string{"first", "second"}, NotifyImportant, md)
	_ = store.SetItemLine(tl.ID, tl.Items[0].ID, 1)
	_ = store.SetItemLine(tl.ID, tl.Items[1].ID, 2)
	_ = store.SetMode(tl.ID, ModePlanner)
	item1 := tl.Items[0].ID

	// S1,S2 -> item 1 (S2 after S1); S3 -> item 2, after S2. So the waves are
	// {S1}, {S2}, {S3} and item 1 is fully green after wave 2.
	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "a"},
		{ID: "S2", ItemID: "1", Text: "b", DependsOn: []string{"S1"}},
		{ID: "S3", ItemID: "2", Text: "c", DependsOn: []string{"S2"}},
	}}

	var mu sync.Mutex
	var events []string
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(name, data string) {
			mu.Lock()
			events = append(events, name+"|"+data)
			mu.Unlock()
		},
		WithAutoApprovePlan(true),
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			p := plan
			p.ListID = listID
			return &p, nil
		}),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			return "did " + step.ID, nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	mu.Lock()
	seq := append([]string(nil), events...)
	mu.Unlock()

	idxItem1Done, idxS3Done, idxFinished := -1, -1, -1
	for i, e := range seq {
		switch {
		case e == "tasklist:item_done|"+tl.ID+":"+item1 && idxItem1Done == -1:
			idxItem1Done = i
		case e == "taskloop:step_done|"+tl.ID+":S3":
			idxS3Done = i
		case strings.HasPrefix(e, "tasklist:finished|"):
			idxFinished = i
		}
	}
	if idxItem1Done == -1 {
		t.Fatalf("no tasklist:item_done for item 1; events: %v", seq)
	}
	if idxS3Done == -1 {
		t.Fatalf("no taskloop:step_done for S3; events: %v", seq)
	}
	if idxItem1Done > idxS3Done {
		t.Fatalf("item 1 was only marked done after S3 ran (index %d vs %d) — not incremental; events: %v", idxItem1Done, idxS3Done, seq)
	}
	if idxFinished != -1 && idxItem1Done > idxFinished {
		t.Fatalf("item 1 done fired after tasklist:finished; events: %v", seq)
	}

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" || got.Items[1].Status != "done" {
		t.Fatalf("final item statuses = %q/%q, want done/done", got.Items[0].Status, got.Items[1].Status)
	}
	mdGot, _ := os.ReadFile(md)
	if string(mdGot) != "- [x] first\n- [x] second\n" {
		t.Fatalf("Task.md not mirrored:\n%q", string(mdGot))
	}
}

func TestEngine_PlanExec_StepRetriesBeforeStuck(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"the item"})
	_ = store.SetMode(tl.ID, ModePlanner)
	plan := Plan{Steps: []PlanStep{{ID: "S1", ItemID: "1", Text: "flaky"}}}

	var checkCalls int32
	eng, _, _ := newPlanExecEngine(t, store, plan,
		WithAutoApprovePlan(true),
		WithMaxExecutorAttempts(3),
		WithAcceptanceChecker(func(ctx context.Context, listID string, step PlanStep) (bool, string, error) {
			// fail the first two acceptance checks, pass the third
			return atomic.AddInt32(&checkCalls, 1) >= 3, "not yet", nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 18*time.Second)

	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item status = %q, want done (retry should have recovered it)", got.Items[0].Status)
	}
	if n := atomic.LoadInt32(&checkCalls); n < 3 {
		t.Fatalf("acceptance checked %d times, want >= 3 (2 retries)", n)
	}
}

func TestStore_SetAutoApprovePlan(t *testing.T) {
	s := newStore(t)
	tl, _ := s.Create("c", "t", []string{"x"})
	if got, _ := s.Get(tl.ID); got.AutoApprovePlan {
		t.Fatal("default should be false")
	}
	if err := s.SetAutoApprovePlan(tl.ID, true); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Get(tl.ID); !got.AutoApprovePlan {
		t.Fatal("SetAutoApprovePlan did not persist")
	}
}

func TestEngine_PlanExec_ParallelAndStateDoc(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"one item"})
	_ = store.SetMode(tl.ID, ModePlanner)

	// Three independent steps -> all ready at once.
	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "a"},
		{ID: "S2", ItemID: "1", Text: "b"},
		{ID: "S3", ItemID: "1", Text: "c"},
	}}

	var inFlight, maxInFlight int32
	var sawState int32
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			p := plan
			return &p, nil
		}),
		WithAutoApprovePlan(true),
		WithMaxParallelSteps(3),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			n := atomic.AddInt32(&inFlight, 1)
			for {
				m := atomic.LoadInt32(&maxInFlight)
				if n <= m || atomic.CompareAndSwapInt32(&maxInFlight, m, n) {
					break
				}
			}
			time.Sleep(30 * time.Millisecond)
			if step.ID != "S1" && strings.Contains(stateDoc, "S1") {
				atomic.StoreInt32(&sawState, 1)
			}
			atomic.AddInt32(&inFlight, -1)
			return "touched " + step.ID, nil
		}),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	if atomic.LoadInt32(&maxInFlight) < 2 {
		t.Fatalf("steps did not run in parallel (max in flight = %d)", maxInFlight)
	}
	// The state doc persisted and carries every step's summary.
	st := store.GetState(tl.ID)
	for _, id := range []string{"S1", "S2", "S3"} {
		if !strings.Contains(st, id) {
			t.Fatalf("state doc missing %s:\n%s", id, st)
		}
	}
}

func TestEngine_PlanExec_EscalationReplacesStuckStep(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"one item"})
	_ = store.SetMode(tl.ID, ModePlanner)

	plan := Plan{Steps: []PlanStep{
		{ID: "S1", ItemID: "1", Text: "the hard step"},
	}}
	var escalated int32
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			p := plan
			return &p, nil
		}),
		WithAutoApprovePlan(true),
		WithMaxExecutorAttempts(1),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			if step.ID == "S1" {
				return "", errors.New("coder could not do S1")
			}
			return "did " + step.ID, nil
		}),
		WithEscalator(func(ctx context.Context, listID string, step PlanStep, f EscalationInput) ([]PlanStep, error) {
			atomic.AddInt32(&escalated, 1)
			if step.ID != "S1" {
				t.Errorf("escalated unexpected step %s", step.ID)
			}
			return []PlanStep{
				{ItemID: "1", Text: "smaller step a"},
				{ItemID: "1", Text: "smaller step b"},
			}, nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)

	if atomic.LoadInt32(&escalated) != 1 {
		t.Fatalf("escalator calls = %d, want 1", escalated)
	}
	p, _ := store.GetPlan(tl.ID)
	if len(p.Steps) != 2 {
		t.Fatalf("plan after escalation has %d steps, want 2 (S1 replaced)", len(p.Steps))
	}
	for _, s := range p.Steps {
		if s.Status != "done" {
			t.Fatalf("replacement step %s status = %q", s.ID, s.Status)
		}
	}
	got, _ := store.Get(tl.ID)
	if got.Items[0].Status != "done" {
		t.Fatalf("item status = %q, want done", got.Items[0].Status)
	}
}

func TestEngine_PlanExec_OfflineEscalationParksThenResumes(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"one item"})
	_ = store.SetMode(tl.ID, ModePlanner)

	plan := Plan{Steps: []PlanStep{{ID: "S1", ItemID: "1", Text: "step"}}}
	var escCalls int32
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			p := plan
			return &p, nil
		}),
		WithAutoApprovePlan(true),
		WithMaxExecutorAttempts(1),
		WithRetryScheduler(60*time.Millisecond),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			if step.Text == "recovered step" {
				return "ok", nil
			}
			return "", errors.New("coder failed")
		}),
		WithEscalator(func(ctx context.Context, listID string, step PlanStep, f EscalationInput) ([]PlanStep, error) {
			n := atomic.AddInt32(&escCalls, 1)
			if n == 1 {
				return nil, errors.New("dial tcp 1.2.3.4:443: connect: network is unreachable")
			}
			return []PlanStep{{ItemID: "1", Text: "recovered step"}}, nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)

	waitForStatus(t, store, tl.ID, taskListWaitingEscalation, 2*time.Second)
	p, _ := store.GetPlan(tl.ID)
	if p.PendingEscalation == nil || p.PendingEscalation.StepID != "S1" {
		t.Fatalf("PendingEscalation = %+v", p.PendingEscalation)
	}

	// The retry timer fires ~60ms later, resumes, escalator now succeeds.
	waitForStatus(t, store, tl.ID, taskListDone, 3*time.Second)
	p, _ = store.GetPlan(tl.ID)
	if p.PendingEscalation != nil {
		t.Fatal("PendingEscalation not cleared after resume")
	}
}

func TestEngine_PlanExec_PlannerRetriesTransientFailure(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"one item"})
	_ = store.SetMode(tl.ID, ModePlanner)

	var calls int32
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			if atomic.AddInt32(&calls, 1) < 3 {
				return nil, errors.New("planner: LLM stream idle for 5m with no new token")
			}
			return &Plan{Steps: []PlanStep{{ID: "S1", ItemID: "1", Text: "do it"}}}, nil
		}),
		WithAutoApprovePlan(true),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) {
			return "ok", nil
		}),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListDone, 25*time.Second)

	if atomic.LoadInt32(&calls) != 3 {
		t.Fatalf("planner calls = %d, want 3 (2 retries then success)", calls)
	}
}

func TestEngine_PlanExec_PlannerFailsAfterAllRetries(t *testing.T) {
	store, _ := NewStore(t.TempDir())
	tl, _ := store.Create("c1", "T", []string{"x"})
	_ = store.SetMode(tl.ID, ModePlanner)

	var calls int32
	eng := NewEngine(store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "ok", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {}, func(string, string) {},
		WithPlanner(func(ctx context.Context, listID, chatID, root, preamble string, items []string, gran string) (*Plan, error) {
			atomic.AddInt32(&calls, 1)
			return nil, errors.New("planner: dead")
		}),
		WithAutoApprovePlan(true),
		WithStepRunner(func(ctx context.Context, listID string, step PlanStep, stateDoc string) (string, error) { return "ok", nil }),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	_ = eng.Start(ctx, tl.ID)
	waitForStatus(t, store, tl.ID, taskListFailed, 25*time.Second)
	if atomic.LoadInt32(&calls) != int32(plannerMaxAttempts) {
		t.Fatalf("planner calls = %d, want %d", calls, plannerMaxAttempts)
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
