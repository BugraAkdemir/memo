// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"memo/internal/observer"
)

// ─── Matcher ────────────────────────────────────────────────────

func TestMatchInactiveDay(t *testing.T) {
	now := time.Date(2026, 6, 14, 21, 0, 0, 0, time.Local) // Sunday
	p := observer.TimePattern{
		TimePeakSeconds: 21 * 3600,
		StdDevSeconds:   600,
		Confidence:      0.9,
		WeightedScore:   1,
	}
	// No days active → never matches.
	if got := Match(p, now); got != 0 {
		t.Errorf("Match() on inactive day = %f, want 0", got)
	}
}

func TestMatchPeakVsFar(t *testing.T) {
	now := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local) // Monday 21:00
	var days [7]bool
	for i := range days {
		days[i] = true
	}
	p := observer.TimePattern{
		TimePeakSeconds: 21 * 3600,
		StdDevSeconds:   1800,
		Confidence:      0.9,
		DaysActive:      days,
		WeightedScore:   1,
	}
	atPeak := Match(p, now)
	if atPeak < 0.5 {
		t.Errorf("Match() at peak = %f, want high", atPeak)
	}

	far := Match(p, time.Date(2026, 6, 15, 9, 0, 0, 0, time.Local)) // 09:00, 12h away
	if far >= atPeak {
		t.Errorf("Match() far (%f) should be < peak (%f)", far, atPeak)
	}
}

func TestCircularDistance(t *testing.T) {
	// 23:59 and 00:01 → 2 minutes.
	a := 23*3600 + 59*60
	b := 1 * 60
	if got := circularDistance(a, b); got != 120 {
		t.Errorf("circularDistance(23:59,00:01) = %d, want 120", got)
	}
	if got := circularDistance(0, 12*3600); got != 12*3600 {
		t.Errorf("circularDistance(00:00,12:00) = %d, want 43200", got)
	}
}

// ─── Decision parsing ───────────────────────────────────────────

func TestParseDecision(t *testing.T) {
	raw := "Sure, here is my decision:\n```json\n{\"decision\": \"suggest\", \"message\": \"Kod vakti {x}\", \"pattern_id\": \"time:coding\"}\n```\nDone."
	d, err := ParseDecision(raw)
	if err != nil {
		t.Fatalf("ParseDecision() error = %v", err)
	}
	if d.Action != ActionSuggest {
		t.Errorf("Action = %q, want suggest", d.Action)
	}
	if d.PatternID != "time:coding" {
		t.Errorf("PatternID = %q", d.PatternID)
	}
}

func TestParseDecisionInvalidFallsBackToNone(t *testing.T) {
	d, _ := ParseDecision(`{"decision": "explode", "message": "x"}`)
	if d.Action != ActionNone {
		t.Errorf("unknown action should map to none, got %q", d.Action)
	}
	if _, err := ParseDecision("no json here"); err == nil {
		t.Error("expected error when no JSON object present")
	}
}

// ─── Feedback ───────────────────────────────────────────────────

func TestOutcomeFromResponse(t *testing.T) {
	cases := map[string]Outcome{
		"Evet":               OutcomeAccepted,
		"yes please":         OutcomeAccepted,
		"tamam 👍":            OutcomeAccepted,
		"Hayır":              OutcomeRejected,
		"yok":                OutcomeRejected, // must NOT match "ok"
		"yok istemiyorum":    OutcomeRejected,
		"şimdi değil":        OutcomeRejected,
		"❌":                  OutcomeRejected,
		"artık yapmıyorum":   OutcomeStopped,
		"bir daha söyle":     OutcomeIgnored, // "bir daha" alone is not "stop"
		"hmm something else": OutcomeIgnored,
		"":                   OutcomeIgnored,
	}
	for in, want := range cases {
		if got := OutcomeFromResponse(in); got != want {
			t.Errorf("OutcomeFromResponse(%q) = %q, want %q", in, got, want)
		}
	}
}

