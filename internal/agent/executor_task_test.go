package agent

import (
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
