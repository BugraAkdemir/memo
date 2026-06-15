// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// TestRecordIntentHabitTime verifies that a declared habit time is what the
// analyzer sees — not the time the message happened to be sent. This is the
// core promise of the feature: "her gün 21:00" must be learned as 21:00.
func TestRecordIntentHabitTime(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	rec := NewRecorder(store)

	// Message sent at 14:30, but the declared habit is at 21:00.
	msgTime := time.Date(2026, 6, 15, 14, 30, 0, 0, time.Local)
	habit := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)

	rec.RecordIntent("Her gün 21:00'de kod yazacak", "chat", "", true, &habit, msgTime)

	// Recording is async; wait for it to land.
	waitForCount(t, store, ActivityIntent, 1)

	obs, err := store.Query(context.Background(), QueryFilter{ActivityType: ActivityIntent})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 intent observation, got %d", len(obs))
	}

	wantSeconds := 21 * 3600
	if obs[0].TimeOfDaySeconds != wantSeconds {
		t.Errorf("TimeOfDaySeconds = %d (%.1fh), want %d (21h) — habit time was lost",
			obs[0].TimeOfDaySeconds, float64(obs[0].TimeOfDaySeconds)/3600, wantSeconds)
	}
}

// TestRecordIntentNonHabitUsesMsgTime verifies a non-habit intent keeps the
// message timestamp (no time rewriting).
func TestRecordIntentNonHabitUsesMsgTime(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	rec := NewRecorder(store)
	msgTime := time.Date(2026, 6, 15, 9, 15, 0, 0, time.Local)

	rec.RecordIntent("Yarın halısaha", "whatsapp", "Ahmet", false, nil, msgTime)
	waitForCount(t, store, ActivityIntent, 1)

	obs, err := store.Query(context.Background(), QueryFilter{ActivityType: ActivityIntent})
	if err != nil {
		t.Fatal(err)
	}
	if len(obs) != 1 {
		t.Fatalf("expected 1 observation, got %d", len(obs))
	}
	wantSeconds := 9*3600 + 15*60
	if obs[0].TimeOfDaySeconds != wantSeconds {
		t.Errorf("TimeOfDaySeconds = %d, want %d", obs[0].TimeOfDaySeconds, wantSeconds)
	}

	// Metadata should carry the summary, source, and contact.
	var meta IntentMeta
	if err := json.Unmarshal([]byte(obs[0].Metadata), &meta); err != nil {
		t.Fatalf("metadata not valid JSON: %v", err)
	}
	if meta.Source != "whatsapp" || meta.ContactName != "Ahmet" {
		t.Errorf("metadata = %+v, want source=whatsapp contact=Ahmet", meta)
	}
}

func waitForCount(t *testing.T, s *Store, activity string, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		obs, err := s.Query(context.Background(), QueryFilter{ActivityType: activity})
		if err == nil && len(obs) >= want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for %d %q observations", want, activity)
}
