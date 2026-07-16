// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"context"
	"fmt"
	"memo/internal/logx"
	"math"
	"sort"
	"time"
)

// frequencySaturation is the number of distinct active days (or recency-weighted
// occurrences) at which the frequency / volume factor reaches 1.0. Two weeks of
// near-daily activity counts as a fully established habit.
const frequencySaturation = 14.0

// Analyzer turns raw observations into TimePatterns and persists them. It is the
// "Analysis Layer" of the learning system and is meant to run periodically
// (hourly) in the background.
type Analyzer struct {
	store    *Store
	patterns *PatternStore
}

// NewAnalyzer builds an analyzer over an observation store and a pattern store.
func NewAnalyzer(store *Store, patterns *PatternStore) *Analyzer {
	return &Analyzer{store: store, patterns: patterns}
}

// Run reads the recent observation window, detects patterns, and persists them.
// It is safe to call on a nil/zero analyzer (returns nil) so callers need not
// guard every invocation.
func (a *Analyzer) Run(ctx context.Context) error {
	if a == nil || a.store == nil || a.patterns == nil {
		return nil
	}
	now := time.Now()
	obs, err := a.store.Query(ctx, QueryFilter{Since: now.Add(-AnalysisWindow)})
	if err != nil {
		return fmt.Errorf("observer.Analyzer.Run: query: %w", err)
	}
	patterns := AnalyzePatterns(obs, now)

	suppressed, serr := a.patterns.SuppressedSet()
	if serr != nil {
		suppressed = nil
	}

	// Drop any patterns the user explicitly retired, so they are not re-learned.
	if len(suppressed) > 0 {
		kept := patterns[:0]
		for _, p := range patterns {
			if !suppressed[p.ID] {
				kept = append(kept, p)
			}
		}
		patterns = kept
	}

	// Declared (explicitly stated) habits are a separate, guaranteed layer —
	// see TimePattern.Declared — that this statistical recomputation knows
	// nothing about, since it only reads passively-observed rows from the
	// Store. Re-merge them back in (still honoring suppression above) so a
	// declared habit survives every periodic analyzer run indefinitely,
	// instead of being wiped by the next wholesale Save below.
	if existing, lerr := a.patterns.Load(); lerr == nil {
		for _, p := range existing {
			if p.Declared && !suppressed[p.ID] {
				patterns = append(patterns, p)
			}
		}
	}

	if err := a.patterns.Save(patterns); err != nil {
		return fmt.Errorf("observer.Analyzer.Run: save: %w", err)
	}
	logx.Printf("OBSERVER: analyzed %d observations into %d pattern(s)", len(obs), len(patterns))
	return nil
}

// patternKey is the behavioural unit a pattern is built around: the topic when
// present (e.g. "coding"), otherwise the raw activity type (e.g. "session").
func patternKey(o Observation) string {
	if o.Topic != "" {
		return o.Topic
	}
	return o.ActivityType
}

// AnalyzePatterns is the pure pattern-detection core: given a set of
// observations and the current time, it returns the detected TimePatterns. It
// performs no I/O and is the primary unit under test.
func AnalyzePatterns(obs []Observation, now time.Time) []TimePattern {
	groups := make(map[string][]Observation)
	for _, o := range obs {
		// Sessions carry length, not time-of-day intent; still group them, but
		// skip rows with no usable signal.
		key := patternKey(o)
		if key == "" {
			continue
		}
		groups[key] = append(groups[key], o)
	}

	patterns := make([]TimePattern, 0, len(groups))
	for key, items := range groups {
		if p, ok := buildTimePattern(key, items, now); ok {
			patterns = append(patterns, p)
		}
	}

	// Deterministic ordering: strongest patterns first, then by id for stability.
	sort.Slice(patterns, func(i, j int) bool {
		if patterns[i].Confidence != patterns[j].Confidence {
			return patterns[i].Confidence > patterns[j].Confidence
		}
		return patterns[i].ID < patterns[j].ID
	})
	return patterns
}

// buildTimePattern computes a single pattern from a group's observations.
func buildTimePattern(key string, items []Observation, now time.Time) (TimePattern, bool) {
	if len(items) < MinObservations {
		return TimePattern{}, false
	}

	var (
		sumCos, sumSin float64
		daysActive     [7]bool
		firstSeen      = items[0].Timestamp
		lastSeen       = items[0].Timestamp
		distinctDays   = make(map[string]struct{})
		weightedScore  float64
	)

	for _, o := range items {
		theta := 2 * math.Pi * float64(o.TimeOfDaySeconds) / float64(secondsPerDay)
		sumCos += math.Cos(theta)
		sumSin += math.Sin(theta)

		if o.DayOfWeek >= 0 && o.DayOfWeek < 7 {
			daysActive[o.DayOfWeek] = true
		}
		if o.Timestamp.Before(firstSeen) {
			firstSeen = o.Timestamp
		}
		if o.Timestamp.After(lastSeen) {
			lastSeen = o.Timestamp
		}
		distinctDays[o.Timestamp.Local().Format("2006-01-02")] = struct{}{}

		daysSince := now.Sub(o.Timestamp).Hours() / 24.0
		if daysSince < 0 {
			daysSince = 0
		}
		weightedScore += math.Pow(DailyDecay, daysSince)
	}

	n := float64(len(items))
	meanAngle := math.Atan2(sumSin, sumCos)
	if meanAngle < 0 {
		meanAngle += 2 * math.Pi
	}
	peakSec := int(math.Round(meanAngle / (2 * math.Pi) * float64(secondsPerDay)))
	peakSec = ((peakSec % secondsPerDay) + secondsPerDay) % secondsPerDay

	// R is the circular resultant length in [0,1]: 1 = perfectly consistent
	// timing, 0 = uniformly spread across the day.
	r := math.Sqrt(sumCos*sumCos+sumSin*sumSin) / n
	consistency := clamp01(r)
	stdDev := circularStdDevSeconds(r)

	// frequency: how many distinct days the habit appeared on, saturating.
	frequency := clamp01(float64(len(distinctDays)) / frequencySaturation)

	// recency: decay of the single most recent occurrence.
	recency := ApplyDecay(1.0, lastSeen, now)

	confidence := clamp01(consistency * frequency * recency)
	weighted := clamp01(weightedScore / frequencySaturation)

	return TimePattern{
		ID:              "time:" + key,
		ActivityType:    key,
		TimePeakSeconds: peakSec,
		StdDevSeconds:   stdDev,
		Confidence:      confidence,
		DaysActive:      daysActive,
		TotalCount:      len(items),
		FirstSeen:       firstSeen,
		LastSeen:        lastSeen,
		WeightedScore:   weighted,
	}, true
}

// circularStdDevSeconds converts a resultant length R into a standard deviation
// in seconds on the 24h circle, clamped to a sensible range.
func circularStdDevSeconds(r float64) int {
	if r >= 1.0 {
		return minStdDevSeconds
	}
	if r <= 0.0 {
		return maxStdDevSeconds
	}
	stdRad := math.Sqrt(-2 * math.Log(r))
	stdSec := stdRad / (2 * math.Pi) * float64(secondsPerDay)
	if stdSec < minStdDevSeconds {
		return minStdDevSeconds
	}
	if stdSec > maxStdDevSeconds {
		return maxStdDevSeconds
	}
	return int(math.Round(stdSec))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
