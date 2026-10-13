// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"memo/internal/provider"
	"memo/internal/webchat"
	"memo/internal/webchat/webchattest"
)

// webChatHarness wires a real App to the web chat provider pointing at a
// local fake chat page, with a real headless Chromium in between. It skips
// when no Chromium is installed.
func webChatHarness(t *testing.T) (*Harness, *webchattest.FakeChat) {
	t.Helper()
	if testing.Short() {
		t.Skip("real-Chromium test")
	}
	h := NewHarness(t)
	fake := webchattest.New(t)
	t.Cleanup(webchat.SetStartURLForTest("chatgpt", fake.URL()))
	if st := webchat.For(webchat.SiteByID("chatgpt")).Status(t.Context()); !st.Installed {
		t.Skip("no Chromium available")
	}
	h.SetBeta(true)
	h.SetWebSearchEnabled(false) // plain chat → the provider's streaming path
	if err := h.App.UpdateProvider(provider.ProviderConfig{
		Type: provider.ProviderWebChatGPT, Name: "ChatGPT (Web)", Model: "chatgpt-web", Enabled: true,
	}); err != nil {
		t.Fatalf("UpdateProvider(web-chatgpt): %v", err)
	}
	h.App.SetActiveProvider("ChatGPT (Web)")
	return h, fake
}

func (h *Harness) webChatAction(action string) *http.Response {
	return h.postJSON("/api/webchat", map[string]any{"action": action, "site": "chatgpt"})
}

func TestWebChat_NothingHappensUntilTheNoticeIsConfirmed(t *testing.T) {
	h, fake := webChatHarness(t)

	events := h.SendMessageStream(h.NewChat(), "hello there")
	last := events[len(events)-1]
	if !strings.Contains(last.Error, "notice") {
		t.Fatalf("last event = %+v, want the notice-not-confirmed sentence", last)
	}
	if fake.Sends() != 0 {
		t.Fatalf("the chat page received %d message(s) before consent", fake.Sends())
	}
}

func TestWebChat_ARealTurnThroughTheAppAndTheBrowser(t *testing.T) {
	h, fake := webChatHarness(t)
	if resp := h.webChatAction("consent"); resp.StatusCode != http.StatusOK {
		t.Fatalf("consent: %d", resp.StatusCode)
	}

	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "Say hello")
	reply := FinalContent(events)
	for _, want := range []string{"Hello **world**", "- one", "```go", "Done."} {
		if !strings.Contains(reply, want) {
			t.Errorf("reply missing %q:\n%s", want, reply)
		}
	}
	if last := events[len(events)-1]; last.Error != "" {
		t.Fatalf("turn ended with an error: %q", last.Error)
	}
	if got := fake.LastMessage(); !strings.Contains(got, "Say hello") {
		t.Errorf("the page received %q", got)
	}

	// Background helper calls (chat title, fact extraction…) must NOT reach the
	// web chat: each would be a real conversation in the person's account.
	time.Sleep(1500 * time.Millisecond) // let post-turn background work run
	if n := fake.Sends(); n != 1 {
		t.Fatalf("the chat page received %d messages for one user turn; helper calls must be skipped", n)
	}
}

func TestWebChat_SignedOutShowsAPlainSentenceAndSendsNothing(t *testing.T) {
	h, fake := webChatHarness(t)
	h.webChatAction("consent")
	fake.SetMode("login_text")

	events := h.SendMessageStream(h.NewChat(), "secret question")
	last := events[len(events)-1]
	if !strings.Contains(last.Error, "not signed in") {
		t.Fatalf("error = %q, want the not-signed-in sentence", last.Error)
	}
	for _, bad := range []string{"chromedp", "rpc error", "context deadline", "CDP"} {
		if strings.Contains(last.Error, bad) {
			t.Errorf("error leaks %q: %s", bad, last.Error)
		}
	}
	if fake.Sends() != 0 {
		t.Fatalf("a signed-out page received %d message(s)", fake.Sends())
	}
}

func TestWebChat_ABrokenWebChatDoesNotBreakOtherProviders(t *testing.T) {
	h, fake := webChatHarness(t)
	h.webChatAction("consent")
	fake.SetMode("silent")

	events := h.SendMessageStream(h.NewChat(), "hi")
	if events[len(events)-1].Error == "" {
		t.Fatal("a silent page should have produced an error")
	}

	// Back to the ordinary fake provider: works exactly as before.
	h.App.SetActiveProvider("e2e-fake")
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse { return FakeChatResponse{Text: "still fine"} }
	got := FinalContent(h.SendMessageStream(h.NewChat(), "hello"))
	if !strings.Contains(got, "still fine") {
		t.Fatalf("other provider after a web chat failure: %q", got)
	}
}
