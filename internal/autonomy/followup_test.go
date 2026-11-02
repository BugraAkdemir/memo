package autonomy

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"memo/internal/twin"
)

// followUpFixture opens a radar with a 1 hour wait and records one sent
// question to the fixture contact, then moves the engine clock past the due time.
func followUpFixture(t *testing.T) (*Engine, *int, *[]twin.Action) {
	t.Helper()
	e, in := engineFixture(t)
	ctx := context.Background()
	p, err := e.Store.Preferences(ctx)
	if err != nil {
		t.Fatal(err)
	}
	p.Radar = twin.RadarSettings{Enabled: true, FollowUpHours: 1}
	if err = e.Store.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	sent := time.Now().Add(-10 * time.Minute)
	m := twin.Message{Channel: "mail", Account: "account", EventID: "out-1", Address: in.Sender, Outgoing: true, Body: "Fiyat teklifini inceledin mi?", SentAt: sent, Subject: "Teklif", InReplyTo: "<m1@example.com>"}
	if err = e.Store.RecordMessage(ctx, m, ""); err != nil {
		t.Fatal(err)
	}
	e.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	calls := 0
	e.Generate = func(_ context.Context, system, user string) (string, error) {
		calls++
		if system != followUpPrompt {
			t.Errorf("unexpected prompt: %.60s", system)
		}
		if !strings.Contains(user, "Fiyat teklifini inceledin mi?") {
			t.Errorf("the owner's original message is missing: %s", user)
		}
		return `{"body":"Merhaba, teklifle ilgili bir geri dönüşün olursa sevinirim."}`, nil
	}
	saved := []twin.Action{}
	e.SaveDraft = func(_ context.Context, a twin.Action) error { saved = append(saved, a); return nil }
	return e, &calls, &saved
}

func followUpActions(t *testing.T, e *Engine) []twin.Action {
	t.Helper()
	all, err := e.Store.Actions(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	out := []twin.Action{}
	for _, a := range all {
		if a.Reason == "follow_up" || strings.HasPrefix(a.Reason, "follow_up") {
			out = append(out, a)
		}
	}
	return out
}

func TestFollowUpPreparesReviewOnlyDraftOnce(t *testing.T) {
	e, calls, saved := followUpFixture(t)
	ctx := context.Background()
	n, err := e.PrepareFollowUps(ctx)
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	got := followUpActions(t, e)
	if len(got) != 1 {
		t.Fatalf("actions=%+v", got)
	}
	a := got[0]
	if a.State != "pending" || a.Mode != "awaiting_approval" || a.ExecuteAt != nil || a.Channel != "mail" || a.Recipient != "friend@example.com" || a.SourceAddress != "friend@example.com" || a.Subject != "Teklif" || a.InReplyTo != "<m1@example.com>" || a.DeliveryVersion != 1 || a.ContactID == "" || !a.DraftSaved || a.Body == "" {
		t.Fatalf("not a review-only mail draft: %+v", a)
	}
	if len(*saved) != 1 || *calls != 1 {
		t.Fatalf("saved=%d generated=%d", len(*saved), *calls)
	}
	open, _ := e.Store.Commitments(ctx)
	if len(open) != 1 || open[0].Status != "reminded" {
		t.Fatalf("commitment should be marked reminded: %+v", open)
	}
	// A second scan never prepares the same reminder again.
	if n, err = e.PrepareFollowUps(ctx); err != nil || n != 0 || *calls != 1 || len(followUpActions(t, e)) != 1 {
		t.Fatalf("repeat n=%d err=%v calls=%d", n, err, *calls)
	}
}

func TestFollowUpDoesNothingWhenNotDueOrRadarOff(t *testing.T) {
	e, calls, _ := followUpFixture(t)
	ctx := context.Background()
	e.Now = func() time.Time { return time.Now() } // the hour has not passed
	if n, err := e.PrepareFollowUps(ctx); err != nil || n != 0 || *calls != 0 {
		t.Fatalf("not due: n=%d err=%v calls=%d", n, err, *calls)
	}
	e.Now = func() time.Time { return time.Now().Add(2 * time.Hour) }
	p, _ := e.Store.Preferences(ctx)
	p.Radar.Enabled = false
	if err := e.Store.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PrepareFollowUps(ctx); err != nil || n != 0 || *calls != 0 {
		t.Fatalf("radar off: n=%d err=%v calls=%d", n, err, *calls)
	}
	if open, _ := e.Store.Commitments(ctx); len(open) != 1 || open[0].Status != "open" {
		t.Fatalf("turning the radar off must not consume the commitment: %+v", open)
	}
}

func TestFollowUpSkippedWhenContactAnsweredFirst(t *testing.T) {
	e, calls, _ := followUpFixture(t)
	ctx := context.Background()
	reply := twin.Message{Channel: "mail", Account: "account", EventID: "in-1", Address: "friend@example.com", Body: "Baktım, uygun.", SentAt: time.Now()}
	if err := e.Store.RecordMessage(ctx, reply, ""); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PrepareFollowUps(ctx); err != nil || n != 0 || *calls != 0 {
		t.Fatalf("n=%d err=%v calls=%d", n, err, *calls)
	}
}

func TestFollowUpNeverForContactThatForbidsReplies(t *testing.T) {
	e, calls, saved := followUpFixture(t)
	ctx := context.Background()
	open, _ := e.Store.Commitments(ctx)
	if _, err := e.Store.SaveContact(ctx, twin.Contact{ID: open[0].ContactID, Name: "Friend", AutonomyPolicy: "never", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}}); err != nil {
		t.Fatal(err)
	}
	if n, err := e.PrepareFollowUps(ctx); err != nil || n != 0 || *calls != 0 || len(*saved) != 0 {
		t.Fatalf("n=%d err=%v calls=%d saved=%d", n, err, *calls, len(*saved))
	}
	if left, _ := e.Store.Commitments(ctx); len(left) != 0 {
		t.Fatalf("a forbidden contact must leave no pending reminder: %+v", left)
	}
}

