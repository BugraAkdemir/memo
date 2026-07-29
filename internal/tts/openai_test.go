package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestOpenAIProvider_SynthesizeSuccess(t *testing.T) {
	var gotBody openAISpeechRequest
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Header().Set("Content-Type", "audio/wav")
		w.Write([]byte("fake-wav-bytes"))
	}))
	defer srv.Close()

	p := &openAIProvider{baseURL: srv.URL, apiKey: "sk-test", client: srv.Client()}

	audio, err := p.Synthesize(context.Background(), "merhaba dünya", "alloy")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if string(audio) != "fake-wav-bytes" {
		t.Errorf("expected fake-wav-bytes, got %q", audio)
	}
	if gotAuth != "Bearer sk-test" {
		t.Errorf("expected Bearer sk-test, got %q", gotAuth)
	}
	if gotBody.Input != "merhaba dünya" || gotBody.Voice != "alloy" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
	if gotBody.ResponseFormat != "wav" {
		t.Errorf("expected response_format=wav (Content-Type: audio/wav is hardcoded downstream), got %q", gotBody.ResponseFormat)
	}
}

func TestOpenAIProvider_SynthesizeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Incorrect API key provided"}}`))
	}))
	defer srv.Close()

	p := &openAIProvider{baseURL: srv.URL, apiKey: "sk-bad", client: srv.Client()}

	_, err := p.Synthesize(context.Background(), "merhaba", "alloy")
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "Incorrect API key provided") {
		t.Errorf("expected unwrapped error message in error, got: %v", err)
	}
}

func TestOpenAIProvider_NameAndDisplayName(t *testing.T) {
	p, err := newOpenAIProvider(ProviderConfig{APIKey: "sk-x"})
	if err != nil {
		t.Fatalf("newOpenAIProvider: %v", err)
	}
	if p.Name() != ProviderOpenAI {
		t.Errorf("expected %s, got %s", ProviderOpenAI, p.Name())
	}
	if p.DisplayName() != "OpenAI" {
		t.Errorf("expected OpenAI, got %s", p.DisplayName())
	}
}
