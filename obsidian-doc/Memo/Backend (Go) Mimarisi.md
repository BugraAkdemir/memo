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

### 9. `routine` (v3.3.3'te YENİ)
- **Görev:** Doğal dilde tanımlanan zamanlanmış otomasyonlar (Routines).
- **Özellik:** Cihaz saat dilimine göre tetikleme + her (yeniden) bağlantıda offset resenkronizasyonu, basit prompt ya da tam agent çalışması olarak tetikleme, dile duyarlı üretilen metin.

### 10. `agentcli` (v3.3.4'te YENİ, Beta)
- **Görev:** Claude Code CLI / Codex CLI'ı sohbet sağlayıcısı olarak subprocess üzerinden çalıştırma.
- **Özellik:** `provider.Provider` arayüzünü uygular ama HTTP API yerine kurulu `claude`/`codex` komut satırı aracını çalıştırır; `App.lifecycleCtx`'e bağlı, sohbet-bazlı `cliJobs` kilidiyle çalışır.

### 11. `anthropicapi` (v3.3.3'te YENİ)
- **Görev:** Geliştirici API Ağ Geçidi'nin Anthropic Messages API wire-format çevirisi.
- **Özellik:** Anthropic `tool_use`/`tool_result` blokları ↔ OpenAI `tool_calls`/`role:"tool"` çevirisi, `POST /v1/messages`.

### 12. `tts` (v3.3.4'te YENİ, Beta)
- **Görev:** Sesli Mod / Live Mode metin-konuşma motoru yönlendirmesi.
- **Özellik:** Varsayılan yerel Piper, opsiyonel harici OpenAI TTS, yerel ses seçici/indirici, "düşünme" dolgu sesi.

## Kararlılık: Panic Recovery (v3.3.4)

Go, arka planda çalışan goroutine'lere HTTP handler'ların aksine otomatik bir panic koruması sağlamıyor. Bu sürümden önce kod tabanının sadece birkaç köşesinde buna karşı bir koruma vardı — hafıza kaydı, bir routine tetiklemesi, bir WhatsApp mesaj işleyicisi, proaktif öneri kontrolü, akan bir yanıt gibi neredeyse her arka plan işindeki beklenmedik bir hata **tüm** Memo sürecini çökertebiliyordu. Artık arka ucun tamamındaki arka plan işleri (hafıza, sohbet akışı, WhatsApp, bulut senk., yerel model yönetimi, STT, routine'ler, proaktif öneriler, bildirimler, uzaktan erişim tünelleri ve daha fazlası) korumalı: içlerinden birinde bir şey ters giderse kaydediliyor ve orada durduruluyor.

## Bridge Deseni
`app.go` dosyası, tüm bu modülleri bir araya getiren ana "Beyin" görevi görür. Web sunucusu, `AppBridge`/`FullBridge` arayüzü üzerinden bu motorla konuşur. `FullBridge`, `AppBridge`'i sağlayıcı yönetimi, ajan kontrolü ve orkestra yapılandırması gibi Flutter'a özel handler'larla genişletir.

### Bağlantılı Notlar:
- [[Mimari Yapı]]
- [[API Dökümantasyonu]]
- [[Llama.cpp Entegrasyonu]]
