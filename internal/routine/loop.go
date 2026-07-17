// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"context"
	"fmt"
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
}

// NewRoutineLoop creates a RoutineLoop. generate, deliver and emit must not be nil.
func NewRoutineLoop(store *Store, generate GenerateFn, deliver DeliverFn, emit Emitter) *RoutineLoop {
	return &RoutineLoop{store: store, generate: generate, deliver: deliver, emit: emit}
}

// Start runs the tick loop until ctx is done. Meant to be run in its own
// goroutine. Checks every minute.
func (r *RoutineLoop) Start(ctx context.Context) {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			r.tick(ctx, now)
		}
	}
}

func (r *RoutineLoop) tick(ctx context.Context, now time.Time) {
	today := now.Format("2006-01-02")
	for _, rt := range r.store.List() {
		if !rt.Enabled || rt.LastRunDate == today {
			continue
		}
		if !rt.Schedule.FiresOn(now.Weekday()) {
			continue
		}
		fireTime, err := ParseFireTime(rt.Schedule.TimeOfDay, now)
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

		if rt.LastGeneratedForDate != today {
			content, err := r.generate(ctx, rt)
			if err != nil {
				logx.Printf("routine: generate %s: %v", rt.ID, err)
				continue
			}
			rt.LastGeneratedContent = content
			rt.LastGeneratedAt = now
			rt.LastGeneratedForDate = today
			updated, err := r.store.Update(rt)
			if err != nil {
				logx.Printf("routine: save generated content %s: %v", rt.ID, err)
				continue
			}
			rt = *updated
			if rt.DeliveryMobile {
				r.emit("routine:ready", rt.ID)
			}
		}

		if now.Before(fireTime) {
			continue // mobile content pre-generated; wait for the real fire time
		}

		if rt.DeliveryWhatsApp {
			if err := r.deliver(ctx, rt, rt.LastGeneratedContent); err != nil {
				logx.Printf("routine: deliver %s: %v", rt.ID, err)
				continue
			}
		}

		rt.LastRunDate = today
		if _, err := r.store.Update(rt); err != nil {
			logx.Printf("routine: mark run %s: %v", rt.ID, err)
		}
	}
}

// ParseFireTime resolves "HH:MM" into an absolute instant on now's date, in
// now's location.
func ParseFireTime(timeOfDay string, now time.Time) (time.Time, error) {
	t, err := time.ParseInLocation("15:04", timeOfDay, now.Location())
	if err != nil {
		return time.Time{}, fmt.Errorf("routine: parse time_of_day %q: %w", timeOfDay, err)
	}
	return time.Date(now.Year(), now.Month(), now.Day(), t.Hour(), t.Minute(), 0, 0, now.Location()), nil
}
