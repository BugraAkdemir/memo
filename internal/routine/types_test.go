// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"testing"
	"time"
)

// TestRoutine_LastGeneratedDate covers TD-5: LastGeneratedForDate was removed
// as a redundant field (always set together with LastGeneratedAt, from the
// same value, in the same code path) in favor of deriving it on demand.
func TestRoutine_LastGeneratedDate(t *testing.T) {
	var zero Routine
	if got := zero.LastGeneratedDate(); got != "" {
		t.Errorf("zero-value Routine.LastGeneratedDate() = %q, want empty (never generated)", got)
	}

	r := Routine{LastGeneratedAt: time.Date(2026, 7, 17, 18, 30, 0, 0, time.UTC)}
	if got := r.LastGeneratedDate(); got != "2026-07-17" {
		t.Errorf("LastGeneratedDate() = %q, want 2026-07-17", got)
	}
}
