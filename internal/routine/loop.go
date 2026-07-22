// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"context"
	"fmt"
	"sync"
	"time"

	"memo/internal/logx"
)

// mobileLeadDuration is how far ahead of the scheduled fire time a
// mobile-delivered routine's content is generated. The mobile app has no
// push channel (see project docs) — it can only pick up ready content on its
// own poll cycle and schedule a local OS notification for the exact fire
// instant, so that content must already exist somewhat before the moment it
// needs to fire. WhatsApp-only routines need no lead — the backend delivers
// directly, so content is generated exactly at fire time.
const mobileLeadDuration = 2 * time.Hour

// generateTimeout bounds a single routine's GenerateFn call (an LLM call, or
// a full multi-step agent turn for agent-mode routines) so a stuck provider
// or a runaway tool loop can't starve every other routine forever — see
// tick's doc comment for why this matters given each routine now runs in its
// own goroutine rather than blocking the tick itself.
const generateTimeout = 5 * time.Minute

// Emitter publishes an AppEvent, mirroring calendar.Emitter/proactive.Emitter.
type Emitter func(name, data string)

// GenerateFn produces a routine's content (running either a plain LLM call or
// a full agent turn, decided by the caller based on Routine.AgentMode).
type GenerateFn func(ctx context.Context, r Routine) (string, error)

// DeliverFn sends already-generated content through the routine's requested
// channels that the loop itself can act on directly (currently: WhatsApp).
// Mobile delivery has no explicit deliver step — the phone picks up ready
// content on its own poll and schedules its own local notification.
type DeliverFn func(ctx context.Context, r Routine, content string) error

// RoutineLoop fires due routines on a one-minute tick, mirroring
// internal/calendar/reminder.go's ReminderLoop shape.
type RoutineLoop struct {
	store    *Store
	generate GenerateFn
	deliver  DeliverFn
	emit     Emitter

	runningMu sync.Mutex
	running   map[string]bool
	wg        sync.WaitGroup // lets tests wait for a tick's launched goroutines to finish
}

// NewRoutineLoop creates a RoutineLoop. generate, deliver and emit must not be nil.
func NewRoutineLoop(store *Store, generate GenerateFn, deliver DeliverFn, emit Emitter) *RoutineLoop {
	return &RoutineLoop{store: store, generate: generate, deliver: deliver, emit: emit, running: make(map[string]bool)}
}

// Start runs the tick loop until ctx is done. Meant to be run in its own
// goroutine. Checks every minute.
//
// An immediate tick before entering the ticker-wait loop closes the same
// gap fixed in calendar.ReminderLoop.Start (BUG-M7): time.NewTicker's first
// tick doesn't fire until a full interval has elapsed, so without this a
// routine due right at app startup wouldn't be checked for up to a minute.
// tick's own same-day dedup (LastRunDate) means this can never double-fire
// a routine — worst case a routine not yet due is checked one extra time
// and skipped.
func (r *RoutineLoop) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	r.tick(ctx, time.Now())

	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.tick(ctx, now)
		}
	}
}

// tick scans for due routines and fires each one in its own goroutine,
// rather than processing them one at a time inline. A routine's GenerateFn
// can be an arbitrarily long agent turn (tool calls, LLM latency) — running
// it inline used to block tick() itself for that whole duration, which in
// turn blocked Start's ticker select from ever reaching the next minute:
// every other routine due in the meantime (including zero-lead WhatsApp-only
// ones) sat starved until the slow routine finished, and a truly hung call
// stopped the entire routine system permanently. generateTimeout bounds the
// worst case per routine; the running map prevents the same routine from
// being launched twice concurrently if it's still mid-flight on the next
// tick (same-day dedup via LastRunDate only gets written after a run
// completes, so without this a slow routine could otherwise be picked up
// again before its first run finishes).
func (r *RoutineLoop) tick(ctx context.Context, now time.Time) {
	today := now.Format("2006-01-02")
	for _, rt := range r.store.List() {
		if !rt.Enabled || rt.LastRunDate == today {
			continue
		}
		if !rt.Schedule.FiresOn(now.Weekday()) {
			continue
		}
		fireTime, err := ParseFireTime(rt.Schedule.TimeOfDay, rt.Schedule.UTCOffsetMinutes, now)
		if err != nil {
			logx.Printf("routine: bad schedule for %s: %v", rt.ID, err)
			continue
		}

		lead := time.Duration(0)
		if rt.DeliveryMobile {
			lead = mobileLeadDuration
		}
		if now.Before(fireTime.Add(-lead)) {
			continue // not time to generate yet
		}

		if !r.tryMarkRunning(rt.ID) {
			continue // already in flight from an earlier tick
		}
		r.wg.Add(1)
		go func(rt Routine) {
			defer r.wg.Done()
			defer r.clearRunning(rt.ID)
			r.processDueRoutine(ctx, rt, now, today, fireTime)
		}(rt)
	}
}

