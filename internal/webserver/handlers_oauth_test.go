package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"memo/internal/provider"
)

// stubBridgeWithProviders wraps swarmStubBridge to inject specific
// provider configs for tests that need an API key/BaseURL configured —
// swarmStubBridge.GetProviders() always returns nil, which only exercises
// the "nothing configured" path.
type stubBridgeWithProviders struct {
	*swarmStubBridge
	providers []provider.ProviderConfig
}

func (b *stubBridgeWithProviders) GetProviders() []provider.ProviderConfig {
	return b.providers
}

// TestFetchOpenRouterModelEffortLevels_ParsesSupportedEfforts is the
// regression for the one vendor in provider/effort.go's design that gets
// genuine runtime discovery instead of a hand-authored table — OpenRouter's
// own /api/v1/models response carries each model's actual supported_efforts.
func TestFetchOpenRouterModelEffortLevels_ParsesSupportedEfforts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization header = %q, want %q", got, "Bearer test-key")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id": "anthropic/claude-sonnet-4.6",
					"reasoning": map[string]interface{}{
						"supported_efforts": []string{"low", "medium", "high"},
					},
				},
				{
					"id": "openai/gpt-4o", // no "reasoning" field at all
				},
			},
		})
	}))
	defer srv.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = srv.URL
	defer func() { openRouterModelsURL = orig }()

	levels, err := fetchOpenRouterModelEffortLevels("test-key", "anthropic/claude-sonnet-4.6")
	if err != nil {
		t.Fatalf("fetchOpenRouterModelEffortLevels() error = %v", err)
	}
	want := []string{"low", "medium", "high"}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
	for i, l := range want {
		if levels[i] != l {
			t.Errorf("levels[%d] = %q, want %q", i, levels[i], l)
		}
	}
}

// TestFetchOpenRouterModelEffortLevels_NoReasoningFieldReturnsNilNoError
// guards the "genuinely doesn't support it" case from being treated as a
// failure — a model missing "reasoning" entirely is a valid, common answer.
func TestFetchOpenRouterModelEffortLevels_NoReasoningFieldReturnsNilNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "openai/gpt-4o"},
			},
		})
	}))
	defer srv.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = srv.URL
	defer func() { openRouterModelsURL = orig }()

	levels, err := fetchOpenRouterModelEffortLevels("test-key", "openai/gpt-4o")
	if err != nil {
		t.Fatalf("fetchOpenRouterModelEffortLevels() error = %v, want nil error for a model with no reasoning support", err)
	}
	if levels != nil {
		t.Errorf("levels = %v, want nil", levels)
	}
}

// TestFetchOpenRouterModelEffortLevels_ModelNotFound guards against
// silently returning an empty/wrong answer for a typo'd or stale model ID.
func TestFetchOpenRouterModelEffortLevels_ModelNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []map[string]interface{}{}})
	}))
	defer srv.Close()

	orig := openRouterModelsURL
	openRouterModelsURL = srv.URL
	defer func() { openRouterModelsURL = orig }()

	_, err := fetchOpenRouterModelEffortLevels("test-key", "does/not-exist")
	if err == nil {
		t.Fatal("expected an error for a model absent from the catalog, got nil")
	}
}

// TestHandleProviderEffortLevels_UndiscoveredTypeReturnsEmpty covers every
// provider type with no known capability signal at all (see effort.go's
// package doc comment) — must return an empty list, never a guessed one.
// openai is the sharpest case: verified live that a non-reasoning OpenAI
// model 400s on reasoning_effort, so a non-empty static answer here would
// be actively unsafe.
func TestHandleProviderEffortLevels_UndiscoveredTypeReturnsEmpty(t *testing.T) {
	for _, ptype := range []string{"openai", "grok", "groq", "llamacpp", "opencode-zen", "opencode-go", "custom"} {
		t.Run(ptype, func(t *testing.T) {
			s := New(&swarmStubBridge{})

			r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type="+ptype, nil)
			w := httptest.NewRecorder()
			s.handleProviderEffortLevels(w, r)

			if w.Code != http.StatusOK {
				t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
			}
			var got struct {
				Levels []string `json:"levels"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
				t.Fatalf("unmarshal response: %v", err)
			}
			if len(got.Levels) != 0 {
				t.Errorf("levels = %v, want empty — %q has no known capability signal", got.Levels, ptype)
			}
		})
	}
}

// TestHandleProviderEffortLevels_GeminiNoModelYet mirrors OpenRouter's
// "nothing chosen yet" case — Gemini's discovery is per-model now too
// (fetchGeminiModelEffortLevels), not an unconditional static list.
func TestHandleProviderEffortLevels_GeminiNoModelYet(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=gemini", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	var got struct {
		Levels []string `json:"levels"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Levels) != 0 {
		t.Errorf("levels = %v, want empty (no model selected yet)", got.Levels)
	}
}

// TestHandleProviderEffortLevels_GeminiLiveDiscovery is the end-to-end
// path for the new Gemini discovery: a configured API key + a live
// "thinking": true response must surface EffortLevelsForGemini()'s names.
func TestHandleProviderEffortLevels_GeminiLiveDiscovery(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "gem-key" {
			t.Errorf("x-goog-api-key = %q, want %q", got, "gem-key")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "models/gemini-3-flash", "thinking": true})
	}))
	defer srv.Close()
	orig := geminiModelsBaseURL
	geminiModelsBaseURL = srv.URL
	defer func() { geminiModelsBaseURL = orig }()

	s := New(&stubBridgeWithProviders{
		swarmStubBridge: &swarmStubBridge{},
		providers:       []provider.ProviderConfig{{Type: provider.ProviderGemini, APIKey: "gem-key"}},
	})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=gemini&model=gemini-3-flash", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"medium"`) {
		t.Errorf("body = %s, want it to contain a known gemini effort level", w.Body.String())
	}
}

