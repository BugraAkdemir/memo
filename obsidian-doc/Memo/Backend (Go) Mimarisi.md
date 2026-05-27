# 🖥️ Backend (Go) Mimarisi

Memo'nun arka ucu, hız, güvenilirlik ve düşük kaynak tüketimi için Go dili ile yazılmış "Headless" bir sunucudur.

## Modüler Yapı (`internal/`)
Kod tabanı, her biri belirli bir sorumluluğa sahip olan modüllere ayrılmıştır:

### 1. `webserver`
- **Görev:** REST API katmanını yönetir.
- **Teknoloji:** Gin Gonic framework.
- **Özellik:** Flutter ile güvenli yerel haberleşme ve isteğe bağlı uzaktan erişim desteği.

### 2. `llama`
- **Görev:** `llama-server` (C++) süreçlerini yönetir.
- **Özellik:** Otomatik kurulum (`llamaInstaller`), GPU/VRAM tespiti ve çoklu süreç (chat + embedding) yönetimi.

### 3. `memory`
- **Görev:** Semantik hafıza ve vektör veritabanı.
- **Özellik:** Kosinüs benzerliği hesaplamaları, `.gob` serileştirme ve RAM-tabanlı hızlı indeksleme.

### 4. `cloudsync`
- **Görev:** Google Drive entegrasyonu.
- **Özellik:** AES-256 E2E (Uçtan Uca) şifreleme ile veri yedekleme.

### 5. `identity`
- **Görev:** Sistem promptlarını ve kullanıcı kimliklerini (persona) yönetir.

## Bridge Deseni
`app.go` dosyası, tüm bu modülleri bir araya getiren ana "Beyin" görevi görür. Web sunucusu, `AppBridge` arayüzü üzerinden bu motorla konuşur.

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[API Dökümantasyonu]]
- [[Llama.cpp Entegrasyonu]]
