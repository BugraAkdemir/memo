package provider

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestOpenRouterProvider(t *testing.T, srv *httptest.Server) *openRouterProvider {
	t.Helper()
	p, err := newOpenRouterProvider(ProviderConfig{
		Type:    ProviderOpenRouter,
		BaseURL: srv.URL,
		Model:   "inclusionai/ming-image-0.1-design",
		APIKey:  "test-key-123",
	})
	if err != nil {
		t.Fatalf("newOpenRouterProvider() error = %v", err)
	}
	return p
}

func TestOpenRouterProvider_GenerateImage_PostsToImagesEndpoint(t *testing.T) {
	resetImageModelCache()
	wantPNG := []byte{0x89, 'P', 'N', 'G', 1, 2, 3}

	var gotPath, gotAuth string
	var gotBody openRouterImageRequest
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"created":1,"data":[{"b64_json":"` +
			base64.StdEncoding.EncodeToString(wantPNG) +
			`","media_type":"image/png"}],"usage":{"prompt_tokens":7,"completion_tokens":4175,"total_tokens":4182}}`))
	}))
	defer srv.Close()

	p := newTestOpenRouterProvider(t, srv)
	resp, err := p.GenerateImage(context.Background(), ImageRequest{Prompt: "a cat flying in space"})
	if err != nil {
		t.Fatalf("GenerateImage() error = %v", err)
	}

	// The whole point of this path: images go to /images, never to
	// /chat/completions (which answers an image model with a 404).
	if gotPath != "/images" {
		t.Errorf("request path = %q, want /images", gotPath)
	}
	if gotAuth != "Bearer test-key-123" {
		t.Errorf("Authorization = %q, want Bearer test-key-123", gotAuth)
	}
	if gotBody.Model != "inclusionai/ming-image-0.1-design" {
		t.Errorf("model = %q, want the configured one", gotBody.Model)
	}
	if gotBody.Prompt != "a cat flying in space" {
		t.Errorf("prompt = %q, want the user's text", gotBody.Prompt)
	}
	// Unset optionals must not be sent at all, so the model's own defaults win.
	if gotBody.N != 0 || gotBody.Size != "" || gotBody.AspectRatio != "" || gotBody.OutputFormat != "" {
		t.Errorf("unset optional fields leaked into the request: %+v", gotBody)
	}

	if len(resp.Images) != 1 {
		t.Fatalf("len(Images) = %d, want 1", len(resp.Images))
	}
	decoded, err := base64.StdEncoding.DecodeString(resp.Images[0].B64JSON)
	if err != nil {
		t.Fatalf("returned b64_json does not decode: %v", err)
	}
	if string(decoded) != string(wantPNG) {
		t.Errorf("decoded image bytes = %v, want %v", decoded, wantPNG)
	}
	if resp.Images[0].MediaType != "image/png" {
		t.Errorf("MediaType = %q, want image/png", resp.Images[0].MediaType)
	}
	if resp.Usage == nil || resp.Usage.CompletionTokens != 4175 {
		t.Errorf("Usage = %+v, want completion_tokens 4175", resp.Usage)
	}
}

func TestOpenRouterProvider_GenerateImage_EmptyDataIsAnError(t *testing.T) {
	resetImageModelCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"created":1,"data":[]}`))
	}))
	defer srv.Close()

	p := newTestOpenRouterProvider(t, srv)
	if _, err := p.GenerateImage(context.Background(), ImageRequest{Prompt: "x"}); err == nil {
		t.Fatal("GenerateImage() error = nil, want an error — an HTTP 200 with no images is not a usable turn")
	}
}

// The image-output-only models are absent from OpenRouter's default /models
// response and only appear under ?output_modalities=image — detection must
// use that filter or it finds nothing at all.
func TestOpenRouterProvider_IsImageOnlyModel_UsesOutputModalitiesFilter(t *testing.T) {
	resetImageModelCache()
	var gotQuery string
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		gotQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{"data":[{"id":"inclusionai/ming-image-0.1-design"},{"id":"qwen/qwen-image-3"}]}`))
	}))
	defer srv.Close()

	p := newTestOpenRouterProvider(t, srv)
	ctx := context.Background()

	if !p.IsImageOnlyModel(ctx, "inclusionai/ming-image-0.1-design") {
		t.Error("IsImageOnlyModel() = false for a catalogued image model, want true")
	}
	if !strings.Contains(gotQuery, "output_modalities=image") {
		t.Errorf("catalog query = %q, want it to carry output_modalities=image", gotQuery)
	}
	if p.IsImageOnlyModel(ctx, "anthropic/claude-sonnet-4.6") {
		t.Error("IsImageOnlyModel() = true for a chat model, want false")
	}
	if p.IsImageOnlyModel(ctx, "") {
		t.Error("IsImageOnlyModel() = true for an empty model id, want false")
	}
	if calls != 1 {
		t.Errorf("catalog fetched %d times, want 1 — the result must be cached, not re-fetched per message", calls)
	}
}

// A catalog that can't be reached must not divert an ordinary chat model to
// the images endpoint — best-effort means "false on doubt".
func TestOpenRouterProvider_IsImageOnlyModel_UnreachableCatalogReportsFalse(t *testing.T) {
	resetImageModelCache()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"error":{"message":"nope"}}`, http.StatusInternalServerError)
	}))
	defer srv.Close()

	p := newTestOpenRouterProvider(t, srv)
	if p.IsImageOnlyModel(context.Background(), "inclusionai/ming-image-0.1-design") {
		t.Error("IsImageOnlyModel() = true when the catalog was unreachable, want false")
	}
}

// Router.ImageGenerator answers for the provider a turn would actually hit
// (the highest-priority live entry), not for any image-capable provider
// sitting further down the fallback chain.
func TestRouter_ImageGenerator_OnlyConsidersTheFirstEntry(t *testing.T) {
	r := NewRouter([]ProviderConfig{
		{Type: ProviderOpenAI, Name: "openai", Model: "gpt-4o", APIKey: "k", Enabled: true, Priority: 10},
		{Type: ProviderOpenRouter, Name: "openrouter", Model: "inclusionai/ming-image-0.1-design", APIKey: "k", Enabled: true, Priority: 1},
	})
	if _, _, ok := r.ImageGenerator(); ok {
		t.Error("ImageGenerator() ok = true when the top-priority provider is a chat provider, want false")
	}

	r.SetActiveProvider("openrouter")
	gen, model, ok := r.ImageGenerator()
	if !ok {
		t.Fatal("ImageGenerator() ok = false for an explicitly active OpenRouter provider, want true")
	}
	if gen == nil {
		t.Error("ImageGenerator() returned a nil generator")
	}
	if model != "inclusionai/ming-image-0.1-design" {
		t.Errorf("model = %q, want the configured one", model)
	}
}
