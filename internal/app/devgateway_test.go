// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"memo/internal/config"
	"memo/internal/provider"
)

func TestSplitGatewayModelSpec(t *testing.T) {
	cases := []struct {
		name      string
		spec      string
		wantType  string
		wantModel string
		wantErr   bool
	}{
		{"local", "local/qwen2.5", "local", "qwen2.5", false},
		{"provider with slash in model id", "openai/gpt-4o", "openai", "gpt-4o", false},
		{"openrouter style nested model id", "openrouter/openai/gpt-4o", "openrouter", "openai/gpt-4o", false},
		{"no slash", "local", "", "", true},
		{"empty", "", "", "", true},
		{"empty type", "/model", "", "", true},
		{"empty model", "local/", "", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			typ, model, err := splitGatewayModelSpec(c.spec)
			if (err != nil) != c.wantErr {
				t.Fatalf("err = %v, wantErr = %v", err, c.wantErr)
			}
			if err != nil {
				return
			}
			if typ != c.wantType || model != c.wantModel {
				t.Errorf("got (%q, %q), want (%q, %q)", typ, model, c.wantType, c.wantModel)
			}
		})
	}
}

func TestMergeSystemBlock(t *testing.T) {
	t.Run("blank block is a no-op", func(t *testing.T) {
		msgs := []provider.Message{{Role: "user", Content: "hi"}}
		got := mergeSystemBlock(msgs, "")
		if len(got) != 1 || got[0].Content != "hi" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("no existing system message: one is added", func(t *testing.T) {
		msgs := []provider.Message{{Role: "user", Content: "hi"}}
		got := mergeSystemBlock(msgs, "user's favorite color is orange")
		if len(got) != 2 {
			t.Fatalf("got %+v", got)
		}
		if got[0].Role != "system" || got[0].Content != "user's favorite color is orange" {
			t.Errorf("system message = %+v", got[0])
		}
		if got[1].Role != "user" || got[1].Content != "hi" {
			t.Errorf("user message = %+v", got[1])
		}
	})

	t.Run("existing system message: block is appended, not replaced", func(t *testing.T) {
		msgs := []provider.Message{
			{Role: "system", Content: "You are Claude Code."},
			{Role: "user", Content: "hi"},
		}
		got := mergeSystemBlock(msgs, "user's favorite color is orange")
		if len(got) != 2 {
			t.Fatalf("got %+v", got)
		}
		want := "You are Claude Code.\nuser's favorite color is orange"
		if got[0].Content != want {
			t.Errorf("system message content = %q, want %q", got[0].Content, want)
		}
		// Original slice must not be mutated.
		if msgs[0].Content != "You are Claude Code." {
			t.Errorf("original messages mutated: %+v", msgs)
		}
	})
}

// TestGetSetDevGatewayConfig exercises the getter/in-memory-mutation
// directly rather than through the real SetDevGatewayConfig (which calls
// config.Save — a process-wide global, per internal/config/config.go's
// cfgPath/instance caching, that a unit test shouldn't perturb). Get/Set's
// own field plumbing is what's under test here, not config.Save's disk I/O.
func TestGetSetDevGatewayConfig(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	requireKey, useMemory, systemPrompt := a.GetDevGatewayConfig()
	if requireKey || useMemory || systemPrompt != "" {
		t.Errorf("zero-value config should default to (false, false, \"\"), got (%v, %v, %q)", requireKey, useMemory, systemPrompt)
	}

	a.cfg.DevGateway.RequireAPIKey = true
	a.cfg.DevGateway.UseMemory = true
	a.cfg.DevGateway.SystemPrompt = "always answer in Turkish"
	requireKey, useMemory, systemPrompt = a.GetDevGatewayConfig()
	if !requireKey || !useMemory || systemPrompt != "always answer in Turkish" {
		t.Errorf("after setting all three, GetDevGatewayConfig returned (%v, %v, %q)", requireKey, useMemory, systemPrompt)
	}
}

func TestMaybeSaveGatewayMemory_RespectsToggle(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	// UseMemory defaults to false — must be a no-op (and, crucially, must not
	// panic despite memorySaveCh being nil, since saveMemoryAsync is never
	// reached when the toggle is off).
	a.MaybeSaveGatewayMemory("hi", "hello there")
}

func TestListGatewayModels_NoLocalNoProviders(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	models := a.ListGatewayModels()
	if len(models) != 0 {
		t.Errorf("expected no models, got %+v", models)
	}
}

func TestListGatewayModels_EnabledProvidersOnly(t *testing.T) {
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderOpenAI, Name: "my-openai", Model: "gpt-4o", Enabled: true})
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderClaude, Name: "disabled-claude", Model: "claude-sonnet-4-20250514", Enabled: false})

	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr}
	models := a.ListGatewayModels()
	if len(models) != 1 {
		t.Fatalf("expected exactly 1 enabled provider, got %+v", models)
	}
	if models[0].ID != "openai/gpt-4o" || models[0].Type != "openai" {
		t.Errorf("got %+v", models[0])
	}
}

