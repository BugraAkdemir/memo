// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/config"
)

// TestAgent_SaveCodePlanWritesRealFile is the real E2E proof behind Unit 3 of
// docs/plans/PLAN_code_submodes.md: a real agent turn where the fake
// provider actually calls save_code_plan, and — the part a unit test with a
// fake PlanSaver can't prove — a real plan.md really lands on disk under
// Memo's own data directory (config.DataPath("plans", ...)), not the
// project directory the chat is scoped to. save_code_plan is DangerLevel
// Safe, so this never raises a permission_request — SendMessageStream's
// simple synchronous drain is enough, no concurrent permission resolution
// needed (contrast internal/e2e/agent_permission_e2e_test.go's Dangerous
// tool cases).
func TestAgent_SaveCodePlanWritesRealFile(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	projectDir := t.TempDir()
	// t.TempDir()'s own last path component is always a plain, already-safe
	// (lowercase alphanumeric) string — internal/app's unexported
	// projectSlug would produce the same value here, so this black-box test
	// can predict the resulting directory without reaching into that
	// package's internals.
	slug := strings.ToLower(filepath.Base(projectDir))
	planDir := config.DataPath("plans", slug)
	t.Cleanup(func() { _ = os.RemoveAll(planDir) })

	// saveCodePlan trims leading/trailing whitespace (it also rejects an
	// empty/whitespace-only plan outright) — the written file has no
	// trailing newline even though the tool call's own JSON argument does.
	const planContent = "# Implementation Plan\n\n1. Do the thing\n2. Test the thing"

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "save_code_plan",
				Arguments: `{"content":"# Implementation Plan\n\n1. Do the thing\n2. Test the thing\n"}`,
			}}}
		}
		return FakeChatResponse{Text: "plan hazır, build'e geçelim mi?"}
	}

	chatID := h.NewAgentChat(projectDir)

	var finalText strings.Builder
	sawDone := false
	for _, ev := range h.SendMessageStream(chatID, "bu özelliği nasıl uygularız, bir plan çıkar") {
		if ev.Error != "" {
			t.Fatalf("stream error: %s", ev.Error)
		}
		finalText.WriteString(ev.Content)
		if ev.Done {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatal("stream never reached Done")
	}
	if !strings.Contains(finalText.String(), "build'e geçelim mi?") {
		t.Errorf("final reply = %q, want it to contain the model's own follow-up text", finalText.String())
	}

	planPath := filepath.Join(planDir, "plan.md")
	data, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("plan.md was not written to %s: %v", planPath, err)
	}
	if string(data) != planContent {
		t.Errorf("plan.md content = %q, want %q", data, planContent)
	}

	// The tool must not have touched the project directory at all — the
	// whole point of save_code_plan bypassing write_file's sandbox is that
	// it writes to Memo's own storage, never the user's project.
	entries, err := os.ReadDir(projectDir)
	if err != nil {
		t.Fatalf("ReadDir(projectDir): %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("project directory is not empty after save_code_plan: %v", entries)
	}
}
