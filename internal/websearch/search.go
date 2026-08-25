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

// Search, Bing'i birincil motor olarak kullanır; Bing engellenir/challenge
// döndürürse (ErrBlocked/ErrChallenge) DuckDuckGo'ya düşer — DuckDuckGo'nun
// resmi bir JavaScript'siz HTML endpoint'i olduğu için gosearch'ün en
// güvenilir motoru olması nedeniyle son çare olarak seçildi. API key
// gerektirmez.
func Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	logx.Info("WEBSEARCH: query", "query", query, "max_results", maxResults, "engine", "bing", "fallback", "duckduckgo")

	results, err := gosearchSearch(ctx, query, gosearch.Bing,
		gosearch.WithFallback(gosearch.DuckDuckGo),
		gosearch.WithMaxResults(maxResults),
	)
	if err != nil {
		// gosearch's public Search() doesn't expose which engine actually
		// answered (or whether the fallback triggered) — only the query and
		// the final outcome are observable from here. A block/challenge
		// error usually means Bing rejected it and DuckDuckGo's own fallback
		// attempt then also failed (or was never reached, if the error isn't
		// ErrBlocked/ErrChallenge in the first place).
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