func TestDevGatewayChatStream_InvalidModelSpec(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChatStream(context.Background(), "no-slash-here", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error for a modelSpec without a \"/\"")
	}
}

func TestDevGatewayChatStream_LocalWithNoModelLoaded(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChatStream(context.Background(), "local/whatever", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error when no local model is loaded")
	}
}

func TestDevGatewayChatStream_NoProviderConfigured(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChatStream(context.Background(), "openai/gpt-4o", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error when providerCfgMgr is nil")
	}
}

func TestDevGatewayChatStream_NoEnabledProviderForType(t *testing.T) {
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderClaude, Name: "my-claude", Model: "claude-sonnet-4-20250514", Enabled: true})

	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr}
	_, _, err := a.DevGatewayChatStream(context.Background(), "openai/gpt-4o", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error: no enabled provider of type openai is configured")
	}
}

func TestDevGatewayChat_InvalidModelSpec(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChat(context.Background(), "no-slash-here", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error for a modelSpec without a \"/\"")
	}
}

func TestDevGatewayChat_LocalWithNoModelLoaded(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChat(context.Background(), "local/whatever", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error when no local model is loaded")
	}
}

func TestDevGatewayChat_NoProviderConfigured(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	_, _, err := a.DevGatewayChat(context.Background(), "openai/gpt-4o", provider.ChatRequest{})
	if err == nil {
		t.Fatal("expected an error when providerCfgMgr is nil")
	}
}

// capturedWireMessage mirrors just the fields of the OpenAI-wire
// {"role", "content"} message shape the fake backend server below needs to
// inspect — deliberately not importing internal/provider's own unexported
// openAIMessage type, since these tests only care what actually went out on
// the wire, not Memo's internal representation of it.
type capturedWireMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// newInjectionCaptureServer starts a fake OpenAI-compatible backend that
// records the "messages" array of the last /chat/completions request it
// received (into *captured) and replies with a minimal valid completion —
// enough for provider.openAIProvider.ChatCompletion to succeed. Used by the
// TestDevGateway*_NoUnexpectedInjection tests below to prove exactly what
// the dev gateway sends upstream when the caller's own messages pass
// through it, with no test able to see the outbound HTTP request any other
// way (DevGatewayChat/DevGatewayChatStream mutate req.Messages internally
// and never return it).
func newInjectionCaptureServer(t *testing.T, captured *[]capturedWireMessage) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Messages []capturedWireMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("fake backend: decode request body: %v", err)
		}
		*captured = body.Messages
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"id": "chatcmpl-test", "object": "chat.completion", "created": 1,
			"model": "gpt-4o",
			"choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
			"usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func newInjectionTestApp(t *testing.T, srv *httptest.Server) *App {
	t.Helper()
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	cfgMgr.Set(provider.ProviderConfig{
		Type: provider.ProviderOpenAI, Name: "fake", Model: "gpt-4o",
		BaseURL: srv.URL, Enabled: true,
	})
	return &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr}
}

// TestDevGatewayChat_NoInjectionWhenDefaultConfig is the regression test for
// the "zero prompt injection unless explicitly configured" guarantee: with
// the dev gateway's SystemPrompt empty and UseMemory off (both zero values,
// i.e. an untouched install), the messages that reach the upstream
// provider must be byte-for-byte identical to what the caller (e.g. Claude
// Code, via ANTHROPIC_BASE_URL) sent — no persona, no memory block, no
// extra system message of any kind. The gateway must behave as a pure
// passthrough in this state.
func TestDevGatewayChat_NoInjectionWhenDefaultConfig(t *testing.T) {
	var captured []capturedWireMessage
	srv := newInjectionCaptureServer(t, &captured)
	a := newInjectionTestApp(t, srv)

	req := provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hello"}}}
	if _, _, err := a.DevGatewayChat(context.Background(), "openai/gpt-4o", req); err != nil {
		t.Fatalf("DevGatewayChat: %v", err)
	}

	if len(captured) != 1 || captured[0].Role != "user" || captured[0].Content != "hello" {
		t.Errorf("wire messages = %+v, want exactly the caller's own single user message untouched", captured)
	}
}