// TestHandleProviderEffortLevels_OpenRouterNoModelYet covers the "user
// hasn't picked a model yet" case — must return an empty list, not error,
// since there's nothing to discover against.
func TestHandleProviderEffortLevels_OpenRouterNoModelYet(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=openrouter", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	var got struct {
		Levels []string `json:"levels"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	if len(got.Levels) != 0 {
		t.Errorf("levels = %v, want empty (no model selected yet)", got.Levels)
	}
}

// TestHandleProviderEffortLevels_OpenRouterNoAPIKeyConfigured covers the
// "picked openrouter, picked a model, but never configured an OpenRouter
// API key" case — swarmStubBridge.GetProviders() returns nil by default,
// exercising exactly this path with no extra stub setup needed.
func TestHandleProviderEffortLevels_OpenRouterNoAPIKeyConfigured(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=openrouter&model=anthropic/claude-sonnet-4.6", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (no OpenRouter API key configured), body: %s", w.Code, w.Body.String())
	}
}

// TestHandleProviderEffortLevels_MissingType guards the required-param
// validation.
func TestHandleProviderEffortLevels_MissingType(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 for a missing type param", w.Code)
	}
}

// TestFetchClaudeModelEffortLevels_ParsesCapabilities is the regression for
// the new live discovery: Anthropic's GET /v1/models/{id} reports which
// specific effort levels a model supports (verified against current docs,
// 2026-08-18) — this must extract exactly the ones marked supported:true,
// in ascending order, skipping the rest.
func TestFetchClaudeModelEffortLevels_ParsesCapabilities(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-api-key"); got != "claude-key" {
			t.Errorf("x-api-key = %q, want %q", got, "claude-key")
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Error("anthropic-version header missing")
		}
		if !strings.HasSuffix(r.URL.Path, "/claude-opus-4-6") {
			t.Errorf("request path = %q, want it to end with the model id", r.URL.Path)
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "claude-opus-4-6",
			"capabilities": map[string]interface{}{
				"effort": map[string]interface{}{
					"supported": true,
					"low":       map[string]bool{"supported": true},
					"medium":    map[string]bool{"supported": true},
					"high":      map[string]bool{"supported": true},
					"xhigh":     map[string]bool{"supported": false},
					"max":       map[string]bool{"supported": true},
				},
			},
		})
	}))
	defer srv.Close()
	orig := claudeModelsBaseURL
	claudeModelsBaseURL = srv.URL
	defer func() { claudeModelsBaseURL = orig }()

	levels, err := fetchClaudeModelEffortLevels("claude-key", "claude-opus-4-6")
	if err != nil {
		t.Fatalf("fetchClaudeModelEffortLevels() error = %v", err)
	}
	want := []string{"low", "medium", "high", "max"}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
	for i, l := range want {
		if levels[i] != l {
			t.Errorf("levels[%d] = %q, want %q", i, levels[i], l)
		}
	}
}

// TestFetchClaudeModelEffortLevels_EffortNotSupportedReturnsNilNoError is
// the regression for claude.go's known-limitation gate: an older model
// (capabilities.effort.supported: false) must resolve to an empty picker,
// not an error — this is exactly what prevents Memo from ever sending
// adaptive-mode thinking to a model that would 400 on it.
func TestFetchClaudeModelEffortLevels_EffortNotSupportedReturnsNilNoError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"id": "claude-haiku-4-5",
			"capabilities": map[string]interface{}{
				"effort": map[string]interface{}{"supported": false},
			},
		})
	}))
	defer srv.Close()
	orig := claudeModelsBaseURL
	claudeModelsBaseURL = srv.URL
	defer func() { claudeModelsBaseURL = orig }()

	levels, err := fetchClaudeModelEffortLevels("claude-key", "claude-haiku-4-5")
	if err != nil {
		t.Fatalf("fetchClaudeModelEffortLevels() error = %v, want nil error", err)
	}
	if levels != nil {
		t.Errorf("levels = %v, want nil", levels)
	}
}

// TestFetchGeminiModelEffortLevels_ThinkingTrue verifies the "thinking"
// boolean gate surfaces EffortLevelsForGemini()'s static name list —
// Gemini's classic endpoint has no per-model level granularity of its own,
// only a yes/no for the capability.
func TestFetchGeminiModelEffortLevels_ThinkingTrue(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("x-goog-api-key"); got != "gem-key" {
			t.Errorf("x-goog-api-key = %q, want %q", got, "gem-key")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "models/gemini-3-flash", "thinking": true})
	}))
	defer srv.Close()
	orig := geminiModelsBaseURL
	geminiModelsBaseURL = srv.URL
	defer func() { geminiModelsBaseURL = orig }()

	levels, err := fetchGeminiModelEffortLevels("gem-key", "gemini-3-flash")
	if err != nil {
		t.Fatalf("fetchGeminiModelEffortLevels() error = %v", err)
	}
	if len(levels) == 0 {
		t.Fatal("levels = empty, want the static Gemini name list")
	}
}

// TestFetchGeminiModelEffortLevels_ThinkingFalse guards the opposite case
// — a model with no thinking support must yield an empty picker, not the
// full static list unconditionally (the exact class of bug the OpenCode
// Zen/Go fix in effort.go closed).
func TestFetchGeminiModelEffortLevels_ThinkingFalse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"name": "models/gemini-3-flash-lite", "thinking": false})
	}))
	defer srv.Close()
	orig := geminiModelsBaseURL
	geminiModelsBaseURL = srv.URL
	defer func() { geminiModelsBaseURL = orig }()

	levels, err := fetchGeminiModelEffortLevels("gem-key", "gemini-3-flash-lite")
	if err != nil {
		t.Fatalf("fetchGeminiModelEffortLevels() error = %v", err)
	}
	if levels != nil {
		t.Errorf("levels = %v, want nil", levels)
	}
}

// TestFetchOllamaModelEffortLevels_ThinkingCapabilityPresent is the
// regression for querying Ollama's NATIVE /api/show (not the OpenAI-compat
// endpoint ollama.go's ChatCompletion uses, which has no capability
// introspection) — verified against Ollama's own source
// (types/model/capability.go) for the real "thinking" capability string
// and the real think-level value set (low/medium/high/max, not "none").
func TestFetchOllamaModelEffortLevels_ThinkingCapabilityPresent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/show" {
			t.Errorf("path = %q, want /api/show", r.URL.Path)
		}
		var body struct {
			Model string `json:"model"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if body.Model != "deepseek-r1:latest" {
			t.Errorf("request model = %q, want %q", body.Model, "deepseek-r1:latest")
		}
		json.NewEncoder(w).Encode(map[string]interface{}{
			"capabilities": []string{"completion", "thinking"},
		})
	}))
	defer srv.Close()

	// baseURL mirrors the configured provider's OpenAI-compat URL
	// (".../v1") — the function must strip that suffix to reach the
	// native API root the httptest server here stands in for.
	levels, err := fetchOllamaModelEffortLevels(srv.URL+"/v1", "deepseek-r1:latest")
	if err != nil {
		t.Fatalf("fetchOllamaModelEffortLevels() error = %v", err)
	}
	want := []string{"low", "medium", "high", "max"}
	if len(levels) != len(want) {
		t.Fatalf("levels = %v, want %v", levels, want)
	}
	for i, l := range want {
		if levels[i] != l {
			t.Errorf("levels[%d] = %q, want %q", i, levels[i], l)
		}
	}
}

