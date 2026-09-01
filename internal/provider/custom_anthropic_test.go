package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestCustomAnthropicProvider_UsesTheConfiguredBaseURLAndAuth is the whole
// point of ProviderCustomAnthropic: point Memo's Anthropic-shaped client
// (with its now-working tool-calling — see claude_toolcalling_test.go) at
// ANY endpoint that speaks the real Anthropic Messages API wire format,
// self-hosted proxy included, instead of forcing it through an
// OpenAI-compatible translation layer that may — and, found live against a
// real third-party proxy, did — mishandle tool definitions.
func TestCustomAnthropicProvider_UsesTheConfiguredBaseURLAndAuth(t *testing.T) {
	p, err := newCustomAnthropicProvider(ProviderConfig{
		Type:    ProviderCustomAnthropic,
		BaseURL: "http://127.0.0.1:9",
		APIKey:  "test-key",
		Model:   "claude-3-5-sonnet-20241022",
	})
	if err != nil {
		t.Fatalf("newCustomAnthropicProvider() error = %v", err)
	}
	if p.Name() != ProviderCustomAnthropic {
		t.Errorf("Name() = %q, want %q", p.Name(), ProviderCustomAnthropic)
	}
	if p.baseURL != "http://127.0.0.1:9" {
		t.Errorf("baseURL = %q, want the caller-supplied one, not Anthropic's own default", p.baseURL)
	}
}

// TestCustomAnthropicProvider_ValidateRequiresBaseURL: unlike the native
// "claude" type (which falls back to api.anthropic.com), this type has no
// sane default — an empty Base URL must be rejected before any request is
// attempted, same contract "custom" already has.
func TestCustomAnthropicProvider_ValidateRequiresBaseURL(t *testing.T) {
	cfg := ProviderConfig{Type: ProviderCustomAnthropic, Model: "claude-3-5-sonnet-20241022"}
	if err := cfg.Validate(); err == nil {
		t.Fatal("Validate() = nil, want an error for a missing Base URL")
	} else if !strings.Contains(err.Error(), "base URL") {
		t.Errorf("Validate() error = %v, want it to mention the missing base URL", err)
	}

	cfg.BaseURL = "http://127.0.0.1:9999"
	if err := cfg.Validate(); err != nil {
		t.Errorf("Validate() with a Base URL set = %v, want nil", err)
	}
}

// TestCustomAnthropicProvider_RoutesThroughNewProvider confirms the factory
// switch actually wires this type in — not something covered by directly
// calling newCustomAnthropicProvider above.
func TestCustomAnthropicProvider_RoutesThroughNewProvider(t *testing.T) {
	p, err := NewProvider(ProviderConfig{
		Type: ProviderCustomAnthropic, BaseURL: "http://127.0.0.1:9", APIKey: "k", Model: "m",
	})
	if err != nil {
		t.Fatalf("NewProvider() error = %v", err)
	}
	if _, ok := p.(*customAnthropicProvider); !ok {
		t.Errorf("NewProvider(%q) returned %T, want *customAnthropicProvider", ProviderCustomAnthropic, p)
	}
}

// TestCustomAnthropicProvider_InheritsToolCalling is the load-bearing check:
// this type must send/parse tools exactly like the native "claude" type,
// since it's the same claudeProvider underneath with only the endpoint
// swapped. A real httptest server stands in for "the user's own proxy."
func TestCustomAnthropicProvider_InheritsToolCalling(t *testing.T) {
	// Reuses claude_toolcalling_test.go's exact scenario, just constructed
	// via ProviderCustomAnthropic instead of ProviderClaude, to prove the
	// wrapper adds nothing that breaks it and the request still reaches
	// whatever base_url was configured.
	var gotBody claudeRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatalf("read request body: %v", err)
		}
		if err := json.Unmarshal(body, &gotBody); err != nil {
			t.Fatalf("decode outgoing request: %v", err)
		}
		json.NewEncoder(w).Encode(claudeResponse{
			Model: "claude-3-5-sonnet-20241022",
			Content: []claudeBlock{
				{Type: "tool_use", ID: "toolu_1", Name: "list_directory", Input: json.RawMessage(`{}`)},
			},
		})
	}))
	defer srv.Close()

	p, err := newCustomAnthropicProvider(ProviderConfig{
		Type: ProviderCustomAnthropic, BaseURL: srv.URL, APIKey: "test-key", Model: "claude-3-5-sonnet-20241022",
	})
	if err != nil {
		t.Fatalf("newCustomAnthropicProvider() error = %v", err)
	}

	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{TextMessage("user", "list files")},
		Tools: []ToolDefinition{{
			Type:     "function",
			Function: ToolFunction{Name: "list_directory", Parameters: json.RawMessage(`{"type":"object"}`)},
		}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if len(gotBody.Tools) != 1 || gotBody.Tools[0].Name != "list_directory" {
		t.Fatalf("outgoing Tools = %+v, want [list_directory] — tool-calling did not carry through the custom-anthropic wrapper", gotBody.Tools)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Function.Name != "list_directory" {
		t.Fatalf("resp.ToolCalls = %+v, want one call to list_directory", resp.ToolCalls)
	}
}