// ─── Pending store ──────────────────────────────────────────────

func TestPendingStoreLifecycle(t *testing.T) {
	ps := NewPendingStore(filepath.Join(t.TempDir(), "pending.json"))

	if ps.HasPending() {
		t.Fatal("fresh store should have no pending")
	}
	if err := ps.Set(PendingSuggestion{ID: "a", Message: "hi", Action: ActionSuggest}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	if !ps.HasPending() {
		t.Fatal("expected pending after Set")
	}

	// Wrong id → no-op.
	if p, _ := ps.Respond("wrong", "evet"); p != nil {
		t.Error("Respond() with wrong id should return nil")
	}
	p, err := ps.Respond("a", "evet")
	if err != nil || p == nil {
		t.Fatalf("Respond() = %v, %v", p, err)
	}
	if ps.HasPending() {
		t.Error("responded suggestion should not be pending")
	}
}

func TestPendingExpiry(t *testing.T) {
	ps := NewPendingStore(filepath.Join(t.TempDir(), "pending.json"))
	_ = ps.Set(PendingSuggestion{ID: "old", Message: "x", CreatedAt: time.Now().Add(-2 * PendingTTL)})
	if ps.HasPending() {
		t.Error("expired suggestion should not count as pending")
	}
}

// ─── Engine (end-to-end with fakes) ─────────────────────────────

func newEngineWithPattern(t *testing.T, level Level, decideJSON string) (*Engine, *observer.PatternStore, *PendingStore, *[]PendingSuggestion) {
	t.Helper()
	dir := t.TempDir()
	patterns := observer.NewPatternStore(filepath.Join(dir, "patterns.json"))
	pending := NewPendingStore(filepath.Join(dir, "pending.json"))

	now := time.Now()
	var days [7]bool
	for i := range days {
		days[i] = true
	}
	nowSec := now.Hour()*3600 + now.Minute()*60 + now.Second()
	if err := patterns.Save([]observer.TimePattern{{
		ID:              "time:coding",
		ActivityType:    "coding",
		TimePeakSeconds: nowSec,
		StdDevSeconds:   1800,
		Confidence:      0.9,
		DaysActive:      days,
		TotalCount:      10,
		LastSeen:        now,
		WeightedScore:   1,
	}}); err != nil {
		t.Fatalf("Save pattern: %v", err)
	}

	var emitted []PendingSuggestion
	emit := func(p PendingSuggestion) { emitted = append(emitted, p) }
	decide := func(ctx context.Context, sys, user string) (string, error) { return decideJSON, nil }

	eng := NewEngine(Config{Cooldown: time.Millisecond}, patterns, pending, decide, emit, func() Level { return level })
	return eng, patterns, pending, &emitted
}

func TestEngineSuggestsThenAccepts(t *testing.T) {
	eng, patterns, pending, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"suggest","message":"Kod vakti!","pattern_id":"time:coding"}`)

	eng.tick(context.Background())

	if len(*emitted) != 1 {
		t.Fatalf("expected 1 emitted suggestion, got %d", len(*emitted))
	}
	if !pending.HasPending() {
		t.Fatal("expected a pending suggestion after tick")
	}
	id := (*emitted)[0].ID

	before := patternConfidence(t, patterns, "time:coding")
	accepted, err := eng.HandleResponse(id, "evet harika")
	if err != nil {
		t.Fatalf("HandleResponse() error = %v", err)
	}
	if !accepted {
		t.Error("expected accepted=true for 'evet'")
	}
	after := patternConfidence(t, patterns, "time:coding")
	if after <= before {
		t.Errorf("confidence should rise after acceptance: %f -> %f", before, after)
	}
	if pending.HasPending() {
		t.Error("pending should be cleared after response")
	}
}

// TestEngineHandleOutcome_AppliesAlreadyClassifiedOutcome covers the direct
// path a caller uses when it has classified the outcome itself (e.g. via an
// LLM reading free-form, any-language chat text — see
// internal/app/proactive_ambient.go) instead of matching against
// OutcomeFromResponse's fixed English/Turkish keyword list.
func TestEngineHandleOutcome_AppliesAlreadyClassifiedOutcome(t *testing.T) {
	eng, patterns, pending, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"suggest","message":"Kod vakti!","pattern_id":"time:coding"}`)

	eng.tick(context.Background())
	if len(*emitted) != 1 {
		t.Fatalf("expected 1 emitted suggestion, got %d", len(*emitted))
	}
	ps := (*emitted)[0]

	before := patternConfidence(t, patterns, "time:coding")
	accepted, err := eng.HandleOutcome(ps, OutcomeAccepted)
	if err != nil {
		t.Fatalf("HandleOutcome() error = %v", err)
	}
	if !accepted {
		t.Error("expected accepted=true for OutcomeAccepted")
	}
	after := patternConfidence(t, patterns, "time:coding")
	if after <= before {
		t.Errorf("confidence should rise after acceptance: %f -> %f", before, after)
	}
	if pending.HasPending() {
		t.Error("pending should be cleared after HandleOutcome")
	}
}

