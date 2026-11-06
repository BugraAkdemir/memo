package e2e

import (
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"

	"memo/internal/twin"
)

func TestTwinHTTPManagementAndNativeToolsWithAgentOff(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(false)
	var initial struct {
		Contacts    []twin.Contact   `json:"contacts"`
		Preferences twin.Preferences `json:"preferences"`
	}
	resp := h.getJSON("/api/v1/twin/state")
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	decodeInto(t, resp, &initial)
	if len(initial.Contacts) != 0 || initial.Preferences.AllowProjectInspection {
		t.Fatal(initial)
	}
	create := func(name, channel, address string) string {
		t.Helper()
		r := h.postJSON("/api/twin/contact", twin.Contact{Name: name, Aliases: []twin.Alias{{Channel: channel, Address: address}}})
		if r.StatusCode != 200 {
			t.Fatal(r.StatusCode)
		}
		var result map[string]string
		decodeInto(t, r, &result)
		return result["id"]
	}
	id := create("Friend", "mail", "friend@example.com")
	other := create("Friend WA", "whatsapp", "905551234567")
	resp = h.postJSON("/api/twin/merge", map[string]string{"target_id": id, "source_id": other})
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	resp.Body.Close()
	var state struct {
		Contacts []twin.Contact `json:"contacts"`
	}
	decodeInto(t, h.getJSON("/api/twin/state"), &state)
	if len(state.Contacts) != 1 || len(state.Contacts[0].Aliases) != 2 {
		t.Fatal(state)
	}
	resp = h.postJSON("/api/twin/git_project_status", map[string]string{"project_id": "unregistered"})
	if resp.StatusCode != 403 {
		t.Fatal(resp.StatusCode)
	}
	resp.Body.Close()
	prefs := initial.Preferences
	prefs.Timezone = "Europe/Istanbul"
	resp = h.postJSON("/api/twin/preferences", prefs)
	if resp.StatusCode != 200 {
		t.Fatal(resp.StatusCode)
	}
	resp.Body.Close()
	var sawTimeline atomic.Bool
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if len(req.Tools) == 0 {
			return FakeChatResponse{Text: "title"}
		}
		if strings.Contains(string(req.Raw), `"name":"run_command"`) {
			t.Error("ordinary twin chat acquired shell")
		}
		if !strings.Contains(string(req.Raw), `"name":"contact_timeline"`) {
			t.Error("native twin tool absent")
		}
		for _, m := range req.Messages {
			if strings.Contains(string(m), `"tool_call_id":"timeline"`) && strings.Contains(string(m), `"role":"tool"`) {
				sawTimeline.Store(true)
				return FakeChatResponse{Text: "No recorded messages yet."}
			}
		}
		args, _ := json.Marshal(map[string]string{"contact_id": id})
		return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "timeline", Name: "contact_timeline", Arguments: string(args)}}}
	}
	events := h.SendMessageStream(h.NewChat(), "Show this linked contact's history")
	ran := false
	for _, e := range AgentEvents(t, events) {
		if e.Type == "tool_executing" && e.ToolName == "contact_timeline" {
			ran = true
		}
	}
	if !ran || !sawTimeline.Load() || h.App.GetAgentEnabled() {
		t.Fatalf("ran=%v result=%v agent=%v", ran, sawTimeline.Load(), h.App.GetAgentEnabled())
	}
}
