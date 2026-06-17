package websearch

import "testing"

// DDG HTML endpoint'inin gerçek yapısından alınmış örnek — class isimleri
// değişirse bu test patlar ve scraper güncellenmesi gerektiğini gösterir.
const sampleDDGHTML = `
<!DOCTYPE html>
<html>
<body>
<div class="results">
  <div class="result results_links results_links_deep web-result">
    <div class="result__body">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="https://example.com/page1">Go Programlama Dili</a>
      </h2>
      <div class="result__snippet">Go (veya Golang), Google tarafından geliştirilen açık kaynaklı bir programlama dilidir.</div>
    </div>
  </div>
  <div class="result results_links results_links_deep web-result">
    <div class="result__body">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="https://example.com/page2">Go Resmi Sitesi</a>
      </h2>
      <div class="result__snippet">Go'nun resmi web sitesi, dökümantasyon ve araçlar.</div>
    </div>
  </div>
  <div class="result results_links results_links_deep web-result">
    <div class="result__body">
      <h2 class="result__title">
        <a rel="nofollow" class="result__a" href="https://example.com/page3">Go Tutorial</a>
      </h2>
      <div class="result__snippet">Go öğrenmek için adım adım rehber.</div>
    </div>
  </div>
</div>
</body>
</html>
`

// DDG'nin redirect URL formatını simüle eder.
const sampleDDGHTMLWithRedirect = `
<!DOCTYPE html>
<html><body>
<div class="result results_links web-result">
  <div class="result__body">
    <h2 class="result__title">
      <a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fgolang.org%2F">Golang.org</a>
    </h2>
    <div class="result__snippet">Go programlama dilinin resmi sitesi.</div>
  </div>
</div>
</body></html>
`

func TestParseBasic(t *testing.T) {
	results := parse(sampleDDGHTML, 10)
	if len(results) != 3 {
		t.Fatalf("3 sonuç bekleniyor, %d bulundu", len(results))
	}
	if results[0].Title != "Go Programlama Dili" {
		t.Errorf("title[0] = %q, want %q", results[0].Title, "Go Programlama Dili")
	}
	if results[0].URL != "https://example.com/page1" {
		t.Errorf("url[0] = %q, want %q", results[0].URL, "https://example.com/page1")
	}
	if results[0].Snippet == "" {
		t.Error("snippet[0] boş olmamalı")
	}
}

func TestParseMaxResults(t *testing.T) {
	results := parse(sampleDDGHTML, 2)
	if len(results) != 2 {
		t.Fatalf("max=2 ile 2 sonuç bekleniyor, %d bulundu", len(results))
	}
}

func TestParseRedirectURL(t *testing.T) {
	results := parse(sampleDDGHTMLWithRedirect, 10)
	if len(results) == 0 {
		t.Fatal("sonuç bulunamadı")
	}
	if results[0].URL != "https://golang.org/" {
		t.Errorf("redirect URL çözümlenmedi: got %q, want %q", results[0].URL, "https://golang.org/")
	}
}

func TestParseEmptyHTML(t *testing.T) {
	results := parse("", 5)
	if len(results) != 0 {
		t.Errorf("boş HTML'den sonuç beklenmez, got %d", len(results))
	}
}

func TestParseNoMatchingStructure(t *testing.T) {
	results := parse("<html><body><p>Hiç sonuç yok</p></body></html>", 5)
	if len(results) != 0 {
		t.Errorf("eşleşmeyen yapıdan 0 sonuç bekleniyor, got %d", len(results))
	}
}

func TestCleanDDGURL(t *testing.T) {
	cases := []struct {
		raw  string
		want string
	}{
		{"https://example.com", "https://example.com"},
		{"//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com", "https://example.com"},
		{"/search?q=test", "https://duckduckgo.com/search?q=test"},
		{"", ""},
	}
	for _, c := range cases {
		got := cleanDDGURL(c.raw)
		if got != c.want {
			t.Errorf("cleanDDGURL(%q) = %q, want %q", c.raw, got, c.want)
		}
	}
}

func TestFormatForContext(t *testing.T) {
	results := []Result{
		{Title: "Go Dili", URL: "https://example.com", Snippet: "Güzel bir dil"},
	}
	out := FormatForContext("golang", results)
	if out == "" {
		t.Error("sonuçlu FormatForContext boş string döndürmemeli")
	}
	if out == FormatForContext("golang", nil) {
		t.Error("sonuçsuz FormatForContext boş string döndürmeli")
	}
}
