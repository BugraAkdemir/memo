// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import "testing"

func TestContextWindowForModel(t *testing.T) {
	cases := map[string]int{
		"claude-sonnet-4-6":             200 * 1024,
		"claude-opus-4-6-thinking":      200 * 1024,
		"Claude-Haiku-4-5-20251001":     200 * 1024,
		"antigravity/claude-sonnet-4-6": 200 * 1024,
		"gemini-3.7-flash-high":         1024 * 1024,
		"gemini-pro-agent":              1024 * 1024,
		"gemini-3.5-flash-lite":         1024 * 1024,
		"gpt-oss-120b-medium":           0,
		"llama-3.1-8b":                  0,
		"":                              0,
		"  ":                            0,
		"not-a-claude-claude-in-name":   0,
	}
	for in, want := range cases {
		if got := ContextWindowForModel(in); got != want {
			t.Errorf("ContextWindowForModel(%q) = %d, want %d", in, got, want)
		}
	}
}
