# Memo — Stable Release Engel Listesi + RAG Yol Haritası

> **Hedef:** Windows / Linux / macOS masaüstü + mobil için kararlı, güvenilir, cross-platform bir v3.1.0 sürümü.
> Bu dosya önce stable blocker’ları, ardından mevcut RAG bellek yol haritasını içerir.
> Her değişiklikten sonra `go test ./... -race`, `go build ./...` ve `flutter analyze lib/` çalıştırılır.
> Flutter SDK konumu: `/home/bugra/Belgeler/flutter/bin`

---

## 🚨 Stable Release — Acil Düzeltme Planı

### Faz A — Backend Kritik (Önce Bunlar)

- [x] **A1. Cloud sync canlı SQLite yedeğine WAL dosyalarını dahil et**
  - Dosya: `internal/cloudsync/sync_manager.go`
  - `memory.db` + `memory.db-wal` + `memory.db-shm` ve `mood.db` + sidecar’lar arşivleniyor.
  - Test: `TestArchiveIncludesSQLiteWALSidecars`, `TestRestoreExtractsWALSidecars` eklendi.
  - Verification: `go test ./... -race` ✅

- [x] **A2. Provider router’da context cancellation’ı failure sayma**
  - Dosya: `internal/provider/router.go`
  - `ctx.Err()` veya `errors.Is(err, context.Canceled|DeadlineExceeded)` durumunda `recordFailure` çağrılmıyor.
  - Test: `TestRouterContextCancellationNotRecordedAsFailure` eklendi.
  - Verification: `go test ./... -race` ✅

- [x] **A3. `database.DB.ExecContext` bypass’ını kaldır / write-loop’a yönlendir**
  - Dosya: `internal/database/sqlite.go`
  - `ExecContext` artık `Write()` üzerinden serialized write-loop’a yönleniyor.
  - Verification: `go test ./... -race` ✅

- [x] **A4. Agent proje yolunun session’a persist edilmesi**
  - Dosya: `internal/sessions/sessions.go`
  - `NewAgentChat`’te `ProjectPath` set edildikten sonra `save()` çağrılıyor.
  - Test: `TestNewAgentChatPersistsProjectPath` eklendi (restart sonrası path korunuyor).
  - Verification: `go test ./... -race` ✅

- [x] **A5. Memory store re-init’te nil pencere / hata durumunu yönet**
  - Dosya: `internal/app/memory.go`
  - Eski store geçici değişkende tutulup `a.store = nil` yapılıyor; `NewStore` başarısız olursa eski store geri yükleniyor.
  - Verification: `go test ./... -race` ✅

### Faz B — Frontend Kritik

- [x] **B1. Flutter desktop backend URL’sini yapılandırılabilir / discoverable yap**
  - Dosya: `frontend/lib/core/api_client.dart`, `frontend/lib/providers/chat_provider.dart`, `frontend/lib/providers/settings_provider.dart`, `frontend/lib/widgets/settings/tabs/remote_access_tab.dart`
  - `MemoApiClient.baseUrl` artık `required` parametre; `apiClientProvider` `SharedPreferences`’den okuyor.
  - `backendUrlProvider` eklendi (StateNotifier, prefs’e kaydeder).
  - Remote Access ayarlarında "Backend Server URL" bölümü eklendi.
  - Verification: `go test ./... -race` ✅, `flutter analyze` ✅, `flutter test` ✅

- [x] **B2. Windows path separator hatalarını `package:path` ile düzelt**
  - Dosyalar: `chat_provider.dart`, `chat_input.dart`, `chat_screen.dart`, `agent_screen.dart`, `model_store_screen.dart`, `engine_strip.dart`, `recording_provider.dart`
  - Tüm `split('/').last` → `p.basename()`, `'$path/...'` → `p.join()`.
  - Verification: `flutter analyze` ✅, `flutter test` ✅

