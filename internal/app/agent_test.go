package app

import (
	"testing"

	"memo/internal/agent"
)

func TestGetSetAgentEnabled(t *testing.T) {
	a := &App{}

	if a.GetAgentEnabled() {
		t.Fatal("expected agent mode to default to false")
	}

	if err := a.SetAgentEnabled(true); err != nil {
		t.Fatalf("SetAgentEnabled(true): %v", err)
	}
	if !a.GetAgentEnabled() {
		t.Fatal("expected agent mode to be true after SetAgentEnabled(true)")
	}

	if err := a.SetAgentEnabled(false); err != nil {
		t.Fatalf("SetAgentEnabled(false): %v", err)
	}
	if a.GetAgentEnabled() {
		t.Fatal("expected agent mode to be false after SetAgentEnabled(false)")
	}
}

// Every agent.* wrapper method must fail gracefully (no panic, a clear error)
// when the agent executor hasn't been initialized yet — e.g. the app started
// without an active provider, or agent mode was never turned on.
func TestAgentWrappers_NilExecutor(t *testing.T) {
	a := &App{}

	if err := a.HandleAgentPermission("req-1", "allow"); err == nil {
		t.Error("HandleAgentPermission: expected error with nil agentExecutor")
	}

	if perms := a.GetAgentPermissions(); perms == nil || len(perms) != 0 {
		t.Errorf("GetAgentPermissions: expected empty non-nil slice, got %#v", perms)
	}

	if err := a.RevokeAgentPermission("perm-1"); err == nil {
		t.Error("RevokeAgentPermission: expected error with nil agentExecutor")
	}

	// Must not panic even though there's nothing to clear.
	a.ClearAgentPermissions()

	if err := a.UndoLastAgentEdit(); err == nil {
		t.Error("UndoLastAgentEdit: expected error with nil agentExecutor")
	}

	if err := a.SetAgentAutoPermission(true); err == nil {
		t.Error("SetAgentAutoPermission: expected error with nil agentExecutor")
	}

	if a.GetAgentAutoPermission() {
		t.Error("GetAgentAutoPermission: expected false with nil agentExecutor")
	}
}

// With a real (but provider-less) executor wired in, the wrappers should
// delegate to it instead of short-circuiting.
func TestAgentWrappers_WithExecutor(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	exec := agent.NewExecutor(t.TempDir(), nil, nil)
	a := &App{agentExecutor: exec}

	if err := a.SetAgentAutoPermission(true); err != nil {
		t.Fatalf("SetAgentAutoPermission: %v", err)
	}
	if !a.GetAgentAutoPermission() {
		t.Error("expected auto-permission to be true after enabling it")
	}
	if err := a.SetAgentAutoPermission(false); err != nil {
		t.Fatalf("SetAgentAutoPermission: %v", err)
	}
	if a.GetAgentAutoPermission() {
		t.Error("expected auto-permission to be false after disabling it")
	}

	if perms := a.GetAgentPermissions(); len(perms) != 0 {
		t.Errorf("expected no recorded permissions yet, got %#v", perms)
	}

	// No pending request with this ID — a real executor with nothing pending
	// should report that clearly rather than silently succeeding.
	if err := a.HandleAgentPermission("does-not-exist", "allow"); err == nil {
		t.Error("expected error for an unknown/expired permission request ID")
	}

	if err := a.RevokeAgentPermission("does-not-exist"); err == nil {
		t.Error("expected error revoking a permission that was never granted")
	}

	// Must not panic on an executor with no recorded permissions.
	a.ClearAgentPermissions()

	if err := a.UndoLastAgentEdit(); err == nil {
		t.Error("expected error undoing an edit when nothing was ever edited")
	}
}
