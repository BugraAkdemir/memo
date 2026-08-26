package tools

import (
	"context"
	"net/url"
	"strings"
	"sync"
)

type fetchBudgetKey struct{}

// maxFetchDomains caps how many DIFFERENT sites fetch_page may try within a
// single agent turn — the model reads a fetched page, judges relevance
// itself, and may retry another search result if the content turns out
// unrelated to what it's looking for. Re-fetching a domain already tried
// this turn (pagination, a docs site's other pages) never counts against
// this — only a genuinely new host does.
const maxFetchDomains = 5

// fetchBudget is the per-turn state shared by every fetch_page call in one
// agent run.
type fetchBudget struct {
	mu      sync.Mutex
	domains map[string]bool
}

// WithFetchBudget seeds a fresh fetch_page domain budget into ctx. Call once
// per agent run, before the tool-call loop starts, so every fetch_page call
// within that turn shares the same counter — see Pipeline.RunStream.
func WithFetchBudget(ctx context.Context) context.Context {
	return context.WithValue(ctx, fetchBudgetKey{}, &fetchBudget{domains: make(map[string]bool)})
}

// reserveFetchDomain reports whether rawURL's host may be fetched: true if
// its domain was already tried this turn (free retry — same-site
// navigation) or budget remains (and the domain is now reserved); false once
// maxFetchDomains distinct domains have been tried and rawURL's host is a
// new one. triedCount is the number of distinct domains tried so far
// (including this one, when ok is true and it was newly reserved).
//
// Called with no budget in ctx (outside a Pipeline run, e.g. in a unit test
// calling the tool function directly) never blocks — there's no turn to
// track, so it degrades to "always allowed".
func reserveFetchDomain(ctx context.Context, rawURL string) (ok bool, triedCount int) {
	b, _ := ctx.Value(fetchBudgetKey{}).(*fetchBudget)
	if b == nil {
		return true, 0
	}

	host := hostOf(rawURL)

	b.mu.Lock()
	defer b.mu.Unlock()
	if b.domains[host] {
		return true, len(b.domains)
	}
	if len(b.domains) >= maxFetchDomains {
		return false, len(b.domains)
	}
	b.domains[host] = true
	return true, len(b.domains)
}

func hostOf(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil || u.Hostname() == "" {
		return strings.ToLower(rawURL)
	}
	return strings.ToLower(u.Hostname())
}
