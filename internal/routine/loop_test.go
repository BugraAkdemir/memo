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
	loop := NewRoutineLoop(st, generate, deliver)
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

// TestTick_TelegramOnly_GeneratesAndDeliversAtFireTime mirrors
// TestTick_WhatsAppOnly_GeneratesAndDeliversAtFireTime for the Telegram
// channel — both fire at the exact scheduled time now that Mobile (the only
// channel that ever needed an early-generation lead) is gone.
func TestTick_TelegramOnly_GeneratesAndDeliversAtFireTime(t *testing.T) {
	var generateCalls, deliverCalls int
	var deliveredContent string

	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "akşam raporu", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error {
		deliverCalls++
		deliveredContent = content
		return nil
	}

	loop, st := newTestLoop(t, generate, deliver)
	created, err := st.Create(Routine{
		Schedule:         Schedule{TimeOfDay: "20:00"},
		DeliveryTelegram: true,
		Enabled:          true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// Before fire time: not due yet.
	loop.tick(context.Background(), time.Date(2026, 7, 17, 18, 30, 0, 0, time.Local))
	loop.waitIdle()
	if generateCalls != 0 {
		t.Errorf("generateCalls = %d before fire time, want 0", generateCalls)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 20, 1, 0, 0, time.Local))
	loop.waitIdle()

	if generateCalls != 1 {
		t.Errorf("generateCalls = %d, want 1", generateCalls)
	}
	if deliverCalls != 1 {
		t.Errorf("deliverCalls = %d, want 1", deliverCalls)
	}
	if deliveredContent != "akşam raporu" {
		t.Errorf("deliveredContent = %q, want %q", deliveredContent, "akşam raporu")
	}

	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastRunDate != "2026-07-17" {
		t.Errorf("LastRunDate = %q, want 2026-07-17", got.LastRunDate)
	}
}

// TestTick_NoDeliveryChannel_NeverCallsDeliver confirms a routine with
// neither DeliveryWhatsApp nor DeliveryTelegram set still generates and
// marks itself run at fire time, but never invokes deliver at all.
func TestTick_NoDeliveryChannel_NeverCallsDeliver(t *testing.T) {
	var generateCalls, deliverCalls int
	generate := func(ctx context.Context, r Routine) (string, error) {
		generateCalls++
		return "x", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error {
		deliverCalls++
		return nil
	}

	loop, st := newTestLoop(t, generate, deliver)
	created, err := st.Create(Routine{
		Schedule: Schedule{TimeOfDay: "08:00"},
		Enabled:  true,
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	loop.tick(context.Background(), time.Date(2026, 7, 17, 8, 1, 0, 0, time.Local))
	loop.waitIdle()

	if generateCalls != 1 {
		t.Errorf("generateCalls = %d, want 1", generateCalls)
	}
	if deliverCalls != 0 {
		t.Errorf("deliverCalls = %d, want 0 (no delivery channel configured)", deliverCalls)
	}
	got, err := st.Get(created.ID)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.LastRunDate != "2026-07-17" {
		t.Errorf("LastRunDate = %q, want 2026-07-17", got.LastRunDate)
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

// TestParseFireTime_NilOffsetUsesHostLocation is the pre-fix behavior
// preserved for any routine with no recorded UTCOffsetMinutes (created
// before BUG-M4's fix, or by a client that never sends one): "HH:MM" is
// interpreted in now's own location, same as before this field existed.
func TestParseFireTime_NilOffsetUsesHostLocation(t *testing.T) {
	hostLoc := time.FixedZone("HOST", 2*3600) // UTC+2, standing in for "wherever the backend happens to run"
	now := time.Date(2026, 7, 17, 6, 0, 0, 0, hostLoc)

	got, err := ParseFireTime("08:00", nil, now)
	if err != nil {
		t.Fatalf("ParseFireTime: %v", err)
	}
	want := time.Date(2026, 7, 17, 8, 0, 0, 0, hostLoc)
	if !got.Equal(want) {
		t.Errorf("ParseFireTime(nil offset) = %v, want %v (host-local 08:00)", got, want)
	}
}

// TestParseFireTime_OffsetOverridesHostLocation is the regression test for
// BUG-M4: a routine created by a user in a different timezone than the
// backend host must fire at *that user's* "HH:MM", not the host's. Backend
// host is fixed at UTC+2; the routine was created by a user at UTC+7 (e.g.
// Bangkok) who asked for 08:00 their time — that's 03:00 host-local /
// 01:00 UTC, not 08:00 host-local.
func TestParseFireTime_OffsetOverridesHostLocation(t *testing.T) {
	hostLoc := time.FixedZone("HOST", 2*3600)
	now := time.Date(2026, 7, 17, 1, 30, 0, 0, hostLoc) // 03:30 in the UTC+7 user's timezone

	userOffsetMinutes := 7 * 60
	got, err := ParseFireTime("08:00", &userOffsetMinutes, now)
	if err != nil {
		t.Fatalf("ParseFireTime: %v", err)
	}

	gotUTC := got.UTC()
	wantUTC := time.Date(2026, 7, 17, 1, 0, 0, 0, time.UTC) // 08:00 at UTC+7 == 01:00 UTC
	if !gotUTC.Equal(wantUTC) {
		t.Errorf("ParseFireTime(+7h offset) = %v UTC, want %v UTC (the user's own 08:00, not the host's)", gotUTC, wantUTC)
	}
}

// TestParseFireTime_UsesTargetTimezonesOwnCalendarDay guards the subtler
// half of the fix: "today" itself must be computed in the resolved offset,
// not the caller's now.Location(), or a routine near midnight could still
// fire on the wrong calendar day even with the right offset applied to the
// clock time. Host is at 23:30 UTC+2 (so still "today" locally); the
// UTC+7 user is already 4.5 hours into the next calendar day.
func TestParseFireTime_UsesTargetTimezonesOwnCalendarDay(t *testing.T) {
	hostLoc := time.FixedZone("HOST", 2*3600)
	now := time.Date(2026, 7, 17, 23, 30, 0, 0, hostLoc)

	userOffsetMinutes := 7 * 60
	got, err := ParseFireTime("08:00", &userOffsetMinutes, now)
	if err != nil {
		t.Fatalf("ParseFireTime: %v", err)
	}

	inUserZone := got.In(time.FixedZone("USER", userOffsetMinutes*60))
	if inUserZone.Day() != 18 {
		t.Errorf("fire time fell on day %d in the user's own zone, want 18 (the user's actual 'today')", inUserZone.Day())
	}
}

// TestRoutineLoop_Start_FiresImmediatelyWithoutWaitingForTicker guards
// against a startup-latency regression to the same fix applied in
// calendar.ReminderLoop.Start (BUG-M7): time.NewTicker's first tick doesn't
// fire until a full interval elapses, so without an immediate tick before
// entering the ticker-wait loop, a routine due right at app startup
// wouldn't be checked for up to a minute. Uses a routine whose TimeOfDay is
// the current wall-clock minute, so it's already due by the time Start's
// internal time.Now() runs.
func TestRoutineLoop_Start_FiresImmediatelyWithoutWaitingForTicker(t *testing.T) {
	var generateCalls int32
	generate := func(ctx context.Context, r Routine) (string, error) {
		atomic.AddInt32(&generateCalls, 1)
		return "hello", nil
	}
	deliver := func(ctx context.Context, r Routine, content string) error { return nil }

	loop, st := newTestLoop(t, generate, deliver)
	if _, err := st.Create(Routine{
		Schedule:         Schedule{TimeOfDay: time.Now().Format("15:04")},
		DeliveryWhatsApp: true,
		Enabled:          true,
	}); err != nil {
		t.Fatalf("Create: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	loop.Start(ctx)
	loop.waitIdle()

	if got := atomic.LoadInt32(&generateCalls); got != 1 {
		t.Fatalf("generateCalls = %d, want 1 from the immediate startup tick — a routine due within the first minute after Start would otherwise never fire until a minute later", got)
	}
}
