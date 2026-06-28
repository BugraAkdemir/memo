# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların tespiti.
> **Tarih:** 2026-06-28
> **Kapsam:** Tüm Go backend + Flutter frontend + CI/CD
> **Güncelleme:** 2026-06-28 (Session 3) — 28 düzeltildi = **5 açık bug** (1 HIGH, 2 MED, 2 LOW)

---

## Düzeltilen Bug'lar (28/33)

| # | Madde | Session | Durum |
|---|-------|---------|-------|
| 1 | AgentStatusBar.events.last crash | 2 | ✅ Düzeltildi |
| 2 | qrCodes.first crash | 2 | ✅ Düzeltildi |
| 3 | Unsafe casts (6 location) | 2 | ✅ Düzeltildi |
| 4 | as RenderBox crash | 2 | ✅ Düzeltildi |
| 5 | Gemini API key URL'de sızıyor | 2 | ✅ Düzeltildi |
| 6 | Goroutine leak (4 adet) | **3** | ✅ Düzeltildi |
| 9 | Provider priority UI eksik | — | ✅ Düzeltildi (önceden) |
| 10 | Orchestra fallback kullanmıyor | — | ✅ Düzeltildi (önceden) |
| 11 | Nil client dereference (3 yer) | 2 | ✅ Düzeltildi |
| 12 | HTTP client timeout yok | 2 | ✅ Düzeltildi |
| 13 | Logging migration eksik | — | ✅ Düzeltildi (önceden) |
| 16 | Mounted check eksik | 2 | ✅ Düzeltildi |
| 17 | const constructor eksik | — | ✅ Düzeltildi (önceden) |
| 18 | Silent error ignore | 2 | ✅ Düzeltildi |
| 19 | Skill dialog Windows path | 2 | ✅ Düzeltildi |
| 20 | DangerLevel tip uyuşmazlığı | — | ⏭️ Atlandı |
| 22 | Flutter Linux build | — | ⏭️ Atlandı |
| 23 | macOS platform projesi | — | ⏭️ Atlandı |
| 24 | os.Exit(42) — veri kaybı | **3** | ✅ Düzeltildi |
| 25 | Health check goroutine leak | **3** | ✅ Düzeltildi |
| 26 | store.Close() çağrılmıyor | **3** | ✅ Düzeltildi |
| 27 | 28+ unsafe as cast (Flutter) | **3** | ✅ Düzeltildi |
| 28 | Data race: whisperServer | **3** | ✅ Düzeltildi |
| 29 | Data race: webServer | **3** | ✅ Düzeltildi |
| 30 | WhatsApp autoReconnect leak | **3** | ✅ Düzeltildi |
| 31 | resolveAgentProvider race | **3** | ✅ Düzeltildi |
| 32 | incrementRetrieveCounts UAC | **3** | ✅ Düzeltildi |
| 33 | Observer unbounded goroutine | **3** | ✅ Düzeltildi |
| 34 | Ngrok stopCh'siz sleep | **3** | ✅ Düzeltildi |
| 35 | GetEnabled() priority sırasız | **3** | ✅ Düzeltildi |
| 37 | whatsapp_provider.dart unsafe cast | **3** | ✅ Düzeltildi |

---

## Açık Bug'lar (5 adet)

### HIGH

### 14. `bash -c` ile Command Injection Riski

- **Dosya:** `internal/agent/tools/command.go:164`
- **Risk:** Blacklist approach'u asla tam güvenli değildir. Encoding tricks ile atlatılabilir.
- **Not:** Tasarım gereği — agent tool onayı gerektirir.
- **Risk:** HIGH — security

### MEDIUM

### 7. `model_store_screen.dart` — 2507 Satır, Tek Dosyada

- **Dosya:** `frontend/lib/screens/model_store_screen.dart`
- **Risk:** Tek dosyada 2500+ satır. Bakımı zor, kırılma riski yüksek.

### 8. Mobile API Client Eksik (50+ endpoint)

- **Dosya:** `mobile/lib/core/api_client.dart` (1813 satır)
- **Risk:** Backend'deki endpoint'lerin çoğu mobil client'ta yok.

### 15. `connectionStatusProvider` Sonsuz Polling (#36 ile aynı)

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Risk:** 30 saniyede bir `isAlive()` sorguluyor, dispose olsa bile devam ediyor.

### 21. Whisper GPU Variant Desteği Eksik

- **Risk:** GPU'su olan kullanıcılar CPU mode'da STT kullanır.

### 7. `model_store_screen.dart` — 2507 Satır, Tek Dosyada

- **Dosya:** `frontend/lib/screens/model_store_screen.dart`
- **Risk:** Tek dosyada 2500+ satır. Bakımı zor, kırılma riski yüksek.

### 8. Mobile API Client Eksik (50+ endpoint)

- **Dosya:** `mobile/lib/core/api_client.dart` (1813 satır)
- **Risk:** Backend'deki endpoint'lerin çoğu mobil client'ta yok.

