# Memo — RAG Bellek Sistemi: Tam Yol Haritası

> **Tek vaat:** "Senin hafızan olan AI." Her sohbet, kullanıcıyı daha iyi tanıyan bir fırsattır.
> Bellekler otomatik oluşur, önemli olanlar öne çıkar, gereksizler unutulur.

---

## Mevcut Durum (2026-06-25) — **~%90 Hazır**

### Tamamlananlar ✅

**Faz 0 — Kritik Bug Düzeltmeleri**
- [x] FTS5 phrase search bug → token search'e düzeltildi
- [x] `minSimilarity` RRF sonrası uygulanmıyor → minRRFScore filtresi eklendi
- [x] Orta chunk'larda `assist_msg` boş → tüm chunk'larda saklanıyor
- [x] `candidateK` sabit → `min(max(topK*5, 20), 100)` dinamik hesap

**Faz 1 — Retrieval Kalitesi**
- [x] Sorgu zenginleştirme (son 3 user mesajı + mevcut mesaj)
- [x] Metadata alanları (session_id, importance, tags, source, retrieve_count, pending_deletion)
- [x] Retrieve sayacı (incrementRetrieveCounts goroutine)
- [x] Importance boost (×0.9 – ×1.3, re-sort)
- [x] Explicit bellek komutları (`/remember`, `/forget`) — Flutter + backend
- [x] Bellek prompt formatı yükseltme (relevance%, importance label, source, age)
- [x] `SaveExplicit` / `DeleteByContent` / `Export` / `Import` — Store + App + Bridge + HTTP

**Faz 2 — Bellek Zekası**
- [x] Importance scoring motoru (`runImportanceDecay`, 5-min delay, 24h cycle)
- [x] Importance decay + boost kuralları (`applyImportanceRules`)
- [x] Soft delete (`pending_deletion`, 7 gün grace → hard delete)
- [x] `MarkStaleForDeletion` (importance≤2, retrieve=0, >180 gün, non-explicit)
- [x] `PurgePendingDeletions` (>187 gün)
- [x] Multi-query retrieval (`expandQuery`, ilk 5 kelime topic anchor, ikinci embed+search, RRF merge)

**Faz 3 — Production & Ölçek**
- [x] 5 performans indeksi (timestamp, importance, source, retrieve_count, pending_deletion)
- [x] Bellek Export / Import (JSON v2, UUID dedup INSERT OR IGNORE, re-embed)
- [x] Tarih ve etiket filtreleme (`FilteredSearch` + `sqlFilteredFallback` + HTTP endpoint)
- [x] Bellek Analytics Dashboard (Flutter — stat chips + most accessed, GET /api/memory/stats)
- [x] `Stats()` tek sorguya indirgendi (SUM CASE WHEN), rows.Err() kontrolü eklendi

**Güvenlik / Doğruluk Düzeltmeleri (code review)**
- [x] `_loadStats` mounted check eklendi (Flutter crash fix)
- [x] Stats fetch hataları kullanıcıya SnackBar ile gösteriliyor
- [x] Handler `since` paramı RFC3339 doğrulaması + HTTP 400
- [x] `FilteredSearch` date filter için SQL fallback (semantik pass boş döndüğünde)

---

## Pipeline (Mevcut)

```
[son 3 user tur + userMsg]
  → embed (+ expandQuery ikinci embed uzun sorgularda)
  → vecSearch (+ goSearch fallback)
  → multi-query RRF merge
  → ftsSearch
  → RRF (vec + fts)
  → minRRFScore filtresi
  → importance boost (×0.9–×1.3)
  → re-sort
  → top-K
  → [Memory N | relevance=XX% | importance=LABEL | source=SRC | AGE]
  → prompt
```

---

## Kalan Görevler

### ⬜ 3.1 Embedding Auto-Setup

Kullanıcı "Belleği Etkinleştir" toggle'ına bastığında:

1. `nomic-embed-text-v1.5.Q4_K_M.gguf` otomatik indir
2. `llamaEmbedServer` otomatik başlat
3. Store initialize et
4. "Hazır" bildirimi gönder

Kullanıcıdan ek kurulum istenmez. Şu an embedding ayrı yapılandırma gerektiriyor.

### ⬜ 2.2 Bellek Birleştirme (Consolidation) — Ertelenmiş

Kosinüs benzerliği > 0.92 olan çiftleri LLM ile birleştir. LLM bağımlılığı nedeniyle şimdilik ertelendi.

---

## Başarı Kriterleri

| Kriter | Şu an | Hedef |
|---|---|---|
| "3 ay önceki oteli hatırla" | ✅ FilteredSearch + SQL fallback | ✅ Tam |
| "Buna ne demiştik?" | ✅ Query enrichment (son 3 tur) | ✅ Tam |
| `/remember` komutları | ✅ | ✅ Tam |
| 10K bellekte < 500ms | ✅ 5 indeks + tek sorgu Stats | ✅ Tam |
| "Neden hatırladın?" | ✅ matchType + debug UI | ✅ Tam |
| Phrase search düzgün | ✅ Token search | ✅ Tam |
| Kalite filtresi | ✅ minRRFScore + importance boost | ✅ Tam |
| Bellek analytics | ✅ Flutter dashboard | ✅ Tam |
| Export / Import | ✅ JSON v2, UUID dedup | ✅ Tam |
| Tek toggle kurulum | ⬜ Embedding hâlâ manuel | ⬜ Faz 3.1 ile tam |
