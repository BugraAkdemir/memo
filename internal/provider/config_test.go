package provider

import (
	"path/filepath"
	"testing"
)

// TestNewConfigManager_FreshInstallStartsWithNoProviders is the regression
// test for defaultConfigs() no longer pre-seeding every known provider
// type as a disabled placeholder — direct user feedback: the Settings >
// API Providers list looked cluttered with OpenAI/Gemini/Grok/Claude/
// OpenRouter/Groq/Ollama/OpenCode Zen/OpenCode Go cards the user never
// added, all showing "Disabled." A fresh install (no providers.json on
// disk yet) must now start with an empty list — only providers actually
// added via "Add Provider" should ever appear.
func TestNewConfigManager_FreshInstallStartsWithNoProviders(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(filepath.Join(dir, "providers.json"), []byte("test-key-32-bytes-padded-out!!!!"))

	got := cm.GetAll()
	if len(got) != 0 {
		t.Errorf("GetAll() on a fresh install = %d providers, want 0 (nothing pre-seeded): %+v", len(got), got)
	}
}

// TestNewConfigManager_AddedProviderPersistsAndIsTheOnlyOne confirms the
// empty starting state doesn't break the actual add-a-provider path —
// Set() must still append normally with nothing pre-seeded to match
// against.
func TestNewConfigManager_AddedProviderPersistsAndIsTheOnlyOne(t *testing.T) {
	dir := t.TempDir()
	cm := NewConfigManager(filepath.Join(dir, "providers.json"), []byte("test-key-32-bytes-padded-out!!!!"))

	cm.Set(ProviderConfig{Type: ProviderKilo, Name: "Kilo Code", Model: "kilo-auto/balanced", Enabled: true})

	got := cm.GetAll()
	if len(got) != 1 {
		t.Fatalf("GetAll() after adding one provider = %d, want 1: %+v", len(got), got)
	}
	if got[0].Type != ProviderKilo || got[0].Name != "Kilo Code" {
		t.Errorf("GetAll()[0] = %+v, want the just-added Kilo config", got[0])
	}
}
