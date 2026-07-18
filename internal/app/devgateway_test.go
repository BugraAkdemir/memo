// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"context"
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

func TestMergeMemoryBlock(t *testing.T) {
	t.Run("blank block is a no-op", func(t *testing.T) {
		msgs := []provider.Message{{Role: "user", Content: "hi"}}
		got := mergeMemoryBlock(msgs, "")
		if len(got) != 1 || got[0].Content != "hi" {
			t.Errorf("got %+v", got)
		}
	})

	t.Run("no existing system message: one is added", func(t *testing.T) {
		msgs := []provider.Message{{Role: "user", Content: "hi"}}
		got := mergeMemoryBlock(msgs, "user's favorite color is orange")
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
		got := mergeMemoryBlock(msgs, "user's favorite color is orange")
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
	requireKey, useMemory := a.GetDevGatewayConfig()
	if requireKey || useMemory {
		t.Errorf("zero-value config should default both to false, got (%v, %v)", requireKey, useMemory)
	}

	a.cfg.DevGateway.RequireAPIKey = true
	a.cfg.DevGateway.UseMemory = true
	requireKey, useMemory = a.GetDevGatewayConfig()
	if !requireKey || !useMemory {
		t.Errorf("after setting both true, GetDevGatewayConfig returned (%v, %v)", requireKey, useMemory)
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
