// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func windowOf(t *testing.T, cfg ProviderConfig, model string) int {
	t.Helper()
	p, err := NewProvider(cfg)
	if err != nil {
		t.Fatal(err)
	}
	cw, ok := p.(ContextWindower)
	if !ok {
		t.Fatalf("%s provider does not implement ContextWindower", cfg.Type)
	}
	n, err := cw.ContextWindow(context.Background(), model)
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestContextWindow_OpenAICompatibleReadsTheModelsEntry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		w.Write([]byte(`{"data":[
			{"id":"openai/gpt-x"},
			{"id":"anthropic/claude-sonnet-5.5","context_length":1000000,"top_provider":{"context_length":900000,"max_completion_tokens":64000}},
			{"id":"vllm-model","max_model_len":32768},
			{"id":"groq-model","context_window":131072},
			{"id":"only-top","top_provider":{"context_length":65536}}]}`))
	}))
	defer srv.Close()
	cfg := ProviderConfig{Type: ProviderCustom, BaseURL: srv.URL + "/v1", APIKey: "k"}
	for model, want := range map[string]int{
		"anthropic/claude-sonnet-5.5": 1000000, // top-level field wins over top_provider
		"vllm-model":                  32768,
		"groq-model":                  131072,
		"only-top":                    65536,
		"openai/gpt-x":                0, // listed, but publishes no window
		"not-listed":                  0,
	} {
		if got := windowOf(t, cfg, model); got != want {
			t.Errorf("%s: got %d, want %d", model, got, want)
		}
	}
}

func TestContextWindow_OllamaPrefersTheRunningWindow(t *testing.T) {
	var psLoaded bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/ps":
			if psLoaded {
				w.Write([]byte(`{"models":[{"name":"llama3.2:latest","model":"llama3.2:latest","context_length":4096}]}`))
			} else {
				w.Write([]byte(`{"models":[]}`))
			}
		case "/api/show":
			if r.Method != http.MethodPost {
				t.Errorf("api/show must be POST, got %s", r.Method)
			}
			w.Write([]byte(`{"parameters":"stop \"<|eot|>\"\nnum_ctx 8192","model_info":{"general.architecture":"llama","llama.context_length":131072}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	cfg := ProviderConfig{Type: ProviderOllama, BaseURL: srv.URL + "/v1"}

	psLoaded = true
	if got := windowOf(t, cfg, "llama3.2:latest"); got != 4096 {
		t.Errorf("loaded model: got %d, want the running 4096", got)
	}
	psLoaded = false
	if got := windowOf(t, cfg, "llama3.2:latest"); got != 8192 {
		t.Errorf("Modelfile num_ctx: got %d, want 8192", got)
	}
}

func TestContextWindow_OllamaFallsBackToTrainedMaximum(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/show" {
			w.Write([]byte(`{"model_info":{"general.architecture":"gemma4","gemma4.context_length":262144}}`))
			return
		}
		http.NotFound(w, r) // an older Ollama has no /api/ps context
	}))
	defer srv.Close()
	if got := windowOf(t, ProviderConfig{Type: ProviderOllama, BaseURL: srv.URL + "/v1"}, "gemma4"); got != 262144 {
		t.Errorf("got %d, want 262144", got)
	}
}

func TestContextWindow_LlamaServerProps(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/props" {
			http.NotFound(w, r)
			return
		}
		w.Write([]byte(`{"default_generation_settings":{"n_ctx":16384}}`))
	}))
	defer srv.Close()
	if got := windowOf(t, ProviderConfig{Type: ProviderLlamaCPP, BaseURL: srv.URL + "/v1"}, "x"); got != 16384 {
		t.Errorf("got %d, want 16384", got)
	}
}

func TestContextWindow_AnthropicMaxInputTokens(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/models/claude-sonnet-5-5" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-api-key") != "k" {
			t.Errorf("missing api key header")
		}
		w.Write([]byte(`{"id":"claude-sonnet-5-5","type":"model","max_input_tokens":1000000,"max_tokens":64000}`))
	}))
	defer srv.Close()
	cfg := ProviderConfig{Type: ProviderClaude, BaseURL: srv.URL + "/v1", APIKey: "k"}
	if got := windowOf(t, cfg, "claude-sonnet-5-5"); got != 1000000 {
		t.Errorf("claude: got %d, want 1000000", got)
	}
	cfg.Type = ProviderCustomAnthropic
	if got := windowOf(t, cfg, "claude-sonnet-5-5"); got != 1000000 {
		t.Errorf("custom-anthropic: got %d, want 1000000", got)
	}
}

func TestContextWindow_GeminiInputTokenLimit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models/gemini-x" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("x-goog-api-key") != "k" {
			t.Errorf("missing api key header")
		}
		w.Write([]byte(`{"name":"models/gemini-x","inputTokenLimit":1048576,"outputTokenLimit":65536}`))
	}))
	defer srv.Close()
	cfg := ProviderConfig{Type: ProviderGemini, BaseURL: srv.URL + "/v1beta", APIKey: "k"}
	for _, id := range []string{"gemini-x", "models/gemini-x"} {
		if got := windowOf(t, cfg, id); got != 1048576 {
			t.Errorf("%s: got %d, want 1048576", id, got)
		}
	}
}

func TestContextWindow_ErrorStatusIsAnErrorNotAWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":"bad key"}`))
	}))
	defer srv.Close()
	p, _ := NewProvider(ProviderConfig{Type: ProviderCustom, BaseURL: srv.URL + "/v1"})
	if n, err := p.(ContextWindower).ContextWindow(context.Background(), "m"); err == nil || n != 0 {
		t.Errorf("got (%d, %v), want an error", n, err)
	}
}

