// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/database"
)

// The daily telemetry summary is built from these rows, so two promises live
// here: a Minimal Mode turn is marked private and NEVER comes back from
// EventsBetween, and the time to first token survives the round trip.

func TestEventsBetween_ReturnsOnlyNonPrivateEventsInTheRange(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	day := time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC)
	rec := func(ts time.Time, e Event) {
		t.Helper()
		e.Timestamp = ts
		if err := store.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}
	rec(day.Add(3*time.Hour), Event{Provider: "local", Model: "qwen3-8b.gguf", PromptTokens: 100, CompletionTokens: 40, TokensPerSecond: 31, TTFTMs: 640})
	rec(day.Add(5*time.Hour), Event{Provider: "openai", Model: "gpt-5.5", Category: "agent", PromptTokens: 900, CachedPromptTokens: 800, CompletionTokens: 10})
	rec(day.Add(6*time.Hour), Event{Provider: "openai", Model: "gpt-5.5", PromptTokens: 5, CompletionTokens: 5, Private: true}) // Minimal Mode turn
	rec(day.Add(-time.Second), Event{Provider: "local", Model: "before"})                                                       // the day before
	rec(day.Add(24*time.Hour), Event{Provider: "local", Model: "after"})                                                         // the day after (to is exclusive)

	got, err := store.EventsBetween(ctx, day, day.Add(24*time.Hour))
	if err != nil {
		t.Fatalf("EventsBetween: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (the private one and the neighbouring days must be left out): %+v", len(got), got)
	}
	for _, e := range got {
		if e.Private {
			t.Errorf("a private event came back: %+v", e)
		}
	}
	if got[0].Model != "qwen3-8b.gguf" || got[0].TTFTMs != 640 || got[0].TokensPerSecond != 31 {
		t.Errorf("first event lost its fields: %+v", got[0])
	}
	if got[1].Category != "agent" || got[1].CachedPromptTokens != 800 {
		t.Errorf("second event lost its fields: %+v", got[1])
	}
}

func TestMigrateColumns_AddsPrivateAndTTFTToAnOldTable(t *testing.T) {
	dir := t.TempDir()
	// A usage.db exactly as an older Memo left it: no category, cache, private or ttft columns.
	db, err := database.Open(database.Config{Path: filepath.Join(dir, "usage.db"), MaxPool: 1})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `CREATE TABLE usage_events (
		id INTEGER PRIMARY KEY AUTOINCREMENT, ts INTEGER NOT NULL, provider TEXT NOT NULL, model TEXT NOT NULL,
		prompt_tokens INTEGER NOT NULL DEFAULT 0, completion_tokens INTEGER NOT NULL DEFAULT 0,
		duration_secs REAL NOT NULL DEFAULT 0, tokens_per_second REAL NOT NULL DEFAULT 0)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(context.Background(), `INSERT INTO usage_events (ts, provider, model, prompt_tokens) VALUES (?, 'local', 'old', 7)`, time.Date(2026, 10, 26, 9, 0, 0, 0, time.UTC).Unix()); err != nil {
		t.Fatal(err)
	}
	db.Close()

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore on the old table: %v", err)
	}
	defer store.Close()

	got, err := store.EventsBetween(context.Background(), time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), time.Date(2026, 10, 27, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("EventsBetween: %v", err)
	}
	if len(got) != 1 || got[0].Model != "old" || got[0].PromptTokens != 7 || got[0].Private || got[0].TTFTMs != 0 {
		t.Fatalf("the old row must survive the migration unchanged, as a non-private event: %+v", got)
	}
}