// TestFetchOllamaModelEffortLevels_ThinkingCapabilityAbsent guards a
// non-reasoning local model (e.g. llama3.2) yielding an empty picker.
func TestFetchOllamaModelEffortLevels_ThinkingCapabilityAbsent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"capabilities": []string{"completion", "vision"},
		})
	}))
	defer srv.Close()

	levels, err := fetchOllamaModelEffortLevels(srv.URL+"/v1", "llama3.2:latest")
	if err != nil {
		t.Fatalf("fetchOllamaModelEffortLevels() error = %v", err)
	}
	if levels != nil {
		t.Errorf("levels = %v, want nil", levels)
	}
}

// TestHandleProviderEffortLevels_ClaudeNoAPIKeyConfigured mirrors
// OpenRouter's equivalent — a model chosen but no Claude API key
// configured must 400, not silently discover nothing.
func TestHandleProviderEffortLevels_ClaudeNoAPIKeyConfigured(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=claude&model=claude-opus-4-6", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400 (no Claude API key configured), body: %s", w.Code, w.Body.String())
	}
}

// TestHandleProviderEffortLevels_OllamaUsesConfiguredBaseURL is the
// end-to-end path for Ollama discovery: a configured provider's own
// BaseURL (findProviderBaseURLFor) must be the one queried, not a
// hardcoded default — proven by pointing that BaseURL at an httptest
// server and asserting it actually received the request. (The
// empty-BaseURL fallback to provider.DefaultBaseURL is a one-line
// assignment exercised by construction here having no easy way to hit
// without a real network call to a possibly-running local Ollama, which
// would make the test's outcome depend on the machine it runs on — not
// worth the flakiness for what the code makes obvious.)
func TestHandleProviderEffortLevels_OllamaUsesConfiguredBaseURL(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{"capabilities": []string{"completion", "thinking"}})
	}))
	defer srv.Close()

	s := New(&stubBridgeWithProviders{
		swarmStubBridge: &swarmStubBridge{},
		providers:       []provider.ProviderConfig{{Type: provider.ProviderOllama, BaseURL: srv.URL + "/v1"}},
	})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=ollama&model=deepseek-r1:latest", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"low"`) {
		t.Errorf("body = %s, want it to contain the thinking-capable Ollama level set", w.Body.String())
	}
}

