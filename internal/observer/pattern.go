// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// Decay & analysis constants (see README §4–§5).
const (
	// AnalysisWindow is the sliding window of observations considered. Older
	// rows are ignored by the analyzer and may be pruned from storage.
	AnalysisWindow = 30 * 24 * time.Hour

	// DailyDecay is the per-day multiplier applied to a pattern's recency
	// weight: 0.95^30 ≈ 0.21, so a habit not seen for a month nearly vanishes.
	DailyDecay = 0.95

	// MinObservations is the minimum number of samples before a pattern is
	// emitted (cold start, README §10: 3+ repeats).
	MinObservations = 3

	// secondsPerDay is the length of the time-of-day circle.
	secondsPerDay = 24 * 3600

	// minStdDevSeconds / maxStdDevSeconds clamp the derived spread so a single
	// tight cluster is not treated as infinitely precise, nor a scattered one
	// as covering the whole day.
	minStdDevSeconds = 10 * 60  // 10 minutes
	maxStdDevSeconds = 6 * 3600 // 6 hours
)

// TimePattern is a learned habit: an activity that recurs around a particular
// time of day on particular weekdays. See README §4.1.
type TimePattern struct {
	ID              string    `json:"id"`
	ActivityType    string    `json:"activity_type"`
	TimePeakSeconds int       `json:"time_peak_seconds"`
	StdDevSeconds   int       `json:"std_dev_seconds"`
	Confidence      float64   `json:"confidence"`
	DaysActive      [7]bool   `json:"days_active"`
	TotalCount      int       `json:"total_count"`
	FirstSeen       time.Time `json:"first_seen"`
	LastSeen        time.Time `json:"last_seen"`
	WeightedScore   float64   `json:"weighted_score"`
}

// ApplyDecay returns a confidence reduced for elapsed time since lastSeen,
// matching the README's exponential-decay formula. Phase 3's matcher uses this
// to keep stale patterns from firing between analyzer runs.
func ApplyDecay(confidence float64, lastSeen, now time.Time) float64 {
	daysSince := now.Sub(lastSeen).Hours() / 24.0
	if daysSince < 0 {
		daysSince = 0
	}
	return confidence * math.Pow(DailyDecay, daysSince)
}

// patternFile is the on-disk envelope for persisted patterns.
//
// Suppressed holds pattern ids the user explicitly retired ("Artık yapmıyorum"
// / Forget). The analyzer must skip these so a retired habit is not silently
// re-learned on the next analysis pass.
type patternFile struct {
	Version    int           `json:"version"`
	UpdatedAt  time.Time     `json:"updated_at"`
	Patterns   []TimePattern `json:"patterns"`
	Suppressed []string      `json:"suppressed,omitempty"`
}

const patternFileVersion = 1

// PatternStore persists learned patterns as JSON (data/profile/patterns.json).
// It is safe for concurrent use: the analyzer writes while the UI/engine reads.
type PatternStore struct {
	path string
	mu   sync.RWMutex
}

// NewPatternStore returns a store backed by the given file path.
func NewPatternStore(path string) *PatternStore {
	return &PatternStore{path: path}
}

// readFile loads the whole envelope. A missing file yields a zero envelope (not
// an error). Callers must hold the lock.
func (ps *PatternStore) readFile() (patternFile, error) {
	data, err := os.ReadFile(ps.path)
	if err != nil {
		if os.IsNotExist(err) {
			return patternFile{}, nil
		}
		return patternFile{}, err
	}
	var pf patternFile
	if len(data) == 0 {
		return patternFile{}, nil
	}
	if err := json.Unmarshal(data, &pf); err != nil {
		return patternFile{}, err
	}
	return pf, nil
}

