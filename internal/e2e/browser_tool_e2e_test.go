// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"context"
	"strings"
	"sync"
	"testing"

	"memo/internal/agent/tools"
)

// fakeBrowserSession is a no-op stand-in for *browserengine.Session — these
// tests are about the tool-call/permission-flow mechanics (does
// browser_navigate correctly gate behind a Medium-danger permission prompt,
// does the model's approval actually reach the session), not about chromedp
// or a real Chromium process. See internal/browserengine's own
// session_real_test.go for the one test in this repo that actually launches
// a real browser.
type fakeBrowserSession struct {
	mu          sync.Mutex
	navigatedTo []string
	screenshotN int
	closed      bool
}

func (f *fakeBrowserSession) Navigate(_ context.Context, url string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.navigatedTo = append(f.navigatedTo, url)
	return nil
}

func (f *fakeBrowserSession) Screenshot(_ context.Context) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.screenshotN++
	// Minimal valid PNG signature + IHDR-less body is unnecessary here —
	// BrowserScreenshot only base64-encodes whatever bytes come back, it
	// doesn't decode/validate them, so any non-empty payload proves the
	// plumbing works end-to-end.
	return []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A}, nil
}

func (f *fakeBrowserSession) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

// fakeBrowserManager hands back the same fakeBrowserSession every time,
// mirroring browserengine.Manager's real single-global-session behavior
// (StartSession reuses an existing session rather than erroring).
type fakeBrowserManager struct {
	mu      sync.Mutex
	session *fakeBrowserSession
	starts  int // every StartSession call, whether or not it creates a new session
	created int // only when a new underlying session was actually created
}

func (f *fakeBrowserManager) StartSession(context.Context) (tools.BrowserSession, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	if f.session == nil {
		f.session = &fakeBrowserSession{}
		f.created++
	}
	return f.session, nil
}

func (f *fakeBrowserManager) GetSession() (tools.BrowserSession, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.session == nil {
		return nil, false
	}
	return f.session, true
}

func (f *fakeBrowserManager) StopSession() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.session == nil {
		return nil
	}
	err := f.session.Close()
	f.session = nil
	return err
}

// withFakeBrowser substitutes tools.InteractiveBrowser for the duration of
// the test and restores whatever Harness wired up (a real
// browserToolAdapter over a real, not-yet-started browserengine.Manager)
// afterward.
func withFakeBrowser(t *testing.T) *fakeBrowserManager {
	t.Helper()
	orig := tools.InteractiveBrowser
	fake := &fakeBrowserManager{}
	tools.InteractiveBrowser = fake
	t.Cleanup(func() { tools.InteractiveBrowser = orig })
	return fake
}

// TestAgent_BrowserNavigate_RequiresPermissionThenExecutes proves
// browser_navigate gates behind a real permission_request (danger_level
// "medium") the same way delete_file does for Dangerous tools, and that
// approving it actually reaches the session — not just returns a polite
// success string.
func TestAgent_BrowserNavigate_RequiresPermissionThenExecutes(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	fake := withFakeBrowser(t)

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "browser_navigate",
				Arguments: `{"url":"https://example.com"}`,
			}}}
		}
		return FakeChatResponse{Text: "açtım"}
	}

	chatID := h.NewAgentChat(t.TempDir())

	var permReq *AgentEvent
	resolved := false
	for ev := range h.SendMessageStreamAsync(chatID, "example.com'u aç") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type != "permission_request" {
				continue
			}
			cp := ae
			permReq = &cp
			if ae.ToolName != "browser_navigate" {
				t.Errorf("permission_request ToolName = %q, want browser_navigate", ae.ToolName)
			}
			if ae.DangerLevel != "medium" {
				t.Errorf("permission_request DangerLevel = %q, want medium", ae.DangerLevel)
			}
			fake.mu.Lock()
			startedBefore := fake.starts
			fake.mu.Unlock()
			if startedBefore != 0 {
				t.Fatal("StartSession was called before the permission was resolved")
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

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.session == nil || len(fake.session.navigatedTo) != 1 || fake.session.navigatedTo[0] != "https://example.com" {
		t.Fatalf("session.Navigate was not actually called with the expected URL: %+v", fake.session)
	}
}