// stubWindowSources swaps the live lookups and restores them afterwards.
func stubWindowSources(t *testing.T, live func() (int, error), catalog func() int) {
	t.Helper()
	ResetContextWindowCache()
	pl, pc := windowFromProvider, windowFromCatalog
	windowFromProvider = func(context.Context, ProviderConfig, string) (int, error) { return live() }
	windowFromCatalog = func(context.Context, ProviderType, string, string) int { return catalog() }
	t.Cleanup(func() { windowFromProvider, windowFromCatalog = pl, pc; ResetContextWindowCache() })
}

func TestCachedContextWindow_ConfiguredValueWinsAndNothingIsFetched(t *testing.T) {
	stubWindowSources(t, func() (int, error) { t.Error("must not ask the vendor"); return 1, nil }, func() int { return 1 })
	n, src := CachedContextWindow(ProviderConfig{Type: ProviderOpenAI, ContextTokens: 50000, Model: "m"}, "m")
	if n != 50000 || src != "config" {
		t.Errorf("got (%d,%q)", n, src)
	}
}

func TestCachedContextWindow_UnknownIsZeroNotADefault(t *testing.T) {
	stubWindowSources(t, func() (int, error) { return 0, nil }, func() int { return 0 })
	cfg := ProviderConfig{Type: ProviderOpenAI, Model: "mystery-model"}
	if n, _ := WarmContextWindow(context.Background(), cfg, "mystery-model"); n != 0 {
		t.Errorf("got %d, want 0 (unknown)", n)
	}
	if n, src := CachedContextWindow(cfg, "mystery-model"); n != 0 || src != "" {
		t.Errorf("got (%d,%q), want unknown", n, src)
	}
}

func TestCachedContextWindow_VendorFirstThenCatalogue(t *testing.T) {
	vendor := 0
	stubWindowSources(t, func() (int, error) { return vendor, nil }, func() int { return 200000 })
	cfg := ProviderConfig{Type: ProviderOpenAI, Model: "m"}

	n, src := WarmContextWindow(context.Background(), cfg, "m")
	if n != 200000 || src != "models.dev" {
		t.Errorf("catalogue fallback: got (%d,%q)", n, src)
	}

	ResetContextWindowCache()
	vendor = 1000000
	n, src = WarmContextWindow(context.Background(), cfg, "m")
	if n != 1000000 || src != "api" {
		t.Errorf("vendor answer must win over the catalogue: got (%d,%q)", n, src)
	}
}

func TestCachedContextWindow_AnswersFromMemoryThenFillsInBackground(t *testing.T) {
	release := make(chan struct{})
	stubWindowSources(t, func() (int, error) { <-release; return 1000000, nil }, func() int { return 0 })
	cfg := ProviderConfig{Type: ProviderClaude, Model: "claude-x"}

	start := time.Now()
	if n, _ := CachedContextWindow(cfg, "claude-x"); n != 0 {
		t.Errorf("first call has nothing cached, got %d", n)
	}
	if time.Since(start) > time.Second {
		t.Error("CachedContextWindow blocked on the network")
	}
	close(release)
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if n, src := CachedContextWindow(cfg, "claude-x"); n == 1000000 && src == "api" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Error("background refresh never landed")
}

func TestCachedContextWindow_FailedRefreshKeepsTheLastGoodFigure(t *testing.T) {
	fail := false
	stubWindowSources(t, func() (int, error) {
		if fail {
			return 0, errors.New("offline")
		}
		return 300000, nil
	}, func() int { return 0 })
	cfg := ProviderConfig{Type: ProviderGroq, Model: "m"}
	if n, _ := WarmContextWindow(context.Background(), cfg, "m"); n != 300000 {
		t.Fatalf("got %d", n)
	}
	fail = true
	windowCache.mu.Lock()
	for _, e := range windowCache.m {
		e.at = time.Now().Add(-2 * windowTTL) // stale, forces a refresh
	}
	windowCache.mu.Unlock()
	if n, _ := WarmContextWindow(context.Background(), cfg, "m"); n != 300000 {
		t.Errorf("outage wiped the figure: got %d", n)
	}
}

func TestWindowKeyHasNoCredential(t *testing.T) {
	k := windowKey(ProviderConfig{Type: ProviderOpenAI, APIKey: "sk-secret"}, "m")
	if strings.Contains(k, "sk-secret") {
		t.Errorf("key leaks the credential: %q", k)
	}
}
