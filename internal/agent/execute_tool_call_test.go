package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
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
