package stt

import (
	"context"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCustomProvider_TranscribeSuccess(t *testing.T) {
	var gotAuth, gotPath, gotModel string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
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
			if part.FormName() == "model" {
				buf := make([]byte, 64)
				n, _ := part.Read(buf)
				gotModel = string(buf[:n])
			}
		}
		w.Write([]byte(`{"text":"merhaba"}`))
	}))
	defer srv.Close()

	p, err := newCustomProvider(ProviderConfig{BaseURL: srv.URL, APIKey: "custom-key"})
	if err != nil {
		t.Fatalf("newCustomProvider: %v", err)
	}
	text, err := p.Transcribe(context.Background(), []byte("audio"))
	if err != nil {
		t.Fatalf("Transcribe: %v", err)
	}
	if text != "merhaba" {
		t.Errorf("expected 'merhaba', got %q", text)
	}
	if gotPath != "/audio/transcriptions" {
		t.Errorf("expected /audio/transcriptions (Whisper API-compatible path), got %q", gotPath)
	}
	if gotAuth != "Bearer custom-key" {
		t.Errorf("expected Bearer custom-key, got %q", gotAuth)
	}
	if gotModel != customSTTModel {
		t.Errorf("expected model=%s, got %q", customSTTModel, gotModel)
	}
}

func TestCustomProvider_MissingBaseURL(t *testing.T) {
	_, err := newCustomProvider(ProviderConfig{})
	if err == nil {
		t.Fatal("expected an error when base_url is missing")
	}
}
