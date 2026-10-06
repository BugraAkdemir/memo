// SPDX-License-Identifier: AGPL-3.0-or-later

package e2e

import (
	"testing"
	"time"
)

// usageSummary mirrors the fields of internal/stats.Summary this test asserts
// on, decoded from the real GET /api/stats/usage response — the same JSON a
// Flutter client parses into UsageStatsSummary.
type usageSummary struct {
	TotalRequests           int   `json:"total_requests"`
	TotalPromptTokens       int64 `json:"total_prompt_tokens"`
	TotalCompletionTokens   int64 `json:"total_completion_tokens"`
	TotalCachedPromptTokens int64 `json:"total_cached_prompt_tokens"`
	TotalCacheWriteTokens   int64 `json:"total_cache_write_tokens"`
	ModelBreakdown          []struct {
		Model              string `json:"model"`
		PromptTokens       int64  `json:"prompt_tokens"`
		CachedPromptTokens int64  `json:"cached_prompt_tokens"`
	} `json:"model_breakdown"`
	CategoryBreakdown []struct {
		Category           string `json:"category"`
		PromptTokens       int64  `json:"prompt_tokens"`
		CachedPromptTokens int64  `json:"cached_prompt_tokens"`
	} `json:"category_breakdown"`
}

func (h *Harness) usageStats(t *testing.T) usageSummary {
	t.Helper()
	var sum usageSummary
	decodeInto(t, h.getJSON("/api/stats/usage?days=1"), &sum)
	return sum
}

// usageStatsAfterTurn is usageStats for a test that has just finished a chat
// turn. The app records the usage event fire-and-forget (llm.go: `go
// recordUsageEvent`) so a slow stats write can never hold up the reply, which
// means the row can land a moment AFTER the SSE stream ends. Reading
// immediately raced it and failed on a loaded CI runner ("reported no requests
// at all"); poll until the first request shows up instead of asserting on the
// instant. Still fails, with the last summary, if no event ever arrives.
func (h *Harness) usageStatsAfterTurn(t *testing.T) usageSummary {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		sum := h.usageStats(t)
		if sum.TotalRequests > 0 || time.Now().After(deadline) {
			return sum
		}
		time.Sleep(25 * time.Millisecond)
	}
}

// TestUsage_StreamingChatRecordsCacheSplit is the end-to-end proof for the
// whole prompt-cache accounting chain, with nothing mocked below the
// provider's HTTP boundary: a real streaming chat turn, a real SSE response
// whose trailing usage chunk reports cached tokens the way OpenAI does, a real
// stats store write, and the real REST response a Flutter client reads.
//
// Every unit test in this feature could pass while this failed — the pieces
// are a request field, an SSE ordering rule, an app-side overwrite, a DB
// column and a JSON tag, and any one of them silently dropping the number
// leaves the user looking at zeros.
func TestUsage_StreamingChatRecordsCacheSplit(t *testing.T) {
	h := NewHarness(t)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{
			Text: "cached reply",
			Usage: &FakeUsage{
				PromptTokens:     5120,
				CompletionTokens: 12,
				CachedTokens:     4096,
			},
		}
	}

	// Both flags matter, and this cost a debugging round to find: agent mode
	// routes a turn through the pipeline's non-streaming ChatCompletion calls,
	// and so does web search (routeStream sends a plain turn through
	// callWebSearchAgentStream when it is on — the same native tool-calling
	// machinery). Only with both off does a chat message actually reach the
	// provider's SSE path, which is what this test is about.
	h.SetAgentEnabled(false)
	h.SetWebSearchEnabled(false)

	chatID := h.NewChat()
	events := h.SendMessageStream(chatID, "selam")
	if len(events) == 0 {
		t.Fatal("got zero SSE events")
	}
	if got := FinalContent(events); got != "cached reply" {
		t.Fatalf("assembled reply = %q, want the fake provider's exact text", got)
	}

	// The app must have asked for usage on the streaming request. Without the
	// field, a real provider sends no usage chunk at all and every streaming
	// turn silently falls back to a word-count estimate that cannot carry a
	// cache split.
	var sawStreamingRequest bool
	for _, req := range h.Fake.Requests() {
		if !req.Stream {
			continue
		}
		sawStreamingRequest = true
		if !req.WantsUsage {
			t.Errorf("a streaming request went out without stream_options.include_usage: %s", req.Raw)
		}
	}
	if !sawStreamingRequest {
		t.Fatal("no streaming request reached the provider — this test is not exercising the SSE path")
	}

	sum := h.usageStatsAfterTurn(t)
	if sum.TotalRequests == 0 {
		t.Fatal("GET /api/stats/usage reported no requests at all")
	}
	if sum.TotalCachedPromptTokens < 4096 {
		t.Errorf("total_cached_prompt_tokens = %d, want at least 4096 — the cache figure did not survive the path from SSE to the REST response",
			sum.TotalCachedPromptTokens)
	}
	if sum.TotalPromptTokens < 5120 {
		t.Errorf("total_prompt_tokens = %d, want at least 5120 (the provider's real count, not an estimate)", sum.TotalPromptTokens)
	}
	// Nothing on this path writes a cache entry: OpenAI-style automatic
	// caching has no write premium to report, and only Anthropic's explicit
	// breakpoints produce one.
	if sum.TotalCacheWriteTokens != 0 {
		t.Errorf("total_cache_write_tokens = %d, want 0 for an OpenAI-shaped provider", sum.TotalCacheWriteTokens)
	}

	// The split has to reach the breakdowns too — they are what the stats tab
	// renders per model and per category, and each is its own SQL aggregate.
	var cachedInModels, cachedInCategories int64
	for _, m := range sum.ModelBreakdown {
		cachedInModels += m.CachedPromptTokens
	}
	for _, c := range sum.CategoryBreakdown {
		cachedInCategories += c.CachedPromptTokens
	}
	if cachedInModels < 4096 {
		t.Errorf("model breakdown cached total = %d, want at least 4096", cachedInModels)
	}
	if cachedInCategories < 4096 {
		t.Errorf("category breakdown cached total = %d, want at least 4096", cachedInCategories)
	}
}

// TestUsage_ProviderWithoutCacheReportingStaysZero is the companion: a
// provider that reports usage but no prompt_tokens_details (most
// OpenAI-compatible backends) must record real token counts with a zero cache
// split — not a fabricated one, and not a failure.
func TestUsage_ProviderWithoutCacheReportingStaysZero(t *testing.T) {
	h := NewHarness(t)
	h.Fake.Script = func(callNum int, req FakeChatRequest) FakeChatResponse {
		return FakeChatResponse{
			Text:  "plain reply",
			Usage: &FakeUsage{PromptTokens: 900, CompletionTokens: 7},
		}
	}

	chatID := h.NewChat()
	h.SendMessageStream(chatID, "selam")

	sum := h.usageStatsAfterTurn(t)
	if sum.TotalPromptTokens < 900 {
		t.Errorf("total_prompt_tokens = %d, want at least 900", sum.TotalPromptTokens)
	}
	if sum.TotalCachedPromptTokens != 0 || sum.TotalCacheWriteTokens != 0 {
		t.Errorf("cache split = (%d, %d), want (0, 0) — nothing reported a cache figure",
			sum.TotalCachedPromptTokens, sum.TotalCacheWriteTokens)
	}
}
