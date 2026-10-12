package calendar

import (
	"context"
	"testing"
	"time"
)

func TestStoreChangesFollowCommittedWrites(t *testing.T) {
	s, cleanup := newTestStore(t)
	defer cleanup()
	ch, cancel := s.SubscribeChanges()
	defer cancel()
	<-ch
	e, err := s.Add(context.Background(), Event{Title: "first", StartTime: time.Now().Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("add did not notify")
	}
	if _, err := s.Add(context.Background(), e); err == nil {
		t.Fatal("duplicate fixture accepted")
	}
	select {
	case <-ch:
		t.Fatal("failed write notified success")
	default:
	}
	if err := s.Delete(context.Background(), e.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-ch:
	case <-time.After(time.Second):
		t.Fatal("delete did not notify")
	}
	for range 100 {
		s.NotifyChanges()
	}
	<-ch
	select {
	case <-ch:
		t.Fatal("notifications not coalesced")
	default:
	}
	cancel()
	s.NotifyChanges()
	select {
	case <-ch:
		t.Fatal("unsubscribed listener notified")
	default:
	}
}
