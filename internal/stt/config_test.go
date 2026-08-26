package stt

import (
	"path/filepath"
	"testing"
)

func testKey() []byte {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}
	return key
}

func TestConfigManagerEncryptDecryptRoundTrip(t *testing.T) {
	cm := NewConfigManager("", testKey())

	plaintext := "sk-test-stt-key-12345"
	encrypted, err := cm.encrypt(plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}
	if encrypted == plaintext {
		t.Error("encrypted text should differ from plaintext")
	}

	decrypted, err := cm.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != plaintext {
		t.Errorf("decrypted = %q, want %q", decrypted, plaintext)
	}
}

func TestConfigManagerEncryptDecryptEmpty(t *testing.T) {
	cm := NewConfigManager("", testKey())

	encrypted, err := cm.encrypt("")
	if err != nil {
		t.Fatalf("encrypt empty: %v", err)
	}
	decrypted, err := cm.decrypt(encrypted)
	if err != nil {
		t.Fatalf("decrypt empty: %v", err)
	}
	if decrypted != "" {
		t.Errorf("expected empty, got %q", decrypted)
	}
}

func TestConfigManagerSetAndGetAll(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "stt_providers.json"), testKey())

	cfg := ProviderConfig{Type: ProviderElevenLabs, Name: "My ElevenLabs", APIKey: "sk-x", Enabled: true, Priority: 5}
	cm.Set(cfg)

	all := cm.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected 1 config, got %d", len(all))
	}
	if all[0].APIKey != "sk-x" {
		t.Errorf("expected APIKey round-tripped through disk to survive, got %q", all[0].APIKey)
	}
}

func TestConfigManagerSetUpdatesByName(t *testing.T) {
	cm := NewConfigManager("", testKey())

	cm.Set(ProviderConfig{Type: ProviderElevenLabs, Name: "el", Priority: 1, Enabled: true})
	cm.Set(ProviderConfig{Type: ProviderElevenLabs, Name: "el", Priority: 2, Enabled: true})

	all := cm.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected update in place, got %d entries", len(all))
	}
	if all[0].Priority != 2 {
		t.Errorf("expected updated fields, got %+v", all[0])
	}
}

func TestConfigManagerDeleteByName(t *testing.T) {
	cm := NewConfigManager("", testKey())

	cm.Set(ProviderConfig{Type: ProviderElevenLabs, Name: "el", Enabled: true})
	cm.Delete(ProviderElevenLabs, "el")

	if len(cm.GetAll()) != 0 {
		t.Errorf("expected empty after delete, got %d", len(cm.GetAll()))
	}
}

func TestConfigManagerGetEnabledFiltersDisabled(t *testing.T) {
	cm := NewConfigManager("", testKey())
	cm.Set(ProviderConfig{Type: ProviderElevenLabs, Name: "on", Enabled: true})
	cm.Set(ProviderConfig{Type: ProviderCustom, Name: "off", BaseURL: "http://localhost:9999", Enabled: false})

	enabled := cm.GetEnabled()
	if len(enabled) != 1 || enabled[0].Name != "on" {
		t.Errorf("expected only the enabled entry, got %+v", enabled)
	}
}

func TestConfigManagerLoadMissingFileStartsEmpty(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "does-not-exist.json"), testKey())
	if len(cm.GetAll()) != 0 {
		t.Error("expected empty config list when no file exists yet, not a seeded default set")
	}
}

func TestConfigManagerBaseURLSurvivesRoundTrip(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "stt_providers.json"), testKey())
	cm.Set(ProviderConfig{Type: ProviderCustom, Name: "cu", BaseURL: "http://localhost:9999", Enabled: true})

	all := cm.GetAll()
	if len(all) != 1 || all[0].BaseURL != "http://localhost:9999" {
		t.Errorf("expected BaseURL to survive persist/reload, got %+v", all)
	}
}
