package app

import (
	"context"
	"errors"
	"memo/internal/config"
	"memo/internal/provider"
	"strings"
	"testing"
)

type routeFixtureProvider struct {
	calls     int
	stream    []provider.StreamChunk
	responses []*provider.ChatResponse
}

func (p *routeFixtureProvider) Name() provider.ProviderType                  { return provider.ProviderCustom }
func (p *routeFixtureProvider) DisplayName() string                          { return "fixture" }
func (p *routeFixtureProvider) ListModels(context.Context) ([]string, error) { return nil, nil }
func (p *routeFixtureProvider) ChatCompletion(context.Context, provider.ChatRequest) (*provider.ChatResponse, error) {
	p.calls++
	if len(p.responses) > 0 {
		r := p.responses[0]
		p.responses = p.responses[1:]
		return r, nil
	}
	return nil, errors.New("HTTP 503 unavailable")
}
func (p *routeFixtureProvider) ChatCompletionStream(context.Context, provider.ChatRequest) (<-chan provider.StreamChunk, error) {
	p.calls++
	ch := make(chan provider.StreamChunk, len(p.stream))
	for _, c := range p.stream {
		ch <- c
	}
	close(ch)
	return ch, nil
}
func TestAutoRouteNoReplayAfterContentOrThinking(t *testing.T) {
	for _, chunk := range []provider.StreamChunk{{Content: "partial"}, {Thinking: "reasoning"}} {
		p := &routeFixtureProvider{stream: []provider.StreamChunk{chunk, {Error: "HTTP 503 unavailable", Done: true}}}
		f := &autoRouteFallback{prov: p, current: routeCandidate{cfg: provider.ProviderConfig{Model: "first"}}, candidates: []routeCandidate{{cfg: provider.ProviderConfig{Model: "backup"}}}}
		ch, _ := f.ChatCompletionStream(context.Background(), provider.ChatRequest{})
		for range ch {
		}
		if f.next != 0 {
			t.Fatal("replayed visible output")
		}
	}
}
func TestAutoRouteNoReplayAfterToolResponse(t *testing.T) {
	p := &routeFixtureProvider{responses: []*provider.ChatResponse{{ToolCalls: []provider.ToolCall{{ID: "one"}}}}}
	f := &autoRouteFallback{prov: p, current: routeCandidate{cfg: provider.ProviderConfig{Model: "first"}}, candidates: []routeCandidate{{cfg: provider.ProviderConfig{Model: "backup"}}}}
	f.ChatCompletion(context.Background(), provider.ChatRequest{})
	if _, err := f.ChatCompletion(context.Background(), provider.ChatRequest{}); err == nil {
		t.Fatal("expected error")
	}
	if f.next != 0 {
		t.Fatal("replayed a tool turn")
	}
}
func TestAutoRouteRequestBudgetPreservesCurrentInput(t *testing.T) {
	req := provider.ChatRequest{Messages: []provider.Message{provider.TextMessage("system", "rules"), provider.TextMessage("user", strings.Repeat("old", 4000)), {Role: "assistant", ToolCalls: []provider.ToolCall{{ID: "tool"}}}, {Role: "tool", ToolCallID: "tool", Content: "result"}, provider.TextMessage("user", "current")}}
	out, err := budgetAutoRouteRequest(req, routeCandidate{window: 2048, cfg: provider.ProviderConfig{Model: "smaller"}})
	if err != nil {
		t.Fatal(err)
	}
	if out.Messages[0].Role != "system" || out.Messages[len(out.Messages)-1].Content != "current" {
		t.Fatal("lost fixed input")
	}
	req.Messages[len(req.Messages)-1].Content = strings.Repeat("new", 8000)
	if _, err := budgetAutoRouteRequest(req, routeCandidate{window: 2048}); err == nil {
		t.Fatal("oversized current input accepted")
	}
	if req.Messages[1].Content != strings.Repeat("old", 4000) {
		t.Fatal("mutated caller request")
	}
}
func TestAutoRouteColdFailureUsesNextCandidate(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	a.cfg.Llama.BinaryPath = "/nonexistent/llama-server"
	f := &autoRouteFallback{app: a, candidates: []routeCandidate{{localPath: "/nonexistent/model.gguf", window: 8192}, {cfg: provider.ProviderConfig{Type: provider.ProviderCustom, Name: "backup", Model: "chat", Enabled: true, BaseURL: "http://localhost:1/v1"}}}}
	if err := f.advance(context.Background()); err != nil {
		t.Fatal(err)
	}
	if f.current.cfg.Name != "backup" || f.attempts != 2 {
		t.Fatal("cold start failure did not select next candidate")
	}
	f.close()
}
func TestAutoRouteQuotaIsSelectionHint(t *testing.T) {
	empty := routeCandidate{cfg: provider.ProviderConfig{Name: "empty", Priority: 10}, quotaBlocked: true, window: 16000}
	unknown := routeCandidate{cfg: provider.ProviderConfig{Name: "unknown"}, window: 16000}
	if got := eligibleRouteCandidates([]routeCandidate{empty, unknown}, 1000, false, false, false); len(got) != 1 || got[0].cfg.Name != "unknown" {
		t.Fatal("selected exhausted model")
	}
	if got := eligibleRouteCandidates([]routeCandidate{empty}, 1000, false, false, false); len(got) != 1 {
		t.Fatal("quota became a hard request gate")
	}
}

func TestAutoRouteRefusalAndCancellationAreNotRetried(t *testing.T) {
	for _, err := range []error{errors.New("HTTP 403 safety refusal"), errors.New("HTTP 400 invalid request"), context.Canceled, context.DeadlineExceeded} {
		if safeAutoRouteError(context.Background(), err) {
			t.Fatalf("unsafe fallback for %v", err)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if safeAutoRouteError(ctx, errors.New("HTTP 503 unavailable")) {
		t.Fatal("retried cancelled turn")
	}
}
