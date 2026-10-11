// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"memo/internal/agent/tools"
)

// betaOnlyTools are the agent tools that exist only while Settings › Beta
// Features is on: opening an app, and the interactive browser session.
var betaOnlyTools = []string{
	"open_app", "browser_navigate", "browser_click", "browser_type",
	"browser_scroll", "browser_screenshot", "browser_get_text", "browser_close",
}

func toolNames(req FakeChatRequest) map[string]bool {
	names := map[string]bool{}
	for _, raw := range req.Tools {
		s := string(raw)
		for _, n := range append([]string{"read_file", "run_command"}, betaOnlyTools...) {
			if strings.Contains(s, `"name":"`+n+`"`) {
				names[n] = true
			}
		}
	}
	return names
}

// One agent turn that records which tools the model was offered.
func offeredTools(t *testing.T, h *Harness) map[string]bool {
	t.Helper()
	var offered map[string]bool
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		if offered == nil {
			offered = toolNames(req)
		}
		return FakeChatResponse{Text: "tamam"}
	}
	chatID := h.NewAgentChat(t.TempDir())
	for range h.SendMessageStreamAsync(chatID, "merhaba") {
	}
	if offered == nil {
		t.Fatal("the model was never called")
	}
	return offered
}

// With Beta off (the default) the two features do not exist for the model: not
// offered, so it cannot even try, while the ordinary agent tools are untouched.
func TestBeta_Off_TheModelIsNotOfferedOpenAppOrTheBrowserTools(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	offered := offeredTools(t, h)

	if !offered["read_file"] || !offered["run_command"] {
		t.Fatalf("ordinary agent tools missing from the request: %v", offered)
	}
	for _, n := range betaOnlyTools {
		if offered[n] {
			t.Errorf("tool %q was offered to the model with Beta off", n)
		}
	}
}

// Turning Beta on adds them to the very next turn, turning it off removes them
// again — no restart in between.
func TestBeta_TheSwitchTakesEffectOnTheNextTurnBothWays(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)

	h.SetBeta(true)
	on := offeredTools(t, h)
	for _, n := range betaOnlyTools {
		if !on[n] {
			t.Errorf("tool %q missing with Beta on", n)
		}
	}

	h.SetBeta(false)
	off := offeredTools(t, h)
	for _, n := range betaOnlyTools {
		if off[n] {
			t.Errorf("tool %q still offered after Beta was switched off", n)
		}
	}
}

// A model that calls one anyway (from memory, or a stale prompt) gets the
// ordinary "unknown tool" answer and nothing runs: no permission prompt, no
// browser session started.
func TestBeta_Off_ACallToABetaToolIsAnUnknownToolAndNothingRuns(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	fake := withFakeBrowserNoBeta(t)

	var toolResult string
	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "c1", Name: "browser_navigate", Arguments: `{"url":"https://example.com"}`}}}
		}
		for _, m := range req.Messages {
			if s := string(m); strings.Contains(s, "Unknown tool: browser_navigate") {
				toolResult = s
			}
		}
		return FakeChatResponse{Text: "olmadı"}
	}

	chatID := h.NewAgentChat(t.TempDir())
	permissionAsked := false
	for ev := range h.SendMessageStreamAsync(chatID, "example.com'u aç") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type == "permission_request" {
				permissionAsked = true
				h.ResolveAgentPermission(ae.RequestID, "allow_once")
			}
		}
	}

	if permissionAsked {
		t.Error("a permission prompt appeared for a tool that should not exist")
	}
	if toolResult == "" {
		t.Error("the model was not told the tool is unknown")
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.starts != 0 {
		t.Errorf("a browser session was started %d times with Beta off", fake.starts)
	}
}

// The panel's own REST endpoints refuse too, for any client that is not the
// Flutter app. Their contract reports a failed action as 200 with an "error"
// field, so that is what is checked: it must be there and say why.
func TestBeta_Off_TheBrowserPanelEndpointsRefuse(t *testing.T) {
	h := NewHarness(t)

	resp := h.postJSON("/api/browser/session/navigate", map[string]string{"url": "https://example.com"})
	defer resp.Body.Close()
	var out struct {
		Error string `json:"error"`
		URL   string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("navigate answer is not JSON: %v", err)
	}
	if out.Error == "" || out.URL != "" {
		t.Fatalf("navigate with Beta off = %+v, want an error and no page", out)
	}
	if !strings.Contains(strings.ToLower(out.Error), "beta") {
		t.Errorf("the refusal does not say why (Beta): %q", out.Error)
	}

	st := h.getJSON("/api/browser/session/status")
	defer st.Body.Close()
	var status struct {
		Active bool `json:"active"`
	}
	if err := json.NewDecoder(st.Body).Decode(&status); err != nil || st.StatusCode != http.StatusOK {
		t.Fatalf("status endpoint = %d (%v), want it to keep answering", st.StatusCode, err)
	}
	if status.Active {
		t.Error("status reports an active session with Beta off")
	}
}

// withFakeBrowserNoBeta is withFakeBrowser without switching Beta on.
func withFakeBrowserNoBeta(t *testing.T) *fakeBrowserManager {
	t.Helper()
	orig := tools.InteractiveBrowser
	fake := &fakeBrowserManager{installed: true}
	tools.InteractiveBrowser = fake
	t.Cleanup(func() { tools.InteractiveBrowser = orig })
	return fake
}
