package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/subagents"
)

func childRequest(req FakeChatRequest) bool {
	for _, raw := range req.Messages {
		var msg struct{ Role, Content string }
		if json.Unmarshal(raw, &msg) == nil && msg.Role == "system" && strings.Contains(msg.Content, "You are a delegated sub-agent.") {
			return true
		}
	}
	return false
}
func lastSubAgentUser(req FakeChatRequest) string {
	var last string
	for _, raw := range req.Messages {
		var msg struct{ Role, Content string }
		if json.Unmarshal(raw, &msg) == nil && msg.Role == "user" {
			last = msg.Content
		}
	}
	return last
}
func awaitSubAgentValue[T any](t *testing.T, ch <-chan T) T {
	t.Helper()
	select {
	case value := <-ch:
		return value
	case <-time.After(8 * time.Second):
		t.Fatal("timed out waiting for sub-agent transition")
		var zero T
		return zero
	}
}
func awaitSubAgentJob(t *testing.T, h *Harness, chat string, check func(subagents.Job) bool) subagents.Job {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var jobs []subagents.Job
		decodeInto(t, h.getJSON("/api/subagents?chat_id="+chat), &jobs)
		for _, j := range jobs {
			if check(j) {
				return j
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("sub-agent never reached expected state")
	return subagents.Job{}
}
func awaitSubAgentSavedReply(t *testing.T, h *Harness, chat, text string) {
	t.Helper()
	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		var messages []struct{ Role, Content string }
		decodeInto(t, h.getJSON("/api/messages?chat_id="+chat), &messages)
		for _, m := range messages {
			if m.Role == "assistant" && strings.Contains(m.Content, text) {
				return
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("main agent did not save %q", text)
}

// A real streamed main-agent tool call launches a child through the normal
// registry. Its HTTP-provider request proves the child receives only task
// context, Code Mode and an explicitly selected skill; mailbox delivery then
// starts a second main-agent turn without another human send.
func TestChatSubAgents_ToolSpawnResumeAndFollowup(t *testing.T) {
	project := t.TempDir()
	skillDir := writeScopeTestSkill(t, filepath.Join(project, "unused-marker"))
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(project)
	skillName := h.InstallSkill(skillDir)
	h.SetCodeSubModePrompt("build", "CHILD-CODE-DIRECTIVE: inspect the assigned project carefully.")
	children := make(chan FakeChatRequest, 4)
	resumes := make(chan string, 4)
	var spawned atomic.Bool
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if childRequest(req) {
			children <- req
			if strings.Contains(lastSubAgentUser(req), "CHILD_FOLLOWUP") {
				return FakeChatResponse{Text: "CHILD_SECOND_RESULT"}
			}
			return FakeChatResponse{Text: "CHILD_FIRST_RESULT"}
		}
		if strings.Contains(lastSubAgentUser(req), "<memo-subagent-result>") && len(req.Tools) > 0 {
			resumes <- lastSubAgentUser(req)
			return FakeChatResponse{Text: "MAIN_ACCEPTED_CHILD_RESULT"}
		}
		if len(req.Tools) > 0 && strings.Contains(lastSubAgentUser(req), "DELEGATE_INITIAL") && spawned.CompareAndSwap(false, true) {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "spawn-test", Name: "spawn_agent", Arguments: `{"prompt":"CHILD_FIRST_TASK inspect parser","title":"Parser analysis","skills":["` + skillName + `"]}`}}}
		}
		return FakeChatResponse{Text: "Main has delegated the task."}
	}
	events := h.SendMessageStream(chat, "PARENT_PRIVATE_HISTORY_SENTINEL. DELEGATE_INITIAL using the selected skill.")
	for _, ev := range events {
		if ev.Error != "" {
			t.Fatalf("main stream: %s", ev.Error)
		}
	}
	if !spawned.Load() {
		t.Fatal("main agent did not invoke spawn_agent")
	}
	first := awaitSubAgentValue(t, children)
	sys := requestSystemContent(t, first)
	for _, want := range []string{"CHILD-CODE-DIRECTIVE", instructionSentinel} {
		if !strings.Contains(sys, want) {
			t.Errorf("child missing explicit instruction %s", want)
		}
	}
	if strings.Contains(string(first.Raw), "PARENT_PRIVATE_HISTORY_SENTINEL") {
		t.Error("parent conversation leaked into child")
	}
	for _, bad := range []string{"memory.json", "get_calendar_events", "Takvim Hakkında", "Hafıza Hakkında"} {
		if strings.Contains(sys, bad) {
			t.Errorf("child contains personal context %q", bad)
		}
	}
	offered := offeredToolNames(first)
	for _, bad := range []string{"spawn_agent", "send_agent", "get_calendar_events", "whatsapp_send", "run_command", "run_command_readonly", "write_file"} {
		if offered[bad] {
			t.Errorf("read-only child offered %s", bad)
		}
	}
	if !offered["read_file"] || !offered["search_content"] {
		t.Error("child has no project inspection tools")
	}
	completion := awaitSubAgentValue(t, resumes)
	if !strings.Contains(completion, "CHILD_FIRST_RESULT") {
		t.Fatalf("missing child result in resumed main: %s", completion)
	}
	awaitSubAgentSavedReply(t, h, chat, "MAIN_ACCEPTED_CHILD_RESULT")
	job := awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.State == subagents.Awaiting })
	resp := h.postJSON("/api/subagents/"+job.ID+"/send", map[string]string{"chat_id": chat, "prompt": "CHILD_FOLLOWUP verify parser edge cases"})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("followup status %d", resp.StatusCode)
	}
	var followed subagents.Job
	decodeInto(t, resp, &followed)
	if followed.ID != job.ID || followed.Round != 2 {
		t.Fatalf("followup created a different conversation: %+v", followed)
	}
	second := awaitSubAgentValue(t, children)
	if !strings.Contains(string(second.Raw), "CHILD_FIRST_TASK") || !strings.Contains(string(second.Raw), "CHILD_FIRST_RESULT") || !strings.Contains(string(second.Raw), "CHILD_FOLLOWUP") {
		t.Fatalf("followup lost task history: %s", second.Raw)
	}
	completion = awaitSubAgentValue(t, resumes)
	if !strings.Contains(completion, "CHILD_SECOND_RESULT") {
		t.Fatalf("second result lost: %s", completion)
	}
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.ID == job.ID && j.Round == 2 && j.State == subagents.Awaiting })
}

