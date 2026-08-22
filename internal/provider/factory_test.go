package provider

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestNewProvider_DispatchesToCorrectImplementation guards the central
// ProviderType -> constructor switch in NewProvider — the one place a
// copy-paste error (returning the wrong vendor's constructor for a given
// case) would silently misroute every request for that provider type to a
// different vendor's API entirely. Had zero test coverage before this file,
// like every other provider-specific piece of this package.
func TestNewProvider_DispatchesToCorrectImplementation(t *testing.T) {
	types := []ProviderType{
		ProviderOpenAI,
		ProviderGemini,
		ProviderGrok,
		ProviderGroq,
		ProviderClaude,
		ProviderOpenRouter,
		ProviderOllama,
		ProviderLlamaCPP,
		ProviderOpenCodeZen,
		ProviderOpenCodeGo,
		ProviderKilo,
	}
	for _, pt := range types {
		t.Run(string(pt), func(t *testing.T) {
			p, err := NewProvider(ProviderConfig{Type: pt, Model: "some-model"})
			if err != nil {
				t.Fatalf("NewProvider(%q) error = %v", pt, err)
			}
			if p.Name() != pt {
				t.Errorf("NewProvider(%q).Name() = %q, want %q — wrong constructor wired to this case", pt, p.Name(), pt)
			}
		})
	}
}

// TestNewProvider_CustomTypeReportsItself confirms ProviderCustom (any
// OpenAI-compatible endpoint via a user-supplied Base URL) round-trips
// through openAIProvider.Name() as "custom", not silently as "openai" —
// they share a constructor (NewProvider's ProviderOpenAI, ProviderCustom
// case) but must remain distinguishable afterward for routing/display.
func TestNewProvider_CustomTypeReportsItself(t *testing.T) {
	p, err := NewProvider(ProviderConfig{Type: ProviderCustom, BaseURL: "https://example.com/v1", Model: "some-model"})
	if err != nil {
		t.Fatalf("NewProvider(custom) error = %v", err)
	}
	if p.Name() != ProviderCustom {
		t.Errorf("Name() = %q, want %q", p.Name(), ProviderCustom)
	}
	if p.DisplayName() != "Custom" {
		t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), "Custom")
	}
}

func TestNewProvider_UnsupportedTypeReturnsError(t *testing.T) {
	if _, err := NewProvider(ProviderConfig{Type: "not-a-real-provider"}); err == nil {
		t.Fatal("expected an error for an unsupported provider type")
	}
}

// TestDefaultBaseURL_CoversEveryProviderType locks in the exact endpoint
// each provider type resolves to when the user leaves Base URL blank — a
// wrong entry here silently misroutes every request for that vendor with
// no error anywhere (the request would just go to the wrong host, likely
// failing with a confusing connection/DNS error far from this table).
func TestDefaultBaseURL_CoversEveryProviderType(t *testing.T) {
	tests := []struct {
		pt   ProviderType
		want string
	}{
		{ProviderOpenAI, "https://api.openai.com/v1"},
		{ProviderGemini, "https://generativelanguage.googleapis.com/v1beta"},
		{ProviderGrok, "https://api.x.ai/v1"},
		{ProviderGroq, "https://api.groq.com/openai/v1"},
		{ProviderClaude, "https://api.anthropic.com/v1"},
		{ProviderOpenRouter, "https://openrouter.ai/api/v1"},
		{ProviderOllama, "http://127.0.0.1:11434/v1"},
		{ProviderLlamaCPP, "http://127.0.0.1:8081/v1"},
		{ProviderOpenCodeZen, "https://opencode.ai/zen/v1"},
		{ProviderOpenCodeGo, "https://opencode.ai/zen/go/v1"},
		{ProviderKilo, "https://api.kilo.ai/api/gateway"},
		{ProviderCustom, ""},
		{"unknown-type", ""},
	}
	for _, tt := range tests {
		t.Run(string(tt.pt), func(t *testing.T) {
			if got := DefaultBaseURL(tt.pt); got != tt.want {
				t.Errorf("DefaultBaseURL(%q) = %q, want %q", tt.pt, got, tt.want)
			}
		})
	}
}

