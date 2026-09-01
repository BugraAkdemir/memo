package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/sessions"
)

func TestExecuteToolCall_SafeToolRunsWithoutPrompt(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hi there"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	e := NewExecutor(dir, nil, nil, nil)

	args := json.RawMessage(`{"path": "hello.txt"}`)
	var events []AgentEvent
	result, err := e.ExecuteToolCall(context.Background(), "sess-1", "read_file", args, func(ev AgentEvent) {
		events = append(events, ev)
	})
	if err != nil {
		t.Fatalf("ExecuteToolCall: %v", err)
	}
	if result != "hi there" {
		t.Errorf("expected file contents, got %q", result)
	}
	// Safe tools never need a prompt — only executing+result events, no
	// permission_request.
	for _, ev := range events {
		if ev.Type == EventPermissionRequest {
			t.Error("a Safe-danger tool must never trigger a permission_request")
		}
	}
	if len(events) != 2 || events[0].Type != EventToolExecuting || events[1].Type != EventToolResult {
		t.Errorf("expected [tool_executing, tool_result], got %+v", events)
	}
	// The executing event must carry Args: the live task card renders it as
	// the "starting" line for slow tools and printed a bare "Komut …" while
	// this was empty.
	if len(events) > 0 && len(events[0].Args) == 0 {
		t.Error("tool_executing carried no Args — the starting line has nothing to show")
	}
}

func TestExecuteToolCall_UnknownToolReturnsError(t *testing.T) {
	e := NewExecutor(t.TempDir(), nil, nil, nil)
	_, err := e.ExecuteToolCall(context.Background(), "sess-1", "no_such_tool", json.RawMessage(`{}`), nil)
	if err == nil {
		t.Fatal("expected an error for an unknown tool")
	}
}

func TestExecuteToolCall_BypassPermissionsSkipsPrompt(t *testing.T) {
	dir := t.TempDir()
	e := NewExecutor(dir, nil, nil, nil)
	e.SetBypassPermissions(true)

	args := json.RawMessage(fmt.Sprintf(`{"path": %q, "content": "written"}`, "new.txt"))
	var sawPermissionRequest bool
	_, err := e.ExecuteToolCall(context.Background(), "sess-1", "write_file", args, func(ev AgentEvent) {
		if ev.Type == EventPermissionRequest {
			sawPermissionRequest = true
		}
	})
	if err != nil {
		t.Fatalf("ExecuteToolCall: %v", err)
	}
	if sawPermissionRequest {
		t.Error("bypassPermissions must skip the permission prompt entirely")
	}
	if got, readErr := os.ReadFile(filepath.Join(dir, "new.txt")); readErr != nil || string(got) != "written" {
		t.Errorf("expected the write to actually happen, got %q err=%v", got, readErr)
	}
}

