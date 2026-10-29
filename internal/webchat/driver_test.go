// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/chromedp"
	"memo/internal/config"
	"memo/internal/webchat/webchattest"
)

// realBrowserService starts a Service for a copy of the ChatGPT table whose
// start page is the fake site, driving a real headless Chromium. It skips when
// no Chromium is available, like the other real-browser tests in this repo.
func realBrowserService(t *testing.T, f *webchattest.FakeChat) *Service {
	t.Helper()
	if testing.Short() {
		t.Skip("real-Chromium test")
	}
	t.Setenv("MEMO_DATA_DIR", t.TempDir())
	config.ResetForTests()
	t.Cleanup(config.ResetForTests)
	headless = true
	t.Cleanup(func() { headless = false })

	site := chatgptSite
	site.StartURL = f.URL()
	svc := &Service{site: &site, resolve: realResolve}
	if _, err := svc.resolve(context.Background()); err != nil {
		t.Skipf("no usable Chromium: %v", err)
	}
	t.Cleanup(svc.Close)
	return svc
}

func ask(t *testing.T, svc *Service, prompt string) (string, error) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	var sb strings.Builder
	err := svc.Ask(ctx, prompt, func(d string) { sb.WriteString(d) })
	return sb.String(), err
}

func TestAsk_StreamsMarkdownAndFinishes(t *testing.T) {
	f := webchattest.New(t)
	svc := realBrowserService(t, f)

	got, err := ask(t, svc, "Say hello\nin two lines")
	if err != nil {
		t.Fatalf("Ask: %v", err)
	}
	for _, want := range []string{"Hello **world**", "- one", "- two", "```go\nfmt.Println(\"hi\")\n```", "Done."} {
		if !strings.Contains(got, want) {
			t.Errorf("reply missing %q\n--- got ---\n%s", want, got)
		}
	}
	if strings.Contains(got, "Copy") {
		t.Errorf("a button's label leaked into the reply:\n%s", got)
	}
	if msg := f.LastMessage(); !strings.Contains(msg, "Say hello") || !strings.Contains(msg, "in two lines") {
		t.Errorf("the page received %q; the multi-line prompt must arrive whole", msg)
	}
}

func TestAsk_EveryTurnIsAFreshConversation(t *testing.T) {
	f := webchattest.New(t)
	svc := realBrowserService(t, f)
	for i := 0; i < 2; i++ {
		got, err := ask(t, svc, "again")
		if err != nil {
			t.Fatalf("turn %d: %v", i, err)
		}
		// A reused thread would hand back the previous reply too.
		if strings.Count(got, "Hello **world**") != 1 {
			t.Fatalf("turn %d reply repeated earlier content:\n%s", i, got)
		}
	}
}

func TestAsk_ErrorKinds(t *testing.T) {
	cases := []struct {
		mode string
		want ErrKind
	}{
		{"login", KindNeedsLogin},
		{"login_text", KindNeedsLogin},
		{"challenge", KindChallenge},
		{"quota", KindQuota},
		{"silent", KindPageChanged},
	}
	for _, c := range cases {
		t.Run(c.mode, func(t *testing.T) {
			f := webchattest.New(t)
			f.SetMode(c.mode)
			svc := realBrowserService(t, f)
			old := startTimeout
			startTimeout = 2 * time.Second
			t.Cleanup(func() { startTimeout = old })
			_, err := ask(t, svc, "hi")
			if KindOf(err) != c.want {
				t.Fatalf("kind = %q (err %v), want %q", KindOf(err), err, c.want)
			}
			// The message a person reads must be a sentence, never plumbing.
			msg := err.Error()
			for _, bad := range []string{"chromedp", "context deadline", "rpc error", "CDP", "-32000", "evaluate"} {
				if strings.Contains(msg, bad) {
					t.Errorf("user-facing message leaks %q: %s", bad, msg)
				}
			}
			if c.want == KindQuota && !strings.Contains(msg, "usage limit") {
				t.Errorf("quota message lost the allowance wording the quota card keys on: %s", msg)
			}
		})
	}
}

