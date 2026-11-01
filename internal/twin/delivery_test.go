package twin

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func deliveryFixture(t *testing.T, s *Store) Action {
	t.Helper()
	ctx := context.Background()
	id, err := s.SaveContact(ctx, Contact{Name: "Friend", AutonomyPolicy: "draft_only", Aliases: []Alias{{Channel: "mail", Address: "friend@example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	a := Action{ID: "draft", ContactID: id, Channel: "mail", Account: "owner", Recipient: "friend@example.com", Subject: "Meeting", Body: "Original reply", InReplyTo: "<original@example.com>", State: "pending", CreatedAt: time.Now().UTC()}
	if err = s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	return a
}

func TestOwnerDeliveryOnceAndPersistedOutcome(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := deliveryFixture(t, s)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			_, err := s.BeginMailDelivery(ctx, a.ID, "Re: Meeting", "Owner edited reply")
			if err == nil {
				wins.Add(1)
			} else if !errors.Is(err, ErrDeliveryConflict) {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatalf("submissions=%d", wins.Load())
	}
	got, err := s.Action(ctx, a.ID)
	if err != nil || got.Body != "Owner edited reply" || got.InReplyTo != a.InReplyTo || got.State != "sending" {
		t.Fatal(got, err)
	}
	// Late preparation cannot overwrite a reserved send.
	if err = s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	if err = s.FinishMailDelivery(ctx, got, true); err != nil {
		t.Fatal(err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err = s.Action(ctx, a.ID)
	if err != nil || got.State != "sent" || got.SentAt == nil {
		t.Fatal(got, err)
	}
	if _, err = s.BeginMailDelivery(ctx, a.ID, "Re: Meeting", "again"); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatal(err)
	}
}
func TestOwnerDeliveryInterruptedCannotReplay(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := deliveryFixture(t, s)
	if _, err = s.BeginMailDelivery(ctx, a.ID, "Re: Meeting", "edit"); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, err := s.Action(ctx, a.ID)
	if err != nil || got.State != "send_uncertain" {
		t.Fatal(got, err)
	}
	if _, err = s.BeginMailDelivery(ctx, a.ID, "Re: Meeting", "edit"); !errors.Is(err, ErrDeliveryConflict) {
		t.Fatal(err)
	}
}
func TestOwnerDeliveryRechecksContactAndLegacySentEvidence(t *testing.T) {
	for _, scenario := range []string{"never", "history", "cancelled", "header"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, err := NewStore(t.TempDir())
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			a := deliveryFixture(t, s)
			want := ErrDeliveryDenied
			subject := "Re: Meeting"
			switch scenario {
			case "never":
				_, err = s.SaveContact(ctx, Contact{ID: a.ContactID, Name: "Friend", AutonomyPolicy: "never", Aliases: []Alias{{Channel: "mail", Address: a.Recipient}}})
			case "history":
				err = s.RecordMessage(ctx, Message{Channel: "mail", Account: a.Account, Address: a.Recipient, EventID: "sent", Outgoing: true, Body: a.Body, SentAt: time.Now().UTC()}, "Friend")
				want = ErrSentHistory
			case "cancelled":
				err = s.CancelAction(ctx, a.ID)
				want = ErrDeliveryConflict
			case "header":
				subject = "Re: Meeting\r\nBcc: other@example.com"
				want = ErrInvalid
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = s.BeginMailDelivery(ctx, a.ID, subject, "owner edits cannot bypass history"); !errors.Is(err, want) {
				t.Fatalf("got %v want %v", err, want)
			}
		})
	}
}
