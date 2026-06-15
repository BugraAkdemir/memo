// SPDX-License-Identifier: AGPL-3.0-or-later

package calendar

import (
	"context"
	"fmt"
	"time"

	"memo/internal/intent"
)

// AddFromIntent creates a calendar Event from an IntentResult and persists it.
// Returns the new event so the caller can emit a "calendar:added" notification.
// Returns an error (without persisting) when the result has no usable event time.
func AddFromIntent(ctx context.Context, store *Store, r intent.IntentResult) (Event, error) {
	if r.EventTime == nil {
		return Event{}, fmt.Errorf("calendar: intent has no event time")
	}

	title := r.EventTitle
	if title == "" {
		title = r.Summary
	}
	if title == "" {
		title = "Etkinlik"
	}

	src := Source(r.Source)
	if src == "" {
		src = SourceManual
	}

	e := Event{
		Title:       title,
		StartTime:   *r.EventTime,
		Description: r.Summary,
		Source:      src,
		ContactName: r.ContactName,
		CreatedAt:   time.Now(),
	}

	return store.Add(ctx, e)
}
