package webserver

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

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

// TestHandleProviderEffortLevels_StaticType covers the vendor-table path
// (no network call, no bridge dependency beyond fullBridge being non-nil).
func TestHandleProviderEffortLevels_StaticType(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=openai", nil)
	w := httptest.NewRecorder()
	s.handleProviderEffortLevels(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200, body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), `"high"`) {
		t.Errorf("body = %s, want it to contain a known openai effort level", w.Body.String())
	}
}

// TestHandleProviderEffortLevels_Gemini covers the token-budget-mapped
// vendor, which has its own label list separate from effortLevelsByType.
func TestHandleProviderEffortLevels_Gemini(t *testing.T) {
	s := New(&swarmStubBridge{})

	r := httptest.NewRequest(http.MethodGet, "/api/providers/effort-levels?type=gemini", nil)
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
