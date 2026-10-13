package provider

import (
	"context"
	"testing"
)

type optInStub struct{ calls *int }

func (s optInStub) Name() ProviderType  { return ProviderWebClaude }
func (s optInStub) DisplayName() string { return "stub" }
func (s optInStub) ListModels(context.Context) ([]string, error) {
	return []string{"m"}, nil
}
func (s optInStub) ChatCompletion(context.Context, ChatRequest) (*ChatResponse, error) {
	*s.calls++
	return &ChatResponse{Content: "web"}, nil
}
func (s optInStub) ChatCompletionStream(context.Context, ChatRequest) (<-chan StreamChunk, error) {
	*s.calls++
	ch := make(chan StreamChunk, 1)
	ch <- StreamChunk{Done: true}
	close(ch)
	return ch, nil
}

// An opt-in-only provider (a web chat) must not be picked up as a silent
// fallback when no provider is pinned, but must work when pinned by name.
func TestRouter_OptInOnlyProviderIsUsedOnlyWhenPinned(t *testing.T) {
	calls := 0
	old := externalConstructors[ProviderWebClaude]
	RegisterConstructor(ProviderWebClaude, func(ProviderConfig) (Provider, error) { return optInStub{&calls}, nil })
	t.Cleanup(func() {
		if old != nil {
			externalConstructors[ProviderWebClaude] = old
		} else {
			delete(externalConstructors, ProviderWebClaude)
		}
	})

	r := NewRouter([]ProviderConfig{{Name: "web", Type: ProviderWebClaude, Model: "m", Enabled: true}})
	req := ChatRequest{Messages: []Message{TextMessage("user", "hi")}}

	if _, err := r.ChatCompletion(context.Background(), req); err == nil {
		t.Fatal("an unpinned router served a request from an opt-in-only provider")
	}
	if calls != 0 {
		t.Fatalf("opt-in-only provider was called %d time(s) without being pinned", calls)
	}
	r.SetActiveProvider("web")
	if _, err := r.ChatCompletion(context.Background(), req); err != nil {
		t.Fatalf("pinned call failed: %v", err)
	}
	if calls != 1 {
		t.Fatalf("pinned call count = %d, want 1", calls)
	}
}

// "Test connection" sends a real message. For a web chat that is a real
// conversation in the person's account, so it must not send anything.
func TestTestConnection_SendsNothingToAnOptInOnlyProvider(t *testing.T) {
	calls := 0
	old := externalConstructors[ProviderWebClaude]
	RegisterConstructor(ProviderWebClaude, func(ProviderConfig) (Provider, error) { return optInStub{&calls}, nil })
	t.Cleanup(func() {
		if old != nil {
			externalConstructors[ProviderWebClaude] = old
		} else {
			delete(externalConstructors, ProviderWebClaude)
		}
	})
	cm := &ConfigManager{}
	if err := cm.TestConnection(&ProviderConfig{Type: ProviderWebClaude, Name: "w", Model: "m"}); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if calls != 0 {
		t.Fatalf("TestConnection sent %d message(s) to a web chat", calls)
	}
}
