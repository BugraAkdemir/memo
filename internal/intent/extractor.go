// SPDX-License-Identifier: AGPL-3.0-or-later

package intent

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
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
		log.Printf("intent: parse response: %v (raw=%q)", err, truncate(raw, 200))
		return IntentResult{Source: source, ContactName: contact}, nil
	}
	return res, nil
}

// extractorSystemPrompt is the Chief prompt for intent extraction.
const extractorSystemPrompt = `Sen Memo'nun niyet analiz sistemisin.
Görevin: kullanıcının mesajının bir etkinlik planı veya alışkanlık beyanı içerip içermediğini belirlemek.

Tanımlar:
- Etkinlik: Belirli bir zamanda gerçekleşecek tek seferlik şey ("yarın 11'de halısaha", "cuma akşamı sinema")
- Alışkanlık: Tekrarlayan niyet ("her gün saat 21'de kod yazacağım", "sabahları spor yapacağım")
- Hiçbiri: Genel sohbet, soru, şikayet, geçmiş anlatımı

Kurallar:
- Sadece JSON döndür, başka hiçbir şey yazma
- Geçmişteki olayları etkinlik sayma ("dün spora gittim" → has_intent: false)
- Belirsiz niyetleri ("bir gün spor yapmak istiyorum") etkinlik sayma
- event_time_iso ve habit_time_hhmm için ISO 8601 kullan
- Gün adlarını doğru resolve et (referans tarih user prompt'unda verilir)

JSON şeması:
{
  "has_intent": true,
  "is_calendar_event": true,
  "is_habit": false,
  "summary": "Yarın saat 11'de halısaha oynayacak (Ahmet ile)",
  "event_title": "Halısaha",
  "event_time_iso": "2026-06-16T11:00:00",
  "habit_time_hhmm": "",
  "habit_days": []
}`

func buildUserPrompt(text string, now time.Time) string {
	return fmt.Sprintf("Referans tarih/saat: %s (%s)\n\nMesaj: %s",
		now.Format("2006-01-02 15:04"),
		now.Weekday().String(),
		text,
	)
}

// rawIntent is the JSON shape the LLM returns.
type rawIntent struct {
	HasIntent       bool    `json:"has_intent"`
	IsCalendarEvent bool    `json:"is_calendar_event"`
	IsHabit         bool    `json:"is_habit"`
	Summary         string  `json:"summary"`
	EventTitle      string  `json:"event_title"`
	EventTimeISO    string  `json:"event_time_iso"`
	HabitTimeHHMM   string  `json:"habit_time_hhmm"`
	HabitDays       []int   `json:"habit_days"`
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
		Summary:         ri.Summary,
		EventTitle:      ri.EventTitle,
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
