// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestAgent_PlanSubMode_TextConfirmFlow is the real E2E proof for decision 1
// of docs/plans/PLAN_code_submodes.md (plain-text plan confirmation) with
// global auto-permission OFF: after save_code_plan succeeds, the turn ends
// normally (a single Done, no chaining) and the chat is left "awaiting" a
// decision — the very next user message is classified (classifyPlanDecisionReply)
// and, on a match, actually flips the chat's persisted sub-mode.
func TestAgent_PlanSubMode_TextConfirmFlow(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	// Auto-permission deliberately left off (harness default) — this is the
	// "ask in chat, wait for the next message" path.

	projectDir := t.TempDir()
	chatID := h.NewAgentChat(projectDir)
	h.SetChatCodeSubMode(chatID, "plan")

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		switch callCount {
		case 1:
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "save_code_plan",
				Arguments: `{"content":"# Plan\n\n1. do it"}`,
			}}}
		case 2:
			return FakeChatResponse{Text: "plan hazır, build'e geçelim mi?"}
		default:
			return FakeChatResponse{Text: "tamam, build modundayım"}
		}
	}

	var finalText strings.Builder
	doneCount := 0
	for _, ev := range h.SendMessageStream(chatID, "bu iş için bir plan çıkar") {
		if ev.Error != "" {
			t.Fatalf("stream error: %s", ev.Error)
		}
		finalText.WriteString(ev.Content)
		if ev.Done {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Fatalf("expected exactly 1 terminal Done chunk, got %d", doneCount)
	}
	if !strings.Contains(finalText.String(), "build'e geçelim mi?") {
		t.Errorf("final reply = %q, want the model's own follow-up question", finalText.String())
	}

	// Auto-permission was off, so the chat must NOT have been switched yet —
	// it stays in "plan" until the user actually answers.
	if subMode, _ := h.GetChatCodeSubMode(chatID); subMode != "plan" {
		t.Fatalf("sub-mode after the plan turn = %q, want \"plan\" (must not auto-advance with auto-permission off)", subMode)
	}

	// The next message answers the question — classifyPlanDecisionReply
	// should pick "build" (explicitly named) and flip the persisted pin.
	finalText.Reset()
	doneCount = 0
	for _, ev := range h.SendMessageStream(chatID, "evet build'e geçelim") {
		if ev.Error != "" {
			t.Fatalf("stream error: %s", ev.Error)
		}
		finalText.WriteString(ev.Content)
		if ev.Done {
			doneCount++
		}
	}
	if doneCount != 1 {
		t.Fatalf("expected exactly 1 terminal Done chunk on the follow-up turn, got %d", doneCount)
	}
	if subMode, pinned := h.GetChatCodeSubMode(chatID); subMode != "build" || !pinned {
		t.Fatalf("sub-mode after answering \"evet build'e geçelim\" = (%q, pinned=%v), want (\"build\", true)", subMode, pinned)
	}
}

// TestAgent_PlanSubMode_AutoPermissionOn_ChainsToBuildWithinOneResponse is
// the load-bearing regression test for decision 4 + the chaining design in
// callAgentStream (Unit 4 of docs/plans/PLAN_code_submodes.md): with global
// auto-permission on, saving a plan must immediately continue into a real
// build-mode pass — a SECOND agent loop, a SECOND real tool call
// (run_command, only auto-approved in "build", never in "plan"/"auto") — all
// within the SAME SSE response (exactly one terminal Done), never as a
// second stream the client would have to reconnect for. A hang here (the
// per-chat lock being re-acquired from inside the already-locked goroutine)
// would fail this test by timeout, not by a wrong assertion.
func TestAgent_PlanSubMode_AutoPermissionOn_ChainsToBuildWithinOneResponse(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true)

	projectDir := t.TempDir()
	chatID := h.NewAgentChat(projectDir)
	h.SetChatCodeSubMode(chatID, "plan")

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		switch callCount {
		case 1: // plan pass: save the plan
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "save_code_plan",
				Arguments: `{"content":"# Plan\n\n1. run it"}`,
			}}}
		case 2: // plan pass: finish normally -> triggers chaining
			return FakeChatResponse{Text: "plan hazır"}
		case 3: // build pass: a run_command call — Dangerous, build-only auto-approve
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_2",
				Name:      "run_command",
				Arguments: `{"command":"echo built > out.txt"}`,
			}}}
		default: // build pass: finish normally
			return FakeChatResponse{Text: "tamamlandı"}
		}
	}

	var finalText strings.Builder
	doneCount := 0
	sawSubModeChanged := false
	sawSaveCodePlan := false
	sawRunCommand := false
	for _, ev := range h.SendMessageStream(chatID, "bu iş için plan çıkar ve uygula") {
		if ev.Error != "" {
			t.Fatalf("stream error: %s", ev.Error)
		}
		if ev.FinishReason == "code_submode_changed" {
			sawSubModeChanged = true
			if ev.Content != "build" {
				t.Errorf("code_submode_changed content = %q, want \"build\"", ev.Content)
			}
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type != "tool_result" {
				continue
			}
			if ae.ToolName == "save_code_plan" {
				sawSaveCodePlan = true
			}
			if ae.ToolName == "run_command" {
				sawRunCommand = true
			}
		}
		finalText.WriteString(ev.Content)
		if ev.Done {
			doneCount++
		}
	}

	if doneCount != 1 {
		t.Fatalf("expected exactly 1 terminal Done chunk across the whole chained response, got %d", doneCount)
	}
	if !sawSubModeChanged {
		t.Error("never saw a code_submode_changed chunk")
	}
	if !sawSaveCodePlan {
		t.Error("save_code_plan never showed up as a completed tool_result")
	}
	if !sawRunCommand {
		t.Error("run_command never showed up as a completed tool_result — the build continuation never ran, or its tool call was denied")
	}
	if !strings.Contains(finalText.String(), "tamamlandı") {
		t.Errorf("final reply = %q, want it to contain the build pass's own closing text", finalText.String())
	}

	// Proves run_command really executed (not just accepted at the API
	// layer) — same "prove real execution" bar as
	// agent_permission_e2e_test.go's file-deletion assertions.
	data, err := os.ReadFile(filepath.Join(projectDir, "out.txt"))
	if err != nil || strings.TrimSpace(string(data)) != "built" {
		t.Fatalf("run_command did not actually execute in the build continuation: err=%v data=%q", err, data)
	}

	if subMode, pinned := h.GetChatCodeSubMode(chatID); subMode != "build" || !pinned {
		t.Fatalf("sub-mode after the chained turn = (%q, pinned=%v), want (\"build\", true)", subMode, pinned)
	}
}