// ─── Kilo Code AI Gateway model browser ───

// TestFetchKiloModels_UsesIsFreeFieldDirectly is the actual point of not
// re-deriving IsFree from pricing like fetchOpenRouterModels does: an
// auto-routing model (pricing "-1", meaning "depends on whatever gets
// picked") must not be misclassified as free just because it isn't a
// positive number, and a real free model must come through as free.
func TestFetchKiloModels_UsesIsFreeFieldDirectly(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{
					"id":             "kilo-auto/frontier",
					"name":           "Auto Frontier",
					"context_length": 1000000,
					"pricing":        map[string]interface{}{"prompt": "-1", "completion": "-1"},
					"isFree":         false,
				},
				{
					"id":             "kilo-auto/free",
					"name":           "Auto Free",
					"context_length": 256000,
					"pricing":        map[string]interface{}{"prompt": "0", "completion": "0"},
					"isFree":         true,
				},
			},
		})
	}))
	defer srv.Close()

	orig := kiloModelsURL
	kiloModelsURL = srv.URL
	defer func() { kiloModelsURL = orig }()

	models, err := fetchKiloModels()
	if err != nil {
		t.Fatalf("fetchKiloModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].IsFree {
		t.Errorf("kilo-auto/frontier (pricing -1, isFree:false) reported as free")
	}
	if !models[1].IsFree {
		t.Errorf("kilo-auto/free (isFree:true) not reported as free")
	}
}

// TestFetchKiloModels_SkipsEntriesWithNoID mirrors fetchOpenRouterModels'
// own defensive skip — a malformed entry with no id can't be selected as a
// model anyway, so it shouldn't clutter the browser.
func TestFetchKiloModels_SkipsEntriesWithNoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "", "name": "broken"},
				{"id": "openai/gpt-5.6-sol", "name": "GPT-5.6 Sol"},
			},
		})
	}))
	defer srv.Close()

	orig := kiloModelsURL
	kiloModelsURL = srv.URL
	defer func() { kiloModelsURL = orig }()

	models, err := fetchKiloModels()
	if err != nil {
		t.Fatalf("fetchKiloModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "openai/gpt-5.6-sol" {
		t.Errorf("models = %+v, want exactly the one entry with a real id", models)
	}
}

