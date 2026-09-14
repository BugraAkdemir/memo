// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestTaskList_RunningDoesNotBypassPermissionsForUnrelatedInteractiveChat is
// the real E2E proof behind the P0 fix for the taskloop-permission-bypass
// leak: while a Self-Driving task list is actively running, an unrelated
// interactive agent-mode chat (a different chat entirely, nothing to do
// with the task list) must still get a real permission_request for a
// dangerous tool call — it must NOT be silently auto-approved just because
// some other task list happens to be running in the background.
//
// Before the fix, internal/app/app.go wired the taskloop engine's
// ref-counted setBypass callback directly to
// a.agentExecutor.SetBypassPermissions(v) — but a.agentExecutor is the same
// shared executor every interactive/WhatsApp/Telegram agent-mode call uses
// (internal/app/llm.go's callAgentStream default branch), so starting any
// task list silently disabled every permission prompt everywhere until the
// last task list finished.
func TestTaskList_RunningDoesNotBypassPermissionsForUnrelatedInteractiveChat(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	interactiveDir := t.TempDir()
	targetPath := filepath.Join(interactiveDir, "keep-me.txt")
	if err := os.WriteFile(targetPath, []byte("keep"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	const taskMarker = "BYPASS_LEAK_REGRESSION_MARKER"
	workerBlocked := make(chan struct{})
	releaseWorker := make(chan struct{})
	var once sync.Once

	var mu sync.Mutex
	sentInteractiveToolCall := false

	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		// isReviewCall must be checked first: the chief-review prompt also
		// embeds the item text (and so matches taskMarker below) alongside
		// its own distinctive "görev denetleyicisisin" marker.
		if isReviewCall(req) {
			return FakeChatResponse{Text: `{"approved": true, "feedback": ""}`}
		}
		if containsText(req, taskMarker) {
			// The task list's own worker turn — block here so the list is
			// guaranteed to still be "running" (engine.activeCount >= 1)
			// while the test drives the unrelated interactive chat below.
			once.Do(func() { close(workerBlocked) })
			<-releaseWorker
			return FakeChatResponse{Text: "iş tamamlandı"}
		}

		mu.Lock()
		defer mu.Unlock()
		if !sentInteractiveToolCall {
			sentInteractiveToolCall = true
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "delete_file",
				Arguments: `{"path":"keep-me.txt"}`,
			}}}
		}
		return FakeChatResponse{Text: "tamam"}
	}

	taskChatID := h.NewAgentChat(t.TempDir())
	tl := h.CreateTaskList(taskChatID, "bypass leak list", []string{taskMarker + " tek maddelik iş"})
	h.StartTaskList(tl.ID)

	select {
	case <-workerBlocked:
	case <-time.After(10 * time.Second):
		t.Fatal("task list's worker turn never reached the fake provider — can't prove it was running")
	}

	interactiveChatID := h.NewAgentChat(interactiveDir)
	sawPermission := false
	for ev := range h.SendMessageStreamAsync(interactiveChatID, "keep-me.txt dosyasını sil") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type != "permission_request" {
				continue
			}
			sawPermission = true
			if ae.ToolName != "delete_file" {
				t.Errorf("permission_request ToolName = %q, want delete_file", ae.ToolName)
			}
			h.ResolveAgentPermission(ae.RequestID, "deny_once")
		}
	}

	close(releaseWorker)
	final := h.WaitForTaskStatus(tl.ID, 10*time.Second, "done", "failed")
	if final.Status != "done" {
		t.Errorf("task list ended in status %q, want done", final.Status)
	}

	if !sawPermission {
		t.Fatal("no permission_request for the unrelated interactive delete_file call while a task list was running — " +
			"the taskloop's global bypass leaked into the shared interactive agent executor")
	}
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("keep-me.txt was deleted despite the permission being denied: %v", err)
	}
}

// containsText reports whether any message in req contains substr — a
// looser variant of isReviewCall's own check, reused here to spot the task
// worker's turn by the item text it was seeded with.
func containsText(req FakeChatRequest, substr string) bool {
	for _, m := range req.Messages {
		if strings.Contains(string(m), substr) {
			return true
		}
	}
	return false
}