// writeFile atomically persists the envelope. Callers must hold the lock.
func (ps *PatternStore) writeFile(pf patternFile) error {
	if err := os.MkdirAll(filepath.Dir(ps.path), 0o755); err != nil {
		return err
	}
	pf.Version = patternFileVersion
	pf.UpdatedAt = time.Now()
	data, err := json.MarshalIndent(pf, "", "  ")
	if err != nil {
		return err
	}
	tmp := ps.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, ps.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Load reads the persisted patterns. A missing file is not an error — it yields
// an empty slice (cold start).
func (ps *PatternStore) Load() ([]TimePattern, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	pf, err := ps.readFile()
	if err != nil {
		return nil, fmt.Errorf("observer.PatternStore.Load: %w", err)
	}
	return pf.Patterns, nil
}

// Save atomically writes the patterns to disk, preserving the existing
// suppression list (so re-analysis does not resurrect retired patterns).
func (ps *PatternStore) Save(patterns []TimePattern) error {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	existing, err := ps.readFile()
	if err != nil {
		return fmt.Errorf("observer.PatternStore.Save: read: %w", err)
	}
	if err := ps.writeFile(patternFile{Patterns: patterns, Suppressed: existing.Suppressed}); err != nil {
		return fmt.Errorf("observer.PatternStore.Save: %w", err)
	}
	return nil
}

// SuppressedSet returns the set of retired pattern ids.
func (ps *PatternStore) SuppressedSet() (map[string]bool, error) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()
	pf, err := ps.readFile()
	if err != nil {
		return nil, fmt.Errorf("observer.PatternStore.SuppressedSet: %w", err)
	}
	set := make(map[string]bool, len(pf.Suppressed))
	for _, id := range pf.Suppressed {
		set[id] = true
	}
	return set, nil
}

// Suppress retires a pattern: it is removed and its id recorded so the analyzer
// will not re-learn it. Returns false if the pattern was not present (it is
// still added to the suppression list, making Suppress idempotent and effective
// even if the pattern reappears between calls).
func (ps *PatternStore) Suppress(id string) (bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	pf, err := ps.readFile()
	if err != nil {
		return false, fmt.Errorf("observer.PatternStore.Suppress: %w", err)
	}
	kept := make([]TimePattern, 0, len(pf.Patterns))
	found := false
	for _, p := range pf.Patterns {
		if p.ID == id {
			found = true
			continue
		}
		kept = append(kept, p)
	}
	pf.Patterns = kept
	if !slices.Contains(pf.Suppressed, id) {
		pf.Suppressed = append(pf.Suppressed, id)
	}
	if err := ps.writeFile(pf); err != nil {
		return false, fmt.Errorf("observer.PatternStore.Suppress: %w", err)
	}
	return found, nil
}

// Clear removes the pattern file entirely, including the suppression list. Backs
// the "delete all my data" control.
func (ps *PatternStore) Clear() error {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	if err := os.Remove(ps.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("observer.PatternStore.Clear: %w", err)
	}
	return nil
}

// AdjustConfidence nudges a pattern's stored confidence by delta (clamped to
// [0,1]) and refreshes its LastSeen so the change is not immediately undone by
// decay. Used by the feedback loop (README §9). Returns false if no pattern with
// the id exists.
//
// The read-modify-write runs under a single lock so a concurrent analyzer save
// cannot clobber the adjustment (and vice versa). The suppression list is
// preserved.
func (ps *PatternStore) AdjustConfidence(id string, delta float64) (bool, error) {
	ps.mu.Lock()
	defer ps.mu.Unlock()
	pf, err := ps.readFile()
	if err != nil {
		return false, fmt.Errorf("observer.PatternStore.AdjustConfidence: %w", err)
	}
	found := false
	for i := range pf.Patterns {
		if pf.Patterns[i].ID == id {
			pf.Patterns[i].Confidence = clampUnit(pf.Patterns[i].Confidence + delta)
			pf.Patterns[i].LastSeen = time.Now()
			found = true
			break
		}
	}
	if !found {
		return false, nil
	}
	if err := ps.writeFile(pf); err != nil {
		return false, fmt.Errorf("observer.PatternStore.AdjustConfidence: %w", err)
	}
	return true, nil
}

func clampUnit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

// LoadActive returns persisted patterns whose decayed confidence (as of now) is
// at least minConfidence. This is the read path the proactive engine uses.
func (ps *PatternStore) LoadActive(now time.Time, minConfidence float64) ([]TimePattern, error) {
	all, err := ps.Load()
	if err != nil {
		return nil, err
	}
	var active []TimePattern
	for _, p := range all {
		if ApplyDecay(p.Confidence, p.LastSeen, now) >= minConfidence {
			active = append(active, p)
		}
	}
	return active, nil
}
