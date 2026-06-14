// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"context"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := NewStore(StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("NewStore() error = %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestRecordFillsDerivedFields(t *testing.T) {
	s := newTestStore(t)

	// Monday 2026-06-15 at 14:30:15 local time.
	ts := time.Date(2026, 6, 15, 14, 30, 15, 0, time.Local)
	id, err := s.Record(Observation{Timestamp: ts, ActivityType: ActivityChat, Topic: "coding"})
	if err != nil {
		t.Fatalf("Record() error = %v", err)
	}
	if id == 0 {
		t.Fatal("Record() returned id 0")
	}

	got, err := s.Query(context.Background(), QueryFilter{})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("Query() len = %d, want 1", len(got))
	}
	obs := got[0]
	if want := int(time.Monday); obs.DayOfWeek != want {
		t.Errorf("DayOfWeek = %d, want %d", obs.DayOfWeek, want)
	}
	if want := 14*3600 + 30*60 + 15; obs.TimeOfDaySeconds != want {
		t.Errorf("TimeOfDaySeconds = %d, want %d", obs.TimeOfDaySeconds, want)
	}
	if obs.Topic != "coding" {
		t.Errorf("Topic = %q, want coding", obs.Topic)
	}
}

func TestQueryFilterByActivityAndSince(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()

	mustRecord(t, s, Observation{Timestamp: now.Add(-48 * time.Hour), ActivityType: ActivityChat})
	mustRecord(t, s, Observation{Timestamp: now.Add(-1 * time.Hour), ActivityType: ActivityChat})
	mustRecord(t, s, Observation{Timestamp: now.Add(-1 * time.Hour), ActivityType: ActivityAgent})

	chat, err := s.Query(context.Background(), QueryFilter{
		ActivityType: ActivityChat,
		Since:        now.Add(-24 * time.Hour),
	})
	if err != nil {
		t.Fatalf("Query() error = %v", err)
	}
	if len(chat) != 1 {
		t.Fatalf("filtered Query() len = %d, want 1", len(chat))
	}
	if chat[0].ActivityType != ActivityChat {
		t.Errorf("ActivityType = %q, want chat", chat[0].ActivityType)
	}
}

func TestPruneDropsOldRows(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	mustRecord(t, s, Observation{Timestamp: now.Add(-40 * 24 * time.Hour), ActivityType: ActivityChat})
	mustRecord(t, s, Observation{Timestamp: now, ActivityType: ActivityChat})

	deleted, err := s.Prune(now.Add(-30 * 24 * time.Hour))
	if err != nil {
		t.Fatalf("Prune() error = %v", err)
	}
	if deleted != 1 {
		t.Errorf("Prune() deleted = %d, want 1", deleted)
	}
	n, _ := s.Count(context.Background())
	if n != 1 {
		t.Errorf("Count() = %d, want 1", n)
	}
}

func TestClassifyTopic(t *testing.T) {
	cases := map[string]string{
		"bu fonksiyonda bir bug var, kodu düzelt": "coding",
		"bana bu makaleyi özetle ve çevir":        "writing",
		"yapay zeka nedir, açıkla":                "research",
		"yarınki toplantı için bir plan yap":      "planning",
		"merhaba, teşekkür ederim":                "general",
		"":                                        "general",
	}
	for in, want := range cases {
		if got := ClassifyTopic(in); got != want {
			t.Errorf("ClassifyTopic(%q) = %q, want %q", in, got, want)
		}
	}
}

func mustRecord(t *testing.T, s *Store, obs Observation) {
	t.Helper()
	if _, err := s.Record(obs); err != nil {
		t.Fatalf("Record() error = %v", err)
	}
}
