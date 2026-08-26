package stt

import (
	"context"
	"errors"
	"testing"
)

// mockProvider implements STTProvider for testing.
type mockProvider struct {
	name    ProviderType
	display string
	err     error
}

func (m *mockProvider) Name() ProviderType  { return m.name }
func (m *mockProvider) DisplayName() string { return m.display }
func (m *mockProvider) Transcribe(_ context.Context, _ []byte) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	return "merhaba", nil
}

func TestRouterFallbackChain(t *testing.T) {
	t.Run("falls back when first provider fails", func(t *testing.T) {
		failProv := &mockProvider{name: "elevenlabs", display: "ElevenLabs", err: errors.New("rate limited")}
		okProv := &mockProvider{name: "custom", display: "Custom"}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{STTProvider: failProv, cfg: ProviderConfig{Type: "elevenlabs", Enabled: true}},
			{STTProvider: okProv, cfg: ProviderConfig{Type: "custom", Enabled: true}},
		}

		_, err := r.Transcribe(context.Background(), []byte("audio"))
		if err != nil {
			t.Errorf("expected fallback to succeed, got error: %v", err)
		}
	})

	t.Run("returns error when all providers fail", func(t *testing.T) {
		fail1 := &mockProvider{name: "elevenlabs", display: "ElevenLabs", err: errors.New("down")}
		fail2 := &mockProvider{name: "custom", display: "Custom", err: errors.New("down")}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{STTProvider: fail1, cfg: ProviderConfig{Type: "elevenlabs", Enabled: true}},
			{STTProvider: fail2, cfg: ProviderConfig{Type: "custom", Enabled: true}},
		}

		_, err := r.Transcribe(context.Background(), []byte("audio"))
		if err == nil {
			t.Error("expected error when all providers fail")
		}
	})

	t.Run("skips disabled providers", func(t *testing.T) {
		okProv := &mockProvider{name: "custom", display: "Custom"}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{STTProvider: &mockProvider{name: "elevenlabs"}, cfg: ProviderConfig{Type: "elevenlabs", Enabled: false}},
			{STTProvider: okProv, cfg: ProviderConfig{Type: "custom", Enabled: true}},
		}

		_, err := r.Transcribe(context.Background(), []byte("audio"))
		if err != nil {
			t.Errorf("expected success with disabled provider skipped, got: %v", err)
		}
	})

	t.Run("respects priority order", func(t *testing.T) {
		var called []ProviderType
		low := &recordingProvider{ProviderType: "low", calls: &called}
		high := &recordingProvider{ProviderType: "high", calls: &called}

		r := NewRouter(nil)
		r.providers = []*providerEntry{
			{STTProvider: low, cfg: ProviderConfig{Type: "low", Enabled: true, Priority: 1}},
			{STTProvider: high, cfg: ProviderConfig{Type: "high", Enabled: true, Priority: 10}},
		}

		if _, err := r.Transcribe(context.Background(), []byte("audio")); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(called) != 1 || called[0] != "high" {
			t.Errorf("expected high-priority provider to be tried first, got %v", called)
		}
	})
}

// recordingProvider records which provider was actually invoked, to assert
// priority ordering without depending on mockProvider's error behavior.
type recordingProvider struct {
	ProviderType
	calls *[]ProviderType
}

func (p *recordingProvider) Name() ProviderType  { return p.ProviderType }
func (p *recordingProvider) DisplayName() string { return string(p.ProviderType) }
func (p *recordingProvider) Transcribe(_ context.Context, _ []byte) (string, error) {
	*p.calls = append(*p.calls, p.ProviderType)
	return "merhaba", nil
}

func TestRouterContextCancellationNotRecordedAsFailure(t *testing.T) {
	prov := &mockProvider{name: "elevenlabs", display: "ElevenLabs", err: errors.New("connection timeout")}
	r := NewRouter(nil)
	r.providers = []*providerEntry{{
		STTProvider: prov,
		cfg:         ProviderConfig{Type: "elevenlabs", Enabled: true},
	}}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := r.Transcribe(ctx, []byte("audio"))
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	entry := r.providers[0]
	if entry.failCount != 0 {
		t.Errorf("expected cancellation not to count as failure, got failCount %d", entry.failCount)
	}
	if entry.disabled {
		t.Error("expected provider to remain enabled after cancellation")
	}
}

func TestRouterAutoDisable(t *testing.T) {
	t.Run("disables after 3 consecutive failures", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			STTProvider: &mockProvider{name: "elevenlabs", display: "ElevenLabs"},
			cfg:         ProviderConfig{Type: "elevenlabs", Enabled: true},
		}
		r.providers = []*providerEntry{entry}

		for i := 0; i < 3; i++ {
			r.recordFailure(entry)
		}

		if !entry.disabled {
			t.Error("expected provider to be disabled after 3 failures")
		}
	})

	t.Run("reset on success", func(t *testing.T) {
		r := NewRouter(nil)
		entry := &providerEntry{
			STTProvider: &mockProvider{name: "elevenlabs", display: "ElevenLabs"},
			cfg:         ProviderConfig{Type: "elevenlabs", Enabled: true},
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
			STTProvider: &mockProvider{name: "elevenlabs"},
			cfg:         ProviderConfig{Type: "elevenlabs", Enabled: true},
			disabled:    true,
			failCount:   3,
		}
		r.providers = []*providerEntry{entry}
		if r.HasActiveProvider() {
			t.Error("router with only auto-disabled providers should return false")
		}
	})
}

func TestRouterUpdateConfigsSkipsInvalid(t *testing.T) {
	r := NewRouter([]ProviderConfig{
		{Type: ProviderElevenLabs, Name: "el", APIKey: "sk-y", Enabled: true},
		{Type: ProviderCustom, Name: "cu", BaseURL: "http://localhost:9999", Enabled: true},
		{Type: "", Name: "bad", Enabled: true},
	})
	// ElevenLabs and Custom both construct successfully; the empty-Type
	// entry fails Validate — UpdateConfigs must skip it gracefully, not
	// panic or leave a broken entry in r.providers.
	if len(r.providers) != 2 {
		t.Fatalf("expected exactly 2 constructible providers, got %d", len(r.providers))
	}
}

func TestProviderConfigValidate(t *testing.T) {
	cases := []struct {
		name    string
		cfg     ProviderConfig
		wantErr bool
	}{
		{"missing type", ProviderConfig{APIKey: "sk-x"}, true},
		{"missing api key", ProviderConfig{Type: ProviderElevenLabs}, true},
		{"valid elevenlabs", ProviderConfig{Type: ProviderElevenLabs, APIKey: "sk-x"}, false},
		{"custom missing base_url", ProviderConfig{Type: ProviderCustom}, true},
		{"custom without api key is valid", ProviderConfig{Type: ProviderCustom, BaseURL: "http://localhost:9999"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.cfg.Validate()
			if (err != nil) != tc.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tc.wantErr)
			}
		})
	}
}

func TestRouterSetActiveProvider(t *testing.T) {
	r := NewRouter(nil)
	r.SetActiveProvider("elevenlabs")
	if r.activeName != "elevenlabs" {
		t.Errorf("expected elevenlabs, got %s", r.activeName)
	}
	r.SetActiveProvider("")
	if r.activeName != "" {
		t.Errorf("expected empty, got %s", r.activeName)
	}
}