- [x] **B3. `exit(42)` yerine graceful shutdown akışı**
  - Dosyalar: `internal/webserver/bridge.go`, `internal/webserver/handlers_flutter.go`, `internal/app/app.go`, `frontend/lib/core/api_client.dart`, `frontend/lib/widgets/settings/tabs/backup_restore_tab.dart`
  - `FullBridge`'e `Shutdown(ctx)` eklendi; `POST /api/shutdown` handler'ı yazıldı; Flutter `shutdown()` methodu önce backend'i temizliyor sonra `exit(42)`.
  - Verification: `go test ./... -race` ✅, `flutter analyze` ✅, `flutter test` ✅

- [x] **B4. Remote access port’unu sabit 8090’den kurtar**
  - Dosyalar: `internal/webserver/server.go`, `frontend/lib/core/api_client.dart`, `frontend/lib/widgets/settings/tabs/remote_access_tab.dart`
  - `/api/status` artık `port` ve `listen_addr` dönüyor; Fluent `getListenPort()` ile okuyor; `_listenPort` state field ile tüm `setTailscaleMode`/`setRemoteAccess` çağrılarında kullanılıyor.
  - Verification: `go test ./... -race` ✅, `flutter analyze` ✅

### Faz C — Mimari (Polling & Goroutine Leak’ler)

- [x] **C1. Sonsuz polling döngülerini durdur**
  - Dosyalar: `chat_provider.dart`, `models_provider.dart`, `mood_provider.dart`, `version_provider.dart`, `calendar_screen.dart`, `whatsapp_screen.dart`, `whatsapp_provider.dart`
  - Çözüm: `IndexedStack` mount’lu tuttuğu için `VisibilityDetector` / `AppLifecycleListener` / `ref.onDispose` ile durdur.

- [x] **C2. `UpdateProvider` health-check goroutine leak’ini önle**
  - Dosya: `internal/app/providers.go`
  - Eski router iptal edilmeli veya tek health goroutine yönetilmeli.

- [x] **C3. Claude / Gemini / yerel stream HTTP client’larına `ResponseHeaderTimeout` ekle**
  - Dosyalar: `internal/provider/claude.go`, `internal/provider/gemini.go`, `internal/api/client.go`

### Faz D — Cross-Platform / Paketleme

- [x] **D1. macOS build pipeline ekle**
  - Dosya: `build_releases.sh`
  - `darwin` kolu + `.app` / `.dmg` veya `.zip` paketleme.

- [x] **D2. Build script’lerdeki zorla öldürmeleri graceful hale getir**
  - Dosyalar: `build_releases.sh`, `build_releases.bat`
  - `pkill -9` / `taskkill /F` yerine önce SIGTERM/uygulama shutdown, sonra force kill fallback.

- [x] **D3. Windows batch runner backend PID takibi**
  - Dosya: `build_releases.bat` içinde üretilen `run_memo.bat`
  - Frontend crash olursa backend orphan kalmaması için PID kaydet ve cleanup yap.

### Faz E — Orta Öncelikli (Faz A-D tamamlandıktan sonra)

- [x] **E1. Cloud sync şifreleme fallback’ini güçlendir**
  - Dosya: `internal/cloudsync/crypto.go`
  - Passphrase yoksa kullanıcıya zorla; hardware ID fallback kaldır veya en azından uyarı ver.

- [x] **E2. Provider API key encryption Windows ACL + fallback güvenliği**
  - Dosya: `internal/provider/config.go`
  - Windows’ta ACL ayarı; `/etc/machine-id` fallback’i kaldır veya alternatif güvenli kaynak kullan.

- [x] **E3. Calendar reminder claim’i atomik yap**
  - Dosya: `internal/calendar/store.go`
  - `SELECT ... FOR UPDATE` veya `UPDATE ... WHERE reminder_sent=0` + `RETURNING` ile çift uyarı önle.

- [x] **E4. Calendar store doğrudan SQLite yazma yerine `DB.Write` kullan**
  - Dosya: `internal/calendar/store.go`

