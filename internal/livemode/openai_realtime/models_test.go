package openai_realtime

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

func TestListRealtimeModels_FiltersToRealtimeFamilyOnly(t *testing.T) {
	var gotAuth string
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Write([]byte(`{
			"data": [
				{"id":"gpt-realtime-2.1"},
				{"id":"gpt-4o"},
				{"id":"gpt-realtime"}
			]
		}`))
	})

	models, err := ListRealtimeModels(context.Background(), "oa-key")
	if err != nil {
		t.Fatalf("ListRealtimeModels: %v", err)
	}
	if gotAuth != "Bearer oa-key" {
		t.Errorf("expected Bearer oa-key, got %q", gotAuth)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 realtime-family models, got %d: %+v", len(models), models)
	}
}

func TestListRealtimeModels_ErrorStatus(t *testing.T) {
	withDiscoveryServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"message":"Incorrect API key provided"}}`))
	})

	_, err := ListRealtimeModels(context.Background(), "bad-key")
	if err == nil {
		t.Fatal("expected an error")
	}
}
