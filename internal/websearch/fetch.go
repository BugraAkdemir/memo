package websearch

import (
	"context"
	"fmt"

	"github.com/BugraAkdemir/gosearch"
	"memo/internal/logx"
	"memo/internal/truncate"
)

// maxFetchContentRunes, Fetch'in döndürdüğü Markdown içeriğinin üst sınırı —
// mevcut Search'ün ham HTML gövdesini 512KB'de kesmesiyle aynı mantık:
// context bütçesini tek bir sayfa yüzünden patlatmamak için.
const maxFetchContentRunes = 8000

// Page, Fetch ile alınan bir sayfanın okunabilir içeriğidir.
type Page struct {
	URL     string
	Title   string
	Content string
}

// gosearchFetch, gosearch.Fetch çağrısını bir paket değişkeni arkasına
// alır — bkz. gosearchSearch'ün doc yorumu, aynı sebep.
var gosearchFetch = gosearch.Fetch

// BrowserFetcher is the minimal capability Fetch needs from a browser
// engine — satisfied by *internal/browserengine.Manager. Kept as an
// interface here (rather than importing that package directly) so this
// low-level package doesn't depend on the app-level engine-lifecycle
// package; App.Startup() wires the real implementation into Browser below.
type BrowserFetcher interface {
	Fetch(ctx context.Context, url string) (*gosearch.Page, error)
}

// Browser is set once by App.Startup() to the app's shared browser-engine
// manager. Nil (the zero value) until then, and in every test that never
// wires it up — Fetch simply skips the browser-render fallback in that
// case, exactly as if no browser engine were installed.
var Browser BrowserFetcher

// browserFallbackFetch renders a page with a real (headless) browser when
// the fast static fetch above comes back empty — the common case for
// JavaScript-rendered pages (live scores, SPA dashboards, etc.) that
// gosearch.Fetch cannot read at all, since it never executes JavaScript.
// Whether that browser is a fresh one closed right after (the default) or a
// kept-alive shared one is internal/browserengine.Manager's decision, not
// this package's — see its own doc comment.
var browserFallbackFetch = func(ctx context.Context, url string) (*gosearch.Page, error) {
	if Browser == nil {
		return nil, fmt.Errorf("websearch: no browser engine configured")
	}
	return Browser.Fetch(ctx, url)
}

// Fetch, verilen URL'nin okunabilir ana içeriğini GitHub-flavored Markdown
// olarak döndürür (başlıklar, listeler, kod blokları, linkler korunur).
// Önce hızlı statik istekle dener; içerik boş dönerse (JS ile render edilen
// bir sayfa olabilir) sistemde kurulu bir tarayıcı varsa onunla bir kez daha
// dener — bkz. browserFallbackFetch.
func Fetch(ctx context.Context, url string) (*Page, error) {
	logx.Info("WEBSEARCH: fetch", "url", url)

	page, err := gosearchFetch(ctx, url, gosearch.WithMarkdown())
	if err != nil {
		logx.Info("WEBSEARCH: fetch failed", "url", url, "error", err)
		return nil, fmt.Errorf("websearch: fetch: %w", err)
	}

	content := truncate.Text(page.Content, maxFetchContentRunes)
	usedBrowser := false
	if content == "" {
		logx.Info("WEBSEARCH: static fetch empty, trying browser render", "url", url)
		if bpage, berr := browserFallbackFetch(ctx, url); berr != nil {
			logx.Info("WEBSEARCH: browser render unavailable or failed", "url", url, "error", berr)
		} else if bpage.Content != "" {
			page = bpage
			content = truncate.Text(page.Content, maxFetchContentRunes)
			usedBrowser = true
		}
	}

	logx.Info("WEBSEARCH: fetch done", "url", url, "resolved_url", page.URL, "title", page.Title, "content_runes", len([]rune(content)), "content_empty", content == "", "used_browser", usedBrowser)
	// Full Markdown content, separate from the summary line above so a log
	// viewer can filter/skip it easily — this is the actual page text the
	// model sees, useful for diagnosing "fetched but the content is garbage/
	// irrelevant/JS-shell" cases that the one-line summary can't show.
	// Deliberately at Info (not Debug — nothing in this app currently
	// enables debug-level logging, so Debug here would be invisible) for
	// this debugging pass; worth moving behind an actual debug flag once
	// the current search-quality issue is understood, so it doesn't stay
	// this verbose in normal use.
	logx.Info("WEBSEARCH: fetch content", "url", url, "content", content)

	return &Page{URL: page.URL, Title: page.Title, Content: content}, nil
}
