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
	// ContextInsight pre-fetches a time-windowed slice of the user's own
	// conversation memory + mood trend (see internal/app/insight.go) instead
	// of calendar/WhatsApp data — for routines like a weekly self-insight
	// digest ("geçen hafta kendimle ilgili ne fark ettim").
	ContextInsight ContextSourceType = "insight"
	// ContextWebSearch deterministically runs a web search (internal/websearch,
	// the same DuckDuckGo-backed search plain chat's web_search tool uses) and
	// hands the results to the LLM as read-only context — for routines like
	// "her gün 9'da yapay zeka haberlerini getir". Deliberately a Go-side
	// deterministic fetch, not AgentMode's full tool pipeline: an unattended
	// routine calling a real tool loop is a materially bigger risk surface
	// (arbitrary tool selection, no human present for a permission prompt)
	// than one hardcoded, read-only search call.
	ContextWebSearch ContextSourceType = "websearch"
)

// Schedule describes when a routine fires. Weekdays is empty for "every day".
type Schedule struct {
	TimeOfDay string         `json:"time_of_day"` // "HH:MM", in UTCOffsetMinutes if set, else backend host local time
	Weekdays  []time.Weekday `json:"weekdays,omitempty"`

	// UTCOffsetMinutes is the client's UTC offset in minutes (e.g. 180 for
	// UTC+3), captured at the moment TimeOfDay was set. ParseFireTime
	// interprets TimeOfDay against this offset instead of the backend host's
	// own local timezone (BUG-M4): without it, a routine created while the
	// user is in a different timezone than wherever the backend process
	// happens to be running (e.g. remote access while traveling) always
	// fired at the *host's* local "HH:MM", not the user's. nil for any
	// routine created before this field existed, or a client that never
	// sends it — ParseFireTime falls back to the previous host-local-time
	// behavior exactly in that case.
	//
	// Deliberately a fixed offset, not a full IANA zone name: it does not
	// self-correct across a DST transition the user's real zone would
	// observe (a routine set in summer stays on summer time through
	// winter) — capturing a real IANA zone would need a platform
	// timezone-name plugin on the Flutter side (device timezone name isn't
	// available from pure Dart), which isn't worth the added dependency for
	// this. Accepted, documented limitation, not silently wrong.
	UTCOffsetMinutes *int `json:"utc_offset_minutes,omitempty"`
}

// FiresOn reports whether the schedule includes the given weekday.
func (s Schedule) FiresOn(day time.Weekday) bool {
	if len(s.Weekdays) == 0 {
		return true
	}
	return slices.Contains(s.Weekdays, day)
}

// ContextSource optionally attaches deterministic context to a routine's
// prompt (today's calendar agenda, a specific WhatsApp chat's recent
// messages, or a web search) without requiring the LLM to call a tool for it
// unattended.
type ContextSource struct {
	Type         ContextSourceType `json:"type"`
	WhatsAppJID  string            `json:"whatsapp_jid,omitempty"`
	WhatsAppName string            `json:"whatsapp_name,omitempty"` // display only
	// WebSearchQuery is the query internal/websearch.Search runs when Type is
	// ContextWebSearch — extracted from the routine's own free-text
	// description (e.g. "yapay zeka haberleri" from "her gün yapay zeka
	// haberlerini getir"), not the full Prompt itself, since Prompt is the
	// instruction handed to the LLM alongside the results, not a search
	// query.
	WebSearchQuery string `json:"web_search_query,omitempty"`
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
	WhatsAppTargetJID string `json:"whatsapp_target_jid,omitempty"`
	// DeliveryTelegram/TelegramTargetChatID mirror the WhatsApp pair above.
	// Unlike WhatsApp (any contact, human-picked from a chat list),
	// TelegramTargetChatID only ever has one legitimate value — the bot's
	// linked owner (telegram.State.OwnerChatID) — since a Telegram bot has no
	// concept of "other contacts" to pick from; it's still stored explicitly
	// rather than always re-read from telegram.State at delivery time so a
	// routine keeps working (or fails loudly) even if the bot is later
	// disconnected and reconnected to a different owner.
	DeliveryTelegram     bool  `json:"delivery_telegram"`
	TelegramTargetChatID int64 `json:"telegram_target_chat_id,omitempty"`

	// Language is the UI locale ("tr" or "en") active on the client at the
	// moment this routine was created (frontend's MemoLocale, client-side-only
	// today — the backend otherwise has no notion of language at all). Drives
	// which language the backend uses for its own generated text: the "no
	// events/no messages" context filler and the system prompt handed to the
	// LLM for the non-agent execution path (see internal/app/routine.go). Empty for
	// any routine created before this field existed; callers must treat
	// empty (and any value other than "en") as "tr" — this codebase's
	// default target audience (see AGENTS.md's "Turkish + English mixed
	// user-facing text is intentional" note) — rather than requiring a
	// migration.
	Language string `json:"language,omitempty"`

	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// LastRunDate ("2026-07-17") guards against firing more than once per day.
	LastRunDate string `json:"last_run_date,omitempty"`

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