// TestDevGatewayChat_SystemPromptConfigured_OnlyThatIsInjected proves the
// opt-in "Extra System Instruction" field adds exactly the user's own text
// and nothing else (no Memo persona/capability announcements — see
// injectGatewayMemory's doc comment for why those are deliberately
// excluded from gateway traffic).
func TestDevGatewayChat_SystemPromptConfigured_OnlyThatIsInjected(t *testing.T) {
	var captured []capturedWireMessage
	srv := newInjectionCaptureServer(t, &captured)
	a := newInjectionTestApp(t, srv)
	a.cfg.DevGateway.SystemPrompt = "always answer in Turkish"

	req := provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hello"}}}
	if _, _, err := a.DevGatewayChat(context.Background(), "openai/gpt-4o", req); err != nil {
		t.Fatalf("DevGatewayChat: %v", err)
	}

	if len(captured) != 2 {
		t.Fatalf("wire messages = %+v, want exactly [system, user]", captured)
	}
	if captured[0].Role != "system" || captured[0].Content != "always answer in Turkish" {
		t.Errorf("system message = %+v, want exactly the configured text, nothing more", captured[0])
	}
	if captured[1].Role != "user" || captured[1].Content != "hello" {
		t.Errorf("user message = %+v, want untouched", captured[1])
	}
}

// TestDevGatewayChat_UseMemoryOnButGlobalMemoryOff_NoInjection is the
// regression test for the double-gate in DevGatewayChat/DevGatewayChatStream
// (`a.devGatewayMemoryEnabled() && a.GetMemoryEnabled()`): turning on the
// gateway's own "Use Memory" toggle must NOT inject anything while Memo's
// global memory system is itself off — both must be true, not just one.
func TestDevGatewayChat_UseMemoryOnButGlobalMemoryOff_NoInjection(t *testing.T) {
	var captured []capturedWireMessage
	srv := newInjectionCaptureServer(t, &captured)
	a := newInjectionTestApp(t, srv)
	a.cfg.DevGateway.UseMemory = true
	// a.cfg.Memory.MemoryEnabled left at its zero value (false).

	req := provider.ChatRequest{Messages: []provider.Message{{Role: "user", Content: "hello"}}}
	if _, _, err := a.DevGatewayChat(context.Background(), "openai/gpt-4o", req); err != nil {
		t.Fatalf("DevGatewayChat: %v", err)
	}

	if len(captured) != 1 || captured[0].Role != "user" || captured[0].Content != "hello" {
		t.Errorf("wire messages = %+v, want exactly the caller's own single user message untouched (global memory is off)", captured)
	}
}

// TestDevGatewayChat_ToolsRejectedForUnsupportedProviderType is the
// regression test for the gemini/claude/ollama gap: a tools-bearing request
// routed to one of those types must fail clearly, rather than silently
// dropping the tools and returning a plain-text reply the caller would
// wrongly assume came from a tool-aware exchange.
func TestDevGatewayChat_ToolsRejectedForUnsupportedProviderType(t *testing.T) {
	cfgMgr := provider.NewConfigManager(filepath.Join(t.TempDir(), "providers.json"), make([]byte, 32))
	cfgMgr.Set(provider.ProviderConfig{Type: provider.ProviderGemini, Name: "my-gemini", Model: "gemini-2.0-flash", Enabled: true})

	a := &App{cfg: &config.AppConfig{}, providerCfgMgr: cfgMgr}
	req := provider.ChatRequest{
		Tools: []provider.ToolDefinition{{Type: "function", Function: provider.ToolFunction{Name: "get_weather"}}},
	}
	_, _, err := a.DevGatewayChat(context.Background(), "gemini/gemini-2.0-flash", req)
	if err == nil {
		t.Fatal("expected an error: gemini doesn't support tool calling through the dev gateway yet")
	}
}
