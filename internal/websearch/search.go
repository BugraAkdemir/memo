// Package websearch arama motoru sorgulama ve sayfa içeriği çekme işlevlerini
// sağlar. Uygulama içi API anahtarı gerektirmez — gosearch (github.com/
// BugraAkdemir/gosearch) üzerine ince bir sarmalayıcıdır.
package websearch

import (
	"context"
	"fmt"
	"strings"

	"github.com/BugraAkdemir/gosearch"
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

	results, err := gosearchSearch(ctx, query, gosearch.Bing,
		gosearch.WithFallback(gosearch.DuckDuckGo),
		gosearch.WithMaxResults(maxResults),
	)
	if err != nil {
		return nil, fmt.Errorf("websearch: search: %w", err)
	}

	out := make([]Result, len(results))
	for i, r := range results {
		out[i] = Result{Title: r.Title, URL: r.URL, Snippet: r.Snippet}
	}
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
