// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// TestPatternStoreConcurrent hammers the store from many goroutines to validate
// the locking (run with -race). The invariants checked afterwards: the file is
// still valid, the suppression survives, and the surviving pattern's confidence
// stayed within [0,1].
func TestPatternStoreConcurrent(t *testing.T) {
	ps := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	now := time.Now()
	if err := ps.Save([]TimePattern{
		{ID: "time:coding", Confidence: 0.5, LastSeen: now},
		{ID: "time:writing", Confidence: 0.5, LastSeen: now},
	}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(3)
		go func() { defer wg.Done(); _, _ = ps.AdjustConfidence("time:coding", 0.01) }()
		go func() {
			defer wg.Done()
			_ = ps.Save([]TimePattern{
				{ID: "time:coding", Confidence: 0.5, LastSeen: now},
				{ID: "time:writing", Confidence: 0.5, LastSeen: now},
			})
		}()
		go func() { defer wg.Done(); _, _ = ps.SuppressedSet() }()
	}
	// Retire one pattern partway through the storm.
	if _, err := ps.Suppress("time:writing"); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	wg.Wait()

	// File must still be readable and consistent.
	all, err := ps.Load()
	if err != nil {
		t.Fatalf("Load after storm: %v", err)
	}
	for _, p := range all {
		if p.Confidence < 0 || p.Confidence > 1 {
			t.Errorf("confidence out of range: %s = %f", p.ID, p.Confidence)
		}
	}
	set, err := ps.SuppressedSet()
	if err != nil {
		t.Fatalf("SuppressedSet after storm: %v", err)
	}
	if !set["time:writing"] {
		t.Error("suppression lost under concurrent writes")
	}
}

// TestDeclaredHabitPattern_BuildsExpectedPattern checks the pattern a stated
// habit ("her akşam 9'da kod yazarım") turns into: a high starting
// confidence (not the low, needs-repeats confidence a fresh statistical
// observation would get), the declared time-of-day, and every day active
// when no specific days were named.
func TestDeclaredHabitPattern_BuildsExpectedPattern(t *testing.T) {
	habitTime := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)
	p := DeclaredHabitPattern("kod yazma", habitTime, nil)

	if !p.Declared {
		t.Error("Declared = false, want true")
	}
	if p.Confidence < 0.85 {
		t.Errorf("Confidence = %.2f, want a high immediate value (>= 0.85), not a needs-repeats starting point", p.Confidence)
	}
	if want := 21 * 3600; p.TimePeakSeconds != want {
		t.Errorf("TimePeakSeconds = %d, want %d (21:00)", p.TimePeakSeconds, want)
	}
	for d, active := range p.DaysActive {
		if !active {
			t.Errorf("DaysActive[%d] = false, want true (nil habitDays means every day)", d)
		}
	}
	if p.ActivityType != "kod yazma" {
		t.Errorf("ActivityType = %q, want the summary text verbatim", p.ActivityType)
	}
}

// TestDeclaredHabitPattern_SpecificDays checks that only the named weekdays
// are marked active when habitDays is non-empty.
func TestDeclaredHabitPattern_SpecificDays(t *testing.T) {
	habitTime := time.Date(2026, 6, 15, 7, 0, 0, 0, time.Local)
	p := DeclaredHabitPattern("spor yapma", habitTime, []time.Weekday{time.Monday, time.Wednesday, time.Friday})

	for d := range p.DaysActive {
		want := d == int(time.Monday) || d == int(time.Wednesday) || d == int(time.Friday)
		if p.DaysActive[d] != want {
			t.Errorf("DaysActive[%d] = %v, want %v", d, p.DaysActive[d], want)
		}
	}
}

// TestPatternStore_SaveDeclared_BypassesMinObservations is the core promise
// of this feature: a single declaration (TotalCount would be 1, far below
// MinObservations=3) still produces an immediately usable, high-confidence
// pattern — unlike a passively-observed one, which AnalyzePatterns refuses
// to emit at all below MinObservations repeats (see TestAnalyzeMinObservations).
func TestPatternStore_SaveDeclared_BypassesMinObservations(t *testing.T) {
	ps := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	habitTime := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)

	saved, err := ps.SaveDeclared(DeclaredHabitPattern("kod yazma", habitTime, nil))
	if err != nil {
		t.Fatalf("SaveDeclared: %v", err)
	}
	if saved.TotalCount != 1 {
		t.Errorf("TotalCount = %d, want 1 on first declaration", saved.TotalCount)
	}

	active, err := ps.LoadActive(habitTime, 0.5)
	if err != nil {
		t.Fatalf("LoadActive: %v", err)
	}
	if _, ok := findPattern(active, saved.ID); !ok {
		t.Fatal("declared pattern is not active immediately after a single SaveDeclared call")
	}
}

