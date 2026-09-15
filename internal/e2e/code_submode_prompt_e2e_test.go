// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

// requestSystemContent decodes req's first message and returns its content —
// helper for asserting on which system prompt text an agent turn actually
// sent to the model.
func requestSystemContent(t *testing.T, req FakeChatRequest) string {
	t.Helper()
	if len(req.Messages) == 0 {
		t.Fatal("request has no messages")
	}
	var msg struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	if err := json.Unmarshal(req.Messages[0], &msg); err != nil {
		t.Fatalf("decode first message: %v", err)
	}
	if msg.Role != "system" {
		t.Fatalf("first message role = %q, want \"system\"", msg.Role)
	}
	return msg.Content
}

// TestCodeSubModePrompt_OverrideAffectsRealSystemPrompt is the real E2E
// proof for Unit 6 of docs/plans/PLAN_code_submodes.md: an override set via
// POST /api/code-mode/prompt must actually reach the model as that turn's
// system prompt (not just round-trip through config), and resetting it must
// bring back the built-in default.
func TestCodeSubModePrompt_OverrideAffectsRealSystemPrompt(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	const customPrompt = "You are a custom planning assistant for this test. Only ever say the word banana."
	h.SetCodeSubModePrompt("plan", customPrompt)

	projectDir := t.TempDir()
	chatID := h.NewAgentChat(projectDir)
	h.SetChatCodeSubMode(chatID, "plan")

	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{Text: "banana"}
	}

	for range h.SendMessageStream(chatID, "merhaba") {
	}

	reqs := h.Fake.Requests()
	if len(reqs) == 0 {
		t.Fatal("no requests reached the fake provider")
	}
	// Code Mode agent turns append buildAgentSystemPrompt()'s tool-use
	// instructions after the sub-mode directive (chat.go's routeStream) —
	// the override is a prefix of the real system message, not the whole
	// thing.
	if got := requestSystemContent(t, reqs[0]); !strings.HasPrefix(got, customPrompt) {
		t.Fatalf("system prompt = %q, want it to start with the custom override %q", got, customPrompt)
	}

	// Reset falls back to the built-in codePlanDirective — assert it's no
	// longer the custom text (not asserting the exact built-in wording here,
	// that's agent_chat_context_test.go's job — just that the override is gone).
	h.postJSON("/api/code-mode/prompt/reset", map[string]string{"sub_mode": "plan"}).Body.Close()

	chatID2 := h.NewAgentChat(t.TempDir())
	h.SetChatCodeSubMode(chatID2, "plan")
	for range h.SendMessageStream(chatID2, "merhaba tekrar") {
	}
	reqs = h.Fake.Requests()
	last := requestSystemContent(t, reqs[len(reqs)-1])
	if strings.Contains(last, customPrompt) {
		t.Fatalf("system prompt still carries the custom override after reset: %q", last)
	}
}
