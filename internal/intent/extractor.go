// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"memo/internal/logx"
	"strings"
	"time"
)

// Decider is the LLM call interface shared with the proactive engine.
// It receives a system prompt and a user prompt and returns the model's
// raw text response.
type Decider func(ctx context.Context, systemPrompt, userPrompt string) (string, error)

// Extractor runs the two-stage intent pipeline: keyword filter → LLM parse.
type Extractor struct {
	decide Decider
}

// NewExtractor creates an Extractor backed by the given Decider.
func NewExtractor(decide Decider) *Extractor {
	return &Extractor{decide: decide}
}

// Extract analyses text for intent. now is the wall-clock reference used to
// resolve relative time expressions ("yarın", "bu akşam"). The raw message
// text is never stored; only the returned IntentResult fields are persisted.
//
// Returns a zero IntentResult (HasIntent=false) without error when the text
// passes the keyword filter but the LLM finds no actionable intent.
func (e *Extractor) Extract(ctx context.Context, text string, source Source, contact string, now time.Time) (IntentResult, error) {
	if !MightHaveIntent(text) {
		return IntentResult{Source: source, ContactName: contact}, nil
	}

	userPrompt := buildUserPrompt(text, now)
	raw, err := e.decide(ctx, extractorSystemPrompt, userPrompt)
	if err != nil {
		return IntentResult{}, fmt.Errorf("intent: llm decide: %w", err)
	}

	res, err := parseResponse(raw, source, contact, now)
	if err != nil {
		logx.Printf("intent: parse response: %v (raw=%q)", err, truncate(raw, 200))
		return IntentResult{Source: source, ContactName: contact}, nil
	}
	return res, nil
}

// extractorSystemPrompt is the Chief prompt for intent extraction.
// Written in English so LLMs produce the most reliable structured output;
// the user message may be in any language.
const extractorSystemPrompt = `You are Memo's intent analysis system.
Your job: determine whether the message contains a calendar event plan, a habit declaration, or a cancellation.

Definitions:
- Calendar event: A one-off happening at a specific future time ("tomorrow at 11 football", "meeting Friday 3pm", "çarşamba 5'te toplantı", "Rendez-vous demain 14h")
- Habit: A recurring intention ("I will code every day at 21:00", "her gün spor yapacağım", "every morning 7am workout")
- Cancellation: Cancelling an existing plan ("cancel my meeting", "toplantım iptal", "spora gitmeyeceğim"). Set is_cancellation=true, fill event_title and event_time_iso for the cancelled plan.
- None: General chat, questions, complaints, past-tense narration

Rules:
- Return ONLY JSON, no other text
- Past events are NOT calendar events ("dün spora gittim", "I went to the gym yesterday" → has_intent: false)
- Vague wishes are NOT events ("bir gün spor yapmak istiyorum", "I want to learn guitar someday" → has_intent: false)
- Use ISO 8601 for event_time_iso and HH:MM for habit_time_hhmm
- The current date/time AND day-of-week are provided in the user prompt — resolve relative expressions against them

TIME RESOLUTION (CRITICAL — works for ALL languages):
- Explicit time mentioned: "at 5", "11'de", "saat 14", "akşam 8", "14:00", "3pm", "14h", "um 3", "à 5h" → use that time, time_explicit=true
- "öğlenden sonra X" / "after noon X" / "après-midi X" / "tarde X" → X + 12. e.g. "öğlenden sonra 5" = 17:00, "afternoon 3" = 15:00
- "sabah" / "morning" / "mañana" → hour stays as-is if < 12, otherwise default 09:00
- "akşam" / "evening" / "noche" → if hour < 12: hour + 12, otherwise keep. Default evening=19:00
- "öğlen" / "noon" / "mediodía" → 12:00
- "gece" / "night" → 21:00 unless explicit hour given
- "sabah"=09:00, "öğle/öğlen/midday"=12:00, "akşam/evening"=19:00, "gece/night"=21:00
- Hour markers in various languages: "at", "um", "à", "a las", "alle", "ora", "saat" all mean "at (time)"
- Turkish time suffixes: "5'de", "5'te", "3'da" etc. mean "at 5", "at 3"
- Vague time ("let's go out tomorrow", "hafta sonu buluşalım") → ESTIMATE a reasonable time but set time_explicit=false
- When in doubt, still fill event_time_iso — the app decides whether to use estimated times

DAY RESOLUTION:
- Day names in any language map to the nearest matching day from the reference date
- "yarın" / "tomorrow" = reference + 1 day
- "öbürgün" / "day after tomorrow" = reference + 2 days
- "bugün" / "today" = reference date
- Weekdays resolved to the next occurrence from reference date

JSON schema:
{
  "has_intent": true,
  "is_calendar_event": true,
  "is_habit": false,
  "is_cancellation": false,
  "summary": "One-line summary of the intent in English",
  "event_title": "Football",
  "event_time_iso": "2026-06-16T17:00:00",
  "time_explicit": true,
  "habit_time_hhmm": "",
  "habit_days": []
}`