// TestEngineThrottlesConsultations verifies the Chief is not consulted on every
// tick while a pattern keeps matching and the Chief keeps replying "none".
func TestEngineThrottlesConsultations(t *testing.T) {
	dir := t.TempDir()
	patterns := observer.NewPatternStore(filepath.Join(dir, "patterns.json"))
	pending := NewPendingStore(filepath.Join(dir, "pending.json"))

	now := time.Now()
	var days [7]bool
	for i := range days {
		days[i] = true
	}
	nowSec := now.Hour()*3600 + now.Minute()*60 + now.Second()
	_ = patterns.Save([]observer.TimePattern{{
		ID: "time:coding", ActivityType: "coding", TimePeakSeconds: nowSec,
		StdDevSeconds: 1800, Confidence: 0.9, DaysActive: days, LastSeen: now, WeightedScore: 1,
	}})

	var calls int
	decide := func(ctx context.Context, sys, user string) (string, error) {
		calls++
		return `{"decision":"none","message":"","pattern_id":"time:coding"}`, nil
	}
	// Long cooldown so the second tick is throttled.
	eng := NewEngine(Config{Cooldown: time.Hour}, patterns, pending, decide, nil, func() Level { return LevelNormal })

	eng.tick(context.Background())
	eng.tick(context.Background())

	if calls != 1 {
		t.Errorf("Chief consulted %d times across 2 ticks, want 1 (throttled)", calls)
	}
}

func TestEngineOffDoesNothing(t *testing.T) {
	eng, _, pending, emitted := newEngineWithPattern(t, LevelOff,
		`{"decision":"suggest","message":"x","pattern_id":"time:coding"}`)
	eng.tick(context.Background())
	if len(*emitted) != 0 || pending.HasPending() {
		t.Error("engine must stay silent when level is off")
	}
}

func TestEngineStopRemovesPattern(t *testing.T) {
	eng, patterns, _, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"notify","message":"hatırlatma","pattern_id":"time:coding"}`)
	eng.tick(context.Background())
	if len(*emitted) != 1 {
		t.Fatalf("expected 1 emission, got %d", len(*emitted))
	}
	if _, err := eng.HandleResponse((*emitted)[0].ID, "artık yapmıyorum"); err != nil {
		t.Fatalf("HandleResponse() error = %v", err)
	}
	all, _ := patterns.Load()
	if len(all) != 0 {
		t.Errorf("pattern should be removed after 'artık yapmıyorum', got %d", len(all))
	}
}

func patternConfidence(t *testing.T, ps *observer.PatternStore, id string) float64 {
	t.Helper()
	all, err := ps.Load()
	if err != nil {
		t.Fatalf("Load patterns: %v", err)
	}
	for _, p := range all {
		if p.ID == id {
			return p.Confidence
		}
	}
	t.Fatalf("pattern %s not found", id)
	return 0
}
