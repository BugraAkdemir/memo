package orchestra

import "testing"

// TestSanitizeTrimsWhitespace guards against the "Şef planlıyor..." freeze bug:
// a model id with a stray leading tab/space (e.g. "\tdeepseek-v4-pro") was being
// sent verbatim to the provider, producing an invalid model id that stalled the
// orchestra. Sanitize must strip that whitespace everywhere it can appear.
func TestSanitizeTrimsWhitespace(t *testing.T) {
	cfg := OrchestraConfig{
		ChiefType:  " openai ",
		ChiefModel: "\tdeepseek-v4-pro",
		Roles: []RoleConfig{
			{Role: RolePlanner, ModelType: "openai\n", ModelName: "  deepseek-v4-flash\t"},
			{Role: RoleGeneral, ModelType: "claude", ModelName: "claude-opus-4-8"},
		},
	}

	got := cfg.Sanitize()

	if got.ChiefType != "openai" {
		t.Errorf("ChiefType = %q, want %q", got.ChiefType, "openai")
	}
	if got.ChiefModel != "deepseek-v4-pro" {
		t.Errorf("ChiefModel = %q, want %q", got.ChiefModel, "deepseek-v4-pro")
	}
	if got.Roles[0].ModelType != "openai" {
		t.Errorf("Roles[0].ModelType = %q, want %q", got.Roles[0].ModelType, "openai")
	}
	if got.Roles[0].ModelName != "deepseek-v4-flash" {
		t.Errorf("Roles[0].ModelName = %q, want %q", got.Roles[0].ModelName, "deepseek-v4-flash")
	}

	// Sanitize must not mutate the receiver's backing role slice.
	if cfg.Roles[0].ModelName != "  deepseek-v4-flash\t" {
		t.Errorf("Sanitize mutated the original config: %q", cfg.Roles[0].ModelName)
	}
}
