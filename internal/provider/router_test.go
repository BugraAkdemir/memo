package provider

import (
	"context"
	"errors"
	"testing"
)

// mockProvider implements Provider for testing.
type mockProvider struct {
	name    ProviderType
	display string
	models  []string
	err     error
}

func (m *mockProvider) Name() ProviderType                    { return m.name }
func (m *mockProvider) DisplayName() string                    { return m.display }
func (m *mockProvider) ListModels(_ context.Context) ([]string, error) { return m.models, m.err }
func (m *mockProvider) ChatCompletion(_ context.Context, _ ChatRequest) (*ChatResponse, error) {
	if m.err != nil {
		return nil, m.err
	}
	return &ChatResponse{Content: "ok"}, nil
}
func (m *mockProvider) ChatCompletionStream(_ context.Context, _ ChatRequest) (<-chan StreamChunk, error) {
	if m.err != nil {
		return nil, m.err
	}
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Content: "ok", Done: true}
	close(ch)
	return ch, nil
}

func TestRouterFallbackChain(t *testing.T) {
	t.Run("falls back when first provider fails", func(t *testing.T) {
		failProv := &mockProvider{name: "openai", display: "OpenAI"}
		okProv := &mockProvider{name: "gemini", display: "Gemini"}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{Provider: failProv, cfg: ProviderConfig{Type: "openai", Enabled: true}},
			{Provider: okProv, cfg: ProviderConfig{Type: "gemini", Enabled: true}},
		}

		failProv.err = &ProviderError{Provider: "openai", Err: errors.New("rate limited")}

		_, err := r.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "test")}})
		if err != nil {
			t.Errorf("expected fallback to succeed, got error: %v", err)
		}
	})

	t.Run("returns error when all providers fail", func(t *testing.T) {
		fail1 := &mockProvider{name: "openai", display: "OpenAI", err: &ProviderError{Provider: "openai", Err: errors.New("down")}}
		fail2 := &mockProvider{name: "gemini", display: "Gemini", err: &ProviderError{Provider: "gemini", Err: errors.New("down")}}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{Provider: fail1, cfg: ProviderConfig{Type: "openai", Enabled: true}},
			{Provider: fail2, cfg: ProviderConfig{Type: "gemini", Enabled: true}},
		}

		_, err := r.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "test")}})
		if err == nil {
			t.Error("expected error when all providers fail")
		}
	})

	t.Run("skips disabled providers", func(t *testing.T) {
		okProv := &mockProvider{name: "gemini", display: "Gemini"}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{Provider: &mockProvider{name: "openai"}, cfg: ProviderConfig{Type: "openai", Enabled: false}},
			{Provider: okProv, cfg: ProviderConfig{Type: "gemini", Enabled: true}},
		}

		_, err := r.ChatCompletion(context.Background(), ChatRequest{Messages: []Message{TextMessage("user", "test")}})
		if err != nil {
			t.Errorf("expected success with disabled provider skipped, got: %v", err)
		}
	})
}

func TestRouterAutoDisable(t *testing.T) {
	t.Run("records failures incrementally", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			Provider: &mockProvider{name: "openai", display: "OpenAI"},
			cfg:      ProviderConfig{Type: "openai", Enabled: true},
		}
		r.providers = []*providerEntry{entry}

		r.recordFailure(entry)
		if entry.failCount != 1 {
			t.Errorf("expected failCount 1, got %d", entry.failCount)
		}

		r.recordFailure(entry)
		if entry.failCount != 2 {
			t.Errorf("expected failCount 2, got %d", entry.failCount)
		}
	})

	t.Run("disables after 3 consecutive failures", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			Provider: &mockProvider{name: "openai", display: "OpenAI"},
			cfg:      ProviderConfig{Type: "openai", Enabled: true},
		}
		r.providers = []*providerEntry{entry}

		for i := 0; i < 3; i++ {
			r.recordFailure(entry)
		}

		if !entry.disabled {
			t.Error("expected provider to be disabled after 3 failures")
		}
		if entry.failCount != 3 {
			t.Errorf("expected failCount 3, got %d", entry.failCount)
		}
	})

	t.Run("reset on success", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			Provider: &mockProvider{name: "openai", display: "OpenAI"},
			cfg:      ProviderConfig{Type: "openai", Enabled: true},
		}
		r.providers = []*providerEntry{entry}

		r.recordFailure(entry)
		r.recordFailure(entry)
		r.resetFailCount(entry)

		if entry.failCount != 0 {
			t.Errorf("expected failCount 0 after reset, got %d", entry.failCount)
		}
	})
}

func TestRouterHasActiveProvider(t *testing.T) {
	t.Run("empty router returns false", func(t *testing.T) {
		r := NewRouter(nil)
		if r.HasActiveProvider() {
			t.Error("empty router should have no active providers")
		}
	})

	t.Run("auto-disabled provider with no fallback", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			Provider:  &mockProvider{name: "openai"},
			cfg:       ProviderConfig{Type: "openai", Enabled: true},
			disabled:  true,
			failCount: 3,
		}
		r.providers = []*providerEntry{entry}
		if r.HasActiveProvider() {
			t.Error("router with only auto-disabled providers should return false")
		}
	})
}

func TestRouterGetActiveEntries(t *testing.T) {
	t.Run("filters disabled entries", func(t *testing.T) {
		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{Provider: &mockProvider{}, cfg: ProviderConfig{Type: "openai"}, disabled: true},
			{Provider: &mockProvider{}, cfg: ProviderConfig{Type: "gemini"}, disabled: false},
			{Provider: &mockProvider{}, cfg: ProviderConfig{Type: "claude"}, disabled: true},
		}

		active := r.getActiveEntries()
		if len(active) != 1 {
			t.Errorf("expected 1 active entry, got %d", len(active))
		}
		if active[0].cfg.Type != "gemini" {
			t.Errorf("expected gemini, got %s", active[0].cfg.Type)
		}
	})
}

func TestRouterSetActiveProvider(t *testing.T) {
	r := NewRouter(nil)
	r.SetActiveProvider("claude")
	if r.activeType != "claude" {
		t.Errorf("expected claude, got %s", r.activeType)
	}
	r.SetActiveProvider("")
	if r.activeType != "" {
		t.Errorf("expected empty, got %s", r.activeType)
	}
}

func TestConfigManagerEncryption(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	cm := NewConfigManager("", key)
	if len(cm.masterKey) != 32 {
		t.Fatalf("expected 32-byte key, got %d", len(cm.masterKey))
	}

	plaintext := "sk-test-api-key-12345"
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
	key := make([]byte, 32)
	cm := NewConfigManager("", key)

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
