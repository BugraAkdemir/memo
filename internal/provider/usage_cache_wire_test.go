// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

// These tests drive each parser through a real HTTP round trip with the
// vendor's actual JSON shape, rather than only unit-testing the normalization
// helpers. The distinction matters here: the whole reason cached tokens were
// invisible for so long is that the *struct field was missing*, so every
// helper-level test would have passed against a response the decoder silently
// dropped on the floor.

func TestOpenAIProvider_ChatCompletion_ParsesCachedPromptTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Verbatim shape of OpenAI's usage object with automatic prompt
		// caching in play: cached_tokens lives under prompt_tokens_details
		// and is a SUBSET of prompt_tokens.
		w.Write([]byte(`{
			"model": "gpt-4o",
			"choices": [{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
			"usage": {
				"prompt_tokens": 5120,
				"completion_tokens": 64,
				"total_tokens": 5184,
				"prompt_tokens_details": {"cached_tokens": 4096, "audio_tokens": 0}
			}
		}`))
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.PromptTokens != 5120 {
		t.Errorf("PromptTokens = %d, want 5120 (cached tokens are already inside OpenAI's prompt_tokens — must not be added again)", resp.Usage.PromptTokens)
	}
	if resp.Usage.CachedPromptTokens != 4096 {
		t.Errorf("CachedPromptTokens = %d, want 4096", resp.Usage.CachedPromptTokens)
	}
	if resp.Usage.CacheWriteTokens != 0 {
		t.Errorf("CacheWriteTokens = %d, want 0 — automatic caching has no write premium to report", resp.Usage.CacheWriteTokens)
	}
	if got := resp.Usage.FreshPromptTokens(); got != 1024 {
		t.Errorf("FreshPromptTokens() = %d, want 1024", got)
	}
}

// TestOpenAIProvider_ChatCompletion_NoCacheDetailsIsZero is the companion
// case: every OpenAI-compatible backend that doesn't implement the details
// object (ollama, llama.cpp, most self-hosted gateways) must decode cleanly
// to zero rather than erroring or inventing a number.
func TestOpenAIProvider_ChatCompletion_NoCacheDetailsIsZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"model": "llama-3-8b",
			"choices": [{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
			"usage": {"prompt_tokens": 900, "completion_tokens": 20, "total_tokens": 920}
		}`))
	}))
	defer srv.Close()

	p := newTestOpenAIProvider(t, srv)
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Usage.CachedPromptTokens != 0 || resp.Usage.CacheWriteTokens != 0 {
		t.Errorf("cache split = (%d, %d), want (0, 0)", resp.Usage.CachedPromptTokens, resp.Usage.CacheWriteTokens)
	}
	if resp.Usage.PromptTokens != 900 {
		t.Errorf("PromptTokens = %d, want 900", resp.Usage.PromptTokens)
	}
}

func TestGeminiProvider_ChatCompletion_ParsesCachedContentTokenCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"candidates": [{"content":{"role":"model","parts":[{"text":"hi"}]},"finishReason":"STOP"}],
			"usageMetadata": {
				"promptTokenCount": 4000,
				"candidatesTokenCount": 30,
				"totalTokenCount": 4030,
				"cachedContentTokenCount": 3200
			}
		}`))
	}))
	defer srv.Close()

	p := newTestGeminiProvider(t, srv, "gemini-2.5-pro")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.PromptTokens != 4000 {
		t.Errorf("PromptTokens = %d, want 4000 (Gemini's cached count is a subset of promptTokenCount)", resp.Usage.PromptTokens)
	}
	if resp.Usage.CachedPromptTokens != 3200 {
		t.Errorf("CachedPromptTokens = %d, want 3200", resp.Usage.CachedPromptTokens)
	}
	if got := resp.Usage.FreshPromptTokens(); got != 800 {
		t.Errorf("FreshPromptTokens() = %d, want 800", got)
	}
}

// TestClaudeProvider_ChatCompletion_NormalizesCacheTokensIntoPromptTotal is
// the wire-level version of the arithmetic test: Anthropic's input_tokens here
// is 300, but the turn really processed 8100 input tokens — 7800 of them from
// cache. Recording 300 is what the old code did, and it made a cache hit look
// like a tiny prompt.
func TestClaudeProvider_ChatCompletion_NormalizesCacheTokensIntoPromptTotal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"model": "claude-opus-4-6",
			"content": [{"type":"text","text":"hi"}],
			"usage": {
				"input_tokens": 300,
				"output_tokens": 120,
				"cache_read_input_tokens": 7800,
				"cache_creation_input_tokens": 0
			}
		}`))
	}))
	defer srv.Close()

	p := newTestClaudeProvider(t, srv, "claude-opus-4-6")
	resp, err := p.ChatCompletion(context.Background(), ChatRequest{
		Messages: []Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("ChatCompletion() error = %v", err)
	}
	if resp.Usage == nil {
		t.Fatal("Usage is nil")
	}
	if resp.Usage.PromptTokens != 8100 {
		t.Errorf("PromptTokens = %d, want 8100 (300 fresh + 7800 read from cache)", resp.Usage.PromptTokens)
	}
	if resp.Usage.CachedPromptTokens != 7800 {
		t.Errorf("CachedPromptTokens = %d, want 7800", resp.Usage.CachedPromptTokens)
	}
	if resp.Usage.TotalTokens != 8220 {
		t.Errorf("TotalTokens = %d, want 8220", resp.Usage.TotalTokens)
	}
	if got := resp.Usage.FreshPromptTokens(); got != 300 {
		t.Errorf("FreshPromptTokens() = %d, want 300", got)
	}
}