- [x] **E5. Memory save embedding çağrısını `storeMu` dışına al**
  - Dosya: `internal/app/memory.go`
  - Embedding ağır I/O yaparken memory okuma/yazma bloklanmasın.

- [x] **E6. Background migration shutdown gecikmesini önle**
  - Dosya: `internal/memory/store.go`
  - Migration context’ini kısalt veya shutdown sırasında abandon et.

- [x] **E7. WhatsApp contact import’u live message save’den ayır**
  - Dosya: `internal/whatsapp/client.go`
  - Toplu import background’da veya kuyrukta yapılmalı.

### Faz F — Mobil Frontend Parity

- [x] **F1. Mobil API client default URL’ini kaldır / discovery ekle**
  - Dosyalar: `mobile/lib/core/api_client.dart`, `mobile/lib/providers/connection_provider.dart`, `mobile/lib/screens/connect_screen.dart`
  - `MemoApiClient` default URL `’’` oldu. `ConnectionState.baseUrl` default `’’`. `ConnectionNotifier.discoverUrl()` — `NetworkInterface.list()` ile subnet bulup 1-254 arasını paralel tarıyor. Connect ekranında "TARA" butonu eklendi.

- [x] **F2. Mobil client’a eksik backend endpointlerini ekle**
  - Dosya: `mobile/lib/core/api_client.dart`
  - Eklenenler: chat extras (rename, activeChatId, generateTitle, exportChat, deleteMessage, updateMessage, createAgentChat), memory (enabled, settings, saveExplicit, deleteExplicit, export, import, stats, search), system prompt, incognito, mood, version, shutdown, agent extras (undoEdit, autoPermission, permissions), WhatsApp (status, start, stop, logout, send, search, chats, messages, stats, chatMode, stream), proactive (settings, patterns, suggestion), self-interest, system management. Yeni model class’lar: `MemoryStats`, `MemorySearchResult`.

---

## 📋 Günlük Çalışma Kuralı

1. Her gün en fazla 1-2 Faz maddesi seç.
2. Her değişiklik için test yaz veya mevcut testleri güncelle.
3. `go test ./... -race` ve `go build ./...` **PASS** olmadan sonraki maddeye geçme.
4. Flutter dokunduysa `flutter analyze lib/` **PASS** olmalı.
5. Bu dosyadaki checkbox’ları gerçekten tamamlandıkça işaretle.
6. Commit mesajları Conventional Commits formatında: `fix(backend): ...`, `fix(frontend): ...`

---

# Memo — RAG Bellek Sistemi: Tam Yol Haritası

> **Tek vaat:** "Senin hafızan olan AI." Her sohbet, kullanıcıyı daha iyi tanıyan bir fırsattır.
> Bellekler otomatik oluşur, önemli olanlar öne çıkar, gereksizler unutulur.

---

## Mevcut Durum (2026-06-25) — **~%90 Hazır**

### Tamamlananlar ✅

**Faz 0 — Kritik Bug Düzeltmeleri**
- [x] FTS5 phrase search bug → token search'e düzeltildi
- [x] `minSimilarity` RRF sonrası uygulanmıyor → minRRFScore filtresi eklendi
- [x] Orta chunk'larda `assist_msg` boş → tüm chunk'larda saklanıyor
- [x] `candidateK` sabit → `min(max(topK*5, 20), 100)` dinamik hesap

**Faz 1 — Retrieval Kalitesi**
- [x] Sorgu zenginleştirme (son 3 user mesajı + mevcut mesaj)
- [x] Metadata alanları (session_id, importance, tags, source, retrieve_count, pending_deletion)
- [x] Retrieve sayacı (incrementRetrieveCounts goroutine)
- [x] Importance boost (×0.9 – ×1.3, re-sort)
- [x] Explicit bellek komutları (`/remember`, `/forget`) — Flutter + backend
- [x] Bellek prompt formatı yükseltme (relevance%, importance label, source, age)
- [x] `SaveExplicit` / `DeleteByContent` / `Export` / `Import` — Store + App + Bridge + HTTP

