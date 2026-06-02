# Kritik Hatalar (🔴) — Adım Adım Düzeltme

## [x] K1 — Yetim SSE Bağlantıları ✅
- **Dosya:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`, `app.go`, `internal/webserver/bridge.go`
- **Sorun:** İstemci kopunca LLM çalışmaya devam ediyor, GPU/CPU boşa harcanıyor
- **Çözüm:** `FullBridge` stream metodlarına `ctx context.Context` eklendi. `handleSendStream`'den `r.Context()` tüm zincire iletilir. Client kopunca context cancel → LLM HTTP isteği iptal → goroutine doğal sonlanır.
- **İşlemler:**
  - [x] `bridge.go` — `SendMessageStream`/`SendMessageWithImageStream`/`SendMessageWithFileStream` imzalarına `ctx` eklendi
  - [x] `handlers_flutter.go` — `handleSendStream`'de `r.Context()` zincire bağlandı
  - [x] `app.go` — Stream metodları ve `handleIncognitoStream` `ctx` parametresi alıp `callLLMStream`'e iletiyor
  - [x] Go derleme testi yapıldı

---

## [x] K2 — Engine Modu Değişince Tüm Config Sıfırlanıyor ✅
- **Dosya:** `frontend/lib/providers/settings_provider.dart:63-73` ↔ `internal/webserver/handlers_flutter.go:667-685`, `app.go:1148-1151`
- **Sorun:** Kısmi JSON body (örn `{"engine_mode": "cpu"}`) tüm `LlamaConfig` struct'ını sıfırlıyor
- **Çözüm:** `UpdateLlamaConfig`'de tüm struct'ı replace etmek yerine sadece non-zero alanları merge et
- **İşlemler:**
  - [x] `app.go` — `UpdateLlamaConfig`'de partial merge eklendi (her alan ayrı kontrol: sadece non-zero ise güncelle)
  - [x] Go derleme testi yapıldı

---

## [x] K3 — `/api/image` ile Keyfi Dosya Okuma ✅
- **Dosya:** `app.go:866-871` (GetImageBase64), `internal/webserver/handlers_flutter.go:214-226` (handleImage)
- **Sorun:** Path sanitization yok, `/etc/passwd`, `~/.ssh/id_rsa` okunabilir
- **Çözüm:** Çift katmanlı path doğrulama: Handler'da `..` ve absolute path reddi, `GetImageBase64`'te `data/` whitelist + symlink koruması
- **İşlemler:**
  - [x] `handlers_flutter.go` — `handleImage`'de Layer 1 path doğrulaması eklendi (`..` ve absolute path reddedilir)
  - [x] `app.go` — `GetImageBase64`'te Layer 2 whitelist kontrolü eklendi (`filepath.Abs` + `EvalSymlinks` + `data/` prefix)
  - [x] Go derleme testi yapıldı

---

## [x] K4 — Remote Access'te Auth Yok, CORS Açık ✅
- **Dosya:** `internal/webserver/server.go:65-176`, `app.go:155-157`, `app.go:1048-1063`
- **Sorun:** `0.0.0.0:<port>` binding, `Access-Control-Allow-Origin: *`, plaintext passphrase, oturum/token yok
- **Çözüm:** Remote access tamamen devre dışı bırakıldı. Flutter'da da özellik "under construction" olduğu için ileride düzgün implementation ile gelmek üzere kapatıldı.
- **İşlemler:**
  - [x] `server.go` — `Start()` anında hata döndürür hale getirildi
  - [x] `app.go` — `SetRemoteAccess` enable etmeye çalışınca hata döndürür
  - [x] `app.go` — startup'ta remote server başlatılmaz
  - [x] `server.go` — CORS `*` yerine `Origin` header'ını yankılar
  - [x] Go derleme testi yapıldı

---

## [ ] K5 — `a.client` Kilitsiz Yeniden Atanıyor
- **Dosya:** `app.go:1106`, `app.go:1123`, `app.go:1216`, `app.go:1228`
- **Sorun:** `a.client` ve `a.embeddingClient` mutex olmadan yeniden atanıyor → race condition, nil-pointer panic
- **Çözüm:** Tüm client okuma/yazmalarını `a.mu` ile koru
- **İşlemler:**
  - [ ] `app.go` — client atamalarını `a.mu.Lock()`/`Unlock()` içine al
  - [ ] Client okuyan yerlerde `a.mu.RLock()`/`RUnlock()` ekle
  - [ ] Go derleme testi yap

---

## [ ] K6 — `saveMemoryAsync` RLock→Lock Deadlock Riski
- **Dosya:** `app.go:1399-1421`
- **Sorun:** RLock alıp içinde Lock almaya çalışan goroutine → kilitlenme riski
- **Çözüm:** Channel-based worker goroutine kullan
- **İşlemler:**
  - [ ] `app.go` — `saveMemoryAsync`'i channel-based worker pattern'e çevir
  - [ ] Go derleme testi yap

---

## [ ] K7 — Mesaj Başına AnimationController (UI Jank)
- **Dosya:** `frontend/lib/widgets/chat_message_list.dart:92-99`
- **Sorun:** Her mesaj balonu kendi AnimationController'ını oluşturuyor → 50+ mesajda severe jank
- **Çözüm:** Entry animasyonlarını kaldır veya shared animation controller kullan
- **İşlemler:**
  - [ ] `chat_message_list.dart` — AnimationController'ları kaldır/azalt
  - [ ] Flutter derleme testi yap
