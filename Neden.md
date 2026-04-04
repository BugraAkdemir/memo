# Neden Her Sohbet İçin Yeni .gob Dosyası Açılıyor?

## .gob Nedir?

`.gob` (Go Binary) — Go dilinin kendi **binary serialization** formatıdır. JSON veya XML gibi text-based değil, **binary** formattır. Bu yüzden:

- **Daha hızlı** okunur/yazılır (parse etmeye gerek yok)  
- **Daha küçük** dosya boyutu (text overhead yok)
- **Go struct'larını direkt** diske yazar (tip güvenliği korunur)

Go'nun standart kütüphanesindeki `encoding/gob` paketi ile çalışır.

---

## Dosya Yapısı

```
data/memory/
└── 5c0dc939/              ← Collection (koleksiyon) dizini
    ├── 6d13fa80.gob       ← Tek bir hafıza kaydı (document)
    ├── a3f7b2c1.gob       ← Başka bir hafıza kaydı
    ├── 1e9d4a5f.gob       ← ...
    └── _chromem.gob        ← Koleksiyon metadata'sı
```

### Açıklama:

| Yol | Ne İşe Yarar |
|-----|-------------|
| `5c0dc939/` | Collection ID'sinin hash'i. `chromem-go` koleksiyon adını (`conversations`) alıp hash'leyerek dizin adı oluşturur |
| `6d13fa80.gob` | **Tek bir konuşma kaydı (document)**. Her `SaveInteraction()` çağrısında yeni bir dosya oluşur |
| `_chromem.gob` | Koleksiyonun metadata'sı (isim, embedding boyutu vb.) |

---

## Neden Hepsi Tek Dosyada Değil?

Bu mimari kararın **5 kritik nedeni** var:

### 1. Atomik Yazma (Atomic Writes)
Her kayıt kendi dosyasındaysa, bir yazma hatası **sadece o kaydı** etkiler. Tek dosyalı sistemde bir crash tüm veritabanını bozabilir.

```
❌ Tek dosya: Yazarken crash → TÜM VERİ BOZULUR
✅ Ayrı dosya: Yazarken crash → Sadece 1 kayıt etkilenir, gerisi sağlam
```

### 2. Eşzamanlı Erişim (Concurrent Access)
Farklı goroutine'ler farklı dosyalara aynı anda yazabilir. Tek dosyada tüm yazma işlemleri sıralı olmak zorunda (bottleneck).

### 3. Artımlı Güncelleme (Incremental Updates)
Yeni bir mesaj kaydederken sadece **1 yeni dosya** yazarsın. Tek dosya olsaydı, her yeni kayıtta **tüm dosyayı baştan yazmak** gerekirdi (O(n) vs O(1)).

### 4. Lazy Loading
Uygulama açıldığında **tüm dosyaları RAM'e yüklemek zorunda değil**. İhtiyaç oldukça (query geldiğinde) dosyalar okunur. Tek dosyada tüm veri baştan yüklenmek zorunda.

### 5. Silinebilirlik
Tek bir kaydı silmek = tek bir dosyayı silmek. Tek dosyada silme işlemi dosyayı yeniden yazmayı gerektirir.

---

## Her .gob Dosyasının İçeriği

Her `.gob` dosyası şu yapıyı binary olarak saklar:

```go
type Document struct {
    ID        string       // Benzersiz ID (hash)
    Content   string       // "User: merhaba\nAssistant: Selam! Nasılsın?"
    Metadata  map[string]string  // {"timestamp": "2024-...", "type": "conversation"}
    Embedding []float32    // [0.023, -0.841, 0.192, ...] → 768 boyutlu vektör
}
```

- **Content**: Kullanıcı mesajı + asistan cevabı birlikte
- **Embedding**: Bu metnin vektör temsili (LM Studio'daki embedding model ile üretilir)
- **Metadata**: Zaman damgası ve ek bilgiler

---

## Arama Nasıl Çalışıyor?

1. Kullanıcı yeni mesaj yazar: `"ekran kartı öner"`
2. Bu metin embedding modeline gönderilir → `[0.12, -0.55, ...]` vektörü döner
3. Tüm `.gob` dosyalarındaki embedding'lerle **kosinüs benzerliği** hesaplanır
4. En benzer 5 kayıt (Top-K) bulunur
5. Bu kayıtların `Content`'i system prompt'a eklenir
6. LLM artık geçmiş konuşmaları "hatırlayarak" cevap verir

---

## Özet

| Soru | Cevap |
|------|-------|
| .gob nedir? | Go'nun binary serialization formatı |
| Neden her kayıt ayrı dosyada? | Atomik yazma, performans, güvenlik |
| Neden tek dosya değil? | Crash riski, yavaş yazma, scalability |
| Bu dosyalar ne zaman oluşuyor? | Her `SendMessage()` çağrısından sonra (arka planda) |
| Silebilir miyim? | Evet, `data/memory/` klasörünü silersen hafıza sıfırlanır |
