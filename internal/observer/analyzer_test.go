// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// mkObs builds an observation at a concrete local time, filling the derived
// fields the analyzer reads (mirroring what Store.Record would do).
func mkObs(ts time.Time, topic string) Observation {
	t := ts.Local()
	return Observation{
		Timestamp:        ts,
		DayOfWeek:        int(t.Weekday()),
		TimeOfDaySeconds: t.Hour()*3600 + t.Minute()*60 + t.Second(),
		Topic:            topic,
		ActivityType:     ActivityChat,
	}
}

func findPattern(ps []TimePattern, id string) (TimePattern, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return TimePattern{}, false
}

func TestAnalyzeMinObservations(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.Local)
	// Only 2 samples — below MinObservations (3).
	obs := []Observation{
		mkObs(now.Add(-1*24*time.Hour), "coding"),
		mkObs(now.Add(-2*24*time.Hour), "coding"),
	}
	if got := AnalyzePatterns(obs, now); len(got) != 0 {
		t.Fatalf("expected no patterns below threshold, got %d", len(got))
	}
}

func TestAnalyzePeakDetection(t *testing.T) {
	now := time.Date(2026, 6, 14, 23, 0, 0, 0, time.Local)
	var obs []Observation
	// Five days of "coding" around 21:00.
	for d := 1; d <= 5; d++ {
		base := now.Add(time.Duration(-d) * 24 * time.Hour)
		ts := time.Date(base.Year(), base.Month(), base.Day(), 21, 0, 0, 0, time.Local)
		obs = append(obs, mkObs(ts, "coding"))
	}

	got := AnalyzePatterns(obs, now)
	p, ok := findPattern(got, "time:coding")
	if !ok {
		t.Fatalf("expected time:coding pattern, got %+v", got)
	}
	wantPeak := 21 * 3600
	if diff := abs(p.TimePeakSeconds - wantPeak); diff > 600 {
		t.Errorf("peak = %d, want ~%d (diff %d)", p.TimePeakSeconds, wantPeak, diff)
	}
	if p.TotalCount != 5 {
		t.Errorf("TotalCount = %d, want 5", p.TotalCount)
	}
	if p.Confidence <= 0 || p.Confidence > 1 {
		t.Errorf("Confidence = %f, want (0,1]", p.Confidence)
	}
	// Tight cluster → small spread.
	if p.StdDevSeconds > 3600 {
		t.Errorf("StdDevSeconds = %d, want tight (<3600)", p.StdDevSeconds)
	}
}

// TestAnalyzeMidnightBoundary verifies circular statistics: 23:50 and 00:10
// average to ~midnight, not to noon.
func TestAnalyzeMidnightBoundary(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.Local)
	mk := func(daysAgo, hour, min int) Observation {
		base := now.Add(time.Duration(-daysAgo) * 24 * time.Hour)
		ts := time.Date(base.Year(), base.Month(), base.Day(), hour, min, 0, 0, time.Local)
		return mkObs(ts, "writing")
	}
	obs := []Observation{
		mk(1, 23, 50), mk(2, 0, 10), mk(3, 23, 55), mk(4, 0, 5),
	}

	p, ok := findPattern(AnalyzePatterns(obs, now), "time:writing")
	if !ok {
		t.Fatal("expected time:writing pattern")
	}
	// Peak should be within 30 min of midnight (either side of the wrap).
	nearMidnight := p.TimePeakSeconds < 30*60 || p.TimePeakSeconds > secondsPerDay-30*60
	if !nearMidnight {
		t.Errorf("peak = %d, expected near midnight (circular mean failed)", p.TimePeakSeconds)
	}
}

func TestAnalyzeDaysActive(t *testing.T) {
	// Anchor on a known Monday: 2026-06-15 is a Monday.
	monday := time.Date(2026, 6, 15, 10, 0, 0, 0, time.Local)
	if monday.Weekday() != time.Monday {
		t.Fatalf("test anchor is not Monday: %s", monday.Weekday())
	}
	now := time.Date(2026, 6, 20, 12, 0, 0, 0, time.Local) // Saturday
	obs := []Observation{
		mkObs(monday, "planning"),                   // Mon
		mkObs(monday.Add(24*time.Hour), "planning"), // Tue
		mkObs(monday.Add(48*time.Hour), "planning"), // Wed
	}
	p, ok := findPattern(AnalyzePatterns(obs, now), "time:planning")
	if !ok {
		t.Fatal("expected time:planning pattern")
	}
	if !p.DaysActive[time.Monday] || !p.DaysActive[time.Tuesday] || !p.DaysActive[time.Wednesday] {
		t.Errorf("Mon/Tue/Wed should be active: %v", p.DaysActive)
	}
	if p.DaysActive[time.Sunday] || p.DaysActive[time.Saturday] {
		t.Errorf("weekend should be inactive: %v", p.DaysActive)
	}
}