// TestPatternStore_SaveDeclared_UpsertsBySameTimeKey checks that re-declaring
// a habit at the same time-of-day updates the existing pattern (preserving
// FirstSeen, incrementing TotalCount) instead of accumulating a duplicate —
// see DeclaredHabitPattern's doc comment on why the ID is keyed by time only.
func TestPatternStore_SaveDeclared_UpsertsBySameTimeKey(t *testing.T) {
	ps := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	first := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)
	second := time.Date(2026, 6, 20, 21, 0, 0, 0, time.Local)

	p1, err := ps.SaveDeclared(DeclaredHabitPattern("kod yazma", first, nil))
	if err != nil {
		t.Fatalf("first SaveDeclared: %v", err)
	}
	p2, err := ps.SaveDeclared(DeclaredHabitPattern("akşam kodlama", second, nil))
	if err != nil {
		t.Fatalf("second SaveDeclared: %v", err)
	}

	if p1.ID != p2.ID {
		t.Fatalf("same time-of-day produced different ids: %q vs %q", p1.ID, p2.ID)
	}
	if !p2.FirstSeen.Equal(p1.FirstSeen) {
		t.Errorf("FirstSeen changed on re-declaration: %v -> %v, want preserved", p1.FirstSeen, p2.FirstSeen)
	}
	if p2.TotalCount != 2 {
		t.Errorf("TotalCount = %d, want 2 after a second declaration", p2.TotalCount)
	}

	all, err := ps.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(all) != 1 {
		t.Fatalf("Load returned %d patterns, want exactly 1 (upsert, not duplicate)", len(all))
	}
}

// TestAnalyzerRun_PreservesDeclaredPatternAcrossReanalysis is the regression
// test for the actual bug this feature closes: Analyzer.Run's periodic
// statistical recomputation used to Save() a wholesale-replacement pattern
// list computed only from passively-observed rows — which would silently
// wipe out a declared habit the very next time the analyzer ran, since it
// has no corresponding Observation rows backing it.
func TestAnalyzerRun_PreservesDeclaredPatternAcrossReanalysis(t *testing.T) {
	store := newTestStore(t)
	ps := NewPatternStore(filepath.Join(t.TempDir(), "patterns.json"))
	an := NewAnalyzer(store, ps)
	ctx := context.Background()

	habitTime := time.Date(2026, 6, 15, 21, 0, 0, 0, time.Local)
	declared, err := ps.SaveDeclared(DeclaredHabitPattern("kod yazma", habitTime, nil))
	if err != nil {
		t.Fatalf("SaveDeclared: %v", err)
	}

	// Some unrelated, sparse observed activity — nowhere near
	// MinObservations for its own pattern, and nothing to do with the
	// declared habit above.
	mustRecord(t, store, Observation{Timestamp: time.Now(), ActivityType: ActivityChat, Topic: "general"})

	if err := an.Run(ctx); err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	all, err := ps.Load()
	if err != nil {
		t.Fatalf("Load after Run: %v", err)
	}
	got, ok := findPattern(all, declared.ID)
	if !ok {
		t.Fatal("declared pattern was wiped by Analyzer.Run's statistical recomputation")
	}
	if !got.Declared {
		t.Error("surviving pattern lost its Declared flag across re-analysis")
	}
	if got.Confidence != declared.Confidence {
		t.Errorf("Confidence changed across re-analysis: %v -> %v, want unchanged", declared.Confidence, got.Confidence)
	}

	// Suppressing a declared pattern must still work, exactly like a
	// statistically-observed one — the merge-back must respect it.
	if _, err := ps.Suppress(declared.ID); err != nil {
		t.Fatalf("Suppress: %v", err)
	}
	if err := an.Run(ctx); err != nil {
		t.Fatalf("second Run() error = %v", err)
	}
	all, err = ps.Load()
	if err != nil {
		t.Fatalf("Load after second Run: %v", err)
	}
	if _, ok := findPattern(all, declared.ID); ok {
		t.Error("suppressed declared pattern was resurrected by re-analysis merge-back")
	}
}
