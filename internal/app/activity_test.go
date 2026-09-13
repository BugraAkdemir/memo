package app

import (
	"testing"
	"time"

	"memo/internal/agent"
	"memo/internal/models"
)

// resetActivity restores the tracker to its zero state and the real
// wireGlobalActivityHook wiring, so each test starts clean regardless of
// what a previous test (or NewApp, called by other packages' tests in the
// same binary) left behind.
func resetActivity(t *testing.T) *App {
	t.Helper()
	globalActivity.mu.Lock()
	globalActivity.state = models.ActivityIdle
	globalActivity.toolName = ""
	globalActivity.since = time.Now()
	globalActivity.gen++
	globalActivity.mu.Unlock()
	wireGlobalActivityHook()
	return &App{}
}

func TestGetActivityStatus_DefaultsIdle(t *testing.T) {
	a := resetActivity(t)
	got := a.GetActivityStatus()
	if got.State != models.ActivityIdle {
		t.Errorf("State = %q, want idle", got.State)
	}
}

func TestGlobalActivityHook_WriteToolReportsWriting(t *testing.T) {
	a := resetActivity(t)
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolExecuting, ToolName: "edit_file"})

	got := a.GetActivityStatus()
	if got.State != models.ActivityWriting {
		t.Errorf("State = %q, want writing", got.State)
	}
	if got.ToolName != "edit_file" {
		t.Errorf("ToolName = %q, want edit_file", got.ToolName)
	}
}

func TestGlobalActivityHook_OtherToolReportsTool(t *testing.T) {
	a := resetActivity(t)
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolExecuting, ToolName: "run_command"})

	got := a.GetActivityStatus()
	if got.State != models.ActivityTool {
		t.Errorf("State = %q, want tool", got.State)
	}
}

func TestGlobalActivityHook_ResultReportsGenerating(t *testing.T) {
	a := resetActivity(t)
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolExecuting, ToolName: "run_command"})
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolResult, ToolName: "run_command"})

	got := a.GetActivityStatus()
	if got.State != models.ActivityGenerating {
		t.Errorf("State = %q, want generating", got.State)
	}
}

func TestGlobalActivityHook_ErrorReportsIdle(t *testing.T) {
	a := resetActivity(t)
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolExecuting, ToolName: "run_command"})
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventToolError, ToolName: "run_command"})

	got := a.GetActivityStatus()
	if got.State != models.ActivityIdle {
		t.Errorf("State = %q, want idle", got.State)
	}
}

func TestGlobalActivityHook_FinalResponseReportsDone(t *testing.T) {
	a := resetActivity(t)
	agent.GlobalActivityHook(agent.AgentEvent{Type: agent.EventFinalResponse})

	got := a.GetActivityStatus()
	if got.State != models.ActivityDone {
		t.Errorf("State = %q, want done", got.State)
	}
}

func TestSetActivity_AutoIdlesAfterTimeout(t *testing.T) {
	a := resetActivity(t)
	original := activityIdleTimeout
	activityIdleTimeout = 20 * time.Millisecond
	defer func() { activityIdleTimeout = original }()

	setActivity(models.ActivityTool, "run_command")
	if got := a.GetActivityStatus().State; got != models.ActivityTool {
		t.Fatalf("State right after setActivity = %q, want tool", got)
	}

	time.Sleep(60 * time.Millisecond)
	if got := a.GetActivityStatus().State; got != models.ActivityIdle {
		t.Errorf("State after idle timeout = %q, want idle", got)
	}
}

func TestSetActivity_DoneAutoIdlesAfterItsOwnShorterTimeout(t *testing.T) {
	a := resetActivity(t)
	original := activityDoneTimeout
	activityDoneTimeout = 20 * time.Millisecond
	defer func() { activityDoneTimeout = original }()

	setActivity(models.ActivityDone, "")
	if got := a.GetActivityStatus().State; got != models.ActivityDone {
		t.Fatalf("State right after setActivity = %q, want done", got)
	}

	time.Sleep(60 * time.Millisecond)
	if got := a.GetActivityStatus().State; got != models.ActivityIdle {
		t.Errorf("State after done timeout = %q, want idle", got)
	}
}

func TestSetActivity_NewerEventCancelsStaleAutoIdle(t *testing.T) {
	a := resetActivity(t)
	original := activityIdleTimeout
	activityIdleTimeout = 20 * time.Millisecond
	defer func() { activityIdleTimeout = original }()

	setActivity(models.ActivityTool, "run_command")
	time.Sleep(15 * time.Millisecond) // let the first timer get close, not fire
	setActivity(models.ActivityWriting, "edit_file")

	time.Sleep(15 * time.Millisecond) // first timer's original deadline passes now
	if got := a.GetActivityStatus().State; got != models.ActivityWriting {
		t.Errorf("State = %q, want writing (stale timer must not have reset it)", got)
	}
}
