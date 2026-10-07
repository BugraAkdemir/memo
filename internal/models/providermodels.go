package models

// ProviderModel is one model a configured provider offers, as listed live by
// that provider. OwnedBy is only set when the provider says whose model it is
// (the Subscriptions sidecar does: antigravity, claude, codex …); selectors
// use it to group a long list.
type ProviderModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
	// Remaining / ResetAt: the share of the model's allowance left (0..1) and when
	// it refills, for the Subscriptions provider when the vendor reports it.
	Remaining *float64 `json:"remaining,omitempty"`
	ResetAt   string   `json:"reset_at,omitempty"`
}
