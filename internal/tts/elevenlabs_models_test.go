package tts

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withElevenLabsDiscoveryServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	original := elevenLabsDiscoveryBaseURL
	elevenLabsDiscoveryBaseURL = srv.URL
	t.Cleanup(func() { elevenLabsDiscoveryBaseURL = original })
}

func TestListElevenLabsModels_ParsesAndReturnsAll(t *testing.T) {
	withElevenLabsDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" {
			t.Errorf("expected /models, got %q", r.URL.Path)
		}
		if r.Header.Get("xi-api-key") != "el-key" {
			t.Errorf("expected xi-api-key header, got %q", r.Header.Get("xi-api-key"))
		}
		w.Write([]byte(`[
			{"model_id":"eleven_multilingual_v2","name":"Multilingual v2","can_do_text_to_speech":true},
			{"model_id":"eleven_voice_conversion","name":"Voice Conversion","can_do_text_to_speech":false,"can_do_voice_conversion":true}
		]`))
	})

	models, err := ListElevenLabsModels(context.Background(), "el-key")
	if err != nil {
		t.Fatalf("ListElevenLabsModels: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 models, got %d", len(models))
	}
	if !models[0].CanDoTextToSpeech || models[1].CanDoTextToSpeech {
		t.Errorf("expected only the first model to be TTS-capable, got %+v", models)
	}
}

func TestListElevenLabsModels_ErrorStatus(t *testing.T) {
	withElevenLabsDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"detail":{"message":"invalid_api_key"}}`))
	})

	_, err := ListElevenLabsModels(context.Background(), "bad-key")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestListElevenLabsVoices_ParsesVoices(t *testing.T) {
	withElevenLabsDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/voices" {
			t.Errorf("expected /voices, got %q", r.URL.Path)
		}
		w.Write([]byte(`{"voices":[{"voice_id":"abc123","name":"Rachel"}]}`))
	})

	voices, err := ListElevenLabsVoices(context.Background(), "el-key")
	if err != nil {
		t.Fatalf("ListElevenLabsVoices: %v", err)
	}
	if len(voices) != 1 || voices[0].VoiceID != "abc123" || voices[0].Name != "Rachel" {
		t.Errorf("unexpected voices: %+v", voices)
	}
}
