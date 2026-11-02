package autonomy

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"memo/internal/twin"
)

func promiseFixture(t *testing.T, track bool) (*Engine, twin.Message, *int) {
	t.Helper()
	e, in := engineFixture(t)
	ctx := context.Background()
	p, err := e.Store.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 48, TrackPromises: track}
	if err = e.Store.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	e.Now = time.Now
	msg := twin.Message{Channel: "mail", Account: "account", EventID: "out-1", Address: in.Sender, Outgoing: true, Body: "Raporu yarın akşam sana gönderirim.", SentAt: time.Now(), Subject: "Rapor"}
	if err = e.Store.RecordMessage(ctx, msg, ""); err != nil {
		t.Fatal(err)
	}
	calls := 0
	e.Generate = func(_ context.Context, system, user string) (string, error) {
		calls++
		if system != promisePrompt {
			t.Errorf("unexpected prompt %.50s", system)
		}
		if !strings.Contains(user, "Raporu yarın akşam sana gönderirim.") || !strings.Contains(user, "untrusted") {
			t.Errorf("the message must be passed as untrusted data: %s", user)
		}
		if !strings.Contains(user, `"now"`) || !strings.Contains(user, `"timezone"`) {
			t.Errorf("relative dates need the current time and timezone: %s", user)
		}
		due := time.Now().Add(30 * time.Hour).UTC().Format(time.RFC3339)
		return fmt.Sprintf(`{"promises":[{"summary":"Raporu gönder","due_at":"%s"}]}`, due), nil
	}
	return e, msg, &calls
}