// TestFetchKiloModels_PropagatesUpstreamError guards against silently
// returning an empty list when Kilo's API itself is down/erroring — the
// caller (handleKiloModels) needs to tell the difference between "no
// models" and "couldn't fetch models" to show the right thing to the user.
func TestFetchKiloModels_PropagatesUpstreamError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	orig := kiloModelsURL
	kiloModelsURL = srv.URL
	defer func() { kiloModelsURL = orig }()

	if _, err := fetchKiloModels(); err == nil {
		t.Fatal("expected an error when Kilo's API returns 500")
	}
}

// TestHandleKiloModels_NoAPIKeyRequired is the actual behavioral
// difference from handleOpenRouterModels: Kilo's /models endpoint needs no
// credential at all (kilo.ai/docs/gateway/models-and-providers), so a POST
// with a genuinely empty body must still succeed.
func TestHandleKiloModels_NoAPIKeyRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "kilo-auto/balanced", "name": "Auto Balanced", "isFree": false},
			},
		})
	}))
	defer srv.Close()

	orig := kiloModelsURL
	kiloModelsURL = srv.URL
	defer func() { kiloModelsURL = orig }()

	s := New(&swarmStubBridge{})
	r := httptest.NewRequest(http.MethodPost, "/api/kilo/models", nil)
	w := httptest.NewRecorder()
	s.handleKiloModels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want status ok", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "kilo-auto/balanced") {
		t.Errorf("body = %s, want the fetched model id present", w.Body.String())
	}
}

func TestHandleKiloModels_RejectsNonPost(t *testing.T) {
	s := New(&swarmStubBridge{})
	r := httptest.NewRequest(http.MethodGet, "/api/kilo/models", nil)
	w := httptest.NewRecorder()
	s.handleKiloModels(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}

// ─── OpenCode Zen's rich (free-aware) model browser ───

// TestFetchOpenCodeZenModels_DerivesIsFreeFromIDSuffix is the actual point
// of this fetcher over the generic ListModels path: OpenCode Zen's /models
// response carries no pricing/free field at all, only a "-free" suffix on
// the id itself for genuinely free models (verified against the real,
// live catalog).
func TestFetchOpenCodeZenModels_DerivesIsFreeFromIDSuffix(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "claude-sonnet-5", "object": "model", "owned_by": "opencode"},
				{"id": "deepseek-v4-flash-free", "object": "model", "owned_by": "opencode"},
			},
		})
	}))
	defer srv.Close()

	orig := openCodeZenModelsURL
	openCodeZenModelsURL = srv.URL
	defer func() { openCodeZenModelsURL = orig }()

	models, err := fetchOpenCodeZenModels()
	if err != nil {
		t.Fatalf("fetchOpenCodeZenModels() error = %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("got %d models, want 2", len(models))
	}
	if models[0].IsFree {
		t.Errorf("claude-sonnet-5 (no -free suffix) reported as free")
	}
	if !models[1].IsFree {
		t.Errorf("deepseek-v4-flash-free (-free suffix) not reported as free")
	}
}

func TestFetchOpenCodeZenModels_SkipsEntriesWithNoID(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": ""},
				{"id": "gpt-5.6-sol"},
			},
		})
	}))
	defer srv.Close()

	orig := openCodeZenModelsURL
	openCodeZenModelsURL = srv.URL
	defer func() { openCodeZenModelsURL = orig }()

	models, err := fetchOpenCodeZenModels()
	if err != nil {
		t.Fatalf("fetchOpenCodeZenModels() error = %v", err)
	}
	if len(models) != 1 || models[0].ID != "gpt-5.6-sol" {
		t.Errorf("models = %+v, want exactly the one entry with a real id", models)
	}
}

func TestHandleOpenCodeZenModels_NoAPIKeyRequired(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []map[string]interface{}{
				{"id": "claude-sonnet-5"},
			},
		})
	}))
	defer srv.Close()

	orig := openCodeZenModelsURL
	openCodeZenModelsURL = srv.URL
	defer func() { openCodeZenModelsURL = orig }()

	s := New(&swarmStubBridge{})
	r := httptest.NewRequest(http.MethodPost, "/api/opencode-zen/models", nil)
	w := httptest.NewRecorder()
	s.handleOpenCodeZenModels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"status":"ok"`) {
		t.Errorf("body = %s, want status ok", w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "claude-sonnet-5") {
		t.Errorf("body = %s, want the fetched model id present", w.Body.String())
	}
}

func TestHandleOpenCodeZenModels_RejectsNonPost(t *testing.T) {
	s := New(&swarmStubBridge{})
	r := httptest.NewRequest(http.MethodGet, "/api/opencode-zen/models", nil)
	w := httptest.NewRecorder()
	s.handleOpenCodeZenModels(w, r)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", w.Code)
	}
}
