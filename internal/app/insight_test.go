// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"strings"
	"testing"
	"time"

	"memo/internal/routine"

	moodpkg "memo/internal/mood"
)

// TestGenerateSelfInsight_NoHistoryReturnsCannedMessageWithoutCallingLLM
// covers newRoutineTestApp's default App, which has both a.store and a.mood
// nil (no memory store, no mood engine wired) — GenerateSelfInsight must
// degrade to the "not enough history" fast path instead of panicking on a
// nil dereference or reaching the (here, unconfigured) LLM call path.
func TestGenerateSelfInsight_NoHistoryReturnsCannedMessageWithoutCallingLLM(t *testing.T) {
	a := newRoutineTestApp(t)

	got, err := a.GenerateSelfInsight(context.Background(), 30, "en")
	if err != nil {
		t.Fatalf("GenerateSelfInsight() error = %v", err)
	}
	if !strings.Contains(got, "Not enough conversation history") {
		t.Errorf("GenerateSelfInsight() = %q, want the English no-history fallback", got)
	}
}

// TestGenerateSelfInsight_TurkishFallback mirrors the above for the Turkish
// default (empty/unrecognized lang), matching routineLanguageIsEnglish's own
// "default to Turkish" convention.
func TestGenerateSelfInsight_TurkishFallback(t *testing.T) {
	a := newRoutineTestApp(t)

	got, err := a.GenerateSelfInsight(context.Background(), 30, "tr")
	if err != nil {
		t.Fatalf("GenerateSelfInsight() error = %v", err)
	}
	if !strings.Contains(got, "sohbet geçmişi yok") {
		t.Errorf("GenerateSelfInsight() = %q, want the Turkish no-history fallback", got)
	}
}

// TestCreateRoutineFromDraft_AcceptsInsightContextSource is the regression
// test for wiring routine.ContextInsight into CreateRoutineFromDraft's
// switch — before this, an "insight" context_source_type from the LLM
// drafting extractor would silently fall through to the `default:
// contextType = routine.ContextNone` branch, dropping the very thing that
// makes an "insight" routine different from a plain reminder.
func TestCreateRoutineFromDraft_AcceptsInsightContextSource(t *testing.T) {
	a := newRoutineTestApp(t)
	dir := t.TempDir()
	st, err := routine.NewStore(dir)
	if err != nil {
		t.Fatalf("routine.NewStore: %v", err)
	}
	a.routineStore = st

	created, err := a.CreateRoutineFromDraft(
		"her pazartesi sabah kendimle ilgili ne fark ettiğimi söyle",
		routine.Draft{
			TimeOfDay:         "09:00",
			Prompt:            "geçen haftaki kendimle ilgili farkındalıklarımı özetle",
			ContextSourceType: "insight",
		}, "", false, "tr", nil)
	if err != nil {
		t.Fatalf("CreateRoutineFromDraft: %v", err)
	}
	if created.ContextSource.Type != routine.ContextInsight {
		t.Errorf("created.ContextSource.Type = %q, want %q", created.ContextSource.Type, routine.ContextInsight)
	}
}

func TestSummarizeMoodTrend_DedupesFirstLowHighLast(t *testing.T) {
	now := time.Now()
	history := []moodpkg.HistoryPoint{
		{Score: 0, RecordedAt: now.Add(-3 * time.Hour)},
		{Score: -8, RecordedAt: now.Add(-2 * time.Hour)},
		{Score: 6, RecordedAt: now.Add(-1 * time.Hour)},
	}
	got := summarizeMoodTrend(history)
	if lines := strings.Count(got, "\n"); lines != 3 {
		t.Errorf("summarizeMoodTrend() = %q, produced %d lines, want 3 (first/lowest/highest-and-last)", got, lines)
	}
}

func TestSummarizeMoodTrend_SinglePointDedupesToOneLine(t *testing.T) {
	history := []moodpkg.HistoryPoint{{Score: 2, RecordedAt: time.Now()}}
	got := summarizeMoodTrend(history)
	if lines := strings.Count(got, "\n"); lines != 1 {
		t.Errorf("summarizeMoodTrend(single point) = %q, want exactly one line, got %d", got, lines)
	}
}

func TestSummarizeMoodTrend_Empty(t *testing.T) {
	if got := summarizeMoodTrend(nil); got != "" {
		t.Errorf("summarizeMoodTrend(nil) = %q, want empty string", got)
	}
}