func TestFollowUpFailuresAreVisibleAndNeverRetriedBlindly(t *testing.T) {
	cases := []struct {
		name   string
		gen    func(context.Context, string, string) (string, error)
		save   func(context.Context, twin.Action) error
		reason string
		state  string
		drafts int
	}{
		{"provider error", func(context.Context, string, string) (string, error) { return "", errors.New("boom") }, nil, "reply_provider_failed", "failed", 0},
		{"invalid json", func(context.Context, string, string) (string, error) { return "not json", nil }, nil, "reply_invalid_response", "failed", 0},
		{"empty body", func(context.Context, string, string) (string, error) { return `{"body":"  "}`, nil }, nil, "reply_invalid_response", "failed", 0},
		{"secret in draft", func(context.Context, string, string) (string, error) {
			return `{"body":"Anahtarım sk-` + strings.Repeat("a", 40) + `"}`, nil
		}, nil, "secret_api_key", "blocked", 0},
		{"drafts upload fails", nil, func(context.Context, twin.Action) error { return errors.New("imap down") }, "draft_upload_failed", "failed", 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e, _, saved := followUpFixture(t)
			if tc.gen != nil {
				e.Generate = tc.gen
			}
			if tc.save != nil {
				e.SaveDraft = func(ctx context.Context, a twin.Action) error { *saved = append(*saved, a); return tc.save(ctx, a) }
			}
			ctx := context.Background()
			if _, err := e.PrepareFollowUps(ctx); err != nil {
				t.Fatal(err)
			}
			got, err := e.Store.Actions(ctx)
			if err != nil {
				t.Fatal(err)
			}
			if len(got) != 1 || got[0].State != tc.state || got[0].Reason != tc.reason {
				t.Fatalf("want %s/%s got %+v", tc.state, tc.reason, got)
			}
			if len(*saved) != tc.drafts {
				t.Fatalf("drafts uploaded=%d want %d", len(*saved), tc.drafts)
			}
			if open, _ := e.Store.Commitments(ctx); len(open) != 1 || open[0].Status != "reminded" {
				t.Fatalf("a failed attempt must not stay open and loop: %+v", open)
			}
			// No retry on the next scan.
			if n, _ := e.PrepareFollowUps(ctx); n != 0 {
				t.Fatalf("failed reminder was attempted again (n=%d)", n)
			}
		})
	}
}

func TestFollowUpWaitsWhenNoReplyModelIsConfigured(t *testing.T) {
	e, _, _ := followUpFixture(t)
	ctx := context.Background()
	e.Generate = nil
	if n, err := e.PrepareFollowUps(ctx); err != nil || n != 0 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if open, _ := e.Store.Commitments(ctx); len(open) != 1 || open[0].Status != "open" {
		t.Fatalf("without a model the reminder must wait: %+v", open)
	}
	if len(followUpActions(t, e)) != 0 {
		t.Fatal("no action should exist without a model")
	}
}

// The reminder is never eligible for the automatic path, even when the clock
// is exactly at a would-be execution time and autonomy is fully enabled.
func TestFollowUpDraftIsNeverAutomaticallyDelivered(t *testing.T) {
	e, _, _ := followUpFixture(t)
	ctx := context.Background()
	if _, err := e.PrepareFollowUps(ctx); err != nil {
		t.Fatal(err)
	}
	a := followUpActions(t, e)[0]
	if _, err := e.Store.BeginDelivery(ctx, a.ID, a.Subject, a.Body, "automatic", time.Now()); !errors.Is(err, twin.ErrDeliveryDenied) {
		t.Fatalf("automatic delivery of a follow-up was not refused: %v", err)
	}
	if v, _ := e.Store.Action(ctx, a.ID); v.State != "pending" {
		t.Fatalf("a refused delivery must leave the draft pending: %s", v.State)
	}
}

// ...but the owner can approve it through the normal path, exactly once.
func TestFollowUpDraftIsOwnerApprovable(t *testing.T) {
	e, _, _ := followUpFixture(t)
	ctx := context.Background()
	if _, err := e.PrepareFollowUps(ctx); err != nil {
		t.Fatal(err)
	}
	a := followUpActions(t, e)[0]
	if _, err := e.Store.BeginMailDelivery(ctx, a.ID, a.Subject, a.Body); err != nil {
		t.Fatalf("owner approval refused: %v", err)
	}
	if _, err := e.Store.BeginMailDelivery(ctx, a.ID, a.Subject, a.Body); err == nil {
		t.Fatal("a second approval of the same reminder was accepted")
	}
}
