# Handoff — 2026-06-26

## Oturum Özeti

Bu oturumda Faz C, D ve E tamamen tamamlandı. Tüm değişiklikler stage'de bekliyor (commit atılmadı — kullanıcı kendisi atacak).

---

## Yapılan Değişiklikler

### Faz C — Polling & Goroutine Leak'ler

**C1 — Sonsuz polling döngülerini durdur**

- `frontend/lib/screens/app_shell.dart`: `activeTabProvider = StateProvider<int>` eklendi, `_handleTabChange`'de güncelleniyor.
- `frontend/lib/screens/calendar_screen.dart`: Timer artık sadece tab 4 aktifken çalışıyor (`ref.listen(activeTabProvider, ...)`). `_startRefreshTimer()` metodu eklendi.
- `frontend/lib/screens/whatsapp_screen.dart`: `_msgTimer` tab 3'ten çıkınca iptal ediliyor, dönünce resume ediyor.
- `frontend/lib/providers/models_provider.dart`: `modelStatusProvider` / `embeddingStatusProvider` 5s → 30s, `downloadProgressProvider` cancellation flag eklendi.
- `frontend/lib/providers/mood_provider.dart`: `moodScoreProvider` / `moodEnabledProvider` 5s → 10s.
- `frontend/lib/providers/chat_provider.dart`: `connectionStatusProvider` `while(true)` loop'una `ref.onDispose` cancellation flag eklendi.

**C2 — UpdateProvider health-check goroutine leak**

- `internal/app/app.go`: `healthCheckCancel context.CancelFunc` field eklendi.
- `internal/app/providers.go`: `UpdateProvider` her çağrıldığında önceki health-check goroutine iptal ediliyor (`hcancel()`), yeni router için yeni context açılıyor. `context` import eklendi.

**C3 — HTTP stream client'lara ResponseHeaderTimeout**

- `internal/provider/claude.go`: `client` ve `streamCl` transport'larına `ResponseHeaderTimeout: 30s` eklendi.
- `internal/provider/gemini.go`: Aynı.
- `internal/api/client.go`: Shared transport'a `ResponseHeaderTimeout: 30s` eklendi.

---

### Faz D — Cross-Platform / Paketleme

**D1 — macOS build pipeline**

- `build_releases.sh`: `darwin*` OS detection eklendi. macOS bölümü:
  - `go build` darwin için
  - `flutter build macos --release`
  - `Memo.app` bundle kopyalanıyor
  - macOS runner script (`run_memo.sh`) — graceful kill + cleanup
  - `.zip` çıktısı üretiliyor

**D2 — Graceful kill (Linux + macOS runner)**

- `build_releases.sh` içindeki `run_memo.sh` template: `pkill -9` yerine `_graceful_kill()` fonksiyonu — önce SIGTERM, 5s bekle, sonra SIGKILL. Backend cleanup'ta önce shutdown API çağrılıyor, sonra sinyal.

**D2+D3 — Graceful kill + PID takibi (Windows runner)**

- `build_releases.sh` Windows bölümündeki `run_memo.bat` template ve `build_releases.bat` içindeki runner:
  - Shutdown API'si (`POST /api/shutdown`) önce deneniyor, sonra force kill
  - Backend PowerShell `Start-Process -PassThru` ile başlatılıyor, PID yakalanıyor
  - Cleanup'ta `taskkill /F /PID %BACKEND_PID%` (isme göre değil PID'e göre) — başka Memo instance'larını öldürmüyor

---

### Faz E — Orta Öncelikli

**E1 — Cloud sync şifreleme fallback güçlendirmesi**

- `internal/cloudsync/crypto.go`: `encrypt()` ve `decrypt()` boş passphrase'de artık `hardwareID()` kullanmıyor, error döndürüyor.
- `internal/cloudsync/sync_manager.go`: Passphrase yokken açık WARN logu — "machine-specific key, başka makinede restore edilemez".
- `internal/cloudsync/crypto_test.go`: `TestEncryptDecryptEmptyPassphrase` yeni davranışı test ediyor.

**E2 — Provider API key encryption güvenliği**

