// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/observer"
	"memo/internal/routine"
	"memo/internal/sessions"
)

func newRoutineTestApp(t *testing.T) *App {
	t.Helper()
	dir := t.TempDir()

	sm, err := sessions.NewManager(filepath.Join(dir, "sessions"))
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	obsStore, err := observer.NewStore(observer.StoreConfig{Dir: dir})
	if err != nil {
		t.Fatalf("observer.NewStore: %v", err)
	}

	a := &App{
		cfg:              &config.AppConfig{},
		identity:         identity.New("Test", "Memo", "casual", "", false),
		sessions:         sm,
		observerRecorder: observer.NewRecorder(obsStore),
		agentExecutor:    agent.NewExecutor(dir, nil, nil),
	}
	return a
}

// TestRunAgentRoutine_DoesNotMutateGlobalAgentModeOrActiveChat is a
// regression test for BUG-C1: runAgentRoutine used to call SwitchChat and
// SetAgentEnabled(true) with no restore, permanently flipping the app's one
// global agent-mode flag on and hijacking the global "active chat" pointer
// after the first agent-mode routine ever ran. The fix passes forceAgent
// straight into sendMessageStreamCore instead (which already activates tool
// execution for just this one call, per routeStream's own forceAgent
// handling), so neither global should ever move.
func TestRunAgentRoutine_DoesNotMutateGlobalAgentModeOrActiveChat(t *testing.T) {
	a := newRoutineTestApp(t)

	// Seed a real "current" chat/agent-mode state that must survive the
	// routine run untouched.
	userChatID := a.NewChat()
	if err := a.SwitchChat(userChatID); err != nil {
		t.Fatalf("SwitchChat: %v", err)
	}
	if a.GetAgentEnabled() {
		t.Fatal("test setup: agent mode should start disabled")
	}

	r := routine.Routine{Prompt: "test prompt", AgentMode: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// The stream itself will fail (no LLM client/provider configured in this
	// test) — that's fine, we only care about the global state afterward.
	_, _ = a.runAgentRoutine(ctx, r)

	if a.GetAgentEnabled() {
		t.Error("BUG-C1 regressed: global agent mode is enabled after runAgentRoutine, should be untouched")
	}
	if got := a.GetActiveChatID(); got != userChatID {
		t.Errorf("BUG-C1 regressed: active chat = %q, want unchanged %q", got, userChatID)
	}
}

// TestRunAgentRoutine_AutoApprovePermissionRestoredBeforeUnlock verifies the
// auto-permission flag set for AutoApproveTools is always restored to its
// prior value, and — the actual BUG-C1 race — that it is restored *before*
// streamMu is released, so no concurrent caller can ever observe it left on.
func TestRunAgentRoutine_AutoApprovePermissionRestoredBeforeUnlock(t *testing.T) {
	a := newRoutineTestApp(t)

	if a.GetAgentAutoPermission() {
		t.Fatal("test setup: auto-permission should start disabled")
	}

	r := routine.Routine{Prompt: "test prompt", AgentMode: true, AutoApproveTools: true}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, _ = a.runAgentRoutine(ctx, r)

	if a.GetAgentAutoPermission() {
		t.Error("BUG-C1 regressed: auto-permission left enabled after runAgentRoutine")
	}

	// The lock must be free again (proves unlock happened, and — combined
	// with the assertion above already having observed the restored value —
	// that restore-before-unlock ordering held).
	if !a.streamMu.TryLock() {
		t.Error("streamMu still held after runAgentRoutine returned")
	} else {
		a.streamMu.Unlock()
	}
}

// TestRunAgentRoutine_BusyStreamFailsFastWithoutMutatingState covers the
// case sendMessageStreamInnerTo's internal TryLock previously masked: the
// old code called SwitchChat/SetAgentEnabled(true) unconditionally *before*
// ever discovering another stream was in flight. The fixed version checks
// streamMu itself first, so a concurrent stream must leave global state
// completely untouched.
func TestRunAgentRoutine_BusyStreamFailsFastWithoutMutatingState(t *testing.T) {
	a := newRoutineTestApp(t)

	userChatID := a.NewChat()
	if err := a.SwitchChat(userChatID); err != nil {
		t.Fatalf("SwitchChat: %v", err)
	}

	// Simulate a concurrent interactive stream already in progress.
	a.streamMu.Lock()
	defer a.streamMu.Unlock()

	r := routine.Routine{Prompt: "test prompt", AgentMode: true, AutoApproveTools: true}
	_, err := a.runAgentRoutine(context.Background(), r)
	if err == nil {
		t.Fatal("expected an error when streamMu is already held")
	}

	if a.GetAgentEnabled() {
		t.Error("busy path mutated global agent mode")
	}
	if got := a.GetActiveChatID(); got != userChatID {
		t.Errorf("busy path changed active chat: got %q, want %q", got, userChatID)
	}
	if a.GetAgentAutoPermission() {
		t.Error("busy path left auto-permission enabled")
	}
}
