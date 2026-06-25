# Memo — RAG Bellek Sistemi: Tam Yol Haritası

> **Tek vaat:** "Senin hafızan olan AI." Her sohbet, kullanıcıyı daha iyi tanıyan bir fırsattır.
> Bellekler otomatik oluşur, önemli olanlar öne çıkar, gereksizler unutulur.

---

## Mevcut Durum (2026-06-25)

### Tamamlananlar ✅
- Temel vektör araması (vec0, cosine distance)
- SQLite WAL modu
- Bellek kaydetme / silme / temizleme
- TopK / MinSimilarity ayarları
- Chunking — 300 kelime, 50 kelime overlap
- Hybrid search — FTS5 + vektör, RRF birleştirme
- `MatchType` alanı (vector / fts / hybrid)
- Debug UI — "Bellek Ara" paneli
- FTS5 / vec0 graceful fallback

### Hazırlık: ~%40

**Pipeline şu an:**
```
userMsg → embed(userMsg) → vecSearch + ftsSearch → RRF → top5 → prompt
```

**Hedef pipeline:**
```
[son 3 tur özeti + userMsg] → embed → vecSearch + ftsSearch → RRF →
→ importance boost → re-rank → top5 → [tarih+önem+kaynak formatlı] prompt
```

---

## Faz 0 — Kritik Bug Düzeltmeleri (Bu Hafta)

> Bunlar mevcut sistemi kıran hatalar. Başka hiçbir şeyden önce yapılmalı.

### 0.1 FTS5 Phrase Search Bug 🔴

**Sorun:** `escapeFTSQuery` tüm sorguyu tırnak içine alıyor → FTS5 her şeyi
**exact phrase** olarak arıyor. "Istanbul oteli" yazıldığında "Istanbul" ve "otel"
ayrı geçse bile eşleşmiyor.

```go
// YANLIŞ (şu an):
func escapeFTSQuery(q string) string {
    q = strings.ReplaceAll(q, `"`, `""`)
    return `"` + q + `"`  // ← tüm sorgu phrase
}

// DOĞRU:
func escapeFTSQuery(q string) string {
    words := strings.Fields(q)
    parts := make([]string, 0, len(words))
    for _, w := range words {
        w = strings.ReplaceAll(w, `"`, `""`)
        w = strings.ReplaceAll(w, `*`, "")
        w = strings.ReplaceAll(w, `(`, "")
        w = strings.ReplaceAll(w, `)`, "")
        if len(w) >= 2 {
            parts = append(parts, `"`+w+`"`)
        }
    }
    if len(parts) == 0 {
        return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
    }
    return strings.Join(parts, " ")  // AND search; her token ayrı aranır
}
```

### 0.2 minSimilarity RRF Sonrası Uygulanmıyor 🔴

**Sorun:** RRF çıktısındaki `Similarity` değeri artık ~0.016 gibi küçük bir RRF skoru.
Config'deki `MinSimilarity=0.3` sadece `vecSearch`'e uygulanıyor.
FTS5 sonuçları tamamen filtresiz prompt'a giriyor.

```go
// RetrieveContext sonunda eklenecek:
const minRRFScore = float32(0.010) // yaklaşık RRF eşiği (k=60, top-5 için)
filtered := memories[:0]
for _, m := range memories {
    if m.MatchType == "vector" && m.Similarity < minSimilarity {
        continue
    }
    if m.MatchType != "vector" && m.Similarity < minRRFScore {
        continue
    }
    filtered = append(filtered, m)
}
memories = filtered
```

### 0.3 Orta Chunk'larda Embed Kalitesi Düşük 🟡

**Sorun:** `saveChunk`'ta embedding sadece `userChunk` üzerinden hesaplanıyor.
Asistan cevabı embedding'e girmiyor. "Bana X önerdin" araması orta chunk'larda
başarısız çünkü orta chunk'ların assist_msg'i de boş.

```go
// saveChunk'ta embed metni:
embedText := userChunk
if assistantMsg != "" {
    // Asistan cevabını da embed'e kat — retrieval kalitesi artar
    embedText = userChunk + "\n" + assistantMsg
}
embedding, err := s.embed(ctx, embedText)
```

Aynı zamanda tüm chunk'larda `assist_msg` sakla (sadece son chunk'ta değil).
Depolama artışı küçük, retrieval kalitesi büyük fark yaratır.

### 0.4 candidateK Sabit, Dinamik Olmalı 🟡

**Sorun:** `candidateK = topK * 3` → TopK=5 ise sadece 15 aday.
10.000 bellekte bu çok az.

```go
candidateK := topK * 5
if candidateK < 20 {
    candidateK = 20
}
if candidateK > 100 {
    candidateK = 100
}
```

---

## Faz 1 — Retrieval Kalitesi (1–2 Hafta)

### 1.1 Sorgu Zenginleştirme (Query Enrichment)

**Sorun:** Retrieve sadece son mesajla yapılıyor.
`"buna ne demiştik?"` gibi bağlamlı sorular başarısız.

```go
// buildMessages içinde:
queryForMemory := a.buildMemoryQuery(userMsg)
memories = a.retrieveMemory(ctx, queryForMemory)

