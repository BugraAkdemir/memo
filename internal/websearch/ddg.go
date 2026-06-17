package websearch

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"golang.org/x/net/html"
)

// ddgClient tek bir transport üzerinden bağlantıları yeniden kullanır.
var ddgClient = &http.Client{Timeout: 10 * time.Second}

// Result tek bir arama sonucunu temsil eder.
type Result struct {
	Title   string
	URL     string
	Snippet string
}

// Search DuckDuckGo HTML üzerinden arama yapar ve en fazla maxResults sonuç döner.
// API key gerektirmez, gizlilik odaklı.
func Search(ctx context.Context, query string, maxResults int) ([]Result, error) {
	if maxResults <= 0 {
		maxResults = 5
	}

	reqURL := "https://html.duckduckgo.com/html/?q=" + url.QueryEscape(query)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return nil, fmt.Errorf("websearch: request build: %w", err)
	}
	// DDG, boş User-Agent'ı bloklar
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; Memo/3.1; +https://github.com/BugraAkdemir/memo)")
	req.Header.Set("Accept-Language", "tr,en;q=0.9")

	resp, err := ddgClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("websearch: http: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024)) // max 512KB
	if err != nil {
		return nil, fmt.Errorf("websearch: read body: %w", err)
	}

	return parse(string(body), maxResults), nil
}

// parse DDG HTML'inden sonuçları çıkarır.
func parse(body string, max int) []Result {
	doc, err := html.Parse(strings.NewReader(body))
	if err != nil {
		return nil
	}

	var results []Result
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if len(results) >= max {
			return
		}
		if isResultDiv(n) {
			r := extractResult(n)
			if r.Title != "" && r.URL != "" {
				results = append(results, r)
			}
			return // alt node'lara girme — zaten işledik
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	return results
}

// isResultDiv bir node'un DDG sonuç kartı olup olmadığını kontrol eder.
func isResultDiv(n *html.Node) bool {
	if n.Type != html.ElementNode || n.Data != "div" {
		return false
	}
	for _, a := range n.Attr {
		if a.Key == "class" && strings.Contains(a.Val, "result__body") {
			return true
		}
	}
	return false
}

// extractResult bir sonuç kartından title, URL ve snippet çıkarır.
func extractResult(n *html.Node) Result {
	var r Result
	var walk func(*html.Node)
	walk = func(node *html.Node) {
		if node.Type == html.ElementNode {
			class := attrVal(node, "class")
			switch {
			case strings.Contains(class, "result__a"):
				r.Title = strings.TrimSpace(textContent(node))
				if href := attrVal(node, "href"); href != "" {
					r.URL = cleanDDGURL(href)
				}
			case strings.Contains(class, "result__snippet"):
				r.Snippet = strings.TrimSpace(textContent(node))
			}
		}
		for c := node.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return r
}

func attrVal(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func textContent(n *html.Node) string {
	if n.Type == html.TextNode {
		return n.Data
	}
	var sb strings.Builder
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		sb.WriteString(textContent(c))
	}
	return sb.String()
}

// cleanDDGURL DDG'nin redirect URL'lerini temizler.
func cleanDDGURL(raw string) string {
	// DDG bazen //duckduckgo.com/l/?uddg=<encoded_url> formatında verir
	if strings.HasPrefix(raw, "//duckduckgo.com/l/") {
		parsed, err := url.Parse("https:" + raw)
		if err == nil {
			if enc := parsed.Query().Get("uddg"); enc != "" {
				if dec, err := url.QueryUnescape(enc); err == nil {
					return dec
				}
			}
		}
	}
	if strings.HasPrefix(raw, "/") {
		return "https://duckduckgo.com" + raw
	}
	return raw
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
