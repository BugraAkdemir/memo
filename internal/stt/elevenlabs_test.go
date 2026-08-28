package stt

import (
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestElevenLabsProvider_TranscribeSuccess(t *testing.T) {
	var gotKey, gotPath, gotModelID string
	var gotFileBytes []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("xi-api-key")
		gotPath = r.URL.Path

		mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "multipart/form-data" {
			t.Fatalf("expected multipart/form-data, got %q (%v)", r.Header.Get("Content-Type"), err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			switch part.FormName() {
			case "model_id":
				buf := make([]byte, 64)
				n, _ := part.Read(buf)
				gotModelID = string(buf[:n])
			case "file":
				buf := make([]byte, 1024)
				n, _ := part.Read(buf)
				gotFileBytes = buf[:n]
			}
		}
		w.Write([]byte(`{"text":"merhaba dünya"}`))
	}))
	defer srv.Close()

	p := &elevenLabsProvider{baseURL: srv.URL, apiKey: "el-key", client: srv.Client()}
	text, err := p.Transcribe(context.Background(), []byte("fake-audio-bytes"))
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "merhaba dünya" {
		t.Errorf("expected 'merhaba dünya', got %q", text)
	}
	if gotPath != "/speech-to-text" {
		t.Errorf("expected /speech-to-text, got %q", gotPath)
	}
	if gotKey != "el-key" {
		t.Errorf("expected xi-api-key header, got %q", gotKey)
	}
	if gotModelID != elevenLabsSTTModel {
		t.Errorf("expected model_id=%s, got %q", elevenLabsSTTModel, gotModelID)
	}
	if string(gotFileBytes) != "fake-audio-bytes" {
		t.Errorf("expected audio bytes to round-trip, got %q", gotFileBytes)
	}
}

func TestElevenLabsProvider_TranscribeErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":{"message":"invalid_api_key"}}`))
	}))
	defer srv.Close()

	p := &elevenLabsProvider{baseURL: srv.URL, apiKey: "bad", client: srv.Client()}
	_, err := p.Transcribe(context.Background(), []byte("audio"))
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
