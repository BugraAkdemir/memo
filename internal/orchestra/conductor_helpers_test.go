package orchestra

import (
	"context"
	"errors"
	"sync"

	"memo/internal/provider"
)

type mockProvider struct {
	name        provider.ProviderType
	display     string
	chatResp    string
	chatErr     error
	streamCh    chan provider.StreamChunk
	streamErr   error
	modelErr    error
}

func (m *mockProvider) Name() provider.ProviderType { return m.name }
func (m *mockProvider) DisplayName() string         { return m.display }
func (m *mockProvider) ListModels(_ context.Context) ([]string, error) {
	return nil, m.modelErr
}

func (m *mockProvider) ChatCompletion(_ context.Context, req provider.ChatRequest) (*provider.ChatResponse, error) {
	if m.chatErr != nil {
		return nil, m.chatErr
	}
	return &provider.ChatResponse{Content: m.chatResp}, nil
}

func (m *mockProvider) ChatCompletionStream(_ context.Context, req provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	if m.streamErr != nil {
		return nil, m.streamErr
	}
	if m.streamCh == nil {
		ch := make(chan provider.StreamChunk, 1)
		ch <- provider.StreamChunk{Content: m.chatResp}
		close(ch)
		return ch, nil
	}
	return m.streamCh, nil
}

var _ provider.Provider = (*mockProvider)(nil)

type mockFactory struct {
	mu        sync.Mutex
	providers map[string]*mockProvider
	// lastCfg records the ProviderConfig the conductor actually passed to
	// pf(cfg) for each provider type it created, keyed by type — lets
	// BUG-H3 regression tests assert the fallback path used a provider's
	// own configured Model instead of overwriting it with a different
	// vendor's model name.
	lastCfg map[string]provider.ProviderConfig
}

func (f *mockFactory) get(modelType string) *mockProvider {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.providers[modelType]
}

func (f *mockFactory) set(modelType string, p *mockProvider) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.providers[modelType] = p
}

func (f *mockFactory) lastConfigFor(modelType string) (provider.ProviderConfig, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	cfg, ok := f.lastCfg[modelType]
	return cfg, ok
}

func (f *mockFactory) factory(cfg provider.ProviderConfig) (provider.Provider, error) {
	f.mu.Lock()
	if f.lastCfg == nil {
		f.lastCfg = make(map[string]provider.ProviderConfig)
	}
	f.lastCfg[string(cfg.Type)] = cfg
	f.mu.Unlock()

	p := f.get(string(cfg.Type))
	if p == nil {
		return nil, errors.New("provider not found")
	}
	return p, nil
}

func testGetConfigs() []provider.ProviderConfig {
	return []provider.ProviderConfig{
		{Name: "test-openai", Type: "openai", Enabled: true, Model: "gpt-4o"},
	}
}

func newMockFactory() *mockFactory {
	return &mockFactory{providers: make(map[string]*mockProvider)}
}

func openAIMock(chatResp string) *mockProvider {
	return &mockProvider{
		name:     "openai",
		display:  "OpenAI",
		chatResp: chatResp,
	}
}

func newConductor(cfg OrchestraConfig, f *mockFactory) *Conductor {
	return NewConductor(cfg, f.factory, testGetConfigs)
}

func openAIConductor(cfg OrchestraConfig, chatResp string) (*Conductor, *mockFactory) {
	f := newMockFactory()
	f.set("openai", openAIMock(chatResp))
	return newConductor(cfg, f), f
}

func defaultEnabledConfig() OrchestraConfig {
	cfg := DefaultConfig()
	cfg.Enabled = true
	return cfg
}


