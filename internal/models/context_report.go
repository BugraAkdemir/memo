// SPDX-License-Identifier: AGPL-3.0-or-later

package models

// ContextCategory is one slice of what a chat's last prompt was made of.
type ContextCategory struct {
	// Key is a stable identifier the UI maps to a label: "messages", "summary",
	// "system", "memory", "skills", "tools", "current".
	Key    string `json:"key"`
	Tokens int    `json:"tokens"`
}

// QuotaMeter is one allowance window of the account behind the active model
// (Subscriptions only): the 5-hour "session" meter, the weekly one, or — for
// Antigravity, which meters per model — the model's own single figure.
type QuotaMeter struct {
	// Label is the window length ("5h", "7d"); empty for a per-model figure.
	Label string `json:"label,omitempty"`
	// RemainingPercent is the share left, 0..100.
	RemainingPercent int `json:"remaining_percent"`
	// ResetAt is when it refills (RFC 3339); empty when the vendor does not say.
	ResetAt string `json:"reset_at,omitempty"`
}

// ContextReport answers "how full is this chat's context window?" for the
// ring at the bottom of the chat and the popover behind it.
//
// Used is the size of the conversation as the model last saw it plus its reply
// — the provider's own prompt-token count when it reported one (UsedReal), a
// len/3 estimate otherwise. Categories come from the last prompt Memo actually
// assembled and are estimates; when the provider's real count is known they are
// scaled to add up to it. A chat that has not been used since Memo started has
// only its stored messages to go on.
type ContextReport struct {
	Provider string `json:"provider"`
	Model    string `json:"model"`
	// Window is the model's context window in tokens (0 when unknown).
	Window int `json:"window"`
	// Used is the current context size in tokens.
	Used     int  `json:"used"`
	UsedReal bool `json:"used_real"`
	// Percent is Used as a share of Window, 0..100 (0 when Window is unknown).
	Percent    int               `json:"percent"`
	Categories []ContextCategory `json:"categories"`
	// AutoCompactEnabled / AutoCompactPct describe the threshold at which the
	// oldest turns are condensed into a summary on the next message.
	AutoCompactEnabled bool `json:"auto_compact_enabled"`
	AutoCompactPct     int  `json:"auto_compact_pct"`
	// SummarizedMessages is how many of the chat's oldest messages are currently
	// replaced by a summary in the prompt (0 = none).
	SummarizedMessages int `json:"summarized_messages"`
	// Limits are the allowance meters of the account behind the active model;
	// empty for providers that have none Memo can read.
	Limits []QuotaMeter `json:"limits"`
	// LimitsVendor names whose allowance Limits describes ("antigravity",
	// "openai", "anthropic").
	LimitsVendor string `json:"limits_vendor,omitempty"`
}
