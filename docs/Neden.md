# Hafıza SQLite + sqlite-vec ile Nasıl Çalışıyor?

## SQLite + vec0 Nedir?

Memo artık hafıza depolaması için **SQLite** veritabanını `sqlite-vec` eklentisiyle birlikte kullanır:

- **SQLite**: Gömülü, sunucusuz, ACID uyumlu ilişkisel veritabanı
- **sqlite-vec**: Vektör benzerlik araması için `vec0` sanal tablo eklentisi
- **ANN İndeksi**: Yaklaşık en yakın komşu (ANN) araması ile O(log N) karmaşıklık
- **Tek Dosya**: Tüm hafıza kayıtları `data/memory/memo.db` dosyasında saklanır

---

## Veritabanı Şeması

```
data/memory/
└── memo.db                ← Tek SQLite veritabanı dosyası
    ├── vec0 tablosu       ← Vektör ANN indeksi (sqlite-vec)
    ├── documents tablosu  ← İçerik ve metadata
    └── metadata tablosu   ← Koleksiyon bilgisi
```

### Tablo Yapısı:

| Tablo | Sütunlar | Açıklama |
|-------|----------|----------|
| `documents` | `id`, `content`, `created_at`, `metadata_json` | Hafıza kayıtlarının içerik ve zaman bilgisi |
| `vec0` | `id`, `embedding` | Vektör ANN indeksi (sqlite-vec sanal tablosu) |
| `metadata` | `key`, `value` | Koleksiyon metadata'sı |

---

## SQLite'ın Sağladığı Avantajlar

### 1. Atomik Yazma (ACID Transactions)
SQLite'ın yerleşik işlem (transaction) desteği sayesinde yazma sırasında oluşabilecek hatalar tüm veritabanını bozmaz. Ya tüm işlem başarılı olur ya da hiçbir değişiklik kalıcı olmaz.

```
✅ SQLite: Yazarken crash → Son transaction geri alınır (ROLLBACK), veri sağlam kalır
```

### 2. Eşzamanlı Erişim (Concurrent Access)
SQLite, WAL (Write-Ahead Logging) modunda okuma ve yazma işlemlerinin eşzamanlı çalışmasına izin verir. Go rutinleri aynı anda sorgulama yapabilir.

### 3. Artımlı Güncelleme
Yeni bir mesaj kaydederken sadece **INSERT** sorgusu çalışır. Tüm veritabanını yeniden yazmak gerekmez.

### 4. Lazy Loading Gerekmez
Veriler diskte yapılandırılmış şekilde durur. Sorgular sadece gerekli kayıtları getirir — tüm veriyi RAM'e yüklemek gerekmez.

### 5. Silinebilirlik
Tek bir `DELETE` sorgusu ile istenen kayıt silinir.

---

## Kayıt Yapısı

Her hafıza kaydı SQLite'da şu şekilde saklanır:

```sql
-- documents tablosu
INSERT INTO documents (id, content, created_at, metadata_json)
VALUES ('abc123', 'User: merhaba\nAssistant: Selam!', '2024-...', '{"type": "conversation"}');

-- vec0 ANN indeksi
INSERT INTO vec0 (id, embedding)
VALUES ('abc123', '[0.023, -0.841, 0.192, ...]');  -- 768 boyutlu vektör
```

- **Content**: Kullanıcı mesajı + asistan cevabı birlikte
- **Embedding**: Bu metnin vektör temsili (embedding model ile üretilir)
- **Metadata**: Zaman damgası ve ek bilgiler (JSON formatında)

---

## Arama Nasıl Çalışıyor?

1. Kullanıcı yeni mesaj yazar: `"ekran kartı öner"`
2. Bu metin embedding modeline gönderilir → `[0.12, -0.55, ...]` vektörü döner
3. `vec0` ANN indeksi üzerinden **yaklaşık en yakın komşu (ANN)** araması yapılır
4. En benzer 5 kayıt (Top-K) bulunur
5. Bu kayıtların `content`'i system prompt'a eklenir
6. LLM artık geçmiş konuşmaları "hatırlayarak" cevap verir

---

## Özet

| Soru | Cevap |
|------|-------|
| Veritabanı nedir? | SQLite + sqlite-vec |
| ANN indeksi ne işe yarar? | O(log N) vektör benzerlik araması |
| Neden SQLite? | ACID, sıfır konfigürasyon, tek dosya |
| Kayıtlar ne zaman oluşuyor? | Her `SendMessage()` çağrısından sonra (arka planda) |
| Hafızayı sıfırlayabilir miyim? | Evet, `data/memory/memo.db` dosyasını silersen hafıza sıfırlanır |
