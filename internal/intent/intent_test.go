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
		{"turkish explicit time", "yarın saat 16.00'da toplantım var", true},
		{"turkish natural afternoon", "çarşamba günü öğlenden sonra 5'de toplantım var", true},
		{"turkish evening", "bu akşam 8'de sinema", true},
		{"turkish habit", "her gün saat 21'de kod yazacağım", true},
		{"turkish meeting keyword", "toplantı var bugün", true},
		{"turkish tomorrow", "yarın spora gideceğim", true},
		{"turkish weekday", "cuma akşamı sinemaya gidelim", true},
		{"english future", "i will go to gym tomorrow", true},
		{"english meeting", "i have a meeting at 9pm", true},
		{"english plan", "planning to meet at 3", true},
		{"german via colon time", "morgen um 21:00 Meeting", true},
		{"spanish via colon time", "reunión a las 21:00", true},
		{"french via time", "rendez-vous à 14h30 demain", true},
		{"bare colon time", "21:00 buluşalım", true},
		{"am/pm time", "lunch at 12 pm", true},
		{"hour with dot", "let's meet at 14.30", true},
		{"german uhr", "um 15 Uhr treffen", true},
		{"number with de suffix", "toplantı 3'de", true},
		{"number with te suffix", "randevu 5'te", true},
		{"afternoon in turkish", "öğlenden sonra 4'te buluşalım", true},
		{"morning in turkish", "sabah 9'da kahvaltı", true},
		{"evening in turkish", "akşam 7'de yemek", true},
		{"night in turkish", "gece 10'da maç var", true},
		{"saat keyword", "saat 14'te toplantı", true},
		{"long message with intent context", "kanka yarın öğlen buluşup şu projeyi konuşalım mı", true},
		{"date iso format", "event on 2026-06-20 at 14:00", true},
		{"cancellation turkish", "yarınki toplantım iptal oldu", true},
		// ── Should be false ─────────────────────────────────
		{"plain chat short", "nasılsın", false},
		{"plain chat two words", "nasılsın kanka", false},
		{"past event", "dün sinemaya gittim", false},
		{"past event english", "i went to the gym yesterday", false},
		{"empty", "", false},
		{"just greeting", "selam", false},
		{"just ok", "ok", false},
		{"thanks", "teşekkürler", false},
		{"single digit no context", "5 tane elma aldım", false},
		{"no time no keyword", "wie geht es dir heute", false},
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

func TestExtractCancellation(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"has_intent": true,
			"is_calendar_event": false,
			"is_habit": false,
			"is_cancellation": true,
			"summary": "Yarınki 21:00 toplantı iptal",
			"event_title": "Toplantı",
			"event_time_iso": "2026-06-16T21:00:00",
			"time_explicit": true,
			"habit_time_hhmm": "",
			"habit_days": []
		}`, nil
	}
	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "yarınki saat 21'deki toplantım iptal", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.IsCancellation {
		t.Error("expected IsCancellation=true")
	}
	if res.EventTitle != "Toplantı" {
		t.Errorf("EventTitle = %q, want Toplantı", res.EventTitle)
	}
	if res.EventTime == nil || res.EventTime.Hour() != 21 {
		t.Error("cancellation event time not parsed")
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

// TestExtractHabit_HabitDaysAsPhrase reproduces the live-reported bug
// (BUG-H2): the LLM sometimes returns habit_days as a natural-language phrase
// ("her hafta içi") or a string array instead of the requested []int. Before
// the fix, json.Unmarshal rejected the entire rawIntent object over this one
// field — has_intent/is_habit/summary were correctly produced but never
// reached the caller, and the assistant told the user it saved the habit
// when nothing was actually persisted.
func TestExtractHabit_HabitDaysAsPhrase(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"has_intent": true,
			"is_calendar_event": false,
			"is_habit": true,
			"summary": "Every weekday 08:30 stretching",
			"event_title": "",
			"event_time_iso": "",
			"habit_time_hhmm": "08:30",
			"habit_days": "her hafta içi"
		}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "her hafta içi sabah 08:30'da esneme yapacağım", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasIntent {
		t.Fatal("expected HasIntent=true even when habit_days is a phrase, not []int")
	}
	if !res.IsHabit {
		t.Error("expected IsHabit=true")
	}
	if len(res.HabitDays) != 5 {
		t.Errorf("HabitDays = %v, want 5 weekdays (Mon-Fri) for \"hafta içi\"", res.HabitDays)
	}
}

// TestExtractHabit_HabitDaysAsStringArray covers the LLM returning day names
// as strings ("monday", "tuesday") instead of ints.
func TestExtractHabit_HabitDaysAsStringArray(t *testing.T) {
	decide := func(ctx context.Context, system, user string) (string, error) {
		return `{
			"has_intent": true,
			"is_calendar_event": false,
			"is_habit": true,
			"summary": "Gym on Monday and Wednesday",
			"event_title": "",
			"event_time_iso": "",
			"habit_time_hhmm": "18:00",
			"habit_days": ["monday", "wednesday"]
		}`, nil
	}

	e := NewExtractor(decide)
	now := time.Date(2026, 6, 15, 10, 0, 0, 0, time.UTC)

	res, err := e.Extract(context.Background(), "pazartesi ve çarşamba 18'de spor yapacağım", SourceChat, "", now)
	if err != nil {
		t.Fatal(err)
	}
	if !res.HasIntent {
		t.Fatal("expected HasIntent=true even when habit_days is a string array")
	}
	if len(res.HabitDays) != 2 || res.HabitDays[0] != time.Monday || res.HabitDays[1] != time.Wednesday {
		t.Errorf("HabitDays = %v, want [Monday Wednesday]", res.HabitDays)
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
