package twin

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestApprovalCancellationWinsBeforeReservationAndSurvivesLatePreparation(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := deliveryFixture(t, s)
	if err = s.CancelAction(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	if err = s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	if v, _ := s.Action(ctx, a.ID); v.State != "cancelled" {
		t.Fatal(v)
	}
	if _, err = s.BeginMailDelivery(ctx, a.ID, a.Subject, a.Body); err != ErrDeliveryConflict {
		t.Fatal(err)
	}
}

// DT6-3: cancelling used to flip only the state column, so the payload kept
// reason "drafted" and the UI described a cancelled reply as "prepared".
func TestCancelActionRecordsOwnerCancellationReason(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := deliveryFixture(t, s)
	if err = s.CancelAction(ctx, a.ID); err != nil {
		t.Fatal(err)
	}
	v, err := s.Action(ctx, a.ID)
	if err != nil {
		t.Fatal(err)
	}
	if v.State != "cancelled" || v.Reason != "user_cancelled" {
		t.Fatalf("state=%q reason=%q", v.State, v.Reason)
	}
	all, err := s.Actions(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range all {
		if x.ID == a.ID && (x.State != "cancelled" || x.Reason != "user_cancelled") {
			t.Fatalf("listed state=%q reason=%q", x.State, x.Reason)
		}
	}
}

// Rows cancelled before DT6-3 was fixed still say "drafted"; opening the store
// repairs them so existing history reads correctly too.
func TestStoreOpenRepairsLegacyCancelledReason(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	legacy := Action{ID: "mail:legacy", Channel: "mail", Account: "a", Recipient: "x@example.com", State: "cancelled", Reason: "drafted", Mode: "awaiting_approval"}
	if err = s.SaveAction(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	failed := Action{ID: "mail:failed", Channel: "mail", Account: "a", Recipient: "y@example.com", State: "failed", Reason: "reply_timeout", Mode: "awaiting_approval"}
	if err = s.SaveAction(ctx, failed); err != nil {
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
	if v, _ := s.Action(ctx, "mail:legacy"); v.State != "cancelled" || v.Reason != "user_cancelled" {
		t.Fatalf("legacy state=%q reason=%q", v.State, v.Reason)
	}
	if v, _ := s.Action(ctx, "mail:failed"); v.Reason != "reply_timeout" {
		t.Fatalf("unrelated row changed: %q", v.Reason)
	}
}

// DT6-1: the dedup claim and the first visible record must commit together, and
// a preparation interrupted by a restart must surface as reviewable, never as
// an invisible claimed event that is silently dropped or replayed.
func TestClaimIncomingActionIsVisibleAndInterruptedOnRestart(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := Action{ID: "ev-1", Channel: "mail", Account: "a", Recipient: "x@example.com", State: "preparing", Reason: "preparing", Stage: "analyzing"}
	bad := a
	bad.State = "pending"
	if _, err = s.ClaimIncomingAction(ctx, bad); err != ErrInvalid {
		t.Fatalf("non-preparing claim err=%v, want ErrInvalid", err)
	}
	if ok, err := s.ClaimIncomingAction(ctx, a); err != nil || !ok {
		t.Fatalf("first claim ok=%v err=%v", ok, err)
	}
	if ok, err := s.ClaimIncomingAction(ctx, a); err != nil || ok {
		t.Fatalf("duplicate claim ok=%v err=%v, want false", ok, err)
	}
	if v, err := s.Action(ctx, "ev-1"); err != nil || v.State != "preparing" {
		t.Fatalf("claimed event not visible: %+v err=%v", v, err)
	}
	if err = s.Close(); err != nil {
		t.Fatal(err)
	}
	s, err = NewStore(dir) // process restarted mid-preparation
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	v, err := s.Action(ctx, "ev-1")
	if err != nil || v.State != "failed" || v.Reason != "reply_cancelled" {
		t.Fatalf("after restart: %+v err=%v", v, err)
	}
	if ok, err := s.ClaimIncoming(ctx, "ev-1"); err != nil || ok {
		t.Fatalf("interrupted event must never be replayed: ok=%v err=%v", ok, err)
	}
}
func TestApprovalCancellationAndParallelReservationsHaveOneWinner(t *testing.T) {
	ctx := context.Background()
	s, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	a := deliveryFixture(t, s)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for range 4 {
		wg.Go(func() {
			if _, err := s.BeginMailDelivery(ctx, a.ID, a.Subject, a.Body); err == nil {
				wins.Add(1)
			}
		})
	}
	wg.Go(func() {
		if err := s.CancelAction(ctx, a.ID); err == nil {
			wins.Add(1)
		}
	})
	wg.Wait()
	if wins.Load() != 1 {
		t.Fatal(wins.Load())
	}
}
func TestApprovalNotificationPersistenceQueueBeyondUIAndPause(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	s, err := NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	a := deliveryFixture(t, s)
	at := time.Now().Add(time.Minute)
	a.Mode = "grace_period_auto"
	a.ExecuteAt = &at
	if err = s.SaveAction(ctx, a); err != nil {
		t.Fatal(err)
	}
	for i := range 101 {
		v := a
		v.ID = fmt.Sprint("later-", i)
		v.Mode = "awaiting_approval"
		v.ExecuteAt = nil
		if err = s.SaveAction(ctx, v); err != nil {
			t.Fatal(err)
		}
	}
	if v, err := s.ApprovalActions(ctx); err != nil || len(v) != 102 {
		t.Fatal(len(v), err)
	}
	if ok, err := s.ClaimNotification(ctx, a.ID, "pending", "telegram:owner"); !ok || err != nil {
		t.Fatal(ok, err)
	}
	// Any settings edit pauses existing countdowns in the same transaction.
	p, _ := s.Preferences(ctx)
	if err = s.SavePreferences(ctx, p); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s, err = NewStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	got, _ := s.Action(ctx, a.ID)
	if got.ExecuteAt != nil || got.Mode != "awaiting_approval" || got.ApprovalCode == 0 {
		t.Fatal(got)
	}
	if ok, err := s.ClaimNotification(ctx, a.ID, "pending", "telegram:owner"); ok || err != nil {
		t.Fatal(ok, err)
	}
}
