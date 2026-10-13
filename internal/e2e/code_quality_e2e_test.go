// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"strings"
	"testing"
)

func offeredToolNames(req FakeChatRequest) map[string]bool {
	names := map[string]bool{}
	for _, raw := range req.Tools {
		var x struct {
			Function struct {
				Name string `json:"name"`
			} `json:"function"`
		}
		if json.Unmarshal(raw, &x) == nil && x.Function.Name != "" {
			names[x.Function.Name] = true
		}
	}
	return names
}

// A project (Code Mode) chat is for editing code. It must offer the content
// search a coding agent needs, must not offer messaging / calendar / routine /
// self-management tools, and its instructions must be about code — not about
// memory files and calendars.
func TestCodeMode_OffersCodeToolsAndCodeInstructionsOnly(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	chatID := h.NewAgentChat(t.TempDir())
	for range h.SendMessageStreamAsync(chatID, "fix the bug in main.go") {
	}
	reqs := h.Fake.Requests()
	if len(reqs) == 0 {
		t.Fatal("the model was never called")
	}
	tools := offeredToolNames(reqs[0])
	for _, want := range []string{"search_content", "edit_file", "read_file", "run_command"} {
		if !tools[want] {
			t.Errorf("Code Mode must offer %s; offered: %v", want, tools)
		}
	}
	for _, bad := range []string{"whatsapp_send", "whatsapp_search", "get_calendar_events", "create_routine", "configure_provider", "self_clone"} {
		if tools[bad] {
			t.Errorf("Code Mode must not offer %s", bad)
		}
	}
	sys := requestSystemContent(t, reqs[0])
	if !strings.Contains(sys, "search_content") {
		t.Error("the Code Mode instructions should tell the model to look up every use with search_content")
	}
	for _, bad := range []string{"memory.json", "get_calendar_events", "Takvim Hakkında", "Hafıza Hakkında"} {
		if strings.Contains(sys, bad) {
			t.Errorf("Code Mode instructions still carry the non-code section %q", bad)
		}
	}
	if !strings.Contains(sys, "start_self_driving_task") {
		t.Error("the Self-Driving / Task.md section must stay: those tools are still offered")
	}
}
