package models

// ProviderModel is one model a configured provider offers, as listed live by
// that provider. OwnedBy is only set when the provider says whose model it is
// (the Subscriptions sidecar does: antigravity, claude, codex …); selectors
// use it to group a long list.
type ProviderModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by,omitempty"`
}
