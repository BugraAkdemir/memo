package livemode

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

	plaintext := "sk-test-livemode-key-12345"
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

func TestConfigManagerSetAndGet(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "livemode_engines.json"), testKey())

	cm.Set(EngineConfig{Type: EngineGoogleLive, APIKey: "sk-x", Model: "gemini-live-x", Enabled: true})

	cfg, ok := cm.Get(EngineGoogleLive)
	if !ok {
		t.Fatal("expected a saved config for EngineGoogleLive")
	}
	if cfg.APIKey != "sk-x" || cfg.Model != "gemini-live-x" {
		t.Errorf("unexpected round-tripped config: %+v", cfg)
	}
}

func TestConfigManagerSetOverwritesByType(t *testing.T) {
	cm := NewConfigManager("", testKey())

	cm.Set(EngineConfig{Type: EngineElevenLabs, APIKey: "old-key", Enabled: true})
	cm.Set(EngineConfig{Type: EngineElevenLabs, APIKey: "new-key", Enabled: true})

	all := cm.GetAll()
	if len(all) != 1 {
		t.Fatalf("expected exactly 1 entry for EngineElevenLabs, got %d", len(all))
	}
	if all[0].APIKey != "new-key" {
		t.Errorf("expected the second Set to overwrite the first, got %q", all[0].APIKey)
	}
}

func TestConfigManagerMultipleEnginesCoexist(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "livemode_engines.json"), testKey())

	cm.Set(EngineConfig{Type: EngineGoogleLive, APIKey: "google-key", Enabled: false})
	cm.Set(EngineConfig{Type: EngineElevenLabs, APIKey: "el-key", Enabled: true})

	all := cm.GetAll()
	if len(all) != 2 {
		t.Fatalf("expected 2 saved engine configs, got %d", len(all))
	}
	// Switching the active engine (config.LiveModeConfig.ActiveEngine, a
	// separate concept from this package) must not lose the other engine's
	// saved credentials — that's the whole point of keying by EngineType
	// rather than only ever storing one entry.
	if _, ok := cm.Get(EngineGoogleLive); !ok {
		t.Error("expected EngineGoogleLive's config to still be retrievable")
	}
}

func TestConfigManagerDelete(t *testing.T) {
	cm := NewConfigManager("", testKey())
	cm.Set(EngineConfig{Type: EngineCustom, BaseURL: "http://localhost:9999", Enabled: true})
	cm.Delete(EngineCustom)

	if _, ok := cm.Get(EngineCustom); ok {
		t.Error("expected EngineCustom's config to be gone after Delete")
	}
}

func TestConfigManagerLoadMissingFileStartsEmpty(t *testing.T) {
	cm := NewConfigManager(filepath.Join(t.TempDir(), "does-not-exist.json"), testKey())
	if len(cm.GetAll()) != 0 {
		t.Error("expected empty config when no file exists yet, not a seeded default set")
	}
}
