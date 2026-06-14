// SPDX-License-Identifier: AGPL-3.0-or-later

package observer

import (
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
