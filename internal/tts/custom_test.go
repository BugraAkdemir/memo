package tts

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomProvider_SynthesizeSuccess(t *testing.T) {
	var gotBody openAISpeechRequest
	var gotAuth, gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.Write([]byte("fake-wav-bytes"))
	}))
	defer srv.Close()

	p, err := newCustomProvider(ProviderConfig{BaseURL: srv.URL, APIKey: "custom-key"})
	if err != nil {
		t.Fatalf("newCustomProvider: %v", err)
	}
	audio, err := p.Synthesize(context.Background(), "merhaba", "myvoice")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if string(audio) != "fake-wav-bytes" {
		t.Errorf("expected fake-wav-bytes, got %q", audio)
	}
	if gotPath != "/audio/speech" {
		t.Errorf("expected /audio/speech (OpenAI-compatible path), got %q", gotPath)
	}
	if gotAuth != "Bearer custom-key" {
		t.Errorf("expected Bearer custom-key, got %q", gotAuth)
	}
	if gotBody.Voice != "myvoice" || gotBody.ResponseFormat != "wav" {
		t.Errorf("unexpected request body: %+v", gotBody)
	}
}

func TestCustomProvider_NoAPIKeyOmitsAuthHeader(t *testing.T) {
	var sawAuth bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawAuth = r.Header.Get("Authorization") != ""
		w.Write([]byte("fake-wav-bytes"))
	}))
	defer srv.Close()

	p, err := newCustomProvider(ProviderConfig{BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("newCustomProvider: %v", err)
	}
	if _, err := p.Synthesize(context.Background(), "merhaba", "myvoice"); err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if sawAuth {
		t.Error("expected no Authorization header when APIKey is empty (self-hosted endpoints may not need one)")
	}
}

func TestCustomProvider_MissingBaseURL(t *testing.T) {
	_, err := newCustomProvider(ProviderConfig{})
	if err == nil {
		t.Fatal("expected an error when base_url is missing")
	}
}
