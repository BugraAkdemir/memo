// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/database"
)

func newTestStore(t *testing.T) (*Store, func()) {
	t.Helper()
	dir, err := os.MkdirTemp("", "stats-test-*")
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

func TestRecordEventAndSummary_Totals(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	events := []Event{
		{Provider: "openai", Model: "gpt-4o", PromptTokens: 100, CompletionTokens: 50, DurationSecs: 2, TokensPerSecond: 25},
		{Provider: "openai", Model: "gpt-4o", PromptTokens: 200, CompletionTokens: 80, DurationSecs: 4, TokensPerSecond: 20},
		{Provider: "local", Model: "llama-3-8b", PromptTokens: 50, CompletionTokens: 30, DurationSecs: 1, TokensPerSecond: 30},
	}
	for _, e := range events {
		if err := store.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	sum, err := store.Summary(ctx, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if sum.TotalRequests != 3 {
		t.Errorf("TotalRequests = %d, want 3", sum.TotalRequests)
	}
	if sum.TotalPromptTokens != 350 {
		t.Errorf("TotalPromptTokens = %d, want 350", sum.TotalPromptTokens)
	}
	if sum.TotalCompletionTokens != 160 {
		t.Errorf("TotalCompletionTokens = %d, want 160", sum.TotalCompletionTokens)
	}
	wantAvgTps := (25.0 + 20.0 + 30.0) / 3.0
	if diff := sum.AvgTokensPerSecond - wantAvgTps; diff > 0.01 || diff < -0.01 {
		t.Errorf("AvgTokensPerSecond = %v, want %v", sum.AvgTokensPerSecond, wantAvgTps)
	}
	if sum.MostUsedModel != "gpt-4o" {
		t.Errorf("MostUsedModel = %q, want gpt-4o", sum.MostUsedModel)
	}
	if sum.MostUsedModelRequests != 2 {
		t.Errorf("MostUsedModelRequests = %d, want 2", sum.MostUsedModelRequests)
	}
	if len(sum.ModelBreakdown) != 2 {
		t.Fatalf("ModelBreakdown len = %d, want 2", len(sum.ModelBreakdown))
	}
	if len(sum.Daily) != 1 {
		t.Fatalf("Daily len = %d, want 1 (all events recorded today)", len(sum.Daily))
	}
	if sum.Daily[0].Requests != 3 {
		t.Errorf("Daily[0].Requests = %d, want 3", sum.Daily[0].Requests)
	}
}

func TestSummary_SinceFiltersOldEvents(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	old := Event{Timestamp: time.Now().Add(-48 * time.Hour), Provider: "openai", Model: "gpt-4o", PromptTokens: 10, CompletionTokens: 10, TokensPerSecond: 10}
	recent := Event{Timestamp: time.Now(), Provider: "openai", Model: "gpt-4o", PromptTokens: 20, CompletionTokens: 20, TokensPerSecond: 20}
	if err := store.RecordEvent(ctx, old); err != nil {
		t.Fatal(err)
	}
	if err := store.RecordEvent(ctx, recent); err != nil {
		t.Fatal(err)
	}

	sum, err := store.Summary(ctx, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 1 {
		t.Errorf("TotalRequests = %d, want 1 (old event should be excluded)", sum.TotalRequests)
	}
	if sum.TotalPromptTokens != 20 {
		t.Errorf("TotalPromptTokens = %d, want 20", sum.TotalPromptTokens)
	}
}

func TestRecordEvent_DefaultsEmptyCategoryToChat(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	if err := store.RecordEvent(ctx, Event{Provider: "local", Model: "llama-3-8b", PromptTokens: 10, CompletionTokens: 5}); err != nil {
		t.Fatalf("RecordEvent: %v", err)
	}

	sum, err := store.Summary(ctx, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if len(sum.CategoryBreakdown) != 1 || sum.CategoryBreakdown[0].Category != "chat" {
		t.Fatalf("CategoryBreakdown = %+v, want a single 'chat' entry", sum.CategoryBreakdown)
	}
}

// TestSummary_CategoryBreakdown covers the actual feature this column
// exists for: telling apart how many tokens went to the user-facing chat
// reply versus every background call (Dream, fact extraction, ...) — and
// that the ranking is by total tokens spent, not request count, since a
// handful of big Dream batches should outrank many tiny title-generation
// calls even with far fewer requests.
func TestSummary_CategoryBreakdown(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	events := []Event{
		{Provider: "local", Model: "m", Category: "chat", PromptTokens: 500, CompletionTokens: 200},
		{Provider: "local", Model: "m", Category: "chat", PromptTokens: 500, CompletionTokens: 200},
		// Fewer requests, more total tokens — must still rank first.
		{Provider: "local", Model: "m", Category: "dream", PromptTokens: 3000, CompletionTokens: 100},
		{Provider: "local", Model: "m", Category: "title", PromptTokens: 5, CompletionTokens: 3},
	}
	for _, e := range events {
		if err := store.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent(%s): %v", e.Category, err)
		}
	}

	sum, err := store.Summary(ctx, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if len(sum.CategoryBreakdown) != 3 {
		t.Fatalf("CategoryBreakdown len = %d, want 3, got %+v", len(sum.CategoryBreakdown), sum.CategoryBreakdown)
	}
	if got := sum.CategoryBreakdown[0]; got.Category != "dream" || got.Requests != 1 || got.PromptTokens != 3000 {
		t.Errorf("top category = %+v, want dream (1 request, 3000 prompt tokens) ranked first by total tokens", got)
	}
	if got := sum.CategoryBreakdown[1]; got.Category != "chat" || got.Requests != 2 {
		t.Errorf("second category = %+v, want chat (2 requests)", got)
	}
}

func TestMigrateCategoryColumn_AddsColumnToPreExistingTable(t *testing.T) {
	dir, err := os.MkdirTemp("", "stats-migrate-test-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	// Simulate a database created before the category column existed:
	// open with only the base schema (no migration), insert a row, close.
	db, err := database.Open(database.Config{Path: dir + "/usage.db", MaxPool: 1})
	if err != nil {
		t.Fatalf("database.Open: %v", err)
	}
	if _, err := db.ExecContext(context.Background(), schema); err != nil {
		t.Fatalf("apply base schema: %v", err)
	}
	if _, err := db.ExecContext(context.Background(),
		`INSERT INTO usage_events (ts, provider, model, prompt_tokens, completion_tokens, duration_secs, tokens_per_second)
		 VALUES (?, 'openai', 'gpt-4o', 100, 50, 2, 25)`, time.Now().Unix(),
	); err != nil {
		t.Fatalf("insert pre-migration row: %v", err)
	}
	db.Close()

	// Re-opening via NewStore must migrate the existing table in place and
	// backfill the pre-existing row as category='chat', not error or drop it.
	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore on pre-migration db: %v", err)
	}
	defer store.Close()

	sum, err := store.Summary(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 1 {
		t.Fatalf("TotalRequests = %d, want 1 (pre-migration row must survive)", sum.TotalRequests)
	}
	if len(sum.CategoryBreakdown) != 1 || sum.CategoryBreakdown[0].Category != "chat" {
		t.Fatalf("CategoryBreakdown = %+v, want the pre-migration row backfilled as 'chat'", sum.CategoryBreakdown)
	}
}

func TestSummary_EmptyStoreReturnsZeroValues(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	sum, err := store.Summary(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 0 || sum.MostUsedModel != "" || len(sum.ModelBreakdown) != 0 || len(sum.CategoryBreakdown) != 0 || len(sum.Daily) != 0 {
		t.Errorf("expected zero-value summary on empty store, got %+v", sum)
	}
}

// TestRecordEventAndSummary_CacheSplit covers the prompt-cache accounting end
// to end: per-event storage, the three breakdowns, and the global totals. The
// numbers are deliberately asymmetric (one model reporting a cache split, one
// reporting none) because that is the normal real picture — caching is a
// per-provider capability, not an install-wide setting.
func TestRecordEventAndSummary_CacheSplit(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()
	ctx := context.Background()

	events := []Event{
		// An agent turn on Anthropic: most of the input came from cache.
		{Provider: "claude", Model: "claude-opus", Category: "agent",
			PromptTokens: 8100, CachedPromptTokens: 7800, CompletionTokens: 120},
		// The first turn of that window, which wrote the cache entry.
		{Provider: "claude", Model: "claude-opus", Category: "agent",
			PromptTokens: 8100, CacheWriteTokens: 7800, CompletionTokens: 90},
		// A plain chat turn with no breakpoint at all — no split reported.
		{Provider: "claude", Model: "claude-opus", Category: "chat",
			PromptTokens: 1200, CompletionTokens: 300},
		// A provider that reports no cache accounting whatsoever.
		{Provider: "local", Model: "llama-3-8b", Category: "chat",
			PromptTokens: 500, CompletionTokens: 40},
	}
	for _, e := range events {
		if err := store.RecordEvent(ctx, e); err != nil {
			t.Fatalf("RecordEvent: %v", err)
		}
	}

	sum, err := store.Summary(ctx, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}

	if sum.TotalPromptTokens != 17900 {
		t.Errorf("TotalPromptTokens = %d, want 17900", sum.TotalPromptTokens)
	}
	if sum.TotalCachedPromptTokens != 7800 {
		t.Errorf("TotalCachedPromptTokens = %d, want 7800", sum.TotalCachedPromptTokens)
	}
	if sum.TotalCacheWriteTokens != 7800 {
		t.Errorf("TotalCacheWriteTokens = %d, want 7800", sum.TotalCacheWriteTokens)
	}

	var claude, local *ModelUsage
	for i := range sum.ModelBreakdown {
		switch sum.ModelBreakdown[i].Model {
		case "claude-opus":
			claude = &sum.ModelBreakdown[i]
		case "llama-3-8b":
			local = &sum.ModelBreakdown[i]
		}
	}
	if claude == nil || local == nil {
		t.Fatalf("model breakdown missing an expected model: %+v", sum.ModelBreakdown)
	}
	if claude.CachedPromptTokens != 7800 || claude.CacheWriteTokens != 7800 {
		t.Errorf("claude model split = (%d, %d), want (7800, 7800)", claude.CachedPromptTokens, claude.CacheWriteTokens)
	}
	if local.CachedPromptTokens != 0 || local.CacheWriteTokens != 0 {
		t.Errorf("local model split = (%d, %d), want (0, 0)", local.CachedPromptTokens, local.CacheWriteTokens)
	}

	// The category breakdown is the one that shows caching landing where the
	// request builder actually puts a breakpoint: "agent" turns only.
	var agent, chat *CategoryUsage
	for i := range sum.CategoryBreakdown {
		switch sum.CategoryBreakdown[i].Category {
		case "agent":
			agent = &sum.CategoryBreakdown[i]
		case "chat":
			chat = &sum.CategoryBreakdown[i]
		}
	}
	if agent == nil || chat == nil {
		t.Fatalf("category breakdown missing an expected category: %+v", sum.CategoryBreakdown)
	}
	if agent.CachedPromptTokens != 7800 || agent.CacheWriteTokens != 7800 {
		t.Errorf("agent category split = (%d, %d), want (7800, 7800)", agent.CachedPromptTokens, agent.CacheWriteTokens)
	}
	if chat.CachedPromptTokens != 0 || chat.CacheWriteTokens != 0 {
		t.Errorf("chat category split = (%d, %d), want (0, 0)", chat.CachedPromptTokens, chat.CacheWriteTokens)
	}

	if len(sum.Daily) != 1 {
		t.Fatalf("Daily has %d entries, want 1 (all events recorded now)", len(sum.Daily))
	}
	if sum.Daily[0].CachedPromptTokens != 7800 || sum.Daily[0].CacheWriteTokens != 7800 {
		t.Errorf("daily split = (%d, %d), want (7800, 7800)", sum.Daily[0].CachedPromptTokens, sum.Daily[0].CacheWriteTokens)
	}
}

// TestMigrateColumns_UpgradesPreCacheDatabase opens a database created in the
// pre-cache shape (no category, no cache columns — the table exactly as an
// older Memo left it), then reopens it through NewStore and confirms the
// migration both adds the columns and leaves the existing row readable.
//
// The row's cache figures come back as 0, which is the documented
// "unknown, not measured-zero" case: nothing was recording them when it was
// written, so there is no honest value to backfill.
func TestMigrateColumns_UpgradesPreCacheDatabase(t *testing.T) {
	dir, err := os.MkdirTemp("", "stats-migrate-*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)

	ctx := context.Background()
	raw, err := database.Open(database.Config{Path: filepath.Join(dir, "usage.db"), MaxPool: 1})
	if err != nil {
		t.Fatalf("open raw db: %v", err)
	}
	if _, err := raw.ExecContext(ctx, `
		CREATE TABLE usage_events (
			id                INTEGER PRIMARY KEY AUTOINCREMENT,
			ts                INTEGER NOT NULL,
			provider          TEXT    NOT NULL,
			model             TEXT    NOT NULL,
			prompt_tokens     INTEGER NOT NULL DEFAULT 0,
			completion_tokens INTEGER NOT NULL DEFAULT 0,
			duration_secs     REAL    NOT NULL DEFAULT 0,
			tokens_per_second REAL    NOT NULL DEFAULT 0
		)`); err != nil {
		raw.Close()
		t.Fatalf("create legacy table: %v", err)
	}
	if _, err := raw.ExecContext(ctx, `
		INSERT INTO usage_events (ts, provider, model, prompt_tokens, completion_tokens, duration_secs, tokens_per_second)
		VALUES (?, 'openai', 'gpt-4o', 700, 90, 3, 30)`, time.Now().Unix()); err != nil {
		raw.Close()
		t.Fatalf("insert legacy row: %v", err)
	}
	raw.Close()

	store, err := NewStore(dir)
	if err != nil {
		t.Fatalf("NewStore on legacy db: %v", err)
	}
	defer store.Close()

	// A new event must be insertable against the migrated table — an
	// ALTER TABLE that ran but left the INSERT's column list wrong would
	// otherwise only show up at runtime, fire-and-forget, in a log line.
	if err := store.RecordEvent(ctx, Event{
		Provider: "claude", Model: "claude-opus", Category: "agent",
		PromptTokens: 5000, CachedPromptTokens: 4000, CacheWriteTokens: 500, CompletionTokens: 60,
	}); err != nil {
		t.Fatalf("RecordEvent after migration: %v", err)
	}

	sum, err := store.Summary(ctx, time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 2 {
		t.Errorf("TotalRequests = %d, want 2 (legacy row must survive the migration)", sum.TotalRequests)
	}
	if sum.TotalPromptTokens != 5700 {
		t.Errorf("TotalPromptTokens = %d, want 5700", sum.TotalPromptTokens)
	}
	if sum.TotalCachedPromptTokens != 4000 {
		t.Errorf("TotalCachedPromptTokens = %d, want 4000 (legacy row contributes 0)", sum.TotalCachedPromptTokens)
	}
	if sum.TotalCacheWriteTokens != 500 {
		t.Errorf("TotalCacheWriteTokens = %d, want 500", sum.TotalCacheWriteTokens)
	}
	// The category migration's documented default still has to hold.
	for _, cu := range sum.CategoryBreakdown {
		if cu.Category == "" {
			t.Errorf("a row came back with an empty category: %+v", sum.CategoryBreakdown)
		}
	}
}