// turkishDayNames maps time.Weekday to Turkish day names so the LLM can
// resolve Turkish day references ("çarşamba") against the reference date.
var turkishDayNames = map[time.Weekday]string{
	time.Sunday:    "Pazar",
	time.Monday:    "Pazartesi",
	time.Tuesday:   "Salı",
	time.Wednesday: "Çarşamba",
	time.Thursday:  "Perşembe",
	time.Friday:    "Cuma",
	time.Saturday:  "Cumartesi",
}

func buildUserPrompt(text string, now time.Time) string {
	enDay := now.Weekday().String()
	trDay := turkishDayNames[now.Weekday()]
	return fmt.Sprintf("Reference datetime: %s\nDay of week: %s / %s\n\nMessage: %s",
		now.Format("2006-01-02 15:04"),
		enDay, trDay,
		text,
	)
}

// rawIntent is the JSON shape the LLM returns.
type rawIntent struct {
	HasIntent       bool   `json:"has_intent"`
	IsCalendarEvent bool   `json:"is_calendar_event"`
	IsHabit         bool   `json:"is_habit"`
	IsCancellation  bool   `json:"is_cancellation"`
	Summary         string `json:"summary"`
	EventTitle      string `json:"event_title"`
	EventTimeISO    string `json:"event_time_iso"`
	TimeExplicit    bool   `json:"time_explicit"`
	HabitTimeHHMM   string `json:"habit_time_hhmm"`
	HabitDays       []int  `json:"habit_days"`
}

func parseResponse(raw string, source Source, contact string, now time.Time) (IntentResult, error) {
	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return IntentResult{}, fmt.Errorf("no JSON object found")
	}

	var ri rawIntent
	if err := json.Unmarshal([]byte(jsonStr), &ri); err != nil {
		return IntentResult{}, fmt.Errorf("unmarshal: %w", err)
	}

	if !ri.HasIntent {
		return IntentResult{Source: source, ContactName: contact}, nil
	}

	res := IntentResult{
		HasIntent:       true,
		IsCalendarEvent: ri.IsCalendarEvent,
		IsHabit:         ri.IsHabit,
		IsCancellation:  ri.IsCancellation,
		Summary:         ri.Summary,
		EventTitle:      ri.EventTitle,
		TimeExplicit:    ri.TimeExplicit,
		Source:          source,
		ContactName:     contact,
	}

	if ri.EventTimeISO != "" {
		t, err := parseISO(ri.EventTimeISO, now)
		if err == nil {
			res.EventTime = &t
		}
	}

	if ri.HabitTimeHHMM != "" {
		t, err := time.Parse("15:04", ri.HabitTimeHHMM)
		if err == nil {
			resolved := time.Date(now.Year(), now.Month(), now.Day(),
				t.Hour(), t.Minute(), 0, 0, now.Location())
			res.HabitTime = &resolved
		}
	}

	for _, d := range ri.HabitDays {
		if d >= 0 && d <= 6 {
			res.HabitDays = append(res.HabitDays, time.Weekday(d))
		}
	}

	return res, nil
}

// parseISO tries several common ISO 8601 layouts.
func parseISO(s string, ref time.Time) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05Z07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04",
		"2006-01-02",
	}
	for _, l := range layouts {
		if t, err := time.ParseInLocation(l, s, ref.Location()); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse %q as ISO 8601", s)
}

// extractJSON finds the first balanced JSON object in s. It is string-aware:
// braces inside JSON string literals (and escaped quotes) are ignored so a
// value like "yarın :)}" does not truncate the object early.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start == -1 {
		return ""
	}
	depth := 0
	inString := false
	escaped := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if inString {
			switch {
			case escaped:
				escaped = false
			case c == '\\':
				escaped = true
			case c == '"':
				inString = false
			}
			continue
		}
		switch c {
		case '"':
			inString = true
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
