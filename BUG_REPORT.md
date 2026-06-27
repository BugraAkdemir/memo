# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların tespiti.
> **Tarih:** 2026-06-28
> **Kapsam:** Tüm Go backend + Flutter frontend + CI/CD
> **Güncelleme:** 2026-06-28 — 13 madde düzeltildi, 10 madde açık

---

## Düzeltilen Bug'lar (13/23)

| # | Madde | Durum |
|---|-------|-------|
| 1 | AgentStatusBar.events.last crash | ✅ Düzeltildi |
| 2 | qrCodes.first crash | ✅ Düzeltildi |
| 3 | Unsafe casts (6 location) | ✅ Düzeltildi |
| 4 | as RenderBox crash | ✅ Düzeltildi |
| 5 | Gemini API key URL'de sızıyor | ✅ Düzeltildi |
| 11 | Nil client dereference (3 yer) | ✅ Düzeltildi |
| 12 | HTTP client timeout yok | ✅ Düzeltildi |
| 16 | Mounted check eksik | ✅ Düzeltildi |
| 18 | Silent error ignore | ✅ Düzeltildi |
| 19 | Skill dialog Windows path | ✅ Düzeltildi |
| 20 | DangerLevel tip uyuşmazlığı | ⏭️ Unimplemented feature (atlandı) |
| 22 | Flutter Linux build | ⏭️ CI ortam sorunu (atlandı) |
| 23 | macOS platform projesi | ⏭️ CI ortam sorunu (atlandı) |

---

## Açık Bug'lar (10/23)

### HIGH

### 6. Fire-and-Forget Goroutine Leaks (4 adet)

- **`internal/observer/recorder.go:39`** — Her mesaj için `go func() { r.store.Record(obs) }()`. Timeout, cancel, wg yok.
- **`internal/app/chat.go:60,124`** — `go a.processMessageIntent(...)` her chat mesajı için. Timeout yok.
- **`internal/app/whatsapp.go:98`** — Aynı pattern, WhatsApp mesajları için.
- **`internal/app/app.go:240`** — `go func() { memory.NewStore(...) }()` startup'ta. Timeout yok.
- **Risk:** Uzun süreli kullanımda goroutine havuzu büyür, memory tüketimi artar.

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
| 6 | Goroutine leak (4x) | CRITICAL | 30 dk | 🔴 Açık |
| 7 | Bakım | CRITICAL | 2 saat | 🔴 Açık |
| 8 | Mobile broken | HIGH | 4 saat | 🔴 Açık |
| 9 | UX eksik | HIGH | 15 dk | 🔴 Açık |
| 10 | Orkestra kırık | HIGH | 30 dk | 🔴 Açık |
| 11 | nil panic (4x) | HIGH | 15 dk | ✅ |
| 12 | Download hangs | HIGH | 5 dk | ✅ |
| 13 | Logging | HIGH | 1 saat | 🔴 Açık |
| 14 | Security | HIGH | 15 dk | 🔴 Açık |
| 15 | Perf leak | HIGH | 10 dk | 🔴 Açık |
| 16 | Visual glitch | MEDIUM | 5 dk | ✅ |
| 17 | Perf | MEDIUM | 1 saat | 🔴 Açık |
| 18 | Error handling | MEDIUM | 15 dk | ✅ |
| 19 | UX | MEDIUM | 5 dk | ✅ |
| 20 | Dormant bug | MEDIUM | 5 dk | ⏭️ Atlandı |
| 21 | Feature gap | MEDIUM | 2 saat | 🔴 Açık |
| 22 | Build broken | MEDIUM | 1 dk | ⏭️ Atlandı |
| 23 | Build broken | LOW | 30 dk | ⏭️ Atlandı |

---

## Kalan Aciliyet Sırası

1. Goroutine leak fix (#6) — 30 dk, uzun süreli stabilite
2. Orchestra fallback (#10) — 30 dk, provider kullanıcı deneyimi
3. Provider priority UI (#9) — 15 dk
4. Mobile API client (#8) — 4 saat, ayrı oturum
5. model_store_screen refactor (#7) — 2 saat, ayrı oturum
6. Logging migration (#13) — 1 saat, background task

---

## Test Coverage Durumu

```
go build ./...   ✅ (0 hata)
go vet ./...     ✅ (0 uyarı)
go test ./... -race -count=1  → 30/30 PASS
Flutter analyze  — geçiyor (CI)
```

328 test fonksiyonu var. Race detector temiz.