// TestAgent_BrowserNavigate_AllowSession_SkipsPromptOnSecondCall proves
// approving with "allow_session" lets a second browser_navigate call in the
// same chat skip the prompt entirely — the whole point of Medium (not
// Dangerous) for this tool, since Dangerous never offers allow_session.
func TestAgent_BrowserNavigate_AllowSession_SkipsPromptOnSecondCall(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	fake := withFakeBrowser(t)

	callCount := 0
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		switch callCount {
		case 1:
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "browser_navigate",
				Arguments: `{"url":"https://a.example"}`,
			}}}
		case 2:
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_2",
				Name:      "browser_navigate",
				Arguments: `{"url":"https://b.example"}`,
			}}}
		default:
			return FakeChatResponse{Text: "tamam"}
		}
	}

	chatID := h.NewAgentChat(t.TempDir())

	permissionPrompts := 0
	for ev := range h.SendMessageStreamAsync(chatID, "önce a.example, sonra b.example'a git") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type != "permission_request" {
				continue
			}
			permissionPrompts++
			h.ResolveAgentPermission(ae.RequestID, "allow_session")
		}
	}

	if permissionPrompts != 1 {
		t.Errorf("saw %d permission_request events, want exactly 1 (second call should reuse the session-wide grant)", permissionPrompts)
	}
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.session == nil || len(fake.session.navigatedTo) != 2 {
		t.Fatalf("expected both navigate calls to reach the session, got: %+v", fake.session)
	}
	if fake.created != 1 {
		t.Errorf("a new session was created %d times, want exactly 1 (second browser_navigate should reuse the existing session, not launch a new one — StartSession itself is still called twice, once per tool invocation, which is expected)", fake.created)
	}
}

// TestAgent_BrowserScreenshot_SafeNoPromptReturnsImageData proves
// browser_screenshot (Safe) never triggers a permission prompt and its
// tool result actually carries the captured image data.
func TestAgent_BrowserScreenshot_SafeNoPromptReturnsImageData(t *testing.T) {
	h := NewHarness(t)
	h.SetAgentEnabled(true)
	fake := withFakeBrowser(t)
	if _, err := fake.StartSession(context.Background()); err != nil {
		t.Fatalf("seed session: %v", err)
	}

	callCount := 0
	var toolResult string
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		callCount++
		if callCount == 1 {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{
				ID:        "call_1",
				Name:      "browser_screenshot",
				Arguments: `{}`,
			}}}
		}
		// The fake provider's second call receives the tool result as part
		// of req — capture it here rather than needing a separate hook.
		for _, m := range req.Messages {
			if s := string(m); strings.Contains(s, "data:image/png;base64,") {
				toolResult = s
			}
		}
		return FakeChatResponse{Text: "işte ekran görüntüsü"}
	}

	chatID := h.NewAgentChat(t.TempDir())

	sawPermission := false
	for ev := range h.SendMessageStreamAsync(chatID, "ekran görüntüsü al") {
		if ev.FinishReason != "agent_event" {
			continue
		}
		for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
			if ae.Type == "permission_request" {
				sawPermission = true
			}
		}
	}

	if sawPermission {
		t.Error("browser_screenshot (Safe) should never trigger a permission_request")
	}
	fake.mu.Lock()
	screenshotN := fake.session.screenshotN
	fake.mu.Unlock()
	if screenshotN != 1 {
		t.Errorf("session.Screenshot called %d times, want exactly 1", screenshotN)
	}
	if toolResult == "" {
		t.Error("tool result never reached the model with the expected data:image/png;base64, prefix")
	}
}
