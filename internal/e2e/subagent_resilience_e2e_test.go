package e2e

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/subagents"
)

// A provider hiccup (5xx, rate limit) in the middle of a delegated task must not
// kill the task: the child retries by itself, and tells the model that part of
// the work may already be on disk.
func TestChatSubAgents_TransientProviderErrorIsRetried(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(t.TempDir())
	h.App.SetSubAgentTimingForTests(time.Minute, time.Hour, []time.Duration{10 * time.Millisecond, 10 * time.Millisecond})
	t.Cleanup(func() { h.App.SetSubAgentTimingForTests(0, 0, nil) })
	var failures atomic.Int32
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if childRequest(req) {
			// Fail every request until the child itself reports the interruption.
			if !strings.Contains(lastSubAgentUser(req), "interrupted by a provider error") {
				failures.Add(1)
				return FakeChatResponse{Status: 503, RawBody: `{"error":"upstream unavailable"}`}
			}
			return FakeChatResponse{Text: "RECOVERED_AFTER_RETRY"}
		}
		return FakeChatResponse{Text: "MAIN"}
	}
	raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"DO_THE_TASK"}`))
	if err != nil {
		t.Fatal(err)
	}
	var job subagents.Job
	if err := json.Unmarshal([]byte(raw), &job); err != nil {
		t.Fatal(err)
	}
	done := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Awaiting })
	if done.Result != "RECOVERED_AFTER_RETRY" {
		t.Fatalf("result = %q", done.Result)
	}
	if failures.Load() == 0 {
		t.Fatal("the provider never failed, so the retry path was not exercised")
	}
	retried := false
	for _, e := range done.Events {
		if e.Type == "retry" {
			retried = true
		}
	}
	if !retried {
		t.Fatal("the retry must be visible in the child's events")
	}
}

// A child that stops making progress (a hung provider) fails with a clear
// reason after the idle limit instead of occupying a slot until a wall-clock
// deadline that also kills healthy long runs.
func TestChatSubAgents_StalledChildFailsWithIdleReason(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(t.TempDir())
	h.App.SetSubAgentTimingForTests(300*time.Millisecond, time.Hour, nil)
	t.Cleanup(func() { h.App.SetSubAgentTimingForTests(0, 0, nil) })
	release := make(chan struct{})
	t.Cleanup(func() { close(release) })
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if childRequest(req) {
			<-release
		}
		return FakeChatResponse{Text: "MAIN"}
	}
	raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"HANG"}`))
	if err != nil {
		t.Fatal(err)
	}
	var job subagents.Job
	_ = json.Unmarshal([]byte(raw), &job)
	failed := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Failed })
	if !strings.Contains(failed.Error, "no progress") {
		t.Fatalf("error should say why: %q", failed.Error)
	}
}

// A failure must reach the main agent with enough to act on, and the main
// agent must see every child's live state on its next turn and in list_agents.
func TestChatSubAgents_FailureAndLiveStateReachMainAgent(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(t.TempDir())
	mainRequests := make(chan string, 16)
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if childRequest(req) {
			return FakeChatResponse{Status: 400, RawBody: `{"error":"bad model for this account"}`}
		}
		mainRequests <- lastSubAgentUser(req)
		return FakeChatResponse{Text: "MAIN_ACK"}
	}
	raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"WILL_FAIL","title":"doomed"}`))
	if err != nil {
		t.Fatal(err)
	}
	var job subagents.Job
	_ = json.Unmarshal([]byte(raw), &job)
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Failed })

	review := awaitSubAgentValue(t, mainRequests)
	for _, want := range []string{"<memo-subagent-result>", "bad model for this account", `"hint"`, "send_agent"} {
		if !strings.Contains(review, want) {
			t.Errorf("main's failure review lacks %q:\n%s", want, review)
		}
	}

	// Wait until the review turn is saved so the next send is a fresh turn.
	time.Sleep(300 * time.Millisecond)
	for range h.SendMessageStream(chat, "durum ne?") {
	}
	var next string
	for len(mainRequests) > 0 {
		next = <-mainRequests
	}
	for _, want := range []string{"<memo-subagent-status", job.ID, "FAILED", "bad model for this account"} {
		if !strings.Contains(next, want) {
			t.Errorf("main's next turn lacks %q:\n%s", want, next)
		}
	}

	listed, err := h.App.ChatSubAgentAction(context.Background(), chat, "list", nil)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(listed, `"messages"`) || strings.Contains(listed, `"events"`) {
		t.Errorf("list_agents must return compact summaries, not transcripts:\n%s", listed)
	}
	if !strings.Contains(listed, `"status":"failed"`) || !strings.Contains(listed, "bad model for this account") {
		t.Errorf("list_agents lost the state/error:\n%s", listed)
	}
}

// list_agents is compact, but a child's full output stays reachable by id.
func TestChatSubAgents_ListByIDReturnsFullOutput(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(t.TempDir())
	long := strings.Repeat("LONG_RESULT_LINE ", 200) + "THE_END"
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if childRequest(req) {
			return FakeChatResponse{Text: long}
		}
		return FakeChatResponse{Text: "MAIN"}
	}
	raw, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"PRODUCE"}`))
	if err != nil {
		t.Fatal(err)
	}
	var job subagents.Job
	_ = json.Unmarshal([]byte(raw), &job)
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.State == subagents.Awaiting })
	compact, _ := h.App.ChatSubAgentAction(context.Background(), chat, "list", nil)
	if strings.Contains(compact, "THE_END") && strings.Contains(compact, `"output":`) {
		t.Errorf("the compact list should only carry a tail, not an output field:\n%s", compact)
	}
	full, err := h.App.ChatSubAgentAction(context.Background(), chat, "list", json.RawMessage(`{"id":"`+job.ID+`"}`))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(full, `"output":`) || !strings.Contains(full, "THE_END") {
		t.Errorf("list by id lost the full output:\n%s", full)
	}
}
