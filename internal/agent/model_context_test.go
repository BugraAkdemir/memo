// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/provider"
)

// The pipeline's intra-turn budget follows what the gateway publishes for the
// model — never the model's name — and a configured ContextTokens still wins.
func TestModelContextWindow_FollowsThePublishedWindow(t *testing.T) {
	provider.ResetContextWindowCache()
	t.Cleanup(provider.ResetContextWindowCache)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"data":[{"id":"claude-sonnet-5-5","context_length":1000000},{"id":"gpt-oss-120b-medium"}]}`))
	}))
	defer srv.Close()

	mk := func(model string, ctxTokens int) (*provider.Router, provider.ProviderConfig) {
		cfg := provider.ProviderConfig{Type: provider.ProviderCustom, Name: "gw", BaseURL: srv.URL + "/v1", Model: model, Enabled: true, ContextTokens: ctxTokens}
		r := provider.NewRouter([]provider.ProviderConfig{cfg})
		r.SetActiveProvider("gw")
		return r, cfg
	}
	cases := []struct {
		model string
		cfg   int
		want  int
	}{
		{"claude-sonnet-5-5", 0, 1000000},
		{"gpt-oss-120b-medium", 0, 128 * 1024}, // listed without a window: internal safety budget
		{"claude-sonnet-5-5", 64000, 64000},    // an explicit setting always wins
	}
	for _, c := range cases {
		r, cfg := mk(c.model, c.cfg)
		provider.WarmContextWindow(context.Background(), cfg, c.model)
		if got := modelContextWindow(r, c.model); got != c.want {
			t.Errorf("%s (configured %d): window = %d, want %d", c.model, c.cfg, got, c.want)
		}
	}
}

// Resolving the window must never probe a provider the user did not select.
func TestModelContextWindow_OnlyAsksTheSelectedProvider(t *testing.T) {
	provider.ResetContextWindowCache()
	t.Cleanup(provider.ResetContextWindowCache)
	var chosen, other atomic.Int32
	mk := func(n *atomic.Int32, ctx int) *httptest.Server {
		return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n.Add(1)
			w.Write([]byte(`{"data":[{"id":"m","context_length":` + strconv.Itoa(ctx) + `}]}`))
		}))
	}
	a, b := mk(&chosen, 500000), mk(&other, 64000)
	defer a.Close()
	defer b.Close()
	r := provider.NewRouter([]provider.ProviderConfig{
		{Type: provider.ProviderCustom, Name: "chosen", BaseURL: a.URL + "/v1", Model: "m", Enabled: true},
		{Type: provider.ProviderCustom, Name: "other", BaseURL: b.URL + "/v1", Model: "m", Enabled: true},
	})
	r.SetActiveProvider("chosen")

	modelContextWindow(r, "m") // starts the background lookup
	deadline := time.Now().Add(3 * time.Second)
	for modelContextWindow(r, "m") != 500000 {
		if time.Now().After(deadline) {
			t.Fatal("the selected provider's window never arrived")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if other.Load() != 0 {
		t.Errorf("sent %d requests to a provider that was not selected", other.Load())
	}
}
