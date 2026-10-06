package models

// SubscriptionsState is what Settings → Subscriptions renders: whether the
// bundled CLIProxyAPI sidecar is usable, which vendor accounts are signed in,
// and the models those accounts expose. It carries identity only — never a
// token.
type SubscriptionsState struct {
	// Bundled is true when this build ships a checksum-verified sidecar.
	Bundled bool `json:"bundled"`
	// Version is the bundled sidecar's release tag (empty when not bundled).
	Version string `json:"version,omitempty"`
	// Problem says why the sidecar is unusable ("" when it is fine).
	Problem string `json:"problem,omitempty"`
	// Running is true while the sidecar process is alive.
	Running bool `json:"running"`
	// Providers is every login the sidecar can perform, in UI order.
	Providers []string              `json:"providers"`
	Accounts  []SubscriptionAccount `json:"accounts"`
	Models    []SubscriptionModel   `json:"models"`
	// Model is the model the "Subscriptions" provider currently uses.
	Model string `json:"model,omitempty"`
	// Login is the browser sign-in in flight (or the last one finished).
	Login SubscriptionLogin `json:"login"`
}

// SubscriptionAccount is one signed-in vendor account.
type SubscriptionAccount struct {
	Provider string `json:"provider"`
	Email    string `json:"email"`
	Project  string `json:"project,omitempty"`
	Disabled bool   `json:"disabled,omitempty"`
}

// SubscriptionModel is one model the signed-in accounts can use. OwnedBy is the
// vendor key (antigravity, claude, codex …) the UI groups by.
type SubscriptionModel struct {
	ID      string `json:"id"`
	OwnedBy string `json:"owned_by"`
}

// SubscriptionLogin mirrors cliproxy.LoginState for the REST response.
type SubscriptionLogin struct {
	Provider string `json:"provider,omitempty"`
	Running  bool   `json:"running"`
	URL      string `json:"url,omitempty"`
	Done     bool   `json:"done"`
	Error    string `json:"error,omitempty"`
}
