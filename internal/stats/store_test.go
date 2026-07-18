// SPDX-License-Identifier: AGPL-3.0-or-later

package stats

import (
	"context"
	"os"
	"testing"
	"time"
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

func TestSummary_EmptyStoreReturnsZeroValues(t *testing.T) {
	store, cleanup := newTestStore(t)
	defer cleanup()

	sum, err := store.Summary(context.Background(), time.Time{})
	if err != nil {
		t.Fatalf("Summary: %v", err)
	}
	if sum.TotalRequests != 0 || sum.MostUsedModel != "" || len(sum.ModelBreakdown) != 0 || len(sum.Daily) != 0 {
		t.Errorf("expected zero-value summary on empty store, got %+v", sum)
	}
}
