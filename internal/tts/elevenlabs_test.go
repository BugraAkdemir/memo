package tts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestElevenLabsProvider_SynthesizeSuccess(t *testing.T) {
	var gotPath, gotKey, gotQuery string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotKey = r.Header.Get("xi-api-key")
		gotQuery = r.URL.RawQuery
		w.Write([]byte("fake-wav-bytes"))
	}))
	defer srv.Close()

	p := &elevenLabsProvider{baseURL: srv.URL, apiKey: "el-key", client: srv.Client()}
	audio, err := p.Synthesize(context.Background(), "merhaba", "voice123")
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	if string(audio) != "fake-wav-bytes" {
		t.Errorf("expected fake-wav-bytes, got %q", audio)
	}
	if gotPath != "/text-to-speech/voice123" {
		t.Errorf("expected /text-to-speech/{voice_id} path, got %q", gotPath)
	}
	if gotKey != "el-key" {
		t.Errorf("expected xi-api-key header, got %q", gotKey)
	}
	if !strings.Contains(gotQuery, "output_format="+elevenLabsOutputFormat) {
		t.Errorf("expected output_format=%s in query, got %q", elevenLabsOutputFormat, gotQuery)
	}
}

func TestElevenLabsProvider_SynthesizeMissingVoice(t *testing.T) {
	p := &elevenLabsProvider{baseURL: "http://unused", apiKey: "el-key", client: http.DefaultClient}
	_, err := p.Synthesize(context.Background(), "merhaba", "")
	if err == nil {
		t.Fatal("expected an error when voice is empty")
	}
}

func TestElevenLabsProvider_SynthesizeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":{"message":"invalid_api_key"}}`))
	}))
	defer srv.Close()

	p := &elevenLabsProvider{baseURL: srv.URL, apiKey: "bad", client: srv.Client()}
	_, err := p.Synthesize(context.Background(), "merhaba", "voice123")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestElevenLabsProvider_NameAndDisplayName(t *testing.T) {
	p, err := newElevenLabsProvider(ProviderConfig{APIKey: "el-key"})
	if err != nil {
		t.Fatalf("newElevenLabsProvider: %v", err)
	}
	if p.Name() != ProviderElevenLabs {
		t.Errorf("expected %s, got %s", ProviderElevenLabs, p.Name())
	}
	if p.DisplayName() != "ElevenLabs" {
		t.Errorf("expected ElevenLabs, got %s", p.DisplayName())
	}
}
