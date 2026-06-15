// SPDX-License-Identifier: AGPL-3.0-or-later

// Package intent extracts structured intent from user messages regardless of
// source (chat, WhatsApp). The pipeline runs a fast keyword pre-filter before
// invoking the LLM so that routine messages never incur an API call.
package intent

import "time"

// Source identifies where a message originated.
type Source string

const (
	SourceChat     Source = "chat"
	SourceWhatsApp Source = "whatsapp"
)

// IntentResult holds the structured output of an extraction run.
// Fields are zero-valued when HasIntent is false.
type IntentResult struct {
	HasIntent bool

	// IsCalendarEvent is true when the message describes a one-off future event
	// with a determinable time ("yarın 11'de halısaha").
	IsCalendarEvent bool

	// IsHabit is true when the message declares a recurring intention
	// ("her gün saat 21'de kod yazacağım").
	IsHabit bool

	// IsCancellation is true when the user is cancelling/removing an existing
	// plan ("yarınki toplantım iptal", "cancel the meeting"). The app then
	// deletes the matching calendar event instead of creating one.
	IsCancellation bool

	// Summary is a concise Turkish description of the intent (max ~100 chars).
	// It is stored in the observer; the raw message text is never persisted.
	Summary string

	// EventTitle is a short human-readable label for the calendar entry.
	EventTitle string

	// EventTime is the resolved wall-clock time for a calendar event.
	// nil when the time could not be determined.
	EventTime *time.Time

	// TimeExplicit is true when the message stated a concrete time
	// ("yarın 11'de"), false when the model inferred/guessed one for a vague
	// mention ("yarın dışarı çıkalım"). Lets the app honour the user's
	// "don't guess times" preference.
	TimeExplicit bool

	// HabitTime is the time-of-day at which the habit is intended to occur.
	// nil when the time could not be determined.
	HabitTime *time.Time

	// HabitDays lists the weekdays (time.Weekday values) on which the habit
	// applies. An empty slice means every day.
	HabitDays []time.Weekday

	// Source is the channel through which the message arrived.
	Source Source

	// ContactName is the display name of the other party (WhatsApp only).
	ContactName string
}