func TestChatSubAgents_HTTPOwnershipAndSettings(t *testing.T) {
	project := t.TempDir()
	h := NewHarness(t)
	chat := h.NewAgentChat(project)
	other := h.NewChat()
	created, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", json.RawMessage(`{"prompt":"inspect project"}`))
	if err != nil {
		t.Fatal(err)
	}
	var job subagents.Job
	if err = json.Unmarshal([]byte(created), &job); err != nil {
		t.Fatal(err)
	}
	resp := h.getJSON("/api/subagents/" + job.ID + "?chat_id=" + other)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("cross-chat read status=%d", resp.StatusCode)
	}
	for _, action := range []string{"send", "cancel", "close", "merge", "resume"} {
		resp = h.postJSON("/api/subagents/"+job.ID+"/"+action, map[string]string{"chat_id": other, "prompt": "change another chat"})
		resp.Body.Close()
		if resp.StatusCode < 400 {
			t.Errorf("cross-chat %s accepted", action)
		}
	}
	var jobs []subagents.Job
	decodeInto(t, h.getJSON("/api/subagents?chat_id="+other), &jobs)
	if len(jobs) != 0 {
		t.Fatalf("cross-chat list leaked jobs: %+v", jobs)
	}
	for _, n := range []int{0, -1, 17} {
		resp = h.putJSON("/api/subagents/settings", map[string]int{"max_parallel": n})
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("invalid limit %d status=%d", n, resp.StatusCode)
		}
	}
	resp = h.putJSON("/api/subagents/settings", map[string]int{"max_parallel": 2})
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("valid settings status=%d", resp.StatusCode)
	}
	var setting map[string]int
	decodeInto(t, resp, &setting)
	if setting["max_parallel"] != 2 {
		t.Fatalf("settings=%v", setting)
	}
	decodeInto(t, h.getJSON("/api/subagents/settings"), &setting)
	if setting["max_parallel"] != 2 {
		t.Fatalf("read settings=%v", setting)
	}
}