// buildMemoryQuery:
func (a *App) buildMemoryQuery(userMsg string) string {
    history := a.getSessionHistory()
    if len(history) == 0 {
        return userMsg
    }
    // Son 3 user mesajını özetle ve sorguya ekle
    var recent []string
    count := 0
    for i := len(history) - 1; i >= 0 && count < 3; i-- {
        if history[i].Role == "user" {
            recent = append([]string{history[i].GetTextContent()}, recent...)
            count++
        }
    }
    if len(recent) == 0 {
        return userMsg
    }
    return strings.Join(recent, " | ") + " | " + userMsg
}
```

### 1.2 Metadata Alanları

`memories` tablosuna şu alanları ekle:

```sql
session_id   TEXT NOT NULL DEFAULT '',   -- hangi sohbet oturumu
importance   INTEGER NOT NULL DEFAULT 3, -- 1-5 arası önem puanı
tags         TEXT NOT NULL DEFAULT '',   -- virgülle ayrılmış etiketler
source       TEXT NOT NULL DEFAULT 'conversation',
             -- 'conversation' | 'explicit' | 'document'
retrieve_count INTEGER NOT NULL DEFAULT 0 -- kaç kez retrieve edildi
```

`MemoryResult` struct'ına ekle:
```go
SessionID     string `json:"session_id,omitempty"`
Importance    int    `json:"importance,omitempty"`
Tags          string `json:"tags,omitempty"`
Source        string `json:"source,omitempty"`
RetrieveCount int    `json:"retrieve_count,omitempty"`
```

### 1.3 Retrieve Sayacı

Her başarılı retrieve'de `retrieve_count++` yap:

```go
// RetrieveContext sonunda:
if len(memories) > 0 {
    ids := make([]string, len(memories))
    for i, m := range memories {
        ids[i] = m.ID
    }
    go s.incrementRetrieveCounts(ids)
}
```

Bu veri Faz 3'teki importance scoring için kritik.

### 1.4 Importance Retrieve'de Boost

```go
// vecSearch / ftsSearch sonuçları RRF'e girmeden önce:
for i := range vecMemories {
    imp := float64(vecMemories[i].Importance) // 1-5
    vecMemories[i].Similarity *= float32(0.8 + imp*0.1) // %80-%130 arası boost
}
```

### 1.5 Explicit Bellek Komutları

Frontend'de komut ayrıştırma middleware'i:

| Komut | Davranış |
|---|---|
| `/remember <metin>` | `source='explicit', importance=5` ile belle ekle |
| `/forget <uuid veya içerik>` | Eşleşen belleği sil |
| `/search <sorgu>` | Debug araması yap, sonuçları sohbette göster |

Backend endpoint'leri:
- `POST /api/memory/explicit` — manuel bellek ekle
- `DELETE /api/memory/explicit` — `q` parametresiyle fuzzy sil
- `GET /api/memory/debug-search` — zaten var ✅

### 1.6 Bellek Prompt Formatı Yükseltme

```go
// FormatMemoriesForPrompt:
fmt.Fprintf(&sb,
    "[Bellek %d | Önem: %s | Kaynak: %s | Tarih: %s | Skor: %.0f%%]\n%s\n\n",
    i+1,
    importanceLabel(m.Importance), // "Yüksek", "Orta", "Düşük"
    m.Source,
    formatAge(m.Timestamp),        // "3 gün önce", "2 ay önce"
    m.Similarity*100,
    m.Content,
)
```

System prompt'a kural ekle:
```
Belleklerde olmayan bilgileri uydurma. Tarih veya kaynak belirsizse tahmin etme.
```

---

## Faz 2 — Bellek Zekası (1 Ay)

### 2.1 Importance Scoring Motoru

Önem puanı 1–5 arası. Değiştiren kurallar:

| Kural | Etki |
|---|---|
| `/remember` ile eklendi | +2 (başlangıç 5) |
| `retrieve_count` arttıkça | Her 3 retrieve'de +1, max 5 |
| Sadece selamlaşma / teşekkür | -1 veya -2 (LLM ile tespit) |
| 30 günde hiç retrieve edilmedi | -1 |
| Kullanıcı bellekle etkileşime girdi | +1 |

Scoring bir arka plan goroutine'i tarafından günlük çalıştırılır:

```go
func (s *Store) runImportanceDecay(ctx context.Context) {
    ticker := time.NewTicker(24 * time.Hour)
    defer ticker.Stop()
    for {
        select {
        case <-ctx.Done(): return
        case <-ticker.C:
            s.applyImportanceRules(ctx)
        }
    }
}
```

### 2.2 Bellek Birleştirme (Consolidation)

Haftada bir çalışan "memory janitor":

```go
func (s *Store) consolidate(ctx context.Context) error {
    // 1. Kosinüs benzerliği > 0.92 olan bellek çiftlerini bul
    // 2. İkisini tek özet belleğe birleştir (LLM veya basit metin concat)
    // 3. importance = max(a.importance, b.importance) + 1
    // 4. Orijinal ikisini sil
}
```

### 2.3 Bellek Unutma (Forgetting)

Silme kriterleri:

```sql
DELETE FROM memories
WHERE
    -- 6 aydan eski VE önemsiz VE hiç retrieve edilmemiş
    (julianday('now') - julianday(timestamp) > 180
     AND importance <= 2
     AND retrieve_count = 0)
    OR
    -- Sadece selamlaşma/kısa onay (importance=-1 ile işaretli)
    importance < 1
