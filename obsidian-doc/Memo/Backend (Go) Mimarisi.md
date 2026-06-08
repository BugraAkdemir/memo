# 🖥️ Backend (Go) Mimarisi

Memo'nun arka ucu, hız, güvenilirlik ve düşük kaynak tüketimi için Go dili ile yazılmış "Headless" bir sunucudur.

## Modüler Yapı (`internal/`)
Kod tabanı, her biri belirli bir sorumluluğa sahip olan modüllere ayrılmıştır:

### 1. `webserver`
- **Görev:** REST API katmanını yönetir.
- **Teknoloji:** Standart `http.ServeMux` (harici router yok).
- **Özellik:** Flutter ile güvenli yerel haberleşme (REST + SSE).

### 2. `llama`
- **Görev:** `llama-server` (C++) süreçlerini yönetir.
- **Özellik:** Otomatik kurulum (`llamaInstaller`), GPU/VRAM tespiti ve çoklu süreç (chat + embedding) yönetimi.

### 3. `memory`
- **Görev:** Semantik hafıza ve vektör veritabanı.
- **Özellik:** Kosinüs benzerliği hesaplamaları ve SQLite + sqlite-vec ile vec0 ANN indeksi tabanlı arama.

### 4. `cloudsync`
- **Görev:** Google Drive entegrasyonu.
- **Özellik:** AES-256-GCM E2E şifreleme ile PBKDF2 anahtar türetme ve veri yedekleme.

### 5. `identity`
- **Görev:** Sistem promptlarını ve kullanıcı kimliklerini (persona) yönetir.

### 6. `provider` (v3.0.0'da YENİ)
- **Görev:** Harici LLM sağlayıcı entegrasyonu.
- **Özellik:** OpenAI uyumlu API istemcisi, özel Gemini/Claude implementasyonları, çoklu sağlayıcı router ile fallback zinciri, AES-256 şifreli API anahtarı saklama.

### 7. `agent` (v3.0.0'da YENİ)
- **Görev:** AI ajan çalıştırma motoru.
- **Özellik:** Araç kaydı (8 yerleşik araç), izin yöneticisi (6 politika türü), çalıştırma sandbox'ı (path validasyonu, rate limiting, komut kara listesi), LLM ↔ araç pipeline'ı.

### 8. `orchestra` (v3.0.0'da YENİ)
- **Görev:** Çoklu model orkestrasyonu.
- **Özellik:** Şef model planlar ve uzman rollere dağıtır, paralel/sıralı görev çalıştırma, sonuç sentezleme.

## Bridge Deseni
`app.go` dosyası, tüm bu modülleri bir araya getiren ana "Beyin" görevi görür. Web sunucusu, `AppBridge`/`FullBridge` arayüzü üzerinden bu motorla konuşur. `FullBridge`, `AppBridge`'i sağlayıcı yönetimi, ajan kontrolü ve orkestra yapılandırması gibi Flutter'a özel handler'larla genişletir.

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[API Dökümantasyonu]]
- [[Llama.cpp Entegrasyonu]]
