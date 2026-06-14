// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"context"
	"testing"
	"time"

	"memo/internal/observer"
)

// ─── End-to-End Integration Tests ────────────────────────────────────

// Test_FullPipeline_SuggestsThenAccepts runs the complete proactive flow:
// pattern matches → engine ticks → suggestion emitted → user accepts → confidence rises.
func Test_FullPipeline_SuggestsThenAccepts(t *testing.T) {
	eng, patterns, pending, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"suggest","message":"Kod yazma vaktin geldi! Baslayalim mi?","pattern_id":"time:coding"}`)

	eng.tick(context.Background())

	if len(*emitted) != 1 {
		t.Fatalf("expected 1 emitted suggestion, got %d", len(*emitted))
	}
	if !pending.HasPending() {
		t.Fatal("expected pending suggestion after tick")
	}

	id := (*emitted)[0].ID
	t.Logf("Suggestion emitted: %s (id=%s)", (*emitted)[0].Message, id)

	// User accepts → confidence should increase
	accepted, err := eng.HandleResponse(id, "evet harika baslayalim")
	if err != nil {
		t.Fatalf("HandleResponse(evet) error = %v", err)
	}
	if !accepted {
		t.Error("expected accepted=true for 'evet'")
	}
	if pending.HasPending() {
		t.Error("pending should be cleared after response")
	}

	// Reload pattern to verify confidence changed
	reloaded, _ := patterns.Load()
	for _, p := range reloaded {
		if p.ActivityType == "coding" {
			t.Logf("Confidence after acceptance: %.3f", p.Confidence)
			break
		}
	}
}

// Test_FullPipeline_RejectionWeakens verifies that user rejection
// weakens the pattern (confidence should decrease).
func Test_FullPipeline_RejectionWeakens(t *testing.T) {
	eng, patterns, _, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"suggest","message":"Kod vakti!","pattern_id":"time:coding"}`)

	eng.tick(context.Background())

	id := (*emitted)[0].ID

	// Check confidence before rejection
	beforePat, _ := patterns.Load()
	var beforeConf float64
	for _, p := range beforePat {
		if p.ActivityType == "coding" {
			beforeConf = p.Confidence
			break
		}
	}
	t.Logf("Confidence before rejection: %.3f", beforeConf)

	// User rejects
	accepted, err := eng.HandleResponse(id, "hayir istemiyorum")
	if err != nil {
		t.Fatalf("HandleResponse(hayir) error = %v", err)
	}
	if accepted {
		t.Error("expected accepted=false for 'hayir'")
	}

	// Confidence should have decreased (rejected → -0.15 delta)
	afterPat, _ := patterns.Load()
	for _, p := range afterPat {
		if p.ActivityType == "coding" {
			t.Logf("Confidence after rejection: %.3f (was %.3f)", p.Confidence, beforeConf)
			if p.Confidence >= beforeConf {
				t.Error("confidence should decrease after rejection")
			}
			break
		}
	}
}

// Test_FullPipeline_StopRemovePattern tests the "artik yapmiyorum" flow.
func Test_FullPipeline_StopRemovePattern(t *testing.T) {
	eng, patterns, _, emitted := newEngineWithPattern(t, LevelSubtle,
		`{"decision":"notify","message":"hatirlatma","pattern_id":"time:coding"}`)

	eng.tick(context.Background())
	id := (*emitted)[0].ID

	accepted, err := eng.HandleResponse(id, "artik yapmiyorum, bir daha sorma")
	if err != nil {
		t.Fatalf("HandleResponse(stop) error = %v", err)
	}
	if accepted {
		t.Error("should return accepted=false for stop (pattern removed)")
	}

	reloaded, _ := patterns.Load()
	for _, p := range reloaded {
		if p.ID == "time:coding" {
			t.Error("pattern should be removed after 'artik yapmiyorum'")
		}
	}
}

// Test_FullPipeline_OffModeDoesNothing verifies engine in Off mode.
func Test_FullPipeline_OffModeDoesNothing(t *testing.T) {
	eng, _, pending, emitted := newEngineWithPattern(t, LevelOff,
		`{"decision":"suggest","message":"test","pattern_id":"test"}`)

	eng.tick(context.Background())

	if len(*emitted) != 0 {
		t.Errorf("expected 0 emissions in Off mode, got %d", len(*emitted))
	}
	if pending.HasPending() {
		t.Error("should not have pending in Off mode")
	}
}

// Test_FullPipeline_IgnoredResponse has a small penalty (-0.03 delta)
// for an unrecognized response — the user didn't engage, so confidence slowly decays.
func Test_FullPipeline_IgnoredResponse(t *testing.T) {
	eng, patterns, _, emitted := newEngineWithPattern(t, LevelNormal,
		`{"decision":"suggest","message":"Kod vakti!","pattern_id":"time:coding"}`)

	eng.tick(context.Background())
	id := (*emitted)[0].ID

	beforeConf := patternConfidence(t, patterns, "time:coding")

	// "dusunuyorum" has no accept/reject keywords → Ignored (-0.03 delta)
	_, err := eng.HandleResponse(id, "dusunuyorum")
	if err != nil {
		t.Fatalf("HandleResponse(ignore) error = %v", err)
	}

	afterConf := patternConfidence(t, patterns, "time:coding")
	expected := beforeConf - 0.03
	if afterConf < expected-0.001 || afterConf > expected+0.001 {
		t.Errorf("confidence: want ~%.3f (%.3f - 0.03), got %.3f", expected, beforeConf, afterConf)
	}
}

// Test_FullPipeline_MatcherRespectsDayOfWeek ensures Saturday patterns
// don't match on Saturday (inactive day) but match on Monday (active day).
func Test_FullPipeline_MatcherRespectsDayOfWeek(t *testing.T) {
	now := time.Now()
	var weekdayDays [7]bool
	weekdayDays[time.Monday] = true
	weekdayDays[time.Tuesday] = true
	weekdayDays[time.Wednesday] = true
	weekdayDays[time.Thursday] = true
	weekdayDays[time.Friday] = true

	monday := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)
	saturday := time.Date(2026, 6, 13, 21, 0, 0, 0, time.Local)

	p := observer.TimePattern{
		TimePeakSeconds: 21 * 3600,
		StdDevSeconds:   1800,
		Confidence:      0.9,
		DaysActive:      weekdayDays,
		TotalCount:      15,
		LastSeen:        now,
		WeightedScore:   1,
	}

	mondayMatch := Match(p, monday)
	saturdayMatch := Match(p, saturday)

	t.Logf("Monday match: %.3f, Saturday match: %.3f", mondayMatch, saturdayMatch)

	if mondayMatch <= 0 {
		t.Error("should match on Monday (active day)")
	}
	if saturdayMatch > 0 {
		t.Error("should NOT match on Saturday (inactive day)")
	}
}

// Test_FullPipeline_CircularStatsMidnight verifies the match at midnight.
func Test_FullPipeline_CircularStatsMidnight(t *testing.T) {
	a := 23*3600 + 55*60 // 23:55
	b := 0*3600 + 5*60   // 00:05

	dist := circularDistance(a, b)
	t.Logf("Distance between 23:55 and 00:05 = %d seconds", dist)
	if dist != 600 {
		t.Errorf("expected 600s (10min), got %d", dist)
	}

	dist2 := circularDistance(0, 12*3600)
	if dist2 != 12*3600 {
		t.Errorf("12h distance = %d, want %d", dist2, 12*3600)
	}
}
