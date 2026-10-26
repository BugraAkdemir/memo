// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"testing"
	"time"

	"memo/internal/identity"
	"memo/internal/stats"
)

// The daily telemetry summary reads usage_events through EventsBetween, which
// leaves private rows out. This pins the other half: recordUsageEvent is what
// marks a Minimal Mode turn private, and what carries the time to first token.

func newUsageApp(t *testing.T, minimal bool) *App {
	t.Helper()
	st, err := stats.NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("stats.NewStore: %v", err)
	}
	t.Cleanup(func() { st.Close() })
	return &App{statsStore: st, identity: identity.New("Test", "Memo", "casual", "", minimal)}
}

func recordedToday(t *testing.T, a *App) []stats.Event {
	t.Helper()
	from := time.Now().Add(-time.Hour)
	to := time.Now().Add(time.Hour)
	got, err := a.statsStore.EventsBetween(context.Background(), from, to)
	if err != nil {
		t.Fatalf("EventsBetween: %v", err)
	}
	return got
}

func TestRecordUsageEvent_MinimalModeTurnIsPrivateAndNeverReachesTelemetry(t *testing.T) {
	a := newUsageApp(t, true)
	a.recordUsageEvent(usageMeta{Provider: "local", Model: "m", Category: categoryChat, PromptTokens: 10}, 5, 1, 5)
	if got := recordedToday(t, a); len(got) != 0 {
		t.Fatalf("a Minimal Mode turn reached the telemetry read: %+v", got)
	}
	// ...but it is still in the person's own usage stats.
	sum := a.GetUsageStats(0)
	if sum.TotalRequests != 1 {
		t.Fatalf("the person's own stats must still count it, got %d requests", sum.TotalRequests)
	}
}

func TestRecordUsageEvent_NormalTurnIsReadableWithItsTimeToFirstToken(t *testing.T) {
	a := newUsageApp(t, false)
	a.recordUsageEvent(usageMeta{Provider: "local", Model: "m", Category: categoryChat, PromptTokens: 10, TTFTMs: 480}, 5, 1, 5)
	got := recordedToday(t, a)
	if len(got) != 1 || got[0].TTFTMs != 480 || got[0].Private {
		t.Fatalf("want one non-private event with ttft 480, got %+v", got)
	}
}
