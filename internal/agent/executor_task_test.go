package agent

import (
	"sync"
	"testing"

	"memo/internal/provider"
)

func TestNewTaskExecutor_IndependentRouterAndBypass(t *testing.T) {
	routerA := provider.NewRouter(nil)
	base := NewExecutor(t.TempDir(), routerA, nil, nil)

	routerB := provider.NewRouter(nil)
	taskExec := NewTaskExecutor(base, routerB)

	if taskExec.ActiveRouter() != routerB {
		t.Fatal("task executor did not adopt its own router")
	}
	if base.ActiveRouter() != routerA {
		t.Fatal("constructing a task executor disturbed the base router")
	}

	// Swapping the task executor's router must not move the base's.
	routerC := provider.NewRouter(nil)
	taskExec.SyncRouter(routerC)
	if base.ActiveRouter() != routerA {
		t.Fatal("task SyncRouter leaked into the base executor")
	}
	if taskExec.ActiveRouter() != routerC {
		t.Fatal("task SyncRouter did not take effect")
	}

	// Bypass flag is per-executor.
	taskExec.SetBypassPermissions(true)
	if base.GetBypassPermissions() {
		t.Fatal("task executor bypass leaked into the base executor")
	}
	if !taskExec.GetBypassPermissions() {
		t.Fatal("task executor bypass did not stick")
	}

	// Shared substrate: same registry/permissions/backup instances.
	if taskExec.Registry() != base.Registry() {
		t.Fatal("task executor should share the base registry")
	}
}

func TestTaskExecutors_ConcurrentLimitUpdates(t *testing.T) {
	base := NewExecutor(t.TempDir(), provider.NewRouter(nil), nil, nil)
	base.SetMaxIterations(7)
	base.SetMaxContinuations(2)
	base.SetContextBudget(48_000)
	var updates sync.WaitGroup
	updates.Add(1)
	go func() {
		defer updates.Done()
		for range 100 {
			base.SetMaxIterations(7)
			base.SetMaxContinuations(2)
			base.SetContextBudget(48_000)
		}
	}()
	defer updates.Wait()
	registry := NewRegistry()
	for range 100 {
		for _, clone := range []*Executor{
			NewTaskExecutor(base, nil),
			NewSubAgentExecutor(base, registry, nil, ""),
		} {
			iters, continuations := clone.getLoopBounds()
			if iters != 7 || continuations != 2 || clone.getContextBudget() != 48_000 {
				t.Fatal("concurrent clone lost the configured limits")
			}
		}
	}
}

// Task workers and their children must obey the configured limits just as the
// interactive executor does. Each clone owns a snapshot: changing one worker's
// limits must not silently change an already running sibling or the base.
func TestTaskExecutors_InheritIndependentLimits(t *testing.T) {
	base := NewExecutor(t.TempDir(), provider.NewRouter(nil), nil, nil)
	base.SetMaxIterations(7)
	base.SetMaxContinuations(2)
	base.SetContextBudget(48_000)

	task := NewTaskExecutor(base, provider.NewRouter(nil))
	child := NewSubAgentExecutor(task, NewRegistry(), provider.NewRouter(nil), t.TempDir())
	assertLimits := func(name string, e *Executor, iters, continuations, budget int) {
		t.Helper()
		gotIters, gotContinuations := e.getLoopBounds()
		if gotIters != iters || gotContinuations != continuations || e.getContextBudget() != budget {
			t.Errorf("%s limits = (%d, %d, %d), want (%d, %d, %d)", name,
				gotIters, gotContinuations, e.getContextBudget(), iters, continuations, budget)
		}
	}
	assertLimits("task", task, 7, 2, 48_000)
	assertLimits("child", child, 7, 2, 48_000)

	base.SetMaxIterations(11)
	base.SetMaxContinuations(4)
	base.SetContextBudget(96_000)
	assertLimits("task after base update", task, 7, 2, 48_000)
	assertLimits("child after base update", child, 7, 2, 48_000)

	task.SetMaxIterations(3)
	task.SetMaxContinuations(0)
	task.SetContextBudget(36_000)
	assertLimits("base after task update", base, 11, 4, 96_000)
	assertLimits("child after task update", child, 7, 2, 48_000)
	assertLimits("task after task update", task, 3, 0, 36_000)

	child.SetMaxIterations(1)
	child.SetMaxContinuations(0)
	child.SetContextBudget(32_000)
	assertLimits("task after child update", task, 3, 0, 36_000)
	assertLimits("base after child update", base, 11, 4, 96_000)
}
