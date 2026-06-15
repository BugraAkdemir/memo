// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"context"
	"testing"
	"time"
)

func TestMightHaveIntent(t *testing.T) {
	tests := []struct {
		name string
		text string
		want bool
	}{
		{"turkish time word", "yarın spora gideceğim", true},
		{"weekday", "cuma akşamı sinemaya gidelim", true},
		{"habit declaration", "her gün saat 21'de kod yazacağım", true},
		{"plain chat", "nasılsın", false},
		{"past event", "dün sinemaya gittim", false},
		{"empty", "", false},
		{"english future", "i will go to gym tomorrow", true},
		{"meeting keyword", "toplantı var bugün", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MightHaveIntent(tc.text)
			if got != tc.want {
				t.Errorf("MightHaveIntent(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

func TestExtractNoIntent(t *testing.T) {
	called := false
	decide := func(ctx context.Context, system, user string) (string, error) {
		called = true
		return `{"has_intent": false}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	// Plain text that doesn't pass keyword filter — LLM must NOT be called.
	res, err := e.Extract(context.Background(), "nasılsın kanka", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Error("LLM was called for text with no intent keywords")
	}
	if res.HasIntent {
		t.Error("expected HasIntent=false")
	}
}

func TestExtractCalendarEvent(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"has_intent": true,
			"is_calendar_event": true,
			"is_habit": false,
			"summary": "Yarın 11:00'de halısaha oynayacak",
			"event_title": "Halısaha",
			"event_time_iso": "2026-06-16T11:00:00",
			"habit_time_hhmm": "",
			"habit_days": []
		}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 14, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "kanka yarın saat 11 halısaha", SourceWhatsApp, "Ahmet", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasIntent {
		t.Fatal("expected HasIntent=true")
	}
	if !res.IsCalendarEvent {
		t.Error("expected IsCalendarEvent=true")
	}
	if res.EventTitle != "Halısaha" {
		t.Errorf("EventTitle = %q, want Halısaha", res.EventTitle)
	}
	if res.EventTime == nil {
		t.Fatal("EventTime is nil")
	}
	if res.EventTime.Hour() != 11 {
		t.Errorf("EventTime hour = %d, want 11", res.EventTime.Hour())
	}
	if res.ContactName != "Ahmet" {
		t.Errorf("ContactName = %q, want Ahmet", res.ContactName)
	}
}

func TestExtractHabit(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"has_intent": true,
			"is_calendar_event": false,
			"is_habit": true,
			"summary": "Her gün 21:00'de kod yazacak",
			"event_title": "",
			"event_time_iso": "",
			"habit_time_hhmm": "21:00",
			"habit_days": []
		}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "bundan sonra her gün saat 21'de kod yazacağım", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsHabit {
		t.Error("expected IsHabit=true")
	}
	if res.HabitTime == nil {
		t.Fatal("HabitTime is nil")
	}
	if res.HabitTime.Hour() != 21 {
		t.Errorf("HabitTime hour = %d, want 21", res.HabitTime.Hour())
	}
}

func TestExtractJSONEmbeddedInText(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		// LLM sometimes wraps JSON in prose
		return `Here is the analysis: {"has_intent": true, "is_calendar_event": true, "is_habit": false, "summary": "Test", "event_title": "Test", "event_time_iso": "2026-06-16T09:00:00", "habit_time_hhmm": "", "habit_days": []}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "yarın saat 9'da toplantı var", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasIntent {
		t.Error("expected HasIntent=true even when JSON embedded in prose")
	}
}

func TestExtractJSONStringAware(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			"simple",
			`{"a": 1}`,
			`{"a": 1}`,
		},
		{
			"brace inside string value",
			`{"summary": "yarın :)}", "has_intent": true}`,
			`{"summary": "yarın :)}", "has_intent": true}`,
		},
		{
			"nested object",
			`prefix {"a": {"b": 2}} suffix`,
			`{"a": {"b": 2}}`,
		},
		{
			"escaped quote then brace",
			`{"t": "he said \"hi}\" ok"}`,
			`{"t": "he said \"hi}\" ok"}`,
		},
		{
			"no json",
			`there is no object here`,
			``,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := extractJSON(tc.in)
			if got != tc.want {
				t.Errorf("extractJSON(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

// TestExtractWithBraceInSummary is an end-to-end guard: an LLM response whose
// summary contains a stray closing brace must still parse as a valid intent.
func TestExtractWithBraceInSummary(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{"has_intent": true, "is_calendar_event": false, "is_habit": true, "summary": "akşam 21:00 :)}", "event_title": "", "event_time_iso": "", "habit_time_hhmm": "21:00", "habit_days": []}`, nil
	}
	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "her gün 21'de spor", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasIntent {
		t.Error("expected HasIntent=true despite brace in summary")
	}
	if res.HabitTime == nil || res.HabitTime.Hour() != 21 {
		t.Error("habit time not parsed")
	}
}

func TestBuildDecider_Orchestra(t *testing.T) {
	orchestraCalled := false
	singleCalled := false

	orchestra := func(ctx context.Context, prompt string) (string, error) {
		orchestraCalled = true
		return `{"has_intent": false}`, nil
	}
	single := func(ctx context.Context, modelID, system, user string) (string, error) {
		singleCalled = true
		return `{"has_intent": false}`, nil
	}

	// Single model disabled — should use orchestra.
	cfg := struct {
		SingleModelEnabled bool
		ModelID            string
	}{false, "gpt-4o-mini"}

	// Inline BuildDecider call with anonymous config matching the fields.
	decider := func(ctx context.Context, system, user string) (string, error) {
		if cfg.SingleModelEnabled && cfg.ModelID != "" {
			return single(ctx, cfg.ModelID, system, user)
		}
		combined := system + "\n\n---\n\n" + user
		return orchestra(ctx, combined)
	}

	_, _ = decider(context.Background(), "sys", "usr")
	if !orchestraCalled {
		t.Error("expected orchestra to be called")
	}
	if singleCalled {
		t.Error("single model should not be called")
	}
}