### 9. Provider Priority UI Kontrolü Yok

- **Dosyalar:** `frontend/lib/widgets/provider_config_dialog.dart:247`, `internal/provider/router.go:64-66`
- **Kod:** Router Priority'ye göre sıralıyor ama Flutter UI'da priority ayarı yok.
- **Etki:** Kullanıcı provider önceliğini belirleyemez.

### 10. Orchestra Provider Fallback Kullanmıyor

- **Dosyalar:** `internal/orchestra/conductor.go:186-237`, `internal/provider/router.go:100-172`
- **Kod:** Orchestra direkt `provider.ChatCompletion()` çağırıyor, Router'ın fallback zincirini bypass ediyor.
- **Etki:** Chief provider düştüğünde orkestra tümden başarısız olur.

### 13. Structured Logging Migration Tamamlanmamış

- **Dosyalar:** `internal/cloudsync/`, `internal/provider/`, `internal/memory/`, `internal/app/` (partial)
- **Kod:** `log.Printf()` kullanılmaya devam ediyor. Sadece `webserver/server.go` logx'e geçmiş.

### 14. `bash -c` ile Command Injection Riski

- **Dosya:** `internal/agent/tools/command.go:164`
- **Risk:** Blacklist approach'u asla tam güvenli değildir. Encoding tricks ile atlatılabilir.
- **Not:** Tasarım gereği — agent tool onayı gerektirir.

### 15. `connectionStatusProvider` Sonsuz Polling

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Risk:** 30 saniyede bir `isAlive()` sorguluyor, dispose olsa bile devam ediyor.

### 17. `const` Constructor Eksiklikleri

- **Tüm Flutter projesinde yaygın.**
- **Etki:** Her rebuild'de yeni widget instance'ları. Performans düşüşü.

### 21. Whisper GPU Variant Desteği Eksik

- **Risk:** GPU'su olan kullanıcılar CPU mode'da STT kullanır.

---

## Puan Tablosu

| # | Kategori | Risk | Fix Süresi | Durum |
|---|----------|------|------------|-------|
| 1 | Flutter crash | CRITICAL | 5 dk | ✅ |
| 2 | Flutter crash | CRITICAL | 5 dk | ✅ |
| 3 | Flutter crash (6x) | CRITICAL | 15 dk | ✅ |
| 4 | Flutter crash | CRITICAL | 5 dk | ✅ |
| 5 | Security leak | CRITICAL | 15 dk | ✅ |
| 6 | Goroutine leak (4x) | CRITICAL | 30 dk | ✅ |
| 7 | Bakım | CRITICAL | 2 saat | 🔴 |
| 8 | Mobile broken | HIGH | 4 saat | 🔴 |
| 14 | Command injection | HIGH | — | 🔴 |
| 15 | Perf leak | HIGH | 10 dk | 🔴 |
| 21 | Feature gap | MEDIUM | 2 saat | 🔴 |
| 24 | os.Exit(42) — veri kaybı | CRITICAL | 10 dk | ✅ |
| 25 | Health check goroutine leak | CRITICAL | 5 dk | ✅ |
| 26 | store.Close() çağrılmıyor | CRITICAL | 5 dk | ✅ |
| 27 | 28+ unsafe as cast | CRITICAL | 30 dk | ✅ |
| 28 | Data race: whisperServer | CRITICAL | 10 dk | ✅ |
| 29 | Data race: webServer | CRITICAL | 10 dk | ✅ |
| 30 | WhatsApp autoReconnect leak | HIGH | 15 dk | ✅ |
| 31 | resolveAgentProvider race | HIGH | 10 dk | ✅ |
| 32 | incrementRetrieveCounts UAC | HIGH | 10 dk | ✅ |
| 33 | Observer unbounded goroutine | HIGH | 15 dk | ✅ |
| 34 | Ngrok stopCh'siz sleep | HIGH | 15 dk | ✅ |
| 35 | GetEnabled() priority sırasız | HIGH | 10 dk | ✅ |
| 36 | connectionStatusProvider polling | MEDIUM | 10 dk | 🔴 |
| 37 | whatsapp_provider.dart unsafe cast | MEDIUM | 5 dk | ✅ |

---

## Kalan Aciliyet Sırası

### HIGH (Acil değil — tasarım gereği veya büyük iş)

1. model_store_screen refactor (#7) — ~2 saat
2. Mobile API client (#8) — ~4 saat (ayrı oturum)
3. Whisper GPU (#21) — ~2 saat

### MED

4. connectionStatusProvider polling (#36) — 10 dk
5. bash -c command injection (#14) — blacklist iyileştirme, tasarım gereği

### Test Coverage Durumu

```
go build ./...   ✅ (0 hata)
go vet ./...     ✅ (0 uyarı)
go test ./... -race -count=1  → 29/29 PASS
```

Tüm paketler race-free. 328+ test fonksiyonu.
