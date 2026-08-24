package websearch

import (
	"context"
	"fmt"

	"github.com/BugraAkdemir/gosearch"
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

// Fetch, verilen URL'nin okunabilir ana içeriğini GitHub-flavored Markdown
// olarak döndürür (başlıklar, listeler, kod blokları, linkler korunur).
// JavaScript çalıştırmaz — içeriği tamamen istemci tarafında render edilen
// sayfalarda Content boş dönebilir.
func Fetch(ctx context.Context, url string) (*Page, error) {
	page, err := gosearchFetch(ctx, url, gosearch.WithMarkdown())
	if err != nil {
		return nil, fmt.Errorf("websearch: fetch: %w", err)
	}

	return &Page{URL: page.URL, Title: page.Title, Content: truncate.Text(page.Content, maxFetchContentRunes)}, nil
}
