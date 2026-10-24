package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"memo/internal/subagents"
)

// A failed parent review must not silence later completed children. No human
// message is sent between the first failure and the second automatic review.
func TestChatSubAgents_InterruptedReviewDoesNotBlockNewResults(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(t.TempDir())
	var firstReviews, secondReviews, childRuns atomic.Int32
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		last := lastSubAgentUser(req)
		if childRequest(req) {
			childRuns.Add(1)
			if strings.Contains(last, "CHILD_NEW_RESULT") {
				return FakeChatResponse{Text: "CHILD_NEW_RESULT"}
			}
			return FakeChatResponse{Text: "CHILD_OLD_RESULT"}
		}
		if strings.Contains(last, "<memo-subagent-result>") && len(req.Tools) > 0 {
			if strings.Contains(last, `"result":"CHILD_OLD_RESULT"`) {
				firstReviews.Add(1)
				return FakeChatResponse{Status: 503, RawBody: `{"error":{"message":"parent review temporarily unavailable"}}`}
			}
			if strings.Contains(last, `"result":"CHILD_NEW_RESULT"`) {
				secondReviews.Add(1)
				return FakeChatResponse{Text: "AUTO_REVIEWED_NEW_CHILD_WITHOUT_USER_PROMPT"}
			}
		}
		return FakeChatResponse{Text: "ordinary response"}
	}
	spawn := func(prompt string) subagents.Job {
		t.Helper()
		args, _ := json.Marshal(map[string]string{"prompt": prompt})
		raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", args)
		if err != nil {
			t.Fatal(err)
		}
		var j subagents.Job
		if err := json.Unmarshal([]byte(raw), &j); err != nil {
			t.Fatal(err)
		}
		return j
	}
	old := spawn("CHILD_OLD_RESULT inspect one file")
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == old.ID && j.ResultDelivery == "interrupted" })
	newer := spawn("CHILD_NEW_RESULT inspect another file")
	awaitSubAgentSavedReply(t, h, chat, "AUTO_REVIEWED_NEW_CHILD_WITHOUT_USER_PROMPT")
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool {
		return j.ID == newer.ID && j.State == subagents.Awaiting && j.ResultDelivery == ""
	})
	// The stopped review stays available for explicit resume; no replay/auto-ACK.
	jobs, err := h.App.ListChatSubAgents(chat)
	if err != nil {
		t.Fatal(err)
	}
	for _, j := range jobs {
		if j.ID == old.ID && j.ResultDelivery != "interrupted" {
			t.Fatalf("old review was changed/replayed: %+v", j)
		}
	}
	if firstReviews.Load() != 1 || secondReviews.Load() != 1 || childRuns.Load() != 2 {
		t.Fatalf("reviews old=%d new=%d child runs=%d", firstReviews.Load(), secondReviews.Load(), childRuns.Load())
	}
}
