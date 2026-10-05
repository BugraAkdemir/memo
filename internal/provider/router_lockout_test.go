package provider

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// TestRouter_OnlyProviderIsNeverLockedOut: found live (fake provider on
// :4959). With a single active provider, three consecutive failures — three
// 429s from a busy free-tier model is enough — auto-disabled it, and since
// there was nothing to fall back to, every later message failed instantly
// with "no provider configured or all providers failed" without even being
// sent, until the 5-minute health check re-enabled it. Auto-disable exists to
// skip a failing provider when another can serve; with no alternative it must
// keep trying.
func TestRouter_OnlyProviderIsNeverLockedOut(t *testing.T) {
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		if n <= 3 {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"error":{"message":"slow down"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	r := NewRouter([]ProviderConfig{{Name: "only", Type: ProviderCustom, BaseURL: srv.URL, Model: "m", Enabled: true}})
	r.SetActiveProvider("only")
	req := ChatRequest{Messages: []Message{TextMessage("user", "hi")}}
	for i := 0; i < 3; i++ {
		if _, err := r.ChatCompletion(context.Background(), req); err == nil {
			t.Fatalf("call %d succeeded, want the scripted 429", i+1)
		}
	}
	resp, err := r.ChatCompletion(context.Background(), req)
	if err != nil {
		t.Fatalf("4th call error = %v — the only provider was locked out after 3 failures", err)
	}
	if resp.Content != "ok" || calls.Load() != 4 {
		t.Errorf("content=%q calls=%d, want ok and 4 (the 4th request must actually be sent)", resp.Content, calls.Load())
	}
	// A success must fully restore it, not just zero the counter.
	if got := r.ActiveProviders(); len(got) != 1 {
		t.Errorf("ActiveProviders() = %d entries after a success, want the provider back in service", len(got))
	}
}

// TestRouter_AutoDisableStillSkipsAFailingProviderWhenAnotherCanServe keeps
// the original purpose intact.
func TestRouter_AutoDisableStillSkipsAFailingProviderWhenAnotherCanServe(t *testing.T) {
	var badCalls atomic.Int32
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		badCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer bad.Close()
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer good.Close()

	r := NewRouter([]ProviderConfig{
		{Name: "bad", Type: ProviderCustom, BaseURL: bad.URL, Model: "m", Enabled: true, Priority: 2},
		{Name: "good", Type: ProviderCustom, BaseURL: good.URL, Model: "m", Enabled: true, Priority: 1},
	})
	req := ChatRequest{Messages: []Message{TextMessage("user", "hi")}}
	for i := 0; i < 5; i++ {
		if _, err := r.ChatCompletion(context.Background(), req); err != nil {
			t.Fatalf("call %d: %v", i+1, err)
		}
	}
	if badCalls.Load() != 3 {
		t.Errorf("failing provider was tried %d times, want 3 (then skipped while another serves)", badCalls.Load())
	}
}
