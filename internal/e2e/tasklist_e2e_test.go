// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"strings"
	"testing"
	"time"
)

// isReviewCall detects a taskloop chief-review call by its distinctive
// system prompt text (taskloop.ChiefReviewSystemPrompt) rather than by call
// order — worker and review calls both go through the same active
// provider, and their relative ordering isn't a contract this test should
// depend on.
func isReviewCall(req FakeChatRequest) bool {
	for _, m := range req.Messages {
		if strings.Contains(string(m), "görev denetleyicisisin") {
			return true
		}
	}
	return false
}

// TestTaskList_WorkerModeItemRunsToCompletion is the real E2E proof behind
// BUG-PLAN10/12's territory: a real Self-Driving task list, created through
// the real REST API, actually executed by the real taskloop.Engine — real
// worker turn, real chief-review turn, real state persistence — reaching
// "done" for real, not asserted against a mocked engine.
func TestTaskList_WorkerModeItemRunsToCompletion(t *testing.T) {
	h := NewHarness(t)

	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if isReviewCall(req) {
			return FakeChatResponse{Text: `{"approved": true, "feedback": ""}`}
		}
		return FakeChatResponse{Text: "iş tamamlandı"}
	}

	chatID := h.NewAgentChat(t.TempDir())
	tl := h.CreateTaskList(chatID, "e2e test list", []string{"tek maddelik iş"})
	h.StartTaskList(tl.ID)
	if tl.ID == "" {
		t.Fatal("CreateTaskList returned an empty id")
	}
	if len(tl.Items) != 1 {
		t.Fatalf("got %d items, want 1: %+v", len(tl.Items), tl.Items)
	}

	final := h.WaitForTaskStatus(tl.ID, 10*time.Second, "done", "failed")
	if final.Status != "done" {
		t.Fatalf("task list ended in status %q, want done: items=%+v", final.Status, final.Items)
	}
	if len(final.Items) != 1 || final.Items[0].Status != "done" {
		t.Errorf("item status = %+v, want a single done item", final.Items)
	}
}

// TestTaskList_ChiefReviewRejectionKeepsItemRunning proves the review gate
// actually gates: a first "not approved" verdict must NOT mark the item
// done — only after a second attempt gets approved does the list finish.
// A mocked-engine unit test can assert the state machine's transition
// table; it can't prove the real chief-review call's JSON response is what
// actually drives that transition end to end.
func TestTaskList_ChiefReviewRejectionKeepsItemRunning(t *testing.T) {
	h := NewHarness(t)

	reviewCalls := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if isReviewCall(req) {
			reviewCalls++
			if reviewCalls == 1 {
				return FakeChatResponse{Text: `{"approved": false, "feedback": "eksik, tekrar dene"}`}
			}
			return FakeChatResponse{Text: `{"approved": true, "feedback": ""}`}
		}
		return FakeChatResponse{Text: "iş tamamlandı"}
	}

	chatID := h.NewAgentChat(t.TempDir())
	tl := h.CreateTaskList(chatID, "e2e retry list", []string{"tek maddelik iş"})
	h.StartTaskList(tl.ID)

	final := h.WaitForTaskStatus(tl.ID, 10*time.Second, "done", "failed")
	if final.Status != "done" {
		t.Fatalf("task list ended in status %q, want done after the second review approved it: items=%+v", final.Status, final.Items)
	}
	if reviewCalls < 2 {
		t.Errorf("expected at least 2 chief-review calls (one rejection, one approval), got %d", reviewCalls)
	}
}