// Child 2 finishes while main is blocked processing child 1. A human message
// arrives in between. Provider-side channels prove only one main turn runs at
// once and that both the queued human turn and second result survive.
func TestChatSubAgents_ResultWhileMainBusyQueuesWithHumanTurn(t *testing.T) {
	project := t.TempDir()
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)
	chat := h.NewAgentChat(project)
	child1, child2 := make(chan struct{}), make(chan struct{})
	mainRelease := make(chan struct{})
	var releaseOnce, child1Once, child2Once sync.Once
	t.Cleanup(func() {
		child1Once.Do(func() { close(child1) })
		child2Once.Do(func() { close(child2) })
		releaseOnce.Do(func() { close(mainRelease) })
	})
	childStarted := make(chan string, 2)
	mainStarted := make(chan string, 8)
	var mainBusy atomic.Int32
	var overlap atomic.Bool
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		last := lastSubAgentUser(req)
		if childRequest(req) {
			childStarted <- last
			if strings.Contains(last, "QUEUE_CHILD_ONE") {
				<-child1
				return FakeChatResponse{Text: "QUEUE_RESULT_ONE"}
			}
			<-child2
			return FakeChatResponse{Text: "QUEUE_RESULT_TWO"}
		}
		if len(req.Tools) > 0 && (strings.Contains(last, "<memo-subagent-result>") || strings.Contains(last, "HUMAN_WHILE_MAIN_BUSY")) {
			if mainBusy.Add(1) != 1 {
				overlap.Store(true)
			}
			defer mainBusy.Add(-1)
			label := "human"
			if strings.Contains(last, "<memo-subagent-result>") {
				label = "second"
				if strings.Contains(last, "QUEUE_RESULT_ONE") {
					label = "first"
				}
			}
			mainStarted <- label
			if label == "first" {
				<-mainRelease
			}
			return FakeChatResponse{Text: "QUEUE_MAIN_" + label}
		}
		return FakeChatResponse{Text: "ok"}
	}
	for _, prompt := range []string{"QUEUE_CHILD_ONE", "QUEUE_CHILD_TWO"} {
		args, _ := json.Marshal(map[string]string{"prompt": prompt})
		if _, err := h.App.ChatSubAgentAction(context.Background(), chat, "spawn", args); err != nil {
			t.Fatal(err)
		}
	}
	awaitSubAgentValue(t, childStarted)
	awaitSubAgentValue(t, childStarted)
	child1Once.Do(func() { close(child1) })
	if got := awaitSubAgentValue(t, mainStarted); got != "first" {
		t.Fatalf("first resumed main=%s", got)
	}
	humanDone := make(chan struct{})
	go func() {
		for range h.SendMessageStreamAsync(chat, "HUMAN_WHILE_MAIN_BUSY") {
		}
		close(humanDone)
	}()
	child2Once.Do(func() { close(child2) })
	awaitSubAgentJob(t, h, chat, func(j subagents.Job) bool { return j.Prompt == "QUEUE_CHILD_TWO" && j.State == subagents.Awaiting })
	select {
	case got := <-mainStarted:
		t.Fatalf("main turn %q overlapped blocked result handling", got)
	default:
	}
	releaseOnce.Do(func() { close(mainRelease) })
	got := map[string]bool{}
	for len(got) < 2 {
		label := awaitSubAgentValue(t, mainStarted)
		got[label] = true
	}
	if !got["human"] || !got["second"] {
		t.Fatalf("lost queued turn: %v", got)
	}
	awaitSubAgentValue(t, humanDone)
	awaitSubAgentSavedReply(t, h, chat, "QUEUE_MAIN_second")
	if overlap.Load() {
		t.Error("main provider requests overlapped")
	}
}
