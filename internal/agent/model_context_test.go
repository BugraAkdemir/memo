// SPDX-License-Identifier: AGPL-3.0-or-later

package agent

import (
	"testing"

	"memo/internal/provider"
)

// A custom (OpenAI-compatible) gateway can front any model family, so its
// window follows the model id — and a configured ContextTokens still wins.
func TestModelContextWindow_CustomGatewayFollowsTheModel(t *testing.T) {
	mk := func(model string, ctxTokens int) *provider.Router {
		r := provider.NewRouter([]provider.ProviderConfig{{Type: provider.ProviderCustom, Name: "gw", BaseURL: "http://127.0.0.1:1/v1", Model: model, Enabled: true, ContextTokens: ctxTokens}})
		r.SetActiveProvider("gw")
		return r
	}
	cases := []struct {
		model string
		cfg   int
		want  int
	}{
		{"claude-sonnet-4-6", 0, 200 * 1024},
		{"gemini-3-flash", 0, 1024 * 1024},
		{"gpt-oss-120b-medium", 0, 128 * 1024},
		{"claude-sonnet-4-6", 64000, 64000}, // an explicit setting always wins
	}
	for _, c := range cases {
		if got := modelContextWindow(mk(c.model, c.cfg), c.model); got != c.want {
			t.Errorf("%s (configured %d): window = %d, want %d", c.model, c.cfg, got, c.want)
		}
	}
}