**Faz 2 — Bellek Zekası**
- [x] Importance scoring motoru (`runImportanceDecay`, 5-min delay, 24h cycle)
- [x] Importance decay + boost kuralları (`applyImportanceRules`)
- [x] Soft delete (`pending_deletion`, 7 gün grace → hard delete)
- [x] `MarkStaleForDeletion` (importance≤2, retrieve=0, >180 gün, non-explicit)
- [x] `PurgePendingDeletions` (>187 gün)
- [x] Multi-query retrieval (`expandQuery`, ilk 5 kelime topic anchor, ikinci embed+search, RRF merge)

**Faz 3 — Production & Ölçek**
- [x] 5 performans indeksi (timestamp, importance, source, retrieve_count, pending_deletion)
- [x] Bellek Export / Import (JSON v2, UUID dedup INSERT OR IGNORE, re-embed)
- [x] Tarih ve etiket filtreleme (`FilteredSearch` + `sqlFilteredFallback` + HTTP endpoint)
- [x] Bellek Analytics Dashboard (Flutter — stat chips + most accessed, GET /api/memory/stats)
- [x] `Stats()` tek sorguya indirgendi (SUM CASE WHEN), rows.Err() kontrolü eklendi

**Güvenlik / Doğruluk Düzeltmeleri (code review)**
- [x] `_loadStats` mounted check eklendi (Flutter crash fix)
- [x] Stats fetch hataları kullanıcıya SnackBar ile gösteriliyor
- [x] Handler `since` paramı RFC3339 doğrulaması + HTTP 400
- [x] `FilteredSearch` date filter için SQL fallback (semantik pass boş döndüğünde)

---

## Pipeline (Mevcut)

```
[son 3 user tur + userMsg]
  → embed (+ expandQuery ikinci embed uzun sorgularda)
  → vecSearch (+ goSearch fallback)
  → multi-query RRF merge
  → ftsSearch
  → RRF (vec + fts)
  → minRRFScore filtresi
  → importance boost (×0.9–×1.3)
  → re-sort
  → top-K
  → [Memory N | relevance=XX% | importance=LABEL | source=SRC | AGE]
  → prompt
```

---

## Kalan Görevler

### ⬜ 3.1 Embedding Auto-Setup

Kullanıcı "Belleği Etkinleştir" toggle'ına bastığında:

1. `nomic-embed-text-v1.5.Q4_K_M.gguf` otomatik indir
2. `llamaEmbedServer` otomatik başlat
3. Store initialize et
4. "Hazır" bildirimi gönder

Kullanıcıdan ek kurulum istenmez. Şu an embedding ayrı yapılandırma gerektiriyor.

### ⬜ 2.2 Bellek Birleştirme (Consolidation) — Ertelenmiş

Kosinüs benzerliği > 0.92 olan çiftleri LLM ile birleştir. LLM bağımlılığı nedeniyle şimdilik ertelendi.

---

## Başarı Kriterleri

| Kriter | Şu an | Hedef |
|---|---|---|
| "3 ay önceki oteli hatırla" | ✅ FilteredSearch + SQL fallback | ✅ Tam |
| "Buna ne demiştik?" | ✅ Query enrichment (son 3 tur) | ✅ Tam |
| `/remember` komutları | ✅ | ✅ Tam |
| 10K bellekte < 500ms | ✅ 5 indeks + tek sorgu Stats | ✅ Tam |
| "Neden hatırladın?" | ✅ matchType + debug UI | ✅ Tam |
| Phrase search düzgün | ✅ Token search | ✅ Tam |
| Kalite filtresi | ✅ minRRFScore + importance boost | ✅ Tam |
| Bellek analytics | ✅ Flutter dashboard | ✅ Tam |
| Export / Import | ✅ JSON v2, UUID dedup | ✅ Tam |
| Tek toggle kurulum | ⬜ Embedding hâlâ manuel | ⬜ Faz 3.1 ile tam |