// TestExecuteToolCall_WaitsForHandlePermissionResponse confirms the
// permission_request event carries a RequestID that App.HandleAgentPermission
// (Executor.HandlePermissionResponse) can resolve — the exact mechanism
// Live Mode's "standalone" WorkMode and normal agent-mode share, per
// ExecuteToolCall's own doc comment.
func TestExecuteToolCall_WaitsForHandlePermissionResponse(t *testing.T) {
	dir := t.TempDir()
	e := NewExecutor(dir, nil, nil, nil)

	args := json.RawMessage(fmt.Sprintf(`{"path": %q, "content": "approved"}`, "gated.txt"))

	resultCh := make(chan struct {
		result string
		err    error
	}, 1)
	reqIDCh := make(chan string, 1)
	go func() {
		result, err := e.ExecuteToolCall(context.Background(), "sess-1", "write_file", args, func(ev AgentEvent) {
			if ev.Type == EventPermissionRequest {
				reqIDCh <- ev.RequestID
			}
		})
		resultCh <- struct {
			result string
			err    error
		}{result, err}
	}()

	var reqID string
	select {
	case reqID = <-reqIDCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the permission_request event")
	}

	if err := e.HandlePermissionResponse(reqID, AllowOnce); err != nil {
		t.Fatalf("HandlePermissionResponse: %v", err)
	}

	select {
	case res := <-resultCh:
		if res.err != nil {
			t.Fatalf("ExecuteToolCall: %v", res.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteToolCall never returned after HandlePermissionResponse")
	}
	if got, err := os.ReadFile(filepath.Join(dir, "gated.txt")); err != nil || string(got) != "approved" {
		t.Errorf("expected the write to happen after approval, got %q err=%v", got, err)
	}
}

// TestExecuteToolCall_ChangeDirectoryPersistsAndIsReadBackNextCall is a
// regression test for a real bug found live: change_directory offered to
// standalone-mode Live Mode sessions could never actually work through
// ExecuteToolCall, because nothing wired tools.WithSandboxSetter/
// WithProjectPathSetter into ctx the way RunStream (pipeline.go/executor.go)
// already does — change_directory would fail outright with "internal
// error: no sandbox available to change_directory". Confirms: (1) the call
// no longer errors, (2) the switch is actually persisted via
// sessionManager.SetProjectPath, and (3) — mirroring the read-back
// buildLiveModeToolCallHandler now does before every standalone call, the
// same way llm.go already does for RunStream — passing that persisted path
// back in as ExecuteToolCall's own projectPath argument on a *later* call
// actually takes effect (a relative read_file resolves against the new
// directory, not the executor's original basePath).
func TestExecuteToolCall_ChangeDirectoryPersistsAndIsReadBackNextCall(t *testing.T) {
	t.Setenv("MEMO_DATA_DIR", t.TempDir())

	originalDir := t.TempDir()
	newDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(newDir, "hello.txt"), []byte("from the new directory"), 0644); err != nil {
		t.Fatalf("write test file: %v", err)
	}

	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("sessions.NewManager: %v", err)
	}
	sessionID := sm.NewBackgroundChat("test")

	e := NewExecutor(originalDir, nil, nil, sm)
	e.SetBypassPermissions(true) // this test is about persistence, not the permission flow

	changeArgs := json.RawMessage(fmt.Sprintf(`{"path": %q}`, newDir))
	if _, err := e.ExecuteToolCall(context.Background(), sessionID, "change_directory", changeArgs, nil); err != nil {
		t.Fatalf("ExecuteToolCall(change_directory): %v", err)
	}

	if got := sm.GetProjectPath(sessionID); got != newDir {
		t.Fatalf("expected the switch to persist to sessionManager, got %q want %q", got, newDir)
	}

	// The read-back a caller (buildLiveModeToolCallHandler) must do before
	// its *next* ExecuteToolCall — this call's own effectiveBase would
	// otherwise still be originalDir, per ExecuteToolCall's own doc
	// comment on its projectPath parameter.
	readArgs := json.RawMessage(`{"path": "hello.txt"}`)
	result, err := e.ExecuteToolCall(context.Background(), sessionID, "read_file", readArgs, nil, sm.GetProjectPath(sessionID))
	if err != nil {
		t.Fatalf("ExecuteToolCall(read_file) after directory switch: %v", err)
	}
	if result != "from the new directory" {
		t.Errorf("expected read_file to resolve against the new directory, got %q", result)
	}
}

func TestExecuteToolCall_DenyPreventsExecution(t *testing.T) {
	dir := t.TempDir()
	e := NewExecutor(dir, nil, nil, nil)

	args := json.RawMessage(fmt.Sprintf(`{"path": %q, "content": "should not appear"}`, "denied.txt"))

	resultCh := make(chan error, 1)
	reqIDCh := make(chan string, 1)
	go func() {
		_, err := e.ExecuteToolCall(context.Background(), "sess-1", "write_file", args, func(ev AgentEvent) {
			if ev.Type == EventPermissionRequest {
				reqIDCh <- ev.RequestID
			}
		})
		resultCh <- err
	}()

	var reqID string
	select {
	case reqID = <-reqIDCh:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for the permission_request event")
	}
	if err := e.HandlePermissionResponse(reqID, DenyOnce); err != nil {
		t.Fatalf("HandlePermissionResponse: %v", err)
	}

	select {
	case err := <-resultCh:
		if err == nil {
			t.Fatal("expected an error when permission is denied")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteToolCall never returned after denial")
	}
	if _, err := os.Stat(filepath.Join(dir, "denied.txt")); !os.IsNotExist(err) {
		t.Error("expected the write to never happen after denial")
	}
}
