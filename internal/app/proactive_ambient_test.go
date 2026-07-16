// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/observer"
	"memo/internal/proactive"
)

// newAmbientTestApp builds a minimal but fully wired App for exercising the
// ambient nudge functions: real (temp-file-backed) PatternStore/PendingStore/
// Engine, a proactive-enabled config, and MinimalMode off unless the test
// says otherwise.
func newAmbientTestApp(t *testing.T, minimalMode bool) (*App, *observer.PatternStore) {
	t.Helper()
	dir := t.TempDir()
	patterns := observer.NewPatternStore(filepath.Join(dir, "patterns.json"))
	pending := proactive.NewPendingStore(filepath.Join(dir, "pending.json"))

	a := &App{
		cfg: &config.AppConfig{
			Proactive: config.ProactiveConfig{Enabled: true, Level: "subtle"},
		},
		identity:         identity.New("Test", "Memo", "casual", "", minimalMode),
		observerPatterns: patterns,
		proactivePending: pending,
	}
	a.proactiveEngine = proactive.NewEngine(proactive.Config{}, patterns, pending, nil, nil, a.proactiveLevel)
	return a, patterns
}

// activePattern seeds a pattern that matches "now" strongly: today's weekday
// active, peak time-of-day equal to now, tight std dev, high confidence.
func activePattern(id, activityType string, now time.Time) observer.TimePattern {
	var days [7]bool
	days[int(now.Weekday())] = true
	return observer.TimePattern{
		ID:              id,
		ActivityType:    activityType,
		TimePeakSeconds: now.Hour()*3600 + now.Minute()*60 + now.Second(),
		StdDevSeconds:   600,
		Confidence:      0.9,
		DaysActive:      days,
		WeightedScore:   1,
		LastSeen:        now,
	}
}

func findTestPattern(ps []observer.TimePattern, id string) (observer.TimePattern, bool) {
	for _, p := range ps {
		if p.ID == id {
			return p, true
		}
	}
	return observer.TimePattern{}, false
}

func TestBuildProactiveNudgeBlock_RespectsMinimalMode(t *testing.T) {
	a, patterns := newAmbientTestApp(t, true) // MinimalMode ON
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	if block := a.buildProactiveNudgeBlock(now); block != "" {
		t.Errorf("expected no nudge block in MinimalMode, got %q", block)
	}
	if got := a.takeNudgedPattern(); got != nil {
		t.Errorf("MinimalMode must not record a nudged pattern either, got %+v", got)
	}
}

func TestBuildProactiveNudgeBlock_RespectsProactiveDisabled(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	a.cfg.Proactive.Enabled = false
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	if block := a.buildProactiveNudgeBlock(now); block != "" {
		t.Errorf("expected no nudge block when Proactive.Enabled=false, got %q", block)
	}
}

func TestBuildProactiveNudgeBlock_NoMatchReturnsEmpty(t *testing.T) {
	a, _ := newAmbientTestApp(t, false)
	if block := a.buildProactiveNudgeBlock(time.Now()); block != "" {
		t.Errorf("expected no nudge block with no patterns at all, got %q", block)
	}
}

func TestBuildProactiveNudgeBlock_MatchReturnsBlockAndRecordsPattern(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}

	block := a.buildProactiveNudgeBlock(now)
	if block == "" {
		t.Fatal("expected a nudge block for a strongly-matching pattern, got empty string")
	}
	if !strings.Contains(block, "kod yazma") {
		t.Errorf("nudge block = %q, want it to mention the pattern's ActivityType", block)
	}
	got := a.takeNudgedPattern()
	if got == nil || got.ID != "time:coding" {
		t.Errorf("takeNudgedPattern() = %+v, want the matched pattern (time:coding)", got)
	}
}