func TestAsk_NoLoginMeansNothingWasSent(t *testing.T) {
	f := webchattest.New(t)
	f.SetMode("login")
	svc := realBrowserService(t, f)
	if _, err := ask(t, svc, "secret question"); KindOf(err) != KindNeedsLogin {
		t.Fatalf("want needs_login, got %v", err)
	}
	if n := int32(f.Sends()); n != 0 {
		t.Fatalf("a signed-out page received %d message(s); nothing may be typed before sign-in is confirmed", n)
	}
}

func TestAsk_BrowserRestartsAfterItIsClosed(t *testing.T) {
	f := webchattest.New(t)
	svc := realBrowserService(t, f)
	if _, err := ask(t, svc, "one"); err != nil {
		t.Fatal(err)
	}
	svc.t.cancel() // the browser dies (user closed the window)
	got, err := ask(t, svc, "two")
	if err != nil {
		t.Fatalf("after the browser died: %v", err)
	}
	if !strings.Contains(got, "Done.") {
		t.Fatalf("no reply after restart: %q", got)
	}
}

func TestAsk_OnlyOneTurnAtATime(t *testing.T) {
	f := webchattest.New(t)
	svc := realBrowserService(t, f)
	svc.busy.Lock()
	defer svc.busy.Unlock()
	if _, err := ask(t, svc, "hi"); KindOf(err) != KindBusy {
		t.Fatalf("want busy, got %v", err)
	}
}

func TestStablePrefix(t *testing.T) {
	cases := map[string]string{"": "", "abc": "", "abc\n": "abc\n", "abc\ndef": "abc\n", "a\n\nb": "a\n\n"}
	for in, want := range cases {
		if got := stablePrefix(in); got != want {
			t.Errorf("stablePrefix(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMain(m *testing.M) { os.Exit(m.Run()) }

func waitFor(t *testing.T, what string, within time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(within)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %s", what)
}

// Reported live: the person closed the browser window by accident while
// Connect was waiting for them to sign in; Connect kept polling a dead page
// and, because it still counted as "running", pressing Connect again did
// nothing at all.
func TestConnect_ClosingTheWindowEndsItAndConnectWorksAgain(t *testing.T) {
	f := webchattest.New(t)
	f.SetMode("login")
	svc := realBrowserService(t, f)
	ctx := context.Background()

	svc.StartConnect()
	waitFor(t, "the window to open and wait for sign-in", 20*time.Second, func() bool {
		svc.mu.Lock()
		defer svc.mu.Unlock()
		return svc.t.alive() && svc.connecting
	})

	// The person closes the window.
	svc.mu.Lock()
	closing := svc.t
	svc.mu.Unlock()
	_ = chromedp.Run(closing.ctx, browser.Close())

	waitFor(t, "Connect to notice the closed window", 15*time.Second, func() bool {
		return !svc.Status(ctx).Connecting
	})
	st := svc.Status(ctx)
	if !strings.Contains(st.Error, "closed") {
		t.Fatalf("status error = %q, want the window-closed sentence", st.Error)
	}

	// Now they sign in properly: pressing Connect again must work.
	f.SetMode("ok")
	svc.StartConnect()
	waitFor(t, "the second Connect to finish", 30*time.Second, func() bool {
		return !svc.Status(ctx).Connecting
	})
	st = svc.Status(ctx)
	if st.Error != "" || st.State != "ready" {
		t.Fatalf("after reconnecting: state=%q error=%q, want ready/no error", st.State, st.Error)
	}
}

// Pressing Connect while an earlier attempt is still waiting replaces it.
func TestConnect_PressingItAgainReplacesAWaitingAttempt(t *testing.T) {
	f := webchattest.New(t)
	f.SetMode("login")
	svc := realBrowserService(t, f)
	ctx := context.Background()

	svc.StartConnect()
	// Connecting becomes true before the goroutine has opened/navigated the
	// browser. Wait for the login page too, so this tests replacing a human
	// sign-in wait rather than cancelling an in-flight browser startup.
	waitFor(t, "first connect to wait for sign-in", 20*time.Second, func() bool {
		st := svc.Status(ctx)
		return st.Connecting && st.State == "login"
	})
	f.SetMode("ok")
	svc.StartConnect() // second press, first still waiting
	waitFor(t, "second connect to finish", 30*time.Second, func() bool { return !svc.Status(ctx).Connecting })
	if st := svc.Status(ctx); st.State != "ready" || st.Error != "" {
		t.Fatalf("state=%q error=%q after replacing a waiting attempt", st.State, st.Error)
	}
}
