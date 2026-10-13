// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"memo/internal/beta"
	"memo/internal/config"
	"memo/internal/provider"
)

func gated(t *testing.T, betaOn, consent bool) {
	t.Helper()
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	beta.SetGate(func() bool { return betaOn })
	t.Cleanup(func() { beta.SetGate(nil) })
	if consent {
		if err := GrantConsent("chatgpt"); err != nil {
			t.Fatal(err)
		}
	}
}

func collect(t *testing.T, p *Provider, msgs ...provider.Message) (string, string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ch, err := p.ChatCompletionStream(ctx, provider.ChatRequest{Messages: msgs})
	if err != nil {
		t.Fatal(err)
	}
	var text, errText string
	for c := range ch {
		text += c.Content
		if c.Error != "" {
			errText = c.Error
		}
	}
	return text, errText
}

func fakeProvider(ask func(context.Context, string, func(string)) error) *Provider {
	return &Provider{typ: provider.ProviderWebChatGPT, site: &chatgptSite, model: "chatgpt-web", ask: ask}
}

func TestProvider_RefusesUntilBetaAndConsent(t *testing.T) {
	called := false
	p := fakeProvider(func(context.Context, string, func(string)) error { called = true; return nil })

	gated(t, false, true)
	if _, e := collect(t, p, provider.TextMessage("user", "hi")); !strings.Contains(e, "Beta") {
		t.Errorf("beta off: error %q should name Beta", e)
	}
	gated(t, true, false)
	if _, e := collect(t, p, provider.TextMessage("user", "hi")); !strings.Contains(e, "notice") {
		t.Errorf("no consent: error %q should name the notice", e)
	}
	if called {
		t.Fatal("the browser was driven although beta or consent was missing")
	}
	gated(t, true, true)
	if _, e := collect(t, p, provider.TextMessage("user", "hi")); e != "" {
		t.Errorf("allowed call failed: %s", e)
	}
	if !called {
		t.Fatal("an allowed call never reached the driver")
	}
}

func TestProvider_StreamsDeltasInOrder(t *testing.T) {
	gated(t, true, true)
	p := fakeProvider(func(_ context.Context, _ string, emit func(string)) error {
		emit("Hel")
		emit("lo")
		return nil
	})
	if got, e := collect(t, p, provider.TextMessage("user", "hi")); got != "Hello" || e != "" {
		t.Fatalf("got %q / %q", got, e)
	}
}

func TestProvider_FailuresBecomeSentencesNotPlumbing(t *testing.T) {
	gated(t, true, true)
	cases := []error{
		newErr(KindNeedsLogin, "ChatGPT", errors.New("rpc error: -32000 context deadline exceeded")),
		errors.New("chromedp: websocket: close 1006"),
	}
	for _, in := range cases {
		p := fakeProvider(func(context.Context, string, func(string)) error { return in })
		_, e := collect(t, p, provider.TextMessage("user", "hi"))
		if e == "" {
			t.Fatalf("%v produced no error text", in)
		}
		for _, bad := range []string{"rpc error", "chromedp", "websocket", "-32000", "deadline"} {
			if strings.Contains(e, bad) {
				t.Errorf("%v leaked %q: %s", in, bad, e)
			}
		}
	}
}

func TestProvider_PanicInDriverIsContained(t *testing.T) {
	gated(t, true, true)
	p := fakeProvider(func(context.Context, string, func(string)) error { panic("boom") })
	_, e := collect(t, p, provider.TextMessage("user", "hi"))
	if e == "" || strings.Contains(e, "boom") {
		t.Fatalf("panic must become a plain sentence, got %q", e)
	}
}

func TestProvider_CallerCancelStopsWithoutErrorAndWithoutLeak(t *testing.T) {
	gated(t, true, true)
	started := make(chan struct{})
	p := fakeProvider(func(ctx context.Context, _ string, emit func(string)) error {
		close(started)
		<-ctx.Done()
		return ctx.Err()
	})
	ctx, cancel := context.WithCancel(context.Background())
	ch, _ := p.ChatCompletionStream(ctx, provider.ChatRequest{Messages: []provider.Message{provider.TextMessage("user", "hi")}})
	<-started
	cancel()
	select {
	case c, ok := <-ch:
		if ok && c.Error != "" {
			t.Errorf("a user stop must not surface an error: %q", c.Error)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("stream did not close after the caller cancelled")
	}
}

func TestFlattenMessages(t *testing.T) {
	user := func(s string) provider.Message { return provider.TextMessage("user", s) }
	if got := FlattenMessages([]provider.Message{user("just this")}); got != "just this" {
		t.Errorf("lone user message must go through untouched, got %q", got)
	}
	got := FlattenMessages([]provider.Message{
		provider.TextMessage("system", "Be brief."),
		user("What is Go?"),
		provider.TextMessage("assistant", "A language."),
		user("And Rust?"),
		{Role: "user", Content: []provider.ContentPart{{Type: "text", Text: "last one"}, {Type: "image_url", ImageURL: &provider.ImageURL{URL: "data:image"}}}},
	})
	for _, want := range []string{"Be brief.", "User: What is Go?", "Assistant: A language.", "User: And Rust?", "User: last one"} {
		if !strings.Contains(got, want) {
			t.Errorf("missing %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "data:image") {
		t.Error("an image leaked into the text prompt")
	}
	if i, j := strings.Index(got, "And Rust?"), strings.Index(got, "last one"); i >= j {
		t.Error("the latest message must come last")
	}
	if FlattenMessages(nil) != "" {
		t.Error("no messages must flatten to empty")
	}
}

func TestOptInOnlyTypesNeverJoinTheFallbackChain(t *testing.T) {
	for _, tt := range []provider.ProviderType{provider.ProviderWebChatGPT, provider.ProviderWebClaude} {
		if !provider.IsOptInOnly(tt) {
			t.Errorf("%s must be opt-in only", tt)
		}
	}
	if provider.IsOptInOnly(provider.ProviderOpenAI) {
		t.Error("ordinary providers are not opt-in only")
	}
}

func TestConsent_FailsClosedOnDamagedFile(t *testing.T) {
	gated(t, true, true)
	if !Consented("chatgpt") {
		t.Fatal("granted consent not read back")
	}
	if err := os.WriteFile(consentPath(), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if Consented("chatgpt") {
		t.Fatal("a damaged consent file must read as no consent")
	}
	if err := RevokeConsent("chatgpt"); err != nil {
		t.Fatal(err)
	}
}