func TestBuildProactiveNudgeBlock_PendingSuppressesNudge(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	if err := a.proactivePending.Set(proactive.PendingSuggestion{ID: "x", Message: "m", PatternID: "time:coding", Action: proactive.ActionSuggest, CreatedAt: now}); err != nil {
		t.Fatalf("Set pending: %v", err)
	}

	if block := a.buildProactiveNudgeBlock(now); block != "" {
		t.Errorf("expected no nudge block while a suggestion is already pending, got %q", block)
	}
}

func TestCheckAmbientNudgeSurfaced_YesSetsPending(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	p := activePattern("time:coding", "kod yazma", now)
	if err := patterns.Save([]observer.TimePattern{p}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	a.providerRouter = newExtractionTestRouter(t, "YES")
	a.activeProviderName = "test"

	a.checkAmbientNudgeSurfaced(&p, "Kanka kod vakti değil mi, yazalım mı?")

	pending, err := a.proactivePending.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if pending == nil {
		t.Fatal("expected a pending suggestion after a YES verdict")
	}
	if pending.PatternID != "time:coding" || pending.Action != proactive.ActionAmbient {
		t.Errorf("pending = %+v, want PatternID=time:coding Action=ambient", pending)
	}
}

func TestCheckAmbientNudgeSurfaced_NoDoesNotSetPending(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	p := activePattern("time:coding", "kod yazma", now)
	if err := patterns.Save([]observer.TimePattern{p}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	a.providerRouter = newExtractionTestRouter(t, "NO")
	a.activeProviderName = "test"

	a.checkAmbientNudgeSurfaced(&p, "Bugün hava çok güzel, dışarı çıkalım mı?")

	if pending, err := a.proactivePending.Get(); err != nil || pending != nil {
		t.Errorf("expected no pending suggestion after a NO verdict, got %+v (err=%v)", pending, err)
	}
}

func TestCheckAmbientNudgeSurfaced_RespectsMinimalMode(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	p := activePattern("time:coding", "kod yazma", now)
	if err := patterns.Save([]observer.TimePattern{p}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	// MinimalMode gets turned on *after* the nudge was offered but *before*
	// the reply finished — the exact gap this test targets.
	a.identity.SetMinimalMode(true)
	a.providerRouter = newExtractionTestRouter(t, "YES")
	a.activeProviderName = "test"

	a.checkAmbientNudgeSurfaced(&p, "Kanka kod vakti değil mi, yazalım mı?")

	if pending, err := a.proactivePending.Get(); err != nil || pending != nil {
		t.Errorf("expected no pending suggestion once MinimalMode is on, got %+v (err=%v)", pending, err)
	}
}

func TestCheckAmbientNudgeSurfaced_SkipsLLMErrorReply(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	p := activePattern("time:coding", "kod yazma", now)
	if err := patterns.Save([]observer.TimePattern{p}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	// No provider configured at all — if this reached callLLM it would be a
	// test failure via panic/nil deref, not just a wrong verdict; asserting
	// no pending suggestion appears confirms the isLLMErrorReply guard
	// short-circuits before ever calling the model.
	a.checkAmbientNudgeSurfaced(&p, "⚠️ Yerel model yüklenmemiş.")

	if pending, err := a.proactivePending.Get(); err != nil || pending != nil {
		t.Errorf("expected an error-string reply to be skipped entirely, got %+v (err=%v)", pending, err)
	}
}

func TestCheckAmbientNudgeOutcome_AcceptRaisesConfidenceAndClears(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	if err := a.proactivePending.Set(proactive.PendingSuggestion{
		ID: "sugg-1", Message: "kod yazma", PatternID: "time:coding", Action: proactive.ActionAmbient, CreatedAt: now,
	}); err != nil {
		t.Fatalf("Set pending: %v", err)
	}
	a.providerRouter = newExtractionTestRouter(t, "ACCEPT")
	a.activeProviderName = "test"

	a.checkAmbientNudgeOutcome("evet abi hadi başlayalım")

	all, err := patterns.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, ok := findTestPattern(all, "time:coding")
	if !ok {
		t.Fatal("pattern disappeared")
	}
	if got.Confidence <= 0.9 {
		t.Errorf("Confidence = %.2f, want risen above the seeded 0.9", got.Confidence)
	}
	if pending, _ := a.proactivePending.Get(); pending != nil {
		t.Error("expected pending to be cleared after ACCEPT")
	}
}

func TestCheckAmbientNudgeOutcome_UnclearLeavesPendingUntouched(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	if err := a.proactivePending.Set(proactive.PendingSuggestion{
		ID: "sugg-1", Message: "kod yazma", PatternID: "time:coding", Action: proactive.ActionAmbient, CreatedAt: now,
	}); err != nil {
		t.Fatalf("Set pending: %v", err)
	}
	a.providerRouter = newExtractionTestRouter(t, "UNCLEAR")
	a.activeProviderName = "test"

	a.checkAmbientNudgeOutcome("bu arada yarın toplantı var mıydı?")

	if pending, err := a.proactivePending.Get(); err != nil || pending == nil {
		t.Errorf("expected the pending suggestion to survive an UNCLEAR verdict, got %+v (err=%v)", pending, err)
	}
	all, err := patterns.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	got, ok := findTestPattern(all, "time:coding")
	if !ok || got.Confidence != 0.9 {
		t.Errorf("confidence should be unchanged by an UNCLEAR verdict, got %+v", got)
	}
}

// TestCheckAmbientNudgeOutcome_IgnoresNonAmbientPending is the regression
// test for the most severe bug caught during review: a pending suggestion
// from the *background engine's* own tick (a formal banner the frontend
// shows with explicit Accept/Decline controls) must never be resolved by an
// ordinary chat message the user happened to send while it was showing —
// only Action == ActionAmbient suggestions are fair game here.
func TestCheckAmbientNudgeOutcome_IgnoresNonAmbientPending(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	if err := a.proactivePending.Set(proactive.PendingSuggestion{
		ID: "banner-1", Message: "kod yazma", PatternID: "time:coding", Action: proactive.ActionSuggest, CreatedAt: now,
	}); err != nil {
		t.Fatalf("Set pending: %v", err)
	}
	// If this reached callLLM with no provider configured, it would panic —
	// asserting the banner survives untouched confirms the Action check
	// short-circuits before any LLM call or pending-store mutation.
	a.checkAmbientNudgeOutcome("hayır bugün olmaz, başka bir şey soracaktım")

	pending, err := a.proactivePending.Get()
	if err != nil || pending == nil || pending.ID != "banner-1" {
		t.Errorf("expected the banner suggestion to survive untouched, got %+v (err=%v)", pending, err)
	}
}

func TestCheckAmbientNudgeOutcome_RespectsMinimalMode(t *testing.T) {
	a, patterns := newAmbientTestApp(t, false)
	now := time.Now()
	if err := patterns.Save([]observer.TimePattern{activePattern("time:coding", "kod yazma", now)}); err != nil {
		t.Fatalf("seed Save: %v", err)
	}
	if err := a.proactivePending.Set(proactive.PendingSuggestion{
		ID: "sugg-1", Message: "kod yazma", PatternID: "time:coding", Action: proactive.ActionAmbient, CreatedAt: now,
	}); err != nil {
		t.Fatalf("Set pending: %v", err)
	}
	// MinimalMode turned on *after* the ambient suggestion was armed —
	// resolving it still costs an LLM call/prompt injection unless this
	// path re-checks MinimalMode too, not just the offering side.
	a.identity.SetMinimalMode(true)

	a.checkAmbientNudgeOutcome("evet hadi başlayalım")

	if pending, err := a.proactivePending.Get(); err != nil || pending == nil {
		t.Errorf("expected the pending suggestion untouched (not resolved) once MinimalMode is on, got %+v (err=%v)", pending, err)
	}
}
