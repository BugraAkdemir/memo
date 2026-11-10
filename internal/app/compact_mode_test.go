package app

import (
	"context"
	"strings"
	"testing"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/truncate"
)

func TestIsCompactWindow(t *testing.T) {
	for _, c := range []struct {
		window int
		want   bool
	}{
		{0, false}, {-1, false}, {2048, true}, {4096, true}, {8192, true},
		{9999, true}, {10000, false}, {32768, false}, {1 << 20, false},
	} {
		if got := isCompactWindow(c.window); got != c.want {
			t.Errorf("isCompactWindow(%d) = %v, want %v", c.window, got, c.want)
		}
	}
}

// localRouteCtx is a turn the auto router sent to the local model with the
// given window. It reaches the same compact decision as a turn on a running
// local llama-server (which a unit test cannot start: IsRunning checks a real
// OS process), without needing one.
func localRouteCtx(window int) context.Context {
	return context.WithValue(context.Background(), subscriptionTextRouteKey{},
		&subscriptionTextRoute{provider: "local", window: window, auto: true})
}

func newCompactTestApp(t *testing.T) (*App, *sessions.Manager, string) {
	t.Helper()
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}
	a := &App{
		cfg: &config.AppConfig{
			Memory: config.MemoryConfig{MemoryEnabled: false},
			Llama:  config.LlamaConfig{CtxSize: 4096},
		},
		identity: identity.New("Buğra", "Memo", "casual", "", false),
		sessions: sm,
	}
	return a, sm, sm.GetActiveID()
}

func systemOf(t *testing.T, msgs []api.Message) string {
	t.Helper()
	for _, m := range msgs {
		if m.Role == "system" {
			s, _ := m.Content.(string)
			return s
		}
	}
	t.Fatal("no system message")
	return ""
}

func TestBuildMessagesForSession_CompactWindowGetsCompactPrompt(t *testing.T) {
	a, _, chatID := newCompactTestApp(t)

	sys := systemOf(t, a.buildMessagesForSession(localRouteCtx(4096), chatID, "merhaba naber", nil, nil))

	if want := a.identity.BuildCompactSystemPrompt(false); sys != want {
		t.Fatalf("system prompt on a 4k window =\n%q\nwant the compact prompt\n%q", sys, want)
	}
	if strings.Contains(sys, "Buğra") {
		t.Error("compact prompt carries the user's name; nothing personal may go in")
	}
	if n := truncate.EstimateTokens(sys); n > 150 {
		t.Errorf("compact prompt is ~%d tokens, want <= 150", n)
	}
}

func TestBuildMessagesForSession_LargeWindowKeepsFullPrompt(t *testing.T) {
	a, _, chatID := newCompactTestApp(t)

	sys := systemOf(t, a.buildMessagesForSession(localRouteCtx(32768), chatID, "merhaba naber", nil, nil))

	want := a.identity.BuildSystemPrompt(nil, true, false, false, false, false)
	if sys != want {
		t.Fatalf("a 32k window must keep today's full system prompt;\ngot  %q\nwant %q", truncate.Text(sys, 120), truncate.Text(want, 120))
	}
}

func TestBuildMessagesForSession_CompactWindowLeavesRoomForTheReply(t *testing.T) {
	a, sm, chatID := newCompactTestApp(t)
	long := strings.Repeat("Bu uzun bir geçmiş mesajıdır, pencereyi doldurmak için yazıldı. ", 8)
	for i := 0; i < 60; i++ {
		sm.AddMessageToSession(chatID, "user", long, "", "")
		sm.AddMessageToSession(chatID, "assistant", long, "", "")
	}

	for _, window := range []int{4096, 8192} {
		msgs := a.buildMessagesForSession(localRouteCtx(window), chatID, "son durum ne?", nil, nil)
		total := 0
		for _, m := range msgs {
			total += truncate.EstimateTokens(m.GetTextContent())
		}
		if limit := window - compactReplyReserve(window); total > limit {
			t.Errorf("window %d: prompt is ~%d tokens, want <= %d so the reply keeps its share", window, total, limit)
		}
		if last := msgs[len(msgs)-1].GetTextContent(); !strings.HasPrefix(last, "son durum ne?") {
			t.Errorf("window %d: the current message must be last, got %q", window, truncate.Text(last, 40))
		}
	}
}

func TestBuildMessagesForSession_CompactPromptIsByteStableAcrossTurns(t *testing.T) {
	a, sm, chatID := newCompactTestApp(t)

	first := systemOf(t, a.buildMessagesForSession(localRouteCtx(4096), chatID, "selam", nil, nil))
	sm.AddMessageToSession(chatID, "user", "selam", "", "")
	sm.AddMessageToSession(chatID, "assistant", "Selam! Nasıl yardımcı olabilirim?", "", "")
	second := systemOf(t, a.buildMessagesForSession(localRouteCtx(4096), chatID, "yarın hava nasıl olacak?", nil, nil))

	if first != second {
		t.Fatalf("compact system prompt changed between turns (breaks the KV cache):\n%q\n%q", first, second)
	}
}

func TestBuildCompactSystemPrompt_AgentRuleOnlyWithAgent(t *testing.T) {
	id := identity.New("Buğra", "Memo", "casual", "", false)
	if strings.Contains(id.BuildCompactSystemPrompt(false), identity.CompactAgentRule) {
		t.Error("agent rule present with agent mode off")
	}
	if !strings.Contains(id.BuildCompactSystemPrompt(true), identity.CompactAgentRule) {
		t.Error("agent rule missing with agent mode on")
	}
}
