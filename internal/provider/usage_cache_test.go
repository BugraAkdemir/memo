// SPDX-License-Identifier: AGPL-3.0-or-later

package provider

import "testing"

// TestClaudeUsageToUsage_AddsCacheTokensBackIntoPrompt pins the one piece of
// arithmetic in this whole feature that is easy to get wrong and impossible to
// notice once wrong: Anthropic reports cache_read/cache_creation OUTSIDE
// input_tokens, so a normalized PromptTokens has to add them back. Before this
// existed, a fully-cached agent iteration was recorded as a handful of prompt
// tokens and the stats screen showed caching making the *input smaller*
// instead of cheaper.
func TestClaudeUsageToUsage_AddsCacheTokensBackIntoPrompt(t *testing.T) {
	tests := []struct {
		name                                    string
		in                                      claudeUsage
		wantPrompt, wantCached, wantWrite       int
		wantTotal, wantFresh, wantCompletionTok int
	}{
		{
			name:              "cache hit: most of the input came from cache",
			in:                claudeUsage{InputTokens: 300, OutputTokens: 120, CacheReadInputTokens: 7800},
			wantPrompt:        8100,
			wantCached:        7800,
			wantWrite:         0,
			wantTotal:         8220,
			wantFresh:         300,
			wantCompletionTok: 120,
		},
		{
			name:              "cache write: first turn of a new TTL window",
			in:                claudeUsage{InputTokens: 300, OutputTokens: 90, CacheCreationInputTokens: 7800},
			wantPrompt:        8100,
			wantCached:        0,
			wantWrite:         7800,
			wantTotal:         8190,
			wantFresh:         300,
			wantCompletionTok: 90,
		},
		{
			name:              "read and write together: prefix grew mid-window",
			in:                claudeUsage{InputTokens: 100, OutputTokens: 40, CacheReadInputTokens: 5000, CacheCreationInputTokens: 900},
			wantPrompt:        6000,
			wantCached:        5000,
			wantWrite:         900,
			wantTotal:         6040,
			wantFresh:         100,
			wantCompletionTok: 40,
		},
		{
			name:              "no breakpoint at all: plain chat turn is unchanged",
			in:                claudeUsage{InputTokens: 1200, OutputTokens: 300},
			wantPrompt:        1200,
			wantCached:        0,
			wantWrite:         0,
			wantTotal:         1500,
			wantFresh:         1200,
			wantCompletionTok: 300,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.in.toUsage()
			if got.PromptTokens != tc.wantPrompt {
				t.Errorf("PromptTokens = %d, want %d (input_tokens excludes the cache figures; they must be added back)", got.PromptTokens, tc.wantPrompt)
			}
			if got.CachedPromptTokens != tc.wantCached {
				t.Errorf("CachedPromptTokens = %d, want %d", got.CachedPromptTokens, tc.wantCached)
			}
			if got.CacheWriteTokens != tc.wantWrite {
				t.Errorf("CacheWriteTokens = %d, want %d", got.CacheWriteTokens, tc.wantWrite)
			}
			if got.TotalTokens != tc.wantTotal {
				t.Errorf("TotalTokens = %d, want %d", got.TotalTokens, tc.wantTotal)
			}
			if got.CompletionTokens != tc.wantCompletionTok {
				t.Errorf("CompletionTokens = %d, want %d", got.CompletionTokens, tc.wantCompletionTok)
			}
			if got.FreshPromptTokens() != tc.wantFresh {
				t.Errorf("FreshPromptTokens() = %d, want %d", got.FreshPromptTokens(), tc.wantFresh)
			}
		})
	}
}

// TestOpenAIUsageCachedTokens covers the other normalization direction:
// OpenAI's cached_tokens is already a subset of prompt_tokens, so it must pass
// through untouched — plus the clamps, which exist so a contradictory report
// can't produce a "this turn was free" reading downstream.
func TestOpenAIUsageCachedTokens(t *testing.T) {
	tests := []struct {
		name string
		in   openAIUsage
		want int
	}{
		{
			name: "details absent (every backend that doesn't report caching)",
			in:   openAIUsage{PromptTokens: 5000, CompletionTokens: 100},
			want: 0,
		},
		{
			name: "details present with a hit",
			in:   openAIUsage{PromptTokens: 5000, PromptTokensDetails: &openAIPromptTokensDetails{CachedTokens: 4096}},
			want: 4096,
		},
		{
			name: "details present, explicit zero (a real miss)",
			in:   openAIUsage{PromptTokens: 5000, PromptTokensDetails: &openAIPromptTokensDetails{CachedTokens: 0}},
			want: 0,
		},
		{
			name: "nonsense: cached larger than the prompt it belongs to, clamped",
			in:   openAIUsage{PromptTokens: 500, PromptTokensDetails: &openAIPromptTokensDetails{CachedTokens: 9000}},
			want: 500,
		},
		{
			name: "nonsense: negative, clamped",
			in:   openAIUsage{PromptTokens: 500, PromptTokensDetails: &openAIPromptTokensDetails{CachedTokens: -7}},
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.cachedTokens(); got != tc.want {
				t.Errorf("cachedTokens() = %d, want %d", got, tc.want)
			}
		})
	}
}

func TestGeminiUsageCachedTokens(t *testing.T) {
	tests := []struct {
		name string
		in   geminiUsage
		want int
	}{
		{
			name: "field absent from the response",
			in:   geminiUsage{PromptTokenCount: 4000, CandidatesTokenCount: 50},
			want: 0,
		},
		{
			name: "implicit cache hit on a 2.5 model",
			in:   geminiUsage{PromptTokenCount: 4000, CachedContentTokenCount: 3200},
			want: 3200,
		},
		{
			name: "clamped to the prompt total",
			in:   geminiUsage{PromptTokenCount: 400, CachedContentTokenCount: 9000},
			want: 400,
		},
		{
			name: "negative clamped to zero",
			in:   geminiUsage{PromptTokenCount: 400, CachedContentTokenCount: -1},
			want: 0,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.cachedTokens(); got != tc.want {
				t.Errorf("cachedTokens() = %d, want %d", got, tc.want)
			}
		})
	}
}

// TestFreshPromptTokens_NeverNegative guards the display path: a provider
// reporting a cache split that exceeds its own prompt total must not produce a
// negative "paid full price for" figure on screen.
func TestFreshPromptTokens_NeverNegative(t *testing.T) {
	u := Usage{PromptTokens: 100, CachedPromptTokens: 90, CacheWriteTokens: 90}
	if got := u.FreshPromptTokens(); got != 0 {
		t.Errorf("FreshPromptTokens() = %d, want 0 (clamped)", got)
	}
}
