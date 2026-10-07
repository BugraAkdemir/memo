package provider

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeImageEndpoint behaves like the Subscriptions sidecar as measured on
// 2026-10-07: /images/generations answers a prompt-less request locally — "prompt
// is required" for a model it serves, "is not supported" for any other — and a
// Gemini-style catalogue lists output modalities.
type fakeImageEndpoint struct {
	srv        *httptest.Server
	probes     atomic.Int32
	catalogHit atomic.Int32
	lastPath   atomic.Value // string
	lastBody   atomic.Value // map[string]any
	chatBody   atomic.Value // map[string]any
}

func newFakeImageEndpoint(t *testing.T) *fakeImageEndpoint {
	t.Helper()
	f := &fakeImageEndpoint{}
	f.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		var body map[string]any
		_ = json.Unmarshal(raw, &body)
		model, _ := body["model"].(string)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/images/generations", "/v1/images/edits":
			f.lastPath.Store(r.URL.Path)
			if model != "pic-model" {
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"Model ` + model + ` is not supported on /v1/images/generations or /v1/images/edits."}}`))
				return
			}
			if _, has := body["prompt"]; !has {
				f.probes.Add(1)
				w.WriteHeader(http.StatusBadRequest)
				_, _ = w.Write([]byte(`{"error":{"message":"Invalid request: prompt is required"}}`))
				return
			}
			f.lastBody.Store(body)
			_, _ = w.Write([]byte(`{"data":[{"b64_json":"AAAA"}],"output_format":"webp","usage":{"input_tokens":3,"output_tokens":9,"total_tokens":12}}`))
		case "/v1beta/models":
			f.catalogHit.Add(1)
			_, _ = w.Write([]byte(`{"models":[
				{"name":"models/chat-draws","supportedOutputModalities":["text","image"]},
				{"name":"models/plain-chat","supportedOutputModalities":["text"]}]}`))
		case "/v1/chat/completions":
			f.chatBody.Store(body)
			_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"here","images":[{"type":"image_url","image_url":{"url":"data:image/jpeg;base64,BBBB"}}]}}],"usage":{"prompt_tokens":5,"completion_tokens":7,"total_tokens":12}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(f.srv.Close)
	resetOpenAIImageCaches()
	t.Cleanup(resetOpenAIImageCaches)
	return f
}

func (f *fakeImageEndpoint) provider(t *testing.T, typ ProviderType) *openAIProvider {
	t.Helper()
	p, err := newOpenAIProvider(ProviderConfig{Type: typ, BaseURL: f.srv.URL + "/v1", APIKey: "k", Model: "x"})
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func TestImageModelsAreRecognisedByAskingTheEndpoint(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	ctx := context.Background()

	if !p.IsImageOnlyModel(ctx, "pic-model") {
		t.Error("a model the images endpoint serves must be an image model")
	}
	if !p.IsImageOnlyModel(ctx, "chat-draws") {
		t.Error("a chat model whose catalogue lists image output must be an image model")
	}
	if p.IsImageOnlyModel(ctx, "plain-chat") {
		t.Error("an ordinary chat model must stay on the chat path")
	}
	if p.IsImageOnlyModel(ctx, "never-heard-of-it") {
		t.Error("an unknown model must stay on the chat path")
	}
	if p.IsImageOnlyModel(ctx, "") {
		t.Error("no model is not an image model")
	}
}

func TestImageDetectionIsCachedAndNeverSpendsAChatRequest(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	for i := 0; i < 5; i++ {
		p.IsImageOnlyModel(context.Background(), "pic-model")
		p.IsImageOnlyModel(context.Background(), "plain-chat")
	}
	if got := f.probes.Load(); got != 1 {
		t.Errorf("the prompt-less probe ran %d times for one image model, want 1 (cached)", got)
	}
	if f.catalogHit.Load() != 1 {
		t.Errorf("the catalogue was fetched %d times, want once for the whole endpoint", f.catalogHit.Load())
	}
	if f.chatBody.Load() != nil {
		t.Error("detection must never send a chat request: that reaches the vendor")
	}
}

func TestOnlyACustomEndpointIsEverAsked(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderOpenAI)
	if p.IsImageOnlyModel(context.Background(), "pic-model") {
		t.Error("a real OpenAI provider keeps its chat path")
	}
	if f.probes.Load() != 0 {
		t.Error("a real OpenAI endpoint must not be probed")
	}
}

func TestAnUnreachableEndpointIsAChatModelAndIsNotProbedEveryTurn(t *testing.T) {
	resetOpenAIImageCaches()
	t.Cleanup(resetOpenAIImageCaches)
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()
	p, _ := newOpenAIProvider(ProviderConfig{Type: ProviderCustom, BaseURL: srv.URL + "/v1", Model: "x"})
	for i := 0; i < 4; i++ {
		if p.IsImageOnlyModel(context.Background(), "m") {
			t.Fatal("a probe that failed must not reroute the chat")
		}
	}
	if hits.Load() != 1 {
		t.Errorf("a failing endpoint was probed %d times in a row, want 1 (failure remembered briefly)", hits.Load())
	}
}

func TestGenerateImageThroughTheImagesEndpoint(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	resp, err := p.GenerateImage(context.Background(), ImageRequest{Model: "pic-model", Prompt: "a cat", Size: "1024x1024"})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastPath.Load() != "/v1/images/generations" {
		t.Errorf("path = %v", f.lastPath.Load())
	}
	body := f.lastBody.Load().(map[string]any)
	if body["prompt"] != "a cat" || body["size"] != "1024x1024" || body["response_format"] != "b64_json" {
		t.Errorf("body = %v", body)
	}
	if _, has := body["images"]; has {
		t.Error("a text-to-image call must not carry source images")
	}
	if len(resp.Images) != 1 || resp.Images[0].B64JSON != "AAAA" || resp.Images[0].MediaType != "image/webp" {
		t.Errorf("images = %+v", resp.Images)
	}
	if resp.Usage == nil || resp.Usage.TotalTokens != 12 {
		t.Errorf("usage = %+v", resp.Usage)
	}
}

func TestGenerateImageWithSourcePicturesIsAnEdit(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	_, err := p.GenerateImage(context.Background(), ImageRequest{
		Model: "pic-model", Prompt: "make it night",
		Images: []ImageInput{{B64JSON: "SRC1", MediaType: "image/jpeg"}, {B64JSON: "SRC2"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if f.lastPath.Load() != "/v1/images/edits" {
		t.Errorf("path = %v, want the edits endpoint", f.lastPath.Load())
	}
	imgs, _ := f.lastBody.Load().(map[string]any)["images"].([]any)
	if len(imgs) != 2 {
		t.Fatalf("images = %v", imgs)
	}
	if got := imgs[0].(map[string]any)["image_url"]; got != "data:image/jpeg;base64,SRC1" {
		t.Errorf("first image = %v", got)
	}
	if got := imgs[1].(map[string]any)["image_url"]; got != "data:image/png;base64,SRC2" {
		t.Errorf("a source with no media type defaults to png, got %v", got)
	}
}

func TestGenerateImageThroughAChatModelThatDraws(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	resp, err := p.GenerateImage(context.Background(), ImageRequest{
		Model: "chat-draws", Prompt: "a dog", AspectRatio: "16:9",
		Images: []ImageInput{{B64JSON: "SRC", MediaType: "image/png"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	body := f.chatBody.Load().(map[string]any)
	mods, _ := body["modalities"].([]any)
	if len(mods) != 2 || mods[0] != "image" {
		t.Errorf("modalities = %v, want [image text]", mods)
	}
	if cfg, _ := body["image_config"].(map[string]any); cfg["aspect_ratio"] != "16:9" {
		t.Errorf("image_config = %v", body["image_config"])
	}
	parts := body["messages"].([]any)[0].(map[string]any)["content"].([]any)
	if len(parts) != 2 || parts[1].(map[string]any)["type"] != "image_url" {
		t.Errorf("a source picture must travel as an image_url part, got %v", parts)
	}
	if len(resp.Images) != 1 || resp.Images[0].B64JSON != "BBBB" || resp.Images[0].MediaType != "image/jpeg" {
		t.Errorf("images = %+v", resp.Images)
	}
}

func TestAnEmptyPromptIsRefusedBeforeAnyRequest(t *testing.T) {
	f := newFakeImageEndpoint(t)
	p := f.provider(t, ProviderCustom)
	if _, err := p.GenerateImage(context.Background(), ImageRequest{Model: "pic-model", Prompt: "  "}); err == nil {
		t.Error("expected an error for a blank prompt")
	}
}

func TestParseImageInput(t *testing.T) {
	cases := []struct {
		in    string
		ok    bool
		b64   string
		media string
	}{
		{"data:image/jpeg;base64,QUJD", true, "QUJD", "image/jpeg"},
		{"QUJD", true, "QUJD", "image/png"},
		{"data:text/plain;base64,QUJD", true, "QUJD", "image/png"}, // not an image type: default
		{"https://example.test/a.png", false, "", ""},
		{"data:image/png;charset=utf8,rawtext", false, "", ""},
		{"data:image/png;base64,", false, "", ""},
		{"", false, "", ""},
	}
	for _, c := range cases {
		got, ok := ParseImageInput(c.in)
		if ok != c.ok || got.B64JSON != c.b64 || got.MediaType != c.media {
			t.Errorf("ParseImageInput(%q) = %+v,%v want %s %s,%v", c.in, got, ok, c.b64, c.media, c.ok)
		}
	}
	if !strings.HasPrefix((ImageInput{B64JSON: "x"}).DataURL(), "data:image/png;base64,") {
		t.Error("DataURL default media type")
	}
}
