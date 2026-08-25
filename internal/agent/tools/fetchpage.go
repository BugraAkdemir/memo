package tools

import (
	"context"
	"encoding/json"
	"fmt"

	"memo/internal/websearch"
)

type FetchPageArgs struct {
	URL string `json:"url"`
}

// websearchFetch is a package var (not a direct websearch.Fetch call) so
// tests can substitute a fake and exercise FetchPage's own logic (budget
// check, error wrapping, empty-content message) without hitting the network.
var websearchFetch = websearch.Fetch

// browserInstallChecker is the optional extra capability
// *internal/browserengine.Manager has beyond plain websearch.BrowserFetcher
// (which only needs Fetch). Checked via a type assertion below rather than
// widening BrowserFetcher itself, so websearch stays agnostic of anything
// beyond "can this thing fetch a page."
type browserInstallChecker interface {
	IsInstalled(ctx context.Context) bool
}

// browserEngineMissing reports whether an empty fetch result is explained by
// "no browser engine installed" specifically, as opposed to some other
// reason (a genuinely content-free page, an unconfigured websearch.Browser
// in a test, or a Browser adapter that doesn't expose install status at
// all). Only true in the one case worth telling the model — and by
// extension the user — about.
func browserEngineMissing(ctx context.Context) bool {
	checker, ok := websearch.Browser.(browserInstallChecker)
	if !ok {
		return false
	}
	return !checker.IsInstalled(ctx)
}

func FetchPage(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	var args FetchPageArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	if ok, tried := reserveFetchDomain(ctx, args.URL); !ok {
		return fmt.Sprintf(T(
			"Deneme hakkı doldu: bu turda %d farklı kaynak denendi (limit %d) ve hiçbiri alakalı değildi. Kullanıcıya alakalı bir kaynak bulunamadığını söyle, yeni bir siteye geçmeyi bırak — daha önce denenen bir sitenin başka bir sayfasını (aynı domain) yine de getirebilirsin.",
			"Fetch budget exhausted: %d different sources were already tried this turn (limit %d) and none were relevant. Tell the user you could not find a relevant source and stop trying new domains — you may still fetch a different page on a domain already tried this turn.",
		), tried, maxFetchDomains), nil
	}

	page, err := websearchFetch(ctx, args.URL)
	if err != nil {
		return "", fmt.Errorf(T("sayfa getirilemedi: %w", "could not fetch page: %w"), err)
	}
	if page.Content == "" {
		if browserEngineMissing(ctx) {
			return T(
				"Bu sayfa muhtemelen JavaScript ile render ediliyor ve okuyabilmek için bir tarayıcı motoru (Chromium) gerekiyor, ama şu an kurulu değil. Kullanıcıya bunu söyle, Ayarlar'dan tek tıkla kurabileceğini belirt ve kurmak isteyip istemediğini sor — ya da bunun yerine farklı bir kaynak dene.",
				"This page is likely JavaScript-rendered and needs a browser engine to read, which isn't installed right now. Tell the user this, mention they can install it from Settings with one click, and ask if they'd like to — or try a different source instead.",
			), nil
		}
		return T(
			"Bu sayfadan okunabilir bir içerik alınamadı (JavaScript ile render ediliyor olabilir ya da erişim engellenmiş olabilir). Arama sonuçlarından farklı bir kaynağı dene.",
			"This page returned no readable content (it may be JavaScript-rendered or blocked). Try a different source from the search results.",
		), nil
	}

	return fmt.Sprintf("# %s\n%s\n\n%s", page.Title, page.URL, page.Content), nil
}
