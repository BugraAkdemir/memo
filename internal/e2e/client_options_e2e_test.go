// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// The terminal REPL and the desktop app share one backend whose auto-permission
// flag is global. A client now asks for what it wants per request
// (options.auto_permission) and the backend's own flag never moves, so the two
// can be used side by side without disturbing each other.

func deleteScript(h *Harness, file string) {
	calls := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		calls++
		if calls == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "c1", Name: "delete_file", Arguments: `{"path":"` + file + `"}`}}}
		}
		return FakeChatResponse{Text: "bitti"}
	}
}

func sawPermissionRequest(h *Harness, chatID string, opts map[string]any) bool {
	saw := false
	for ev := range h.SendMessageStreamAsyncWith(chatID, "sil", opts) {
		for _, ae := range AgentEvents(h.t, []SSEEvent{ev}) {
			if ae.Type == "permission_request" {
				saw = true
				h.ResolveAgentPermission(ae.RequestID, "allow_once")
			}
		}
	}
	return saw
}

func TestClientOptions_ExplicitAutoPermissionOffPromptsEvenWhenTheBackendFlagIsOn(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(true) // the desktop app's global flag is ON
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deleteScript(h, "x.txt")
	chat := h.NewAgentChat(dir)

	if !sawPermissionRequest(h, chat, map[string]any{"auto_permission": false}) {
		t.Fatal("a client that asked for auto_permission=false must still be prompted while the global flag is on")
	}
	if !h.autoPermissionOn() {
		t.Fatal("the request changed the backend's global auto-permission flag")
	}
}

func TestClientOptions_ExplicitAutoPermissionOnSkipsThePromptWhenTheBackendFlagIsOff(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(false)
	dir := t.TempDir()
	target := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deleteScript(h, "x.txt")
	chat := h.NewAgentChat(dir)

	if sawPermissionRequest(h, chat, map[string]any{"auto_permission": true}) {
		t.Fatal("auto_permission=true must not prompt")
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("the tool should have run without a prompt (stat err = %v)", err)
	}
	if h.autoPermissionOn() {
		t.Fatal("the request turned the backend's global auto-permission flag on")
	}
}

func TestClientOptions_NoOptionsFollowsTheBackendFlag(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	h.SetAutoPermission(false)
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deleteScript(h, "x.txt")
	chat := h.NewAgentChat(dir)
	if !sawPermissionRequest(h, chat, nil) {
		t.Fatal("without options the backend's own flag (off) decides, as before")
	}
}

func TestClientOptions_WebToolsOffHidesWebSearchAndFetchPage(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	var toolNames []string
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		toolNames = nil
		for _, raw := range req.Tools {
			var d struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			}
			_ = json.Unmarshal(raw, &d)
			toolNames = append(toolNames, d.Function.Name)
		}
		return FakeChatResponse{Text: "tamam"}
	}
	chat := h.NewAgentChat(t.TempDir())

	for range h.SendMessageStreamAsyncWith(chat, "merhaba", map[string]any{"web_tools": false}) {
	}
	joined := strings.Join(toolNames, ",")
	if len(toolNames) == 0 || strings.Contains(joined, "web_search") || strings.Contains(joined, "fetch_page") {
		t.Fatalf("tools with web_tools=false: %v", toolNames)
	}
	if !strings.Contains(joined, "read_file") {
		t.Fatalf("the other tools must stay: %v", toolNames)
	}

	for range h.SendMessageStreamAsyncWith(chat, "tekrar", nil) {
	}
	joined = strings.Join(toolNames, ",")
	if !strings.Contains(joined, "web_search") {
		t.Fatalf("without options the web tools are there as before: %v", toolNames)
	}
}
