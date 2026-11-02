package autonomy

import (
	"context"
	"fmt"
	"memo/internal/twin"
	"strconv"
	"testing"
	"time"
)

func approvalFixture(t *testing.T) (*twin.Store, twin.Action, time.Time) {
	t.Helper()
	ctx := context.Background()
	s, err := twin.NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	id, err := s.SaveContact(ctx, twin.Contact{Name: "Friend", AutonomyPolicy: "full_auto", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}})
	if err != nil {
		t.Fatal(err)
	}
	p, _ := s.Preferences(ctx)
	p.Autonomy = twin.AutonomySettings{Enabled: true, Mail: true, Mode: "full_auto", GraceSeconds: 60}
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	at := now.Add(time.Minute)
	score := .99
	a := twin.Action{ID: "candidate", ContactID: id, Channel: "mail", Account: "owner", Recipient: "friend@example.com", SourceAddress: "friend@example.com", State: "pending", Mode: "grace_period_auto", Body: "Only evidenced facts.", CreatedAt: now, ExecuteAt: &at, DeliveryVersion: 1, DecisionScore: &score, ScoreStatus: "scored"}
	if ok, err := s.ReserveCandidate(ctx, id, "thread", a.ID, now, true); !ok || err != nil {
		t.Fatal(ok, err)
	}
	if err = s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	a, _ = s.Action(ctx, a.ID)
	return s, a, now
}
func TestApprovalBridgeGraceCancelAndAtomicDelivery(t *testing.T) {
	for _, cancel := range []bool{false, true} {
		t.Run(fmt.Sprint(cancel), func(t *testing.T) {
			ctx := context.Background()
			s, a, now := approvalFixture(t)
			calls := 0
			phase := []string{}
			b := ApprovalBridge{Store: s, Now: func() time.Time { return now }, Notify: func(_ context.Context, _ twin.Action, p string) { phase = append(phase, p) }}
			b.Deliver = func(ctx context.Context, id, mode string) error {
				v, err := s.BeginDelivery(ctx, id, a.Subject, a.Body, mode, now)
				if err != nil {
					return err
				}
				calls++
				return s.FinishMailDelivery(ctx, v, true)
			}
			if err := b.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			if calls != 0 {
				t.Fatal("sent before undo window")
			}
			if cancel {
				if err := s.CancelAction(ctx, a.ID); err != nil {
					t.Fatal(err)
				}
			}
			now = now.Add(time.Minute)
			if err := b.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			if err := b.Tick(ctx); err != nil {
				t.Fatal(err)
			}
			want := 1
			if cancel {
				want = 0
			}
			if calls != want {
				t.Fatalf("deliveries=%d want %d", calls, want)
			}
			if len(phase) == 0 {
				t.Fatal("no pending event")
			}
		})
	}
}
func TestApprovalBridgeRevalidatesCandidate(t *testing.T) {
	for _, scenario := range []string{"disabled", "channel_off", "draft_policy", "never", "score", "group", "version", "overdue", "no_reservation"} {
		t.Run(scenario, func(t *testing.T) {
			ctx := context.Background()
			s, a, now := approvalFixture(t)
			p, _ := s.Preferences(ctx)
			switch scenario {
			case "disabled":
				p.Autonomy.Enabled = false
				if err := s.SavePreferences(ctx, p); err != nil {
					t.Fatal(err)
				}
			case "channel_off":
				p.Autonomy.Mail = false
				if err := s.SavePreferences(ctx, p); err != nil {
					t.Fatal(err)
				}
			case "draft_policy", "never":
				policy := "draft_only"
				if scenario == "never" {
					policy = "never"
				}
				_, err := s.SaveContact(ctx, twin.Contact{ID: a.ContactID, Name: "Friend", AutonomyPolicy: policy, Aliases: []twin.Alias{{Channel: "mail", Address: a.Recipient}}})
				if err != nil {
					t.Fatal(err)
				}
			case "score":
				v := .5
				a.DecisionScore = &v
			case "group":
				a.Group = true
			case "version":
				a.DeliveryVersion = 0
			case "overdue":
				now = now.Add(time.Hour)
			case "no_reservation":
				a.ID = "unreserved"
			}
			if scenario == "score" || scenario == "group" || scenario == "version" || scenario == "no_reservation" {
				if err := s.SaveAction(ctx, a); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := s.BeginDelivery(ctx, a.ID, a.Subject, a.Body, "automatic", now.Add(time.Minute)); err == nil {
				t.Fatal("unauthorized automatic send")
			}
		})
	}
}
func TestApprovalOwnerCommandScopeAmbiguityAndNeverRule(t *testing.T) {
	ctx := context.Background()
	s, a, _ := approvalFixture(t)
	sent := 0
	b := ApprovalBridge{Store: s, Deliver: func(ctx context.Context, id, mode string) error {
		v, err := s.BeginDelivery(ctx, id, a.Subject, a.Body, mode, time.Now())
		if err != nil {
			return err
		}
		sent++
		return s.FinishMailDelivery(ctx, v, true)
	}}
	code := strconv.FormatInt(a.ApprovalCode, 10)
	for _, target := range []string{"telegram:other", "whatsapp:owner"} {
		r, err := b.Control(ctx, target, "#onay "+code)
		if err != nil || r != "choose" || sent != 0 {
			t.Fatal(r, err, sent)
		}
	}
	s.ClaimNotification(ctx, a.ID, "pending", "telegram:owner")
	if r, err := b.Control(ctx, "telegram:owner", "#onay "+code); err != nil || r != "approve" || sent != 1 {
		t.Fatal(r, err, sent)
	}
	if r, _ := b.Control(ctx, "telegram:owner", "#onay "+code); r != "choose" || sent != 1 {
		t.Fatal("duplicate send", r, sent)
	}
	// A rule change after delivery still disables future replies to that person.
	if r, err := b.Control(ctx, "telegram:owner", "#asla "+code); err != nil || r != "never" {
		t.Fatal(r, err)
	}
	contacts, _ := s.Contacts(ctx)
	if contacts[0].AutonomyPolicy != "never" {
		t.Fatal(contacts)
	}
}
func TestApprovalBareCommandCannotChooseBetweenDrafts(t *testing.T) {
	ctx := context.Background()
	s, a, _ := approvalFixture(t)
	b := ApprovalBridge{Store: s}
	s.ClaimNotification(ctx, a.ID, "pending", "telegram:owner")
	a.ID = "second"
	if err := s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	s.ClaimNotification(ctx, a.ID, "pending", "telegram:owner")
	for _, text := range []string{"evet", "gönder", "iptal", "bir daha bu kişiye otomatik dönme"} {
		if r, err := b.Control(ctx, "telegram:owner", text); err != nil || r != "choose" {
			t.Fatal(text, r, err)
		}
	}
}
func TestApprovalParserDoesNotAuthorizeConversation(t *testing.T) {
	for _, text := range []string{"please send the report", "he said #onay 1", "#onay 1\nsend all", "#onay 1 2", "#onay -1"} {
		op, _, handled := ParseApprovalCommand(text)
		if handled && op != "invalid" {
			t.Fatal(text, op)
		}
	}
	for _, text := range []string{"#onay 12", "#approve 12", "#iptal 12", "#never 12", "gönder", "bir daha bu kişiye otomatik dönme"} {
		if _, _, ok := ParseApprovalCommand(text); !ok {
			t.Fatal(text)
		}
	}
}
