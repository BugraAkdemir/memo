package models

// QuotaSignal tells the chat UI something about the allowance behind the model
// it is talking to. It travels as the JSON Content of a "quota_exhausted" or
// "quota_low" marker chunk on the chat stream — never as reply text.
type QuotaSignal struct {
	// Kind is "exhausted" (the turn failed because the allowance ran out) or
	// "low" (the turn worked, but little is left).
	Kind     string `json:"kind"`
	Provider string `json:"provider,omitempty"`
	Model    string `json:"model,omitempty"`
	// Vendor is the sidecar's owned_by (antigravity, openai, anthropic …).
	Vendor string `json:"vendor,omitempty"`
	// RemainingPercent is the share left, 0..100; -1 when unknown.
	RemainingPercent int `json:"remaining_percent"`
	// ResetAt is when the allowance refills (RFC 3339); empty when not known, in
	// which case the UI falls back to retrying on its own schedule.
	ResetAt string `json:"reset_at,omitempty"`
	// Window names the allowance window ("5h", "7d") when the vendor has several.
	Window string `json:"window,omitempty"`
}

// Marker finish reasons that carry a QuotaSignal.
const (
	QuotaExhaustedMarker = "quota_exhausted"
	QuotaLowMarker       = "quota_low"
)
