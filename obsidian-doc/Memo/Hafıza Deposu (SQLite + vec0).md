# Hafıza Deposu (SQLite + vec0)

Memo, hafıza depolaması için **SQLite** veritabanını, vektör araması için `sqlite-vec` (`vec0`) ve anahtar kelime araması için SQLite'ın yerleşik `FTS5` uzantısıyla birlikte kullanır.

---

## Genel Bakış

- **SQLite**: Gömülü, sunucusuz, ACID uyumlu ilişkisel veritabanı
- **sqlite-vec (`vec0`)**: Vektör benzerlik araması için sanal tablo eklentisi (ANN — yaklaşık en yakın komşu)
- **FTS5**: Anahtar kelime (tam metin) araması için SQLite'ın kendi uzantısı
- **Tek Dosya**: Tüm hafıza kayıtları `data/memory/memory.db` dosyasında saklanır

İkisi de derleme zamanında (`-tags "sqlite_fts5"`, bkz. [[CGO Bayrakları]]) etkinleştirilmezse veya sistemde mevcut değilse, o kısım sessizce devre dışı kalır — vektör tarafı Go'da yazılmış bir kosinüs-benzerliği taramasına, FTS5 tarafı ise "yok" durumuna düşer. **Bu, projenin gerçek bir hatasıydı**: `-tags "sqlite_fts5"` uzun süre hiçbir derleme betiğinde geçilmediği için FTS5 hiç aktif olmamıştı — 2026-07-15'te düzeltildi (bkz. [[Bilinen Sorunlar]]).

## Veritabanı Şeması

```
data/memory/
└── memory.db
    ├── memories tablosu       ← Asıl satırlar (içerik, embedding, importance, source...)
    ├── memories_fts tablosu   ← FTS5 sanal tablosu (anahtar kelime araması)
    ├── vec_memories tablosu   ← vec0 sanal tablosu (vektör ANN araması)
    └── _metadata tablosu      ← Migration bayrakları, embedding boyutu
```

### Tablo Yapısı

| Tablo | Önemli Sütunlar | Açıklama |
|-------|----------|----------|
| `memories` | `uuid`, `content`, `timestamp`, `importance` (1-5), `source`, `retrieve_count`, `embedding` | Her hafıza kaydının tam içeriği ve meta verisi |
| `memories_fts` | `content`, `user_msg`, `assist_msg` | FTS5 tam metin indeksi |
| `vec_memories` | `embedding` | vec0 vektör ANN indeksi |
| `_metadata` | `key`, `value` | İç durum (embedding boyutu, migration bayrakları) |

`source` sütunu üç değer alabilir:
- `conversation` — normal bir sohbet turu (varsayılan, `importance=3`)
- `explicit` — kalıcı olarak "pinlenmiş" bir gerçek (`/remember` komutu, hafıza içe aktarma, veya otomatik gerçek tespiti — bkz. aşağı), her zaman `importance=5`
- `merged` — gece çalışan birleştirme (consolidation) sürecinin iki benzer hafızayı tek satıra indirmesiyle oluşan satır, `importance=4`

## Arama Nasıl Çalışıyor? (Hibrit: Vektör + Anahtar Kelime)

1. Kullanıcı yeni bir soru sorar.
2. Bu metin embedding modeline gönderilir → bir vektöre dönüşür.
3. **Vektör araması**: `vec0` (varsa) ya da Go'daki yedek kosinüs-benzerliği taraması ile en yakın adaylar bulunur.
4. **Anahtar kelime araması** (FTS5 aktifse): sorunun kelimeleri `OR` ile birleştirilip bm25 sıralamasıyla eşleşen satırlar bulunur (`escapeFTSQuery` — kelimeler `AND` değil `OR` ile birleştirilir, aksi halde çok-konulu bir soru hiçbir satırla eşleşmezdi).
5. **Çok-konulu soru bölme**: soru "ve/ile/and/," gibi bağlaçlara göre ayrı konulara bölünüp her biri kendi vektör aramasını alır (`splitCompoundQuery`) — tek bir harmanlanmış vektöre sıkışıp bir konunun kaybolmasını önler.
6. Vektör + anahtar kelime sonuçları **Reciprocal Rank Fusion (RRF)** ile birleştirilir.
7. Sonuçlar `importance`e göre ağırlıklandırılıp (importance=5 → ×1.30, importance=3 → ×1.10) yeniden sıralanır.
8. En iyi `Top-K` (varsayılan 8) kayıt system prompt'a eklenir.

## Sabitlenmiş (Pinned) Gerçekler — Aramadan Bağımsız Garanti

Yukarıdaki adım 1-8 hâlâ bir "tahmin" sürecidir — çok büyümüş bir hafıza deposunda, kısa ve önemli bir gerçek (isim, doğum günü, evcil hayvan) diğer sıradan sohbetlerle "yarışıp" kaybolabilir. Bunun için ayrı bir garanti katmanı var:

- `GetPinnedFacts` — `source='explicit' AND importance=5` olan en fazla 50 satırı, **hiçbir arama/sıralamaya girmeden**, düz bir SQL filtresiyle çeker.
- Bu satırlar her system prompt'a **koşulsuz olarak** ekleniyor (yukarıdaki hibrit aramanın sonucundan bağımsız).
- Bu listeye girmenin iki yolu var: manuel `/remember` komutu, ya da (2026-07-15'te eklenen) her sohbet turundan sonra arka planda çalışan `extractAndPinFacts` — kalıcı bir kişisel gerçek tespit ederse otomatik olarak pinliyor.
- Gece çalışan birleştirme (consolidation) süreci, pinlenmiş gerçekleri asla otomatik birleştirip "un-pin" etmiyor — `source='explicit'` aday havuzundan hariç tutuluyor.

## Özet

| Soru | Cevap |
|------|-------|
| Veritabanı nedir? | SQLite + sqlite-vec (vektör) + FTS5 (anahtar kelime) |
| Arama neden hibrit? | Salt vektör benzerliği, kısa/kesin gerçekleri çok-konulu sorularda kaçırabiliyor |
| Bir gerçeğin hatırlanması nasıl garanti ediliyor? | `source='explicit'` etiketi — arama/sıralamaya hiç girmeden her promptta yer alır |
| Kayıtlar ne zaman oluşuyor? | Her sohbet turundan sonra (arka planda), artı kalıcı gerçek tespit edilirse ayrıca pinlenmiş bir kopya |
| Hafızayı sıfırlayabilir miyim? | `data/memory/memory.db` dosyasını silerek |

### Bağlantılı Notlar:
- [[RAG ve Semantik Hafıza]]
- [[Vektör Arama Mantığı]]
- [[Veri Katmanı ve Kalıcılık]]
- [[Bilinen Sorunlar]]
