//go:build canary

package agent

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"memo/internal/provider"
)

// scriptedToolCallProvider drives Pipeline.RunStream through a scripted
// sequence of tool calls, simulating a real model calling fetch_page
// repeatedly. Once the script is exhausted it returns a plain final answer
// with no more tool calls, ending the loop.
type scriptedToolCallProvider struct {
	calls []provider.ChatResponse
	n     int
}

func (p *scriptedToolCallProvider) ChatCompletion(_ context.Context, _ provider.ChatRequest) (*provider.ChatResponse, error) {
	if p.n < len(p.calls) {
		resp := p.calls[p.n]
		p.n++
		return &resp, nil
	}
	return &provider.ChatResponse{Content: "final answer"}, nil
}

func fetchPageCall(id, url string) provider.ChatResponse {
	args, _ := json.Marshal(map[string]string{"url": url})
	return provider.ChatResponse{
		ToolCalls: []provider.ToolCall{{
			ID:       id,
			Type:     "function",
			Function: provider.ToolCallFunction{Name: "fetch_page", Arguments: args},
		}},
	}
}

// TestCanary_FetchPageDomainBudgetAcrossRealLoop drives the ACTUAL
// Pipeline.RunStream tool-call loop (registry -> pipeline -> real
// fetch_page -> real gosearch.Fetch, live network) through a scripted
// web-search-agent-style sequence, proving the per-turn domain budget
// wired in tools.WithFetchBudget/registerFetchPageTool works end to end —
// not just reserveFetchDomain in isolation (already covered by
// fetchbudget_test.go's fast, network-free unit tests). 5 distinct domains
// must succeed, a 6th distinct domain must be refused by the tool itself,
// and a same-domain retry mixed in between must not consume budget.
func TestCanary_FetchPageDomainBudgetAcrossRealLoop(t *testing.T) {
	registry := NewRegistry() // real registry — real fetch_page, not a stub
	permissions := NewPermissionManager(t.TempDir())
	sandbox := NewSandbox(DefaultSandboxConfig(t.TempDir()))

	// All five are plain server-rendered HTML (no client-side JS framework)
	// so gosearch's non-JS-executing Fetch can actually extract content —
	// a heavily client-rendered SPA (tried github.com/MDN/python.org here
	// first) legitimately yields empty content, which is correct behavior,
	// not a bug, but makes a poor fixture for this test.
	domains := []string{
		"https://go.dev/doc/",
		"https://en.wikipedia.org/wiki/Go_(programming_language)",
		"https://docs.python.org/3/",
		"https://click.palletsprojects.com/en/stable/",
		"https://httpd.apache.org/docs/2.4/",
	}
	var calls []provider.ChatResponse
	for i, url := range domains {
		calls = append(calls, fetchPageCall(string(rune('a'+i)), url))
	}
	// Free retry on an already-tried domain (a different page on go.dev) —
	// must not consume budget even though 5 distinct domains were already
	// tried.
	calls = append(calls, fetchPageCall("retry", "https://go.dev/blog/"))
	// A genuinely new, 6th domain — must be refused.
	calls = append(calls, fetchPageCall("sixth", "https://news.ycombinator.com/"))

	prov := &scriptedToolCallProvider{calls: calls}
	pipeline := NewPipeline(registry, permissions, sandbox, prov, nil)

	var results []string
	var errs []string
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	ch, err := pipeline.RunStream(ctx, nil, "test-model", func(ev AgentEvent) {
		switch ev.Type {
		case EventToolResult:
			results = append(results, ev.Result)
		case EventToolError:
			errs = append(errs, ev.Error)
		}
	}, nil)
	if err != nil {
		t.Fatalf("RunStream() error = %v", err)
	}
	for range ch {
	}

	if len(errs) != 0 {
		t.Fatalf("unexpected tool errors (a live site may be unreachable from this network): %v", errs)
	}
	if len(results) != len(calls) {
		t.Fatalf("expected %d tool results, got %d: %v", len(calls), len(results), results)
	}

	// The 5 distinct-domain fetches and the same-domain retry must all
	// return real page content, not a budget-exhausted message.
	for i, r := range results[:len(results)-1] {
		if len(r) < 200 {
			t.Errorf("result %d looks too short to be real fetched content (%d chars): %q", i, len(r), r)
		}
	}
	// The 6th, genuinely-new domain must be refused by the budget — short,
	// explanatory message, not real page content.
	last := results[len(results)-1]
	if len(last) > 500 {
		t.Errorf("expected the 6th distinct-domain fetch to be refused by the budget (short message), got %d chars — budget may not be enforced: %q", len(last), last)
	}
	t.Logf("budget-refused message: %q", last)
}
