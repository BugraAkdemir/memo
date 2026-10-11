package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"memo/internal/provider"
)

func TestSubscriptionTextRouteWithoutChatDoesNotSendAnImage(t *testing.T) {
	var calls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()
	cfg := provider.ProviderConfig{Type: provider.ProviderCustom, Name: subsProviderName, BaseURL: server.URL, Model: "selected-image", Enabled: true}
	a := &App{}
	_, err := a.routeSubscriptionText(context.Background(), cfg)
	if err == nil {
		t.Fatal("missing chat model was silently accepted")
	}
	if calls.Load() != 0 {
		t.Fatalf("sent a request without a chat model: %d", calls.Load())
	}
	out := a.subscriptionTextRouteError(context.Background(), err, "question", "")
	chunk := <-out
	if !chunk.Done || chunk.Error == "" {
		t.Fatalf("nonterminal error: %+v", chunk)
	}
}
