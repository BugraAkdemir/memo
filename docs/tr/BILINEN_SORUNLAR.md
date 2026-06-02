# Bilinen Sorunlar ve Teknik Riskler (Kapsamlı Denetim)

Bu belge, Memo projesindeki tüm tespit edilen hataları, mimari kısıtlamaları ve uç durumları takip eder. Derinlemesine yapılan kod denetimi sonrası güncellenmiştir.

**Öncelik kategorileri:**
- 🔴 **Kritik** — çökme, veri kaybı, güvenlik açığı veya tamamen bozuk özellik
- 🟠 **Yüksek** — büyük hata, ciddi performans sorunu veya güvenilirlik problemi
- 🟡 **Orta** — kullanıcı deneyimi düşüklüğü, küçük hata veya kritik olmayan güvenilirlik sorunu
- 🔵 **Düşük** — kozmetik, küçük iyileştirme veya uç durum
- ⚪ **Bilgi** — tasarım notu, risk veya gözlem

Tespit edilen 55 hata da çözüldü. Bu dosya artık sadece tasarım gözlemlerini takip eder.

---

## ⚪ Bilgi / Gözlemler

### B1. GOB Kodlaması ve İleri Uyumluluk
- **Dosya:** `internal/memory/store.go:302-306`
- **Not:** `chromem.Document`, Go'nun `gob` kodlaması ile serileştirilir. Gob, struct alan değişikliklerine duyarlıdır: gelecek bir sürümde alan eklemek, kaldırmak veya yeniden adlandırmak mevcut tüm hafıza dosyalarını okunamaz hale getirecektir. Kendi kendini tanımlayan bir format (JSON, CBOR veya protobuf) düşünün.

### B2. Etkileşim Başına Tek Dosya Tasarımı
- **Dosya:** `internal/memory/store.go`
- **Not:** Her hafıza etkileşimi ayrı bir `.gob` dosyasıdır. Bu tasarım silmeyi (`os.Remove`) basitleştirir ancak şunlar için patolojik davranış yaratır:
  - Başlangıç: O(N) dosya okuma
  - Bulut senkronizasyonu: senkronizasyon başına O(N) API çağrısı
  - Dosya tanıtıcı kullanımı
  - HDD'lerde disk arama süreleri

### B3. Filepath.Walk Hata Yutma
- **Dosya:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`
- **Not:** Birçok `filepath.Walk` geri çağrısı tüm hatalar için `nil` döndürür. İzin reddedilen dizinler ve G/Ç hataları kullanıcıya görünmez.

### B4. Gömme İstemcisi Yeniden Başlatma Sonrası Eski Referans
- **Dosya:** `app.go:148-149`, `app.go:124-125`
- **Not:** `a.client` değiştirildiğinde (yeni LLM endpoint'i), `a.store`'daki gömme fonksiyonu hala eski client'ı referans alır. Store, `reinitMemoryStore` çağrılana kadar önceki endpoint'i kullanmaya devam eder.

### B5. Dosya Adına Göre Model Otomatik Sınıflandırma
- **Dosya:** `internal/modelstore/modelstore.go:58-64`
- **Not:** `isEmbeddingModel`, dosya adı veya repo ID'sinin gömme ile ilgili anahtar kelimeler (bge, e5, vb.) içerip içermediğini kontrol eder. Bu buluşsal yöntem, adında bu string'leri bulunan ancak aslında sohbet modeli olan modelleri yanlış sınıflandırır.

### B6. `unsanitizePath`, Repo ID'lerindeki `__`'den `/` Enjekte Edebilir
- **Dosya:** `internal/modelstore/modelstore.go:345`
- **Not:** `unsanitizePath`, `__`'yi `/` ile değiştirir. Bir HuggingFace repo ID'si doğal olarak `__` içeriyorsa, bu beklenmeyen dizin yapıları oluşturur. Yol geçişi `filepath.Join` normalizasyonu ile önlenir ancak dizin düzeni kullanıcıları şaşırtabilir.

### B7. Llama Sunucusu Stderr'i Uygulama Loglarıyla Karışıyor
- **Dosya:** `internal/llama/llama.go:118-119`
- **Not:** Alt süreç stdout/stderr'i `os.Stdout`/`os.Stderr`'e ayarlanmıştır. Llama.cpp'nin tanılama çıktısı (prompt işleme istatistikleri, zamanlama, uyarılar) ön ek veya filtreleme olmadan doğrudan uygulamanın çıktı akışında görünür.

### B8. `UpdateSyncSettings` ve `ensureSyncManager` Arasında Yarış
- **Dosya:** `app.go:1505-1510`, `app.go:1627-1652`
- **Not:** `ensureSyncManager()`, `a.syncManager`'ı kilit olmadan okur. `UpdateSyncSettings`, `syncManager = nil` ayarlar ve senkronizasyon olmadan yeni bir örnek oluşturur. Eşzamanlı çağrılar güncel olmayan veya çift başlatmaya neden olabilir.

---

> **Son güncelleme:** 2026-06-02  
> **Denetim kapsamı:** Tüm kod tabanı — Go backend (app.go, tüm internal/ paketleri) ve Flutter frontend  
> **Toplam gözlem:** 8
