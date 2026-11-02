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