```

Silme işlemi önce `pending_deletion=1` ile işaretler (soft delete),
7 gün sonra hard delete.

### 2.4 Re-ranking

İlk retrieval sonuçlarını ikinci geçişle sırala:

```go
// Heuristik re-ranker (LLM gerekmez):
func rerank(query string, results []MemoryResult) []MemoryResult {
    for i := range results {
        score := results[i].Similarity

        // Recency boost: son 7 gün içindeyse +%20
        age := time.Since(parseTimestamp(results[i].Timestamp))
        if age < 7*24*time.Hour {
            score *= 1.2
        }

        // Importance boost
        score *= float32(0.8 + float64(results[i].Importance)*0.1)

        // Exact keyword bonus: sorgu kelimeleri içerikte geçiyorsa +%15
        if containsQueryTerms(query, results[i].Content) {
            score *= 1.15
        }

        results[i].Similarity = score
    }
    sort.Slice(results, func(i, j int) bool {
        return results[i].Similarity > results[j].Similarity
    })
    return results
}
```

### 2.5 Multi-Query Retrieval

Kullanıcı sorusundan 2 varyasyon üret, her biriyle ara, birleştir:

```go
// Basit kural tabanlı (LLM gerektirmez):
func expandQuery(q string) []string {
    queries := []string{q}
    // Türkçe → İngilizce anahtar kelime çevirisi (küçük sözlük)
    // "otel" → "hotel", "seyahat" → "travel" gibi
    if en := translateKeywords(q); en != q {
        queries = append(queries, en)
    }
    // Soru ise fiil kısmını çıkar: "ne önerdin?" → "öneri"
    if isQuestion(q) {
        queries = append(queries, extractNoun(q))
    }
    return queries
}
```

---

## Faz 3 — Production & Ölçek (2 Ay)

### 3.1 Embedding Auto-Setup

Kullanıcı "Belleği Etkinleştir" toggle'ına bastığında:

1. `nomic-embed-text-v1.5.Q4_K_M.gguf` otomatik indir (zaten `EmbeddingModelRepo` var)
2. `llamaEmbedServer` otomatik başlat
3. Store initialize et
4. "Hazır" bildirimi gönder

Kullanıcıdan ek kurulum istenmez.

### 3.2 Performans Garantisi (< 500ms)

Binlerce bellekte < 500ms için:

```sql
-- Gerekli indeksler:
CREATE INDEX IF NOT EXISTS idx_memories_timestamp ON memories(timestamp);
CREATE INDEX IF NOT EXISTS idx_memories_importance ON memories(importance);
CREATE INDEX IF NOT EXISTS idx_memories_session ON memories(session_id);

