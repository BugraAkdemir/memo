// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A providers.json written by an older build (or restored from a backup) can
// still carry the per-vendor subscription providers that no longer exist. They
// must be dropped on load — otherwise they linger as entries that fail with
// "unsupported provider type" and that routing, the gateway list and the UI
// all still see — and the cleanup must be written back so it sticks.
func TestConfigManager_DropsLegacySubscriptionProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "providers.json")

	seed := NewConfigManager(path, []byte("k"))
	seed.Set(ProviderConfig{Type: ProviderOpenAI, Name: "My OpenAI", APIKey: "sk-keep", Model: "gpt-4o", Enabled: true})
	seed.Set(ProviderConfig{Type: "gemini-sub", Name: "Google — Gemini (subscription)", Model: "gemini-2.5-pro", Enabled: true})
	seed.Set(ProviderConfig{Type: "claude-sub", Name: "Anthropic Claude (subscription)", Model: "claude-haiku-4-5", Enabled: true})
	seed.Save()
	if raw, _ := os.ReadFile(path); !strings.Contains(string(raw), "gemini-sub") {
		t.Fatalf("the test did not manage to write a legacy entry:\n%s", raw)
	}

	cm := NewConfigManager(path, []byte("k")) // a restart
	var names []string
	for _, p := range cm.GetAll() {
		names = append(names, p.Name)
		if p.Type == "gemini-sub" || p.Type == "claude-sub" {
			t.Errorf("legacy provider %q survived the load", p.Name)
		}
	}
	found := false
	for _, n := range names {
		if n == "My OpenAI" {
			found = true
		}
	}
	if !found {
		t.Errorf("an ordinary provider was lost with the legacy ones: %v", names)
	}
	// And the cleanup was persisted.
	raw, _ := os.ReadFile(path)
	var stored struct {
		Providers []struct {
			Type string `json:"type"`
		} `json:"providers"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		t.Fatal(err)
	}
	for _, p := range stored.Providers {
		if p.Type == "gemini-sub" || p.Type == "claude-sub" {
			t.Errorf("providers.json still carries a %s entry after the migration", p.Type)
		}
	}
}
