// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func newTestLoop(t *testing.T, generate GenerateFn, deliver DeliverFn) (*RoutineLoop, *Store) {
	t.Helper()
	st, err := NewStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	loop := NewRoutineLoop(st, generate, deliver, func(name, data string) {})
	return loop, st
}

func TestTick_WhatsAppOnly_GeneratesAndDeliversAtFireTime(t *testing.T) {
	var generateCalls, deliverCalls int
	var deliveredContent string

	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "hello", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error {
		deliverCalls++
		deliveredContent = content
		return nil
	}

	loop, st := newTestLoop(t, generate, deliver)
	created, err := st.Create(Routine{
		Schedule:         Schedule{TimeOfDay: "08:00"},
		DeliveryWhatsApp: true,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	now := time.Date(2026, 7, 17, 8, 1, 0, 0, time.Local)
	loop.tick(context.Background(), now)
	loop.waitIdle()

	if generateCalls != 1 {
		t.Errorf("generateCalls = %d, want 1", generateCalls)
	}
	if deliverCalls != 1 {
		t.Errorf("deliverCalls = %d, want 1", deliverCalls)
	}
	if deliveredContent != "hello" {
		t.Errorf("deliveredContent = %q, want %q", deliveredContent, "hello")
	}

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastRunDate != "2026-07-17" {
		t.Errorf("LastRunDate = %q, want 2026-07-17", got.LastRunDate)
	}
}

func TestTick_SameDayGuard_DoesNotRefireSameDay(t *testing.T) {
	var generateCalls, deliverCalls int
	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "hello", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error {
		deliverCalls++
		return nil
	}

	loop, st := newTestLoop(t, generate, deliver)
	if _, err := st.Create(Routine{
		Schedule:         Schedule{TimeOfDay: "08:00"},
		DeliveryWhatsApp: true,
		Enabled:          true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 8, 1, 0, 0, time.Local))
	loop.waitIdle()
	loop.tick(context.Background(), time.Date(2026, 7, 17, 9, 0, 0, 0, time.Local))
	loop.waitIdle()

	if generateCalls != 1 {
		t.Errorf("generateCalls = %d after two same-day ticks, want 1", generateCalls)
	}
	if deliverCalls != 1 {
		t.Errorf("deliverCalls = %d after two same-day ticks, want 1", deliverCalls)
	}
}

func TestTick_MobileOnly_GeneratesEarlyButWaitsToMarkDone(t *testing.T) {
	var generateCalls, deliverCalls int
	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "akşam raporu", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error {
		deliverCalls++
		return nil
	}

	loop, st := newTestLoop(t, generate, deliver)
	created, err := st.Create(Routine{
		Schedule:       Schedule{TimeOfDay: "20:00"},
		DeliveryMobile: true,
		Enabled:        true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// 18:30 is within the 2h mobile lead of 20:00: content should be
	// generated now so the phone can pre-schedule a local notification, but
	// the routine must not be marked "done for today" before its real fire
	// time — a later same-day tick still needs to see it as due.
	loop.tick(context.Background(), time.Date(2026, 7, 17, 18, 30, 0, 0, time.Local))
	loop.waitIdle()

	if generateCalls != 1 {
		t.Errorf("generateCalls = %d, want 1 (should generate within mobile lead window)", generateCalls)
	}
	if deliverCalls != 0 {
		t.Errorf("deliverCalls = %d, want 0 (no WhatsApp channel configured)", deliverCalls)
	}
	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastRunDate != "" {
		t.Errorf("LastRunDate = %q, want empty (fire time not reached yet)", got.LastRunDate)
	}
	if got.LastGeneratedContent != "akşam raporu" {
		t.Errorf("LastGeneratedContent = %q, want pre-generated content", got.LastGeneratedContent)
	}

	// Now past the real fire time: should not regenerate, should mark done.
	loop.tick(context.Background(), time.Date(2026, 7, 17, 20, 1, 0, 0, time.Local))
	loop.waitIdle()
	if generateCalls != 1 {
		t.Errorf("generateCalls = %d after fire time, want still 1 (no regeneration)", generateCalls)
	}
	got, err = st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastRunDate != "2026-07-17" {
		t.Errorf("LastRunDate = %q, want 2026-07-17 after fire time reached", got.LastRunDate)
	}
}

// TestTick_SlowRoutineDoesNotBlockOthers is the regression test for BUG-H1:
// tick() used to process due routines one at a time inline, so a slow
// GenerateFn call for one routine delayed every other due routine in the
// same tick (and, in Start's ticker loop, blocked the next minute's tick
// entirely). Firing each routine in its own goroutine means a slow one
// finishing after a fast one doesn't hold the fast one back.
func TestTick_SlowRoutineDoesNotBlockOthers(t *testing.T) {
	slowStarted := make(chan struct{})
	slowRelease := make(chan struct{})
	fastDone := make(chan struct{}, 1)

	generate := func(ctx context.Context, r Routine) (string, error) {
		if r.Prompt == "slow" {
			close(slowStarted)
			<-slowRelease // held open until the test explicitly releases it
			return "slow done", nil
		}
		fastDone <- struct{}{}
		return "fast done", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error { return nil }

	loop, st := newTestLoop(t, generate, deliver)
	if _, err := st.Create(Routine{Prompt: "slow", Schedule: Schedule{TimeOfDay: "08:00"}, DeliveryWhatsApp: true, Enabled: true}); err != nil {
		t.Fatalf("Create slow: %v", err)
	}
	if _, err := st.Create(Routine{Prompt: "fast", Schedule: Schedule{TimeOfDay: "08:00"}, DeliveryWhatsApp: true, Enabled: true}); err != nil {
		t.Fatalf("Create fast: %v", err)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 8, 1, 0, 0, time.Local))

	<-slowStarted // the slow routine is confirmed blocked inside generate...
	select {
	case <-fastDone:
		// ...but the fast routine still completed without waiting for it.
	case <-time.After(2 * time.Second):
		t.Fatal("BUG-H1 regressed: fast routine never ran while slow routine was still in flight")
	}

	close(slowRelease)
	loop.waitIdle()
}

// TestTick_StillRunningRoutineNotRelaunchedByNextTick covers the dedup guard
// tryMarkRunning/clearRunning add: same-day dedup via LastRunDate is only
// written once a routine's run fully completes, so without an in-flight
// guard a routine still mid-generate on tick N could be launched a second
// time, concurrently, by tick N+1.
func TestTick_StillRunningRoutineNotRelaunchedByNextTick(t *testing.T) {
	var generateCalls atomic.Int32
	started := make(chan struct{})
	release := make(chan struct{})

	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls.Add(1)
		close(started)
		<-release
		return "done", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error { return nil }

	loop, st := newTestLoop(t, generate, deliver)
	if _, err := st.Create(Routine{Schedule: Schedule{TimeOfDay: "08:00"}, DeliveryWhatsApp: true, Enabled: true}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 8, 1, 0, 0, time.Local))
	<-started
	// A second tick before the first run has released — the routine is
	// still "running" and must not be launched again.
	loop.tick(context.Background(), time.Date(2026, 7, 17, 8, 2, 0, 0, time.Local))

	close(release)
	loop.waitIdle()

	if got := generateCalls.Load(); got != 1 {
		t.Errorf("generateCalls = %d, want 1 (second tick should have skipped the still-running routine)", got)
	}
}

func TestTick_DisabledRoutine_NeverFires(t *testing.T) {
	var generateCalls int
	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "x", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error { return nil }

	loop, st := newTestLoop(t, generate, deliver)
	if _, err := st.Create(Routine{
		Schedule:         Schedule{TimeOfDay: "08:00"},
		DeliveryWhatsApp: true,
		Enabled:          false,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 9, 0, 0, 0, time.Local))
	loop.waitIdle()
	if generateCalls != 0 {
		t.Errorf("generateCalls = %d, want 0 for a disabled routine", generateCalls)
	}
}