// TestAnalyzeRecencyDecay verifies that older activity yields lower confidence
// than identical-but-recent activity.
func TestAnalyzeRecencyDecay(t *testing.T) {
	now := time.Date(2026, 6, 14, 23, 0, 0, 0, time.Local)
	build := func(startDaysAgo int) TimePattern {
		var obs []Observation
		for d := range 5 {
			base := now.Add(time.Duration(-(startDaysAgo + d)) * 24 * time.Hour)
			ts := time.Date(base.Year(), base.Month(), base.Day(), 21, 0, 0, 0, time.Local)
			obs = append(obs, mkObs(ts, "coding"))
		}
		p, ok := findPattern(AnalyzePatterns(obs, now), "time:coding")
		if !ok {
			t.Fatal("expected coding pattern")
		}
		return p
	}
	fresh := build(1)  // last 5 days
	stale := build(20) // 20–24 days ago
	if stale.Confidence >= fresh.Confidence {
		t.Errorf("stale confidence %f should be < fresh %f", stale.Confidence, fresh.Confidence)
	}
}

func TestApplyDecay(t *testing.T) {
	now := time.Date(2026, 6, 14, 12, 0, 0, 0, time.Local)
	// 30 days → 0.95^30 ≈ 0.2146
	got := ApplyDecay(1.0, now.Add(-30*24*time.Hour), now)
	if got < 0.20 || got > 0.23 {
		t.Errorf("ApplyDecay(30d) = %f, want ~0.214", got)
	}
	// Future lastSeen clamps to no decay.
	if got := ApplyDecay(1.0, now.Add(24*time.Hour), now); got != 1.0 {
		t.Errorf("ApplyDecay(future) = %f, want 1.0", got)
	}
}

func TestPatternStoreRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "patterns.json")
	ps := NewPatternStore(path)

	// Missing file → empty, no error.
	if got, err := ps.Load(); err != nil || got != nil {
		t.Fatalf("Load() on missing = %v, %v; want nil, nil", got, err)
	}

	now := time.Now()
	in := []TimePattern{
		{ID: "time:coding", ActivityType: "coding", Confidence: 0.9, LastSeen: now},
		{ID: "time:writing", ActivityType: "writing", Confidence: 0.05, LastSeen: now.Add(-40 * 24 * time.Hour)},
	}
	if err := ps.Save(in); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	out, err := ps.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("Load() len = %d, want 2", len(out))
	}

	// LoadActive should drop the low-confidence, long-stale pattern.
	active, err := ps.LoadActive(now, 0.3)
	if err != nil {
		t.Fatalf("LoadActive() error = %v", err)
	}
	if len(active) != 1 || active[0].ID != "time:coding" {
		t.Errorf("LoadActive() = %+v, want only time:coding", active)
	}
}

// TestSuppressSurvivesReanalysis is a regression test: a retired pattern must
// not be re-learned by a subsequent analysis pass, and the suppression list must
// survive Save.
func TestSuppressSurvivesReanalysis(t *testing.T) {
	dir := t.TempDir()
	ps := NewPatternStore(filepath.Join(dir, "patterns.json"))
	store := newTestStore(t)
	an := NewAnalyzer(store, ps)
	ctx := context.Background()

	now := time.Now()
	// Seed enough "coding" observations to form a pattern.
	for d := 1; d <= 5; d++ {
		base := now.Add(time.Duration(-d) * 24 * time.Hour)
		ts := time.Date(base.Year(), base.Month(), base.Day(), 21, 0, 0, 0, time.Local)
		mustRecord(t, store, Observation{Timestamp: ts, ActivityType: ActivityChat, Topic: "coding"})
	}

	if err := an.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if _, ok := findByID(t, ps, "time:coding"); !ok {
		t.Fatal("expected time:coding pattern after first analysis")
	}

	// Retire it.
	if _, err := ps.Suppress("time:coding"); err != nil {
		t.Fatalf("Suppress() error = %v", err)
	}
	if _, ok := findByID(t, ps, "time:coding"); ok {
		t.Fatal("pattern should be gone right after Suppress")
	}

	// Re-analyze: it must stay gone.
	if err := an.Run(ctx); err != nil {
		t.Fatalf("Run() #2 error = %v", err)
	}
	if _, ok := findByID(t, ps, "time:coding"); ok {
		t.Fatal("suppressed pattern was resurrected by re-analysis")
	}

	// A plain Save (e.g. from feedback adjustment) must preserve suppression.
	if err := ps.Save([]TimePattern{{ID: "time:writing", Confidence: 0.8}}); err != nil {
		t.Fatalf("Save() error = %v", err)
	}
	set, err := ps.SuppressedSet()
	if err != nil {
		t.Fatalf("SuppressedSet() error = %v", err)
	}
	if !set["time:coding"] {
		t.Error("suppression list lost after Save")
	}
}

func findByID(t *testing.T, ps *PatternStore, id string) (TimePattern, bool) {
	t.Helper()
	all, err := ps.Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	for _, p := range all {
		if p.ID == id {
			return p, true
		}
	}
	return TimePattern{}, false
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