// waitIdle blocks until every routine goroutine launched by ticks so far has
// finished. Only meant for tests — production code has no need to wait
// since the whole point of running routines in their own goroutines is that
// nothing else has to block on them.
func (r *RoutineLoop) waitIdle() {
	r.wg.Wait()
}

func (r *RoutineLoop) tryMarkRunning(id string) bool {
	r.runningMu.Lock()
	defer r.runningMu.Unlock()
	if r.running[id] {
		return false
	}
	r.running[id] = true
	return true
}

func (r *RoutineLoop) clearRunning(id string) {
	r.runningMu.Lock()
	delete(r.running, id)
	r.runningMu.Unlock()
}

// processDueRoutine runs the generate/deliver steps for a single routine
// already confirmed due, bounding the generate call so one stuck provider or
// runaway agent turn can't hang this goroutine forever.
func (r *RoutineLoop) processDueRoutine(ctx context.Context, rt Routine, now time.Time, today string, fireTime time.Time) {
	if rt.LastGeneratedDate() != today {
		genCtx, cancel := context.WithTimeout(ctx, generateTimeout)
		content, err := r.generate(genCtx, rt)
		cancel()
		if err != nil {
			logx.Printf("routine: generate %s: %v", rt.ID, err)
			return
		}
		rt.LastGeneratedContent = content
		rt.LastGeneratedAt = now
		updated, err := r.store.Update(rt)
		if err != nil {
			logx.Printf("routine: save generated content %s: %v", rt.ID, err)
			return
		}
		rt = *updated
		if rt.DeliveryMobile {
			r.emit("routine:ready", rt.ID)
		}
	}

	if now.Before(fireTime) {
		return // mobile content pre-generated; wait for the real fire time
	}

	if rt.DeliveryWhatsApp {
		deliverCtx, cancel := context.WithTimeout(ctx, generateTimeout)
		err := r.deliver(deliverCtx, rt, rt.LastGeneratedContent)
		cancel()
		if err != nil {
			logx.Printf("routine: deliver %s: %v", rt.ID, err)
			return
		}
	}

	rt.LastRunDate = today
	if _, err := r.store.Update(rt); err != nil {
		logx.Printf("routine: mark run %s: %v", rt.ID, err)
	}
}

// ParseFireTime resolves "HH:MM" into an absolute instant on today's date in
// utcOffsetMinutes (BUG-M4) — or, if utcOffsetMinutes is nil (no offset
// recorded on the schedule, see Schedule.UTCOffsetMinutes's doc comment),
// today's date in now's own location, exactly matching this function's
// pre-fix behavior. "Today" itself is computed in the resolved location too
// (not the caller's), so a routine correctly still fires on the fire
// timezone's own calendar day even near midnight, when it can differ from
// the backend host's.
func ParseFireTime(timeOfDay string, utcOffsetMinutes *int, now time.Time) (time.Time, error) {
	loc := now.Location()
	if utcOffsetMinutes != nil {
		loc = time.FixedZone("", *utcOffsetMinutes*60)
	}
	t, err := time.ParseInLocation("15:04", timeOfDay, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("routine: parse time_of_day %q: %w", timeOfDay, err)
	}
	today := now.In(loc)
	return time.Date(today.Year(), today.Month(), today.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}
