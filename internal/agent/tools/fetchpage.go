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
		return T(
			"Bu sayfadan okunabilir bir içerik alınamadı (JavaScript ile render ediliyor olabilir ya da erişim engellenmiş olabilir). Arama sonuçlarından farklı bir kaynağı dene.",
			"This page returned no readable content (it may be JavaScript-rendered or blocked). Try a different source from the search results.",
		), nil
	}

	return fmt.Sprintf("# %s\n%s\n\n%s", page.Title, page.URL, page.Content), nil
}
