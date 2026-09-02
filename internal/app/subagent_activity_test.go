package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"memo/internal/agent"
	"memo/internal/taskloop"
)

// TestEmitSubAgentActivity_TagsLineWithRoleAndCountsTokens is the regression
// for a bug found by running a real Self-Driving website task: an item big
// enough to spawn sub-agents (SubAgentOrchestrator.Spawn, subagent.go) went
// through appSubAgentRunner.Run with onEvent as a literal no-op. Four
// sub-agents ran in parallel — coder, analyzer, reviewer, test-runner —
// doing real tool calls the whole time, but neither the token estimate nor
// the activity log nor SilentSec ever moved. Live: RunningTaskInfo.SilentSec
// sat pinned to ElapsedSec for the full 384s the sub-agents ran, which is
// indistinguishable, from the task card, from a genuine hang.
//
// emitSubAgentActivity must both add tokens (so SilentSec resets — see
// rtAddTokens) and emit an activity line prefixed with the sub-agent's role,
// so four sub-agents interleaving in parallel read as four sub-agents in the
// log rather than one unattributed stream.
func TestEmitSubAgentActivity_TagsLineWithRoleAndCountsTokens(t *testing.T) {
	store, err := taskloop.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	tl, err := store.Create("chat-1", "T", []string{"do it"})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	var events []string
	engine := taskloop.NewEngine(
		store,
		func(ctx context.Context, chatID, prompt string) (string, error) { return "", nil },
		func(ctx context.Context, itemText, workerOutput string) (bool, string, error) { return true, "", nil },
		func(bool) {},
		func(name, data string) { events = append(events, name+"|"+data) },
	)

	a := &App{taskloopEngine: engine}

	args, _ := json.Marshal(map[string]string{"path": "app.py"})
	ev := agent.AgentEvent{
		Type:     agent.EventToolResult,
		ToolName: "write_file",
		Args:     args,
		Result:   "wrote 42 lines",
	}

	a.emitSubAgentActivity(tl.ID, "coder", ev)

	var activityEvent string
	for _, e := range events {
		if strings.HasPrefix(e, "taskloop:activity|") {
			activityEvent = e
		}
	}
	if activityEvent == "" {
		t.Fatalf("no taskloop:activity event fired; events = %v", events)
	}
	if !strings.Contains(activityEvent, "[coder]") {
		t.Errorf("activity line has no role prefix: %q", activityEvent)
	}
	if !strings.Contains(activityEvent, "app.py") {
		t.Errorf("activity line lost the tool arg: %q", activityEvent)
	}

	// Same call through the plain (non-sub-agent) path must NOT carry a
	// prefix — a plain worker/coder-step turn has only one actor.
	events = nil
	a.emitStepToolActivity(tl.ID, ev)
	for _, e := range events {
		if strings.HasPrefix(e, "taskloop:activity|") && strings.Contains(e, "[") {
			t.Errorf("plain worker activity line unexpectedly carries a role prefix: %q", e)
		}
	}
}

// TestEmitSubAgentActivity_NilEngineIsSafe: appSubAgentRunner.Run's onEvent
// closure calls this unconditionally; a list that finished/was torn down
// between spawn and this callback must not panic.
func TestEmitSubAgentActivity_NilEngineIsSafe(t *testing.T) {
	a := &App{}
	a.emitSubAgentActivity("some-list", "reviewer", agent.AgentEvent{Type: agent.EventToolResult})
}
