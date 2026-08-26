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

// TestListLiveModels_FollowsPagination is a regression test for a real bug
// found via live testing: models.list paginates (nextPageToken), and the
// original implementation only ever read the first page, silently dropping
// any live-capable model that sorted past it.
func TestListLiveModels_FollowsPagination(t *testing.T) {
	var gotPageTokens []string
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotPageTokens = append(gotPageTokens, r.URL.Query().Get("pageToken"))
		switch r.URL.Query().Get("pageToken") {
		case "":
			w.Write([]byte(`{
				"models": [
					{"name":"models/gemini-page1-live","supportedGenerationMethods":["bidiGenerateContent"]},
					{"name":"models/gemini-page1-pro","supportedGenerationMethods":["generateContent"]}
				],
				"nextPageToken": "page-2"
			}`))
		case "page-2":
			w.Write([]byte(`{
				"models": [
					{"name":"models/gemini-page2-live","supportedGenerationMethods":["bidiGenerateContent"]}
				]
			}`))
		default:
			t.Errorf("unexpected pageToken: %q", r.URL.Query().Get("pageToken"))
		}
	})

	models, err := ListLiveModels(context.Background(), "g-key")
	if err != nil {
		t.Fatalf("ListLiveModels: %v", err)
	}
	if len(gotPageTokens) != 2 {
		t.Fatalf("expected 2 requests (one per page), got %d: %v", len(gotPageTokens), gotPageTokens)
	}
	if len(models) != 2 {
		t.Fatalf("expected models from both pages, got %d: %+v", len(models), models)
	}
	names := map[string]bool{models[0].Name: true, models[1].Name: true}
	if !names["models/gemini-page1-live"] || !names["models/gemini-page2-live"] {
		t.Errorf("expected live models from both page 1 and page 2, got %+v", models)
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
