// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"os"
	"testing"
	"time"
)

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "calendar-test-*")
	if err != nil {
		t.Fatal(err)
	}
	s, err := NewStore(dir)
	if err != nil {
		os.RemoveAll(dir)
		t.Fatal(err)
	}
	return s, func() {
		s.Close()
		os.RemoveAll(dir)
	}
}

func TestStoreAddAndList(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	e := Event{
		Title:     "Halısaha",
		StartTime: now.Add(2 * time.Hour),
		Source:    SourceWhatsApp,
	}
	added, err := store.Add(ctx, e)
	if err != nil {
		t.Fatal(err)
	}
	if added.ID == "" {
		t.Error("expected non-empty ID")
	}

	events, err := store.List(ctx, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "Halısaha" {
		t.Errorf("Title = %q, want Halısaha", events[0].Title)
	}
}

func TestStoreDelete(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	e, err := store.Add(ctx, Event{Title: "Test", StartTime: now.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.Delete(ctx, e.ID); err != nil {
		t.Fatal(err)
	}

	events, err := store.List(ctx, now, now.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 events after delete, got %d", len(events))
	}
}

func TestStorePendingReminders(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Event in 20 minutes — should appear with 30-min lead.
	soon := Event{Title: "Soon", StartTime: now.Add(20 * time.Minute)}
	// Event in 60 minutes — should NOT appear with 30-min lead.
	later := Event{Title: "Later", StartTime: now.Add(60 * time.Minute)}
	// Past event — should NOT appear.
	past := Event{Title: "Past", StartTime: now.Add(-10 * time.Minute)}

	for _, e := range []Event{soon, later, past} {
		if _, err := store.Add(ctx, e); err != nil {
			t.Fatal(err)
		}
	}

	pending, err := store.PendingReminders(ctx, now, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 1 {
		t.Fatalf("expected 1 pending reminder, got %d", len(pending))
	}
	if pending[0].Title != "Soon" {
		t.Errorf("wrong event: %q", pending[0].Title)
	}
}

func TestStoreMarkReminderSent(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	e, err := store.Add(ctx, Event{Title: "Remind", StartTime: now.Add(10 * time.Minute)})
	if err != nil {
		t.Fatal(err)
	}

	if err := store.MarkReminderSent(ctx, e.ID); err != nil {
		t.Fatal(err)
	}

	// Should no longer appear as pending.
	pending, err := store.PendingReminders(ctx, now, 30)
	if err != nil {
		t.Fatal(err)
	}
	if len(pending) != 0 {
		t.Errorf("expected 0 pending after mark, got %d", len(pending))
	}
}

func TestReminderLoopEmits(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	ctx := context.Background()
	now := time.Now()

	// Add an event 10 minutes from now (within 30-min lead).
	if _, err := store.Add(ctx, Event{
		Title:     "GymTime",
		StartTime: now.Add(10 * time.Minute),
	}); err != nil {
		t.Fatal(err)
	}

	var emitted []string
	loop := NewReminderLoop(store, func() int { return 30 }, func(name, data string) {
		emitted = append(emitted, name)
	})

	loop.tick(ctx, now)

	if len(emitted) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(emitted))
	}
	if emitted[0] != "calendar:reminder" {
		t.Errorf("wrong event name: %q", emitted[0])
	}

	// Second tick must not re-emit (reminder_sent=1).
	loop.tick(ctx, now)
	if len(emitted) != 1 {
		t.Errorf("expected no second emission, got %d total", len(emitted))
	}
}
