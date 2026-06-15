// SPDX-License-Identifier: AGPL-3.0-or-later

// Package calendar manages time-bound events extracted from user messages and
// fires reminder notifications via the app event system before they occur.
package calendar

import "time"

// Source identifies how an event was created.
type Source string

const (
	SourceChat     Source = "chat"
	SourceWhatsApp Source = "whatsapp"
	SourceManual   Source = "manual"
)

// Event is a single calendar entry.
type Event struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	StartTime    time.Time  `json:"start_time"`
	EndTime      *time.Time `json:"end_time,omitempty"`
	Description  string     `json:"description,omitempty"`
	Source       Source     `json:"source"`
	ContactName  string     `json:"contact_name,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	ReminderSent bool       `json:"reminder_sent"`
}

// ReminderPayload is the JSON body of a "calendar:reminder" AppEvent.
type ReminderPayload struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	StartTime   string `json:"start_time"` // RFC3339
	MinutesLeft int    `json:"minutes_left"`
}

// AddedPayload is the JSON body of a "calendar:added" AppEvent.
type AddedPayload struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}
