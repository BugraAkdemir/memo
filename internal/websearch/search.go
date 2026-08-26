// Package websearch arama motoru sorgulama ve sayfa içeriği çekme işlevlerini
// sağlar. Uygulama içi API anahtarı gerektirmez — gosearch (github.com/
// BugraAkdemir/gosearch) üzerine ince bir sarmalayıcıdır.
package websearch

import (
	"context"
	"fmt"
	"strings"

	"github.com/BugraAkdemir/gosearch"
	"memo/internal/logx"
)

// Result tek bir arama sonucunu temsil eder.
type Result struct {
	Title   string
	URL     string
	Snippet string
}

// gosearchSearch, gosearch.Search'e giden çağrıyı bir paket değişkeni
// arkasına alır — testler gerçek ağa çıkmadan sahte bir arama fonksiyonu
// koyabilsin diye (aynı desen gosearch'ün kendi internal `dispatch`
// değişkeninde de var).
var gosearchSearch = gosearch.Search

// Search uses DuckDuckGo as the sole engine — gosearch's own docs mark it
// "the most reliable" (the only engine with an official no-JavaScript HTML
// endpoint), and live testing this same session confirmed it: for identical
// queries Bing (previously primary, with DuckDuckGo only as an
// ErrBlocked/ErrChallenge fallback) consistently returned confident-looking
// but completely unrelated results (see handoff.md for the "Süper Loto"/
// "Süper FM"/Russian-language-Windows-help/random-language-results
// episodes) — and never actually errored, so the fallback never triggered
// in the first place. Deliberately no fallback engine configured: per
// gosearch's own docs, Google/Yandex are heuristic-at-best and Bing has
// proven actively misleading here, so an honest error beats a
// confidently-wrong result. API key not required.
func Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	logx.Info("WEBSEARCH: query", "query", query, "max_results", maxResults, "engine", "duckduckgo")

	results, err := gosearchSearch(ctx, query, gosearch.DuckDuckGo,
		gosearch.WithMaxResults(maxResults),
	)
	if err != nil {
		logx.Info("WEBSEARCH: query failed", "query", query, "error", err)
		return nil, fmt.Errorf("websearch: search: %w", err)
	}

	out := make([]Result, len(results))
	for i, r := range results {
		out[i] = Result{Title: r.Title, URL: r.URL, Snippet: r.Snippet}
		snippet := r.Snippet
		if len(snippet) > 200 {
			snippet = snippet[:200] + "…"
		}
		logx.Info("WEBSEARCH: result", "query", query, "rank", i+1, "title", r.Title, "url", r.URL, "snippet", snippet)
	}
	logx.Info("WEBSEARCH: query done", "query", query, "result_count", len(out))
	return out, nil
}

// FormatForContext arama sonuçlarını LLM context'ine enjekte edilecek metne çevirir.
func FormatForContext(query string, results []Result) string {
	if len(results) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("\n\n--- Web Search Results (query: %q) ---\n", query))
	for i, r := range results {
		sb.WriteString(fmt.Sprintf("[%d] %s\n    %s\n    %s\n\n", i+1, r.Title, r.URL, r.Snippet))
	}
	sb.WriteString("--- End of Web Search Results ---\n")
	sb.WriteString("Use the above search results to answer accurately. Cite sources when relevant.")
	return sb.String()
}
