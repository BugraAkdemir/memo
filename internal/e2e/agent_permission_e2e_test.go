// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAgent_ToolCallRequiresPermissionThenExecutes is the real E2E proof
// behind BUG-PERM1's whole class: a real agent turn, a real tool call the
// fake provider actually asks for, a real permission_request event that
// actually reaches the client over the real SSE stream, a real POST to
// /api/agent/permission resolving it WHILE that same stream is still open
// (the agent pipeline blocks server-side on exactly this, so the test must
// resolve it concurrently rather than after the stream ends — see
// SendMessageStreamAsync's doc comment), and — the part a unit test with a
// mocked executor can't prove — the tool call *actually executing
// afterward*: a real file really disappears from disk.
//
// Uses delete_file (DangerLevel: Dangerous), not write_file — a real,
// deliberate discovery made writing this test: NewAgentChat's chat starts
// in Code Mode (v4.4.0), which auto-approves file-*editing* tools
// (write_file/edit_file/insert_line/delete_lines) with no permission
// prompt at all. That's real, working, documented behavior (confirmed live
// here, not assumed) — delete_file/run_command/change_directory are the
// tools that still gate behind a real permission_request in Code Mode.
func TestAgent_ToolCallRequiresPermissionThenExecutes(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	projectDir := t.TempDir()
	targetPath := filepath.Join(projectDir, "delete-me.txt")
	if err := os.WriteFile(targetPath, []byte("bye"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "delete_file",
				Arguments: `{"path":"delete-me.txt"}`,
			}}}
		}
		return FakeChatResponse{Text: "sildim"}
	}

	chatID := h.NewAgentChat(projectDir)

	var permReq *AgentEvent
	resolved := false
	for ev := range h.SendMessageStreamAsync(chatID, "delete-me.txt dosyasını sil") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type != "permission_request" {
				continue
			}
			cp := ae
			permReq = &cp
			if ae.ToolName != "delete_file" {
				t.Errorf("permission_request ToolName = %q, want delete_file", ae.ToolName)
			}
			// The file must NOT be gone yet — the whole point of the
			// permission gate is that the tool hasn't run until approved.
			if _, err := os.Stat(targetPath); err != nil {
				t.Fatalf("delete-me.txt is already gone before the permission was resolved — the gate did not actually hold the tool call: %v", err)
			}
			h.ResolveAgentPermission(ae.RequestID, "allow_once")
			resolved = true
		}
	}

	if permReq == nil {
		t.Fatal("no permission_request event in the stream")
	}
	if !resolved {
		t.Fatal("saw a permission_request but never resolved it")
	}
	if _, err := os.Stat(targetPath); err == nil {
		t.Fatal("delete-me.txt still exists after approving the permission — the tool never actually ran")
	} else if !os.IsNotExist(err) {
		t.Fatalf("unexpected error checking deleted file: %v", err)
	}
}

// TestAgent_DeniedPermissionNeverExecutesTheTool is the mirror case: a
// denied permission must leave the filesystem untouched, not just return a
// polite error.
func TestAgent_DeniedPermissionNeverExecutesTheTool(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	projectDir := t.TempDir()
	targetPath := filepath.Join(projectDir, "keep-me.txt")
	if err := os.WriteFile(targetPath, []byte("keep"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "delete_file",
				Arguments: `{"path":"keep-me.txt"}`,
			}}}
		}
		return FakeChatResponse{Text: "tamam, silmedim"}
	}

	chatID := h.NewAgentChat(projectDir)
	sawPermission := false
	for ev := range h.SendMessageStreamAsync(chatID, "keep-me.txt dosyasını sil") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type == "permission_request" {
				sawPermission = true
				h.ResolveAgentPermission(ae.RequestID, "deny_once")
			}
		}
	}
	if !sawPermission {
		t.Fatal("no permission_request event in the stream")
	}

	time.Sleep(300 * time.Millisecond) // let any (incorrect) background delete happen if the gate were broken
	if _, err := os.Stat(targetPath); err != nil {
		t.Fatalf("keep-me.txt was deleted despite the permission being denied: %v", err)
	}
}
