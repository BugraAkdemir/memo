# Ayrik Hafiza (Separated Memory) Uygulama Plani

- [x] Mevcut `internal/memory` mimarisini ve `chromem-go` gob disk formatini incele.
- [x] Diskteki mevcut `.gob` veri yapisini bozmadan hafif `MemoryIndex` modeli ekle.
- [x] Uygulama baslangicinda tum gob dosyalarindan sadece `ID` ve `Vector` yukleyen `LoadCache()` fonksiyonunu yaz.
- [x] RAM indeksine erisimleri `sync.RWMutex` ile koru.
- [x] Semantik aramayi disk yerine RAM indeksinde, CPU cekirdeklerine bolunen worker pool ile yap.
- [x] Sadece eslesen kayitlarin gob dosyalarini diskten okuyan retrieval akisini kur.
- [x] Yeni ani kaydinda diske yazma ve RAM indeks guncellemesini senkron tut.
- [x] Silme ve tum hafizayi temizleme akislarinda disk/RAM tutarliligini koru.
- [x] RAM'de bulunan fakat diskte olmayan kayitlarda panic yerine guvenli error/log mekanizmasi kullan.
- [x] `gofmt` ve `go test ./internal/memory` ile dogrula.

## Latency Loglari

- [x] Memory cold start cache yukleme suresini logla.
- [x] Memory retrieval icinde embedding, RAM search ve disk read surelerini ayri logla.
- [x] Chat message build suresini, memory sayisini ve history/message adetlerini logla.
- [x] LLM stream hazir olma, first token ve stream bitis surelerini logla.
- [x] Non-stream LLM cagrisinin toplam suresini logla.
- [x] Async memory save suresini logla.
- [x] `go test ./...` ile dogrula.

## Gelismis Hafiza Ayarlari

- [x] `memory.top_k` ve `memory.min_similarity` ayarlarini REST API uzerinden okunur/yazilir hale getir.
- [x] Ayar degisince `config/config.yaml` dosyasina kaydet ve retrieval akisinda aninda kullan.
- [x] Flutter Settings > Memory sekmesine Top K ve Minimum Similarity alanlari ekle.
- [x] Backend icin `go test ./...` ve `go build ./...` ile dogrula.