func promisesOf(t *testing.T, e *Engine) []twin.Commitment {
	t.Helper()
	all, err := e.Store.Commitments(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := []twin.Commitment{}
	for _, c := range all {
		if c.Kind == "my_promise" {
			out = append(out, c)
		}
	}
	return out
}

func TestExtractPromisesStoresDatedPromisesOnce(t *testing.T) {
	e, msg, calls := promiseFixture(t, true)
	ctx := context.Background()
	n, err := e.ExtractPromises(ctx, msg)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := promisesOf(t, e)
	if len(got) != 1 || got[0].Body != "Raporu gönder" || got[0].Address != "friend@example.com" || got[0].DueAt.Before(time.Now()) {
		t.Fatalf("stored=%+v", got)
	}
	// The same message never costs a second model call or a second promise.
	if n, err = e.ExtractPromises(ctx, msg); err != nil || n != 0 || *calls != 1 || len(promisesOf(t, e)) != 1 {
		t.Fatalf("repeat n=%d err=%v calls=%d promises=%d", n, err, *calls, len(promisesOf(t, e)))
	}
}

// Found live: "yarın akşam" written at 01:26 in Istanbul became TODAY, because the
// model got now=...+03:00 together with timezone=UTC and counted days in UTC. "now"
// must be expressed in the same zone the model is told to use.
func TestPromisePromptGivesNowInTheOwnersTimezone(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct{ tz, wantNow string }{
		{"UTC", "2026-11-02T22:26:32Z"},
		{"Europe/Istanbul", "2026-11-03T01:26:32+03:00"},
		{"America/New_York", "2026-11-02T17:26:32-05:00"},
	} {
		t.Run(tc.tz, func(t *testing.T) {
			e, msg, _ := promiseFixture(t, true)
			p, _ := e.Store.Preferences(ctx)
			p.Timezone = tc.tz
			if err := e.Store.SavePreferences(ctx, p); err != nil {
				t.Fatal(err)
			}
			// The same instant as seen from a machine in Istanbul.
			e.Now = func() time.Time { return time.Date(2026, 11, 3, 1, 26, 32, 0, istanbul) }
			var user string
			e.Generate = func(_ context.Context, _, u string) (string, error) { user = u; return `{"promises":[]}`, nil }
			if _, err := e.ExtractPromises(ctx, msg); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(user, `"now":"`+tc.wantNow+`"`) || !strings.Contains(user, `"timezone":"`+tc.tz+`"`) {
				t.Fatalf("now and timezone disagree: %s", user)
			}
		})
	}
}

func TestExtractPromisesDoesNothingUnlessAskedTo(t *testing.T) {
	ctx := context.Background()
	t.Run("tracking off", func(t *testing.T) {
		e, msg, calls := promiseFixture(t, false)
		if n, err := e.ExtractPromises(ctx, msg); err != nil || n != 0 || *calls != 0 {
			t.Fatalf("n=%d err=%v calls=%d", n, err, *calls)
		}
	})
	t.Run("radar off", func(t *testing.T) {
		e, msg, calls := promiseFixture(t, true)
		p, _ := e.Store.Preferences(ctx)
		p.Radar.Enabled = false
		if err := e.Store.SavePreferences(ctx, p); err != nil {
			t.Fatal(err)
		}
		if n, err := e.ExtractPromises(ctx, msg); err != nil || n != 0 || *calls != 0 {
			t.Fatalf("n=%d err=%v calls=%d", n, err, *calls)
		}
	})
	t.Run("no reply model", func(t *testing.T) {
		e, msg, _ := promiseFixture(t, true)
		e.Generate = nil
		if n, err := e.ExtractPromises(ctx, msg); err != nil || n != 0 {
			t.Fatalf("n=%d err=%v", n, err)
		}
	})
	t.Run("incoming message", func(t *testing.T) {
		e, msg, calls := promiseFixture(t, true)
		msg.Outgoing = false
		if n, err := e.ExtractPromises(ctx, msg); err != nil || n != 0 || *calls != 0 {
			t.Fatalf("someone else's promise is not the owner's: n=%d err=%v calls=%d", n, err, *calls)
		}
	})
	t.Run("too short or too long", func(t *testing.T) {
		e, msg, calls := promiseFixture(t, true)
		msg.Body = "ok"
		if n, _ := e.ExtractPromises(ctx, msg); n != 0 || *calls != 0 {
			t.Fatalf("tiny message reached the model: calls=%d", *calls)
		}
		msg.EventID, msg.Body = "out-2", strings.Repeat("a", 9000)
		if n, _ := e.ExtractPromises(ctx, msg); n != 0 || *calls != 0 {
			t.Fatalf("huge message reached the model: calls=%d", *calls)
		}
	})
}

func TestExtractPromisesNeverInventsOrKeepsBadDates(t *testing.T) {
	e, msg, _ := promiseFixture(t, true)
	ctx := context.Background()
	soon := time.Now().Add(48 * time.Hour).UTC().Format(time.RFC3339)
	e.Generate = func(context.Context, string, string) (string, error) {
		return fmt.Sprintf(`{"promises":[
{"summary":"no date at all"},
{"summary":"unparseable","due_at":"yarın"},
{"summary":"in the past","due_at":"2020-01-01T10:00:00Z"},
{"summary":"far future","due_at":"2099-01-01T10:00:00Z"},
{"summary":"   ","due_at":"%s"},
{"summary":"valid one","due_at":"%s"},
{"summary":"valid two","due_at":"%s"},
{"summary":"valid three","due_at":"%s"},
{"summary":"valid four is over the cap","due_at":"%s"}]}`, soon, soon, soon, soon, soon), nil
	}
	n, err := e.ExtractPromises(ctx, msg)
	if err != nil {
		t.Fatal(err)
	}
	got := promisesOf(t, e)
	if n != len(got) || n > 3 {
		t.Fatalf("n=%d stored=%+v", n, got)
	}
	for _, c := range got {
		if !strings.HasPrefix(c.Body, "valid") {
			t.Fatalf("kept a bad promise: %+v", c)
		}
	}
}

func TestExtractPromisesReportsBrokenModelOutput(t *testing.T) {
	e, msg, _ := promiseFixture(t, true)
	e.Generate = func(context.Context, string, string) (string, error) { return "not json", nil }
	if n, err := e.ExtractPromises(context.Background(), msg); err == nil || n != 0 {
		t.Fatalf("n=%d err=%v, want a visible error", n, err)
	}
	// A failed read must not burn the message: it can be retried.
	e.Generate = func(context.Context, string, string) (string, error) {
		return fmt.Sprintf(`{"promises":[{"summary":"Raporu gönder","due_at":"%s"}]}`, time.Now().Add(24*time.Hour).UTC().Format(time.RFC3339)), nil
	}
	if n, err := e.ExtractPromises(context.Background(), msg); err != nil || n != 1 {
		t.Fatalf("retry n=%d err=%v", n, err)
	}
}