- `internal/provider/config.go`: `defaultMachineKey()` artık `/etc/machine-id`, Windows MachineGuid, macOS IOPlatformUUID kullanmıyor (tahmin edilebilir, secret değil). Her zaman `crypto/rand` ile 32-byte random key üretiyor ve `machine.key`'e yazıyor. Windows'ta `icacls` ile ACL kısıtlaması uygulanıyor. `"strings"` import kaldırıldı.

**E3 — Calendar reminder atomik claim**

- `internal/calendar/store.go`: `ClaimPendingReminders` — `BEGIN TX + SELECT + loop UPDATE + COMMIT` yerine tek `UPDATE events SET reminder_sent=1 WHERE ... RETURNING ...` statement. Race condition tamamen elimine edildi.

**E4 — Calendar store DB.Write'a geçiş**

- `internal/calendar/store.go`: `*sql.DB` → `*database.DB`. `NewStore` artık `database.Open()` kullanıyor. Tüm write'lar write-loop'tan geçiyor. `ClaimPendingReminders` içindeki `UPDATE RETURNING` `db.Write(tx.QueryContext(...))` ile yapılıyor. Import: `os` kaldırıldı, `database` eklendi, `_ "go-sqlite3"` kaldırıldı.

**E5 — Memory save embedding dışarıda**

- `internal/app/memory.go`: `saveMemorySync` — `storeMu.Lock()` artık sadece store pointer'ını almak için tutuluyor, hemen `RUnlock`. `store.SaveInteraction()` (embedding I/O dahil) kilit dışında çalışıyor. Store kendi `db.Write` ile thread-safe.

**E6 — Background migration shutdown gecikmesi**

- `internal/memory/store.go`: FTS ve vec migration goroutineleri artık `s.stopCh` kapanınca `migCancel()` çağırıyor. `Close()` → anında abort, önceden 60-120s bekliyordu.

**E7 — WhatsApp history sync ayrıştırma**

- `internal/whatsapp/client.go`: `handleEvent` içinde `c.handleHistorySync(v)` → `go c.handleHistorySync(v)`. Binlerce mesajlık bulk import artık event handler'ı bloklamıyor.

---

## Test Durumu

```bash
go test ./... -race -count=1   # tüm paketler PASS
flutter analyze lib/           # 0 error, 0 warning (info uyarıları önceden var)
go build ./...                 # OK
bash -n build_releases.sh      # syntax OK
```

---

## Commit Edilmemiş Dosyalar (stage'de bekliyor)

```
frontend/lib/providers/chat_provider.dart
frontend/lib/providers/models_provider.dart
frontend/lib/providers/mood_provider.dart
frontend/lib/screens/app_shell.dart
frontend/lib/screens/calendar_screen.dart
frontend/lib/screens/whatsapp_screen.dart
internal/api/client.go
internal/app/app.go
internal/app/memory.go
internal/app/providers.go
internal/calendar/store.go
internal/cloudsync/crypto.go
internal/cloudsync/crypto_test.go
internal/cloudsync/sync_manager.go
internal/memory/store.go
internal/provider/claude.go
internal/provider/config.go
internal/provider/gemini.go
internal/whatsapp/client.go
build_releases.sh
build_releases.bat
yapılacaklar.md
```

---

## Kalan Görevler

### Faz F — Mobil Frontend Parity

- [ ] **F1. Mobil API client default URL'ini kaldır / discovery ekle**
  - Dosya: `mobile/lib/core/api_client.dart`

- [ ] **F2. Mobil client'a eksik backend endpointlerini ekle**
  - Memory, model store, whatsapp, calendar, agent, skills vb.

### RAG 3.1 — Embedding Auto-Setup

- Kullanıcı "Belleği Etkinleştir" toggle'ına bastığında `nomic-embed` otomatik inip başlamalı.
- `EmbeddingModelRepo` zaten var, sadece orkestrasyon yazılmadı.

### RAG 2.2 — Bellek Birleştirme (Ertelenmiş)

- Kosinüs benzerliği > 0.92 çiftleri LLM ile birleştir. LLM bağımlılığı nedeniyle ertelendi.
