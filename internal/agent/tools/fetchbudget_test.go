package tools

import (
	"context"
	"testing"
)

func TestReserveFetchDomain_NoBudgetInContextAlwaysAllows(t *testing.T) {
	ok, tried := reserveFetchDomain(context.Background(), "https://example.com/a")
	if !ok {
		t.Fatal("expected ok=true with no budget in context")
	}
	if tried != 0 {
		t.Errorf("expected tried=0 with no budget tracked, got %d", tried)
	}
}

func TestReserveFetchDomain_CapsAtMaxDistinctDomains(t *testing.T) {
	ctx := WithFetchBudget(context.Background())

	for i := 0; i < maxFetchDomains; i++ {
		url := "https://site" + string(rune('a'+i)) + ".example"
		ok, tried := reserveFetchDomain(ctx, url)
		if !ok {
			t.Fatalf("attempt %d: expected ok=true within budget, got false", i)
		}
		if tried != i+1 {
			t.Errorf("attempt %d: expected tried=%d, got %d", i, i+1, tried)
		}
	}

	ok, tried := reserveFetchDomain(ctx, "https://one-too-many.example")
	if ok {
		t.Fatal("expected ok=false once budget exhausted on a new domain")
	}
	if tried != maxFetchDomains {
		t.Errorf("expected tried=%d, got %d", maxFetchDomains, tried)
	}
}

func TestReserveFetchDomain_SameDomainRetryIsFree(t *testing.T) {
	ctx := WithFetchBudget(context.Background())

	for i := 0; i < maxFetchDomains; i++ {
		url := "https://site" + string(rune('a'+i)) + ".example"
		if ok, _ := reserveFetchDomain(ctx, url); !ok {
			t.Fatalf("attempt %d: expected ok=true within budget", i)
		}
	}

	// Budget is exhausted for NEW domains, but re-fetching a different page
	// on an already-tried domain (pagination, docs sub-pages) must still work.
	ok, tried := reserveFetchDomain(ctx, "https://sitea.example/page2")
	if !ok {
		t.Fatal("expected same-domain re-fetch to be free even after budget exhausted")
	}
	if tried != maxFetchDomains {
		t.Errorf("same-domain retry must not increase the tried count, got %d", tried)
	}
}

func TestReserveFetchDomain_HostComparisonIsCaseInsensitive(t *testing.T) {
	ctx := WithFetchBudget(context.Background())

	if ok, _ := reserveFetchDomain(ctx, "https://Example.com/a"); !ok {
		t.Fatal("expected first fetch to succeed")
	}
	ok, tried := reserveFetchDomain(ctx, "https://example.COM/b")
	if !ok {
		t.Fatal("expected same host (different case) to be treated as already tried")
	}
	if tried != 1 {
		t.Errorf("expected tried to stay at 1 for a case-differing same host, got %d", tried)
	}
}

func TestReserveFetchDomain_IndependentPerContext(t *testing.T) {
	ctx1 := WithFetchBudget(context.Background())
	ctx2 := WithFetchBudget(context.Background())

	for i := 0; i < maxFetchDomains; i++ {
		url := "https://site" + string(rune('a'+i)) + ".example"
		reserveFetchDomain(ctx1, url)
	}

	// A fresh budget (new agent turn) must not inherit ctx1's exhausted state.
	ok, tried := reserveFetchDomain(ctx2, "https://brand-new.example")
	if !ok {
		t.Fatal("expected a fresh WithFetchBudget context to have its own independent budget")
	}
	if tried != 1 {
		t.Errorf("expected tried=1 in the fresh context, got %d", tried)
	}
}
