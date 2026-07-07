package webserver

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestRateLimitMiddleware_KeyedByIPNotPort is a regression test for
// BUG-QM3: the rate limiter bucketed requests by r.RemoteAddr, which
// includes the ephemeral source port (e.g. "127.0.0.1:54321") — different
// for every new TCP connection — so every connection got its own fresh
// bucket and the 100 req/s limit was trivially bypassed by opening new
// connections instead of reusing one. Requests from the same IP but
// different ports must now share a single bucket.
func TestRateLimitMiddleware_KeyedByIPNotPort(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	stop := make(chan struct{})
	defer close(stop)
	h := rateLimitMiddleware(stop, ok)

	const burst = 200 // must match rateLimitMiddleware's internal burst const

	// Exhaust the burst using a different source port on every request —
	// this is exactly what the old r.RemoteAddr-keyed bucket map failed to
	// collapse into one bucket.
	var lastCode int
	for i := range burst + 5 {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req.RemoteAddr = fmt.Sprintf("203.0.113.7:%d", 40000+i)
		w := httptest.NewRecorder()
		h.ServeHTTP(w, req)
		lastCode = w.Code
	}
	if lastCode != http.StatusTooManyRequests {
		t.Errorf("after exhausting the burst from one IP (varying ports), last status = %d, want %d — bucket is not keyed by IP", lastCode, http.StatusTooManyRequests)
	}

	// A different IP must be unaffected by the first IP's exhausted bucket.
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.RemoteAddr = "203.0.113.99:40000"
	w := httptest.NewRecorder()
	h.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("a different IP's first request got status %d, want %d", w.Code, http.StatusOK)
	}
}
