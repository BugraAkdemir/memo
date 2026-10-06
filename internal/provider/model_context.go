// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import "strings"

// ContextWindowForModel returns the context window, in tokens, that a model id
// is known to have by the family it belongs to, or 0 when the id says nothing.
//
// It exists for OpenAI-compatible gateways (type `custom`): a gateway such as
// the bundled Subscriptions sidecar serves Claude and Gemini models behind one
// base URL, so the provider TYPE no longer says which window applies — the
// per-type fallback sent every one of them a 128K budget, wasting a Gemini's
// million tokens and risking nothing on a Claude's 200K. A configured
// ContextTokens always wins over this; it is only the fallback.
//
// Deliberately conservative: Claude is 200K even where a plan can serve more
// (an unentitled 1M request silently falls back to 200K, and truncating early
// is recoverable while overflowing loses the turn). gpt-oss and unknown ids
// return 0, leaving the caller's default.
func ContextWindowForModel(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	// Gateways sometimes qualify ids ("subs/claude-sonnet-4-6", "vendor/x").
	if i := strings.LastIndex(m, "/"); i >= 0 {
		m = m[i+1:]
	}
	switch {
	case strings.HasPrefix(m, "claude-"):
		return 200 * 1024
	case strings.HasPrefix(m, "gemini"):
		return 1024 * 1024
	}
	return 0
}