-- vec0 zaten indeksli ✅
-- memories_fts zaten indeksli ✅
```

Latency budgeti:
- Embed: ~150ms (model bağımlı)
- vecSearch: ~30ms (10K row)
- ftsSearch: ~20ms
- RRF + re-rank: ~5ms
- **Toplam hedef: < 250ms** (embed dışında < 100ms)

### 3.3 Tarih ve Etiket Filtreleme

```go
type RetrieveOptions struct {
    TopK          int
    MinSimilarity float32
    Since         time.Time   // "geçen haftadan beri"
    Until         time.Time
    Tags          []string    // ["iş", "sağlık"]
    Source        string      // "explicit" | "conversation"
    SessionID     string      // belirli oturum
}
```

### 3.4 Bellek Analytics Dashboard (Flutter)

Ayarlar → Bellek → yeni "Analitik" sekmesi:

```
Toplam bellek:     1,247
Bu hafta eklenen:  43
En çok retrieve:   "İstanbul seyahati" (12 kez)
En önemli:         5 bellek — importance=5
Unutulacaklar:     8 bellek — 7 gün içinde silinecek
FTS5 durumu:       Aktif ✅
vec0 durumu:       Aktif ✅
```

### 3.5 Memory Export / Import

```
GET  /api/memory/export   → memories.json (tüm bellekler)
POST /api/memory/import   → JSON yükle, merge et
```

Format:
```json
{
  "version": 2,
  "exported_at": "2026-06-25T...",
  "memories": [
    {
      "uuid": "mem_...",
      "content": "...",
      "importance": 4,
      "tags": "iş,proje",
      "timestamp": "...",
      "source": "conversation"
    }
  ]
}
```

---

## Dondurulmuş Özellikler

RAG olgunlaşana kadar dokunulmayacak:

- WhatsApp bridge
- Orchestra multi-model
- Skill sistemi
- Mood tracking
- Calendar / reminder
- Agent tools (bellek komutları hariç)

---

## Başarı Kriterleri

| Kriter | Şu an | Hedef |
|---|---|---|
| "3 ay önceki oteli hatırla" | ❌ Tarih filtresi yok | ✅ Faz 1.2 + 3.3 |
| "Buna ne demiştik?" | ❌ Bağlam yok | ✅ Faz 1.1 |
| `/remember` komutları | ❌ | ✅ Faz 1.5 |
| 10K bellekte < 500ms | 🟡 ~300ms tahmin | ✅ Faz 3.2 |
| "Neden hatırladın?" | ✅ Debug UI var | ✅ Faz 1.6 ile daha iyi |
| Tek toggle kurulum | ✅ Temel var | ✅ Faz 3.1 ile tam |
| Phrase search düzgün | ❌ Bug var | ✅ Faz 0.1 |
| Kalite filtresi | ❌ RRF sonrası yok | ✅ Faz 0.2 |

---

## Sıradaki Adım

**Faz 0 bug düzeltmeleri** — 4 değişiklik, hepsi `internal/memory/store.go`:

1. `escapeFTSQuery` → token search
2. `RetrieveContext` → RRF sonrası filtre
3. `saveChunk` → combined embed + tüm chunk'larda assist_msg
4. `candidateK` → dinamik hesap

Bunlar bitmeden Faz 1'e geçilmez.
