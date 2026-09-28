package webserver

import (
	"testing"
	"time"
)

func TestStartOfLocalDay_IsLocalMidnightNotUTC(t *testing.T) {
	ist := time.FixedZone("TRT", 3*60*60)
	now := time.Date(2026, 9, 29, 10, 0, 0, 0, ist) // 07:00 UTC
	got := startOfLocalDay(now)
	want := time.Date(2026, 9, 29, 0, 0, 0, 0, ist)
	if !got.Equal(want) {
		t.Fatalf("startOfLocalDay(%v) = %v, want %v", now, got, want)
	}
	// The old Truncate(24h) answer: 03:00 local — an event at 00:30 local
	// today fell outside the default window.
	if early := time.Date(2026, 9, 29, 0, 30, 0, 0, ist); early.Before(got) {
		t.Errorf("an event at 00:30 local today is before the window start %v", got)
	}
}
