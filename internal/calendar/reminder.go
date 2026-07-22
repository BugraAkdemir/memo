// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"encoding/json"
	"memo/internal/logx"
	"math"
	"time"
)

// Emitter delivers a reminder notification to the UI/mobile layer.
// It receives the AppEvent name and JSON payload.
type Emitter func(name, data string)

// LeadFn returns the current reminder lead time in minutes. It is called on
// every tick so the user can change the setting without restarting.
type LeadFn func() int

// ReminderLoop periodically checks for upcoming events and fires notifications.
type ReminderLoop struct {
	store  *Store
	lead   LeadFn
	emit   Emitter
}

// NewReminderLoop creates a ReminderLoop. leadFn and emit must not be nil.
func NewReminderLoop(store *Store, leadFn LeadFn, emit Emitter) *ReminderLoop {
	return &ReminderLoop{store: store, lead: leadFn, emit: emit}
}

// Start runs the loop until ctx is cancelled. Intended to be launched as a
// goroutine. Checks every minute.
//
// BUG-M7: time.NewTicker's first tick doesn't fire until a full interval
// has elapsed, not immediately — and ClaimPendingReminders' claim window
// has a strictly-increasing, exclusive lower bound (start_time > now) each
// tick. Any reminder due within roughly the first minute after Start is
// called therefore fell before every subsequent window's lower bound too,
// and was silently, permanently never claimed — reminder_sent stayed 0
// forever, with no error anywhere. An immediate tick before entering the
// ticker-wait loop closes that gap: the very first check happens at
// startup instead of a full minute later.
func (r *ReminderLoop) Start(ctx context.Context) {
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

func (r *ReminderLoop) tick(ctx context.Context, now time.Time) {
	lead := r.lead()
	if lead <= 0 {
		return
	}

	events, err := r.store.ClaimPendingReminders(ctx, now, lead)
	if err != nil {
		logx.Printf("calendar: reminder tick: %v", err)
		return
	}

	for _, e := range events {
		minutesLeft := int(math.Round(e.StartTime.Sub(now).Minutes()))
		payload := ReminderPayload{
			ID:          e.ID,
			Title:       e.Title,
			StartTime:   e.StartTime.Format(time.RFC3339),
			MinutesLeft: minutesLeft,
		}
		data, err := json.Marshal(payload)
		if err != nil {
			logx.Printf("calendar: marshal reminder: %v", err)
			continue
		}
		r.emit("calendar:reminder", string(data))
	}
}