// thinWrapperBaseURL extracts the embedded *openAIProvider's baseURL from
// each of the seven OpenAI-compatible wrapper types — reaching into the
// unexported field directly (this test file is in-package) rather than
// going through HTTP, since DefaultBaseURL points at real external hosts
// that must never actually be dialed from a test.
func thinWrapperBaseURL(t *testing.T, p Provider) string {
	t.Helper()
	switch v := p.(type) {
	case *grokProvider:
		return v.baseURL
	case *groqProvider:
		return v.baseURL
	case *ollamaProvider:
		return v.baseURL
	case *llamaCPPProvider:
		return v.baseURL
	case *openCodeZenProvider:
		return v.baseURL
	case *openCodeGoProvider:
		return v.baseURL
	case *openRouterProvider:
		return v.baseURL
	case *kiloProvider:
		return v.baseURL
	default:
		t.Fatalf("unhandled provider type %T in thinWrapperBaseURL", p)
		return ""
	}
}

// TestThinWrapperProviders_UseCorrectDefaultBaseURLAndIdentity covers the
// seven OpenAI-compatible wrappers (grok, groq, ollama, llama.cpp,
// opencode-zen, opencode-go, openrouter, kilo) that embed *openAIProvider — each
// only differs in its default Base URL and Name()/DisplayName(), but a
// mixed-up default URL (e.g. Ollama's constructor accidentally defaulting
// to llama.cpp's port) would silently point every request at the wrong
// local port/host, and nothing else in this package would catch it since
// they all share ChatCompletion/ChatCompletionStream's logic verbatim.
func TestThinWrapperProviders_UseCorrectDefaultBaseURLAndIdentity(t *testing.T) {
	tests := []struct {
		name        string
		pt          ProviderType
		wantURL     string
		wantDisplay string
	}{
		{"grok", ProviderGrok, "https://api.x.ai/v1", "xAI Grok"},
		{"groq", ProviderGroq, "https://api.groq.com/openai/v1", "Groq"},
		{"ollama", ProviderOllama, "http://127.0.0.1:11434/v1", "Ollama"},
		{"llama.cpp", ProviderLlamaCPP, "http://127.0.0.1:8081/v1", "Local (llama.cpp)"},
		{"opencode-zen", ProviderOpenCodeZen, "https://opencode.ai/zen/v1", "OpenCode Zen"},
		{"opencode-go", ProviderOpenCodeGo, "https://opencode.ai/zen/go/v1", "OpenCode Go"},
		{"openrouter", ProviderOpenRouter, "https://openrouter.ai/api/v1", "OpenRouter"},
		{"kilo", ProviderKilo, "https://api.kilo.ai/api/gateway", "Kilo Code"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p, err := NewProvider(ProviderConfig{Type: tt.pt})
			if err != nil {
				t.Fatalf("NewProvider(%q) error = %v", tt.pt, err)
			}
			if p.DisplayName() != tt.wantDisplay {
				t.Errorf("DisplayName() = %q, want %q", p.DisplayName(), tt.wantDisplay)
			}
			if got := thinWrapperBaseURL(t, p); got != tt.wantURL {
				t.Errorf("default baseURL = %q, want %q", got, tt.wantURL)
			}

			p2, err := NewProvider(ProviderConfig{Type: tt.pt, BaseURL: "http://explicit-override.test"})
			if err != nil {
				t.Fatalf("NewProvider(%q, explicit BaseURL) error = %v", tt.pt, err)
			}
			if got := thinWrapperBaseURL(t, p2); got != "http://explicit-override.test" {
				t.Errorf("explicit BaseURL = %q, want it to override the default", got)
			}
		})
	}
}

// TestGroqProvider_ListModels_ParsesModelIDs covers groqProvider's own
// ListModels override — unlike the other six thin wrappers, groq doesn't
// inherit openAIProvider.ListModels verbatim; it has its own copy
// (identical shape, but a separate implementation that could drift).
func TestGroqProvider_ListModels_ParsesModelIDs(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("path = %q, want /models", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{
				{"id": "openai/gpt-oss-20b"},
				{"id": "llama-3.3-70b-versatile"},
			},
		})
	}))
	defer srv.Close()

	p, err := NewProvider(ProviderConfig{Type: ProviderGroq, BaseURL: srv.URL, APIKey: "k"})
	if err != nil {
		t.Fatalf("NewProvider(groq) error = %v", err)
	}
	models, err := p.ListModels(context.Background())
	if err != nil {
		t.Fatalf("ListModels() error = %v", err)
	}
	if len(models) != 2 || models[0] != "openai/gpt-oss-20b" || models[1] != "llama-3.3-70b-versatile" {
		t.Errorf("ListModels() = %v, want [openai/gpt-oss-20b llama-3.3-70b-versatile]", models)
	}
}
