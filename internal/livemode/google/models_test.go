package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func withDiscoveryServer(t *testing.T, handler http.HandlerFunc) {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	original := DiscoveryBaseURL
	DiscoveryBaseURL = srv.URL
	t.Cleanup(func() { DiscoveryBaseURL = original })
}

func TestListLiveModels_FiltersToBidiGenerateContentOnly(t *testing.T) {
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("key") != "g-key" {
			t.Errorf("expected key query param, got %q", r.URL.RawQuery)
		}
		w.Write([]byte(`{
			"models": [
				{"name":"models/gemini-3.1-flash-live-preview","displayName":"Gemini Live","supportedGenerationMethods":["generateContent","bidiGenerateContent"]},
				{"name":"models/gemini-3.1-pro","displayName":"Gemini Pro","supportedGenerationMethods":["generateContent"]}
			]
		}`))
	})

	models, err := ListLiveModels(context.Background(), "g-key")
	if err != nil {
		t.Fatalf("ListLiveModels: %v", err)
	}
	if len(models) != 1 {
		t.Fatalf("expected 1 live-capable model, got %d: %+v", len(models), models)
	}
	if models[0].Name != "models/gemini-3.1-flash-live-preview" {
		t.Errorf("unexpected surviving model: %+v", models[0])
	}
}

func TestListLiveModels_ErrorStatus(t *testing.T) {
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"API key not valid"}}`))
	})

	_, err := ListLiveModels(context.Background(), "bad-key")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestListLiveModels_EmptyResultWhenNoneAreLiveCapable(t *testing.T) {
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"models":[{"name":"models/gemini-3.1-pro","supportedGenerationMethods":["generateContent"]}]}`))
	})

	models, err := ListLiveModels(context.Background(), "g-key")
	if err != nil {
		t.Fatalf("ListLiveModels: %v", err)
	}
	if len(models) != 0 {
		t.Errorf("expected no live-capable models, got %+v", models)
	}
}
