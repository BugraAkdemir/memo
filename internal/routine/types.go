// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"slices"
	"time"
)

// ContextSourceType names where a routine pulls extra context from before
// asking the LLM to produce its content — the "simple prompt" execution path
// (see Routine.AgentMode) fetches this deterministically in Go rather than
// relying on an unattended LLM tool call to fetch it correctly.
type ContextSourceType string

const (
	ContextNone     ContextSourceType = "none"
	ContextCalendar ContextSourceType = "calendar"
	ContextWhatsApp ContextSourceType = "whatsapp"
)

// Schedule describes when a routine fires. Weekdays is empty for "every day".
type Schedule struct {
	TimeOfDay string         `json:"time_of_day"` // "HH:MM", local time
	Weekdays  []time.Weekday `json:"weekdays,omitempty"`
}

// FiresOn reports whether the schedule includes the given weekday.
func (s Schedule) FiresOn(day time.Weekday) bool {
	if len(s.Weekdays) == 0 {
		return true
	}
	return slices.Contains(s.Weekdays, day)
}

// ContextSource optionally attaches deterministic context to a routine's
// prompt (today's calendar agenda, or a specific WhatsApp chat's recent
// messages) without requiring the LLM to call a tool for it unattended.
type ContextSource struct {
	Type         ContextSourceType `json:"type"`
	WhatsAppJID  string            `json:"whatsapp_jid,omitempty"`
	WhatsAppName string            `json:"whatsapp_name,omitempty"` // display only
}

// Routine is a single user-defined scheduled automation.
type Routine struct {
	ID              string   `json:"id"`
	CreatedFromText string   `json:"created_from_text"`
	Schedule        Schedule `json:"schedule"`
	Prompt          string   `json:"prompt"`

	// AgentMode selects the execution path: false runs a plain LLM call with
	// deterministically pre-fetched ContextSource data (safe by construction,
	// no tool access); true runs the full agent/tool pipeline for tasks like
	// "git pull and report status". AutoApproveTools only matters when
	// AgentMode is true — no human is present to answer a permission prompt,
	// so it must be granted explicitly at creation time or the run fails.
	AgentMode        bool          `json:"agent_mode"`
	AutoApproveTools bool          `json:"auto_approve_tools"`
	ContextSource    ContextSource `json:"context_source"`

	DeliveryWhatsApp  bool   `json:"delivery_whatsapp"`
	DeliveryMobile    bool   `json:"delivery_mobile"`
	WhatsAppTargetJID string `json:"whatsapp_target_jid,omitempty"`

	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// LastRunDate ("2026-07-17") guards against firing more than once per day.
	LastRunDate string `json:"last_run_date,omitempty"`

	// Content is generated ahead of the fire time when DeliveryMobile is set,
	// since the mobile app has no push channel and must poll+pre-schedule a
	// local notification (see RoutineLoop's doc comment).
	LastGeneratedContent string    `json:"last_generated_content,omitempty"`
	LastGeneratedAt      time.Time `json:"last_generated_at"`
}

// LastGeneratedDate returns the "2026-07-17"-style date LastGeneratedAt
// falls on, or "" if content was never generated. Guards against
// regenerating more than once per day — derived from LastGeneratedAt rather
// than stored as its own field (an earlier LastGeneratedForDate field was
// removed as redundant: the two were always set together from the same now,
// in the same code path, so a future edit updating one without the other
// could silently desync them).
func (r Routine) LastGeneratedDate() string {
	if r.LastGeneratedAt.IsZero() {
		return ""
	}
	return r.LastGeneratedAt.Format("2006-01-02")
}

// MobilePayload is what the mobile app polls for to pre-schedule a local
// notification ahead of a routine's fire time (see mobileLeadDuration's doc
// comment) — lives here rather than internal/app so internal/webserver can
// reference it without importing internal/app.
type MobilePayload struct {
	ID        string    `json:"id"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	FireAtUTC time.Time `json:"fire_at_utc"`
}
