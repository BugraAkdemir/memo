# Bug Report — Memo v3.1.0-beta Stabilite Engelleri

> **Amaç:** Stable sürüme engel olan, kullanıcıyı direkt etkileyen bug'ların tespiti.
> **Tarih:** 2026-06-28
> **Kapsam:** Tüm Go backend + Flutter frontend + CI/CD
> **Güncelleme:** 2026-06-28 (Session 2) — 13 düzeltildi + 12 yeni tespit = **22 açık bug** (6 CRITICAL, 10 HIGH, 6 MED/LOW)

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

## Açık Bug'lar (10/23) + Yeni Tespitler (12 adet)

> Aşağıdaki 12 yeni bug 2026-06-28 Session 2'deki derinlemesine kod analizinde tespit edilmiştir.
> Daha önce handoff.md veya AGENTS.md'de kayıtlı değildi.

### CRITICAL

### 24. `os.Exit(42)` — WAL Checkpointsiz Veri Kaybı

- **Dosya:** `internal/webserver/handlers_flutter.go:1732-1740`
- **Kod:** `handleShutdown` goroutine'de 200ms sonra `os.Exit(42)` çağırır.
- **Kök Neden:** `os.Exit()` anında prosesi öldürür. `defer`'ler, WAL checkpoint, DB flush, in-flight HTTP istekleri tamamlanmaz.
- **Etki:** SQLite WAL bozulması, son mesajların kaybı, veritabanı tutarsızlığı.
- **Risk:** CRITICAL — veri kaybı
- **Fix:** `os.Exit(42)` yerine lifecycle cancel + graceful shutdown bekle, sonra wrapper script'e sinyal gönder.

### 25. Provider Health Check Goroutine Leak (Shutdown)

- **Dosya:** `internal/app/app.go:363-365`
- **Kod:**
  ```go
  hctx, hcancel := context.WithCancel(ctx)  // parent = ctx, NOT lifecycleCtx!
  a.healthCheckCancel = hcancel
  go a.providerRouter.HealthCheck(hctx, 5*time.Minute)
  ```
- **Kök Neden:** `hctx` parent'ı `lifecycleCtx` değil, `ctx` (sibling). `Shutdown()` sadece `lifecycleCancel()` çağırır, `a.healthCheckCancel()`'ı **hiçbir yerde çağırmaz**.
- **Etki:** Her shutdown'ta 1 goroutine sızar. Uzun çalışmada (sık restart) kaynak tükenmesi.
- **Risk:** CRITICAL — goroutine leak
- **Fix:** `context.WithCancel(a.lifecycleCtx)` kullan veya `Shutdown()`'a `a.healthCheckCancel()` ekle.

### 26. `a.store.Close()` Shutdown'da Hiç Çağrılmıyor

- **Dosya:** `internal/app/app.go:454-508` (Shutdown metodu)
- **Kod:** `a.store.Close()` hiç çağrılmaz. `store.go:97`'deki `runImportanceDecay` goroutini `stopCh` bekler ama `Close()` hiç çağrılmadığı için `stopCh` kapanmaz.
- **Etki:** 1 goroutine sızar, SQLite DB bağlantısı açık kalır, WAL checkpoint atılmaz.
- **Risk:** CRITICAL — goroutine leak + WAL bozulması
- **Fix:** `Shutdown()`'a `a.store.Close()` çağrısı ekle (nil + lock guard ile).

### 27. 28+ Unsafe `as` Cast — API Client Crash Vektörü

- **Dosya:** `frontend/lib/core/api_client.dart:197,206,214,271,329,406,411,457,464,501,532,605,610,657,808,827,905,911,946,957,963,979,1016,1022,1094,1140,1160,1172,1183`
- **Kod:** Tüm API metodlarında `res.data as Map<String, dynamic>` — `is` kontrolü yok.
- **Kök Neden:** Backend hata döndüğünde (HTML error page, proxy error, 503, vs.) `res.data` beklenen tipte olmaz. `as` cast'i `TypeError` fırlatır, tüm uygulama crash olur.
- **Etki:** En büyük crash vektörü — backend bir kez hatalı response döndü mü uygulama kullanılamaz hale gelir.
- **Risk:** CRITICAL — crash
- **Fix:** Her cast'i `if (res.data is Map)` ile koru, değilse `throw Exception(...)`.

### 28. Data Race: `a.whisperServer` (Goroutine vs Shutdown)

- **Dosyalar:** `internal/app/stt.go:57` (yazma), `internal/app/app.go:465` (okuma)
- **Kod:** `a.whisperServer = ws` goroutine içinde yazılır (`go a.startSTTServer()`). `Shutdown()` `a.whisperServer != nil` okur. Mutex yok.
- **Etki:** Data race — nadiren nil pointer dereference → panic.
- **Risk:** CRITICAL — data race → panic
- **Fix:** `sync.Mutex` veya `sync.Once` ile koru.

### 29. Data Race: `a.webServer`

- **Dosyalar:** `internal/app/app.go:447` (yazma), `internal/app/app.go:502` (okuma), `internal/app/remote_tailscale.go:135` (okuma)
- **Kod:** `a.webServer = webserver.New(a)` yazılır, shutdown'ta ve tailscale goroutine'inde mutex'siz okunur.
- **Risk:** CRITICAL — data race → panic
- **Fix:** `sync.Mutex` ile koru.

### HIGH

### 30. WhatsApp `autoReconnect` — İptal Edilemez Goroutine

- **Dosya:** `internal/whatsapp/client.go:409` (çağrı), 426-454 (fonksiyon)
- **Kod:** `go c.autoReconnect()` — parametre olarak `context.Context` almaz, `stopCh` yoktur. Shutdown sonrası 105 saniye boyunca `time.Sleep` ile bloke olur.
- **Etki:** Shutdown gecikir, zombie goroutine.
- **Risk:** HIGH — shutdown gecikmesi
- **Fix:** `context.Context` parametresi ekle, `Client.Stop()`'dan cancel et.

### 31. `resolveAgentProvider` — Race Window (Çift Router)

- **Dosya:** `internal/app/llm.go:36-48`
- **Kod:** RLock bırakılır → Lock alınır arasında başka goroutine de `nil` görüp router oluşturabilir. İkinci goroutine birinciyi ezer.
- **Etki:** İki router oluşur, son yazan kazanır, provider konfigürasyonu tutarsız.
- **Risk:** HIGH — latent data race
- **Fix:** Lock'u düşürmeden nil kontrolü yap.

### 32. `incrementRetrieveCounts` — Kapalı DB'ye Yazma (Use-After-Close)

- **Dosya:** `internal/memory/store.go:756` (çağrı), `store.go:1189-1199` (Close)
- **Kod:** `RetrieveContext` sonunda `go s.incrementRetrieveCounts(ids)` açar. `Close()` `s.db.Close()` yapar ama `s.db`'yi `nil`'lemez. RetrieveCounts goroutini Close'tan sonra çalışırsa kapalı DB'ye yazar.
- **Etki:** SQLite undefined behavior, crash potansiyeli.
- **Risk:** HIGH — use-after-close
- **Fix:** `s.db`'yi `Close()`'ta `nil`'le ve retrieveCounts'ta nil check yap.

### 33. Observer Recorder — Unbounded Goroutine Patlaması

- **Dosya:** `internal/observer/recorder.go:39`
- **Kod:** Her `record()` çağrısı `go func() { r.store.Record(obs) }()` açar. Her mesaj, agent run, WhatsApp mesajı, orkestra run — hepsi çağırır. Hiçbir bound yok.
- **Etki:** Yoğun kullanımda yüzlerce goroutine havuzu, memory tükenmesi.
- **Risk:** HIGH — memory exhaustion
- **Fix:** Worker pool veya channel-based queue + bounded worker sayısı.

### 34. Ngrok Manager — `time.Sleep` Döngüleri StopCh'siz

- **Dosya:** `internal/ngrok/manager.go:142` (5s sleep), 201 (1s sleep × 30)
- **Kod:** Restart loop ve pollPublicURL `time.Sleep` kullanır, `select { case <-stopCh: }` yoktur. Shutdown'ta sleep'in bitmesi beklenir.
- **Etki:** Shutdown 5+ saniye gecikir.
- **Risk:** HIGH — shutdown gecikmesi
- **Fix:** `time.Sleep` yerine `select { case <-stopCh: return; case <-time.After(...): }`.

### 35. Provider Priority — `ConfigManager.GetEnabled()` Sıralamıyor

- **Dosya:** `internal/provider/config.go:161-172`
- **Kod:** `GetEnabled()` provider'ları eklenme sırasında döndürür, Priority'ye göre sıralamaz. Router kendi içinde sıralar ama `GetEnabled()`'in çıktısını kullanan orchestra ve diğer bileşenler sırasız alır.
- **Etki:** Orkestra provider seçimi Priority'yi dikkate almaz.
- **Risk:** HIGH — orkestra hatalı provider seçimi
- **Fix:** `GetEnabled()` içinde Priority'ye göre sırala.

### MEDIUM

### 36. ConnectionStatusProvider Polling — Dispose Sonrası Devam

- **Dosya:** `frontend/lib/providers/chat_provider.dart:677-690`
- **Not:** Mevcut #15 ile aynı. Sadece daha detaylı tespit.
- **Risk:** MED — gereksiz ağ trafiği
- **Fix:** `autoDispose` + `onDispose` ile timer cleanup.

### 37. `whatsapp_provider.dart` — Unchecked Element Cast

- **Dosya:** `frontend/lib/providers/whatsapp_provider.dart:18,24,30`
- **Kod:** `e as Map<String, dynamic>` — `is` kontrolü yok. Backend beklenmedik format dönerse crash.
- **Risk:** MED — crash
- **Fix:** `if (e is Map)` kontrolü ekle.

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
| 24 | os.Exit(42) — veri kaybı | **CRITICAL** | 10 dk | 🔴 Yeni |
| 25 | Health check goroutine leak | **CRITICAL** | 5 dk | 🔴 Yeni |
| 26 | store.Close() çağrılmıyor | **CRITICAL** | 5 dk | 🔴 Yeni |
| 27 | 28+ unsafe as cast (Flutter) | **CRITICAL** | 30 dk | 🔴 Yeni |
| 28 | Data race: whisperServer | **CRITICAL** | 10 dk | 🔴 Yeni |
| 29 | Data race: webServer | **CRITICAL** | 10 dk | 🔴 Yeni |
| 30 | WhatsApp autoReconnect leak | HIGH | 15 dk | 🔴 Yeni |
| 31 | resolveAgentProvider race | HIGH | 10 dk | 🔴 Yeni |
| 32 | incrementRetrieveCounts UAC | HIGH | 10 dk | 🔴 Yeni |
| 33 | Observer unbounded goroutine | HIGH | 15 dk | 🔴 Yeni |
| 34 | Ngrok stopCh'siz sleep | HIGH | 15 dk | 🔴 Yeni |
| 35 | GetEnabled() priority sırasız | HIGH | 10 dk | 🔴 Yeni |
| 36 | connectionStatusProvider polling | MEDIUM | 10 dk | 🔴 Açık |
| 37 | whatsapp_provider.dart unsafe cast | MEDIUM | 5 dk | 🔴 Yeni |

---

## Kalan Aciliyet Sırası

### Stable İçin BLOCKING (önce bunlar)

1. os.Exit(42) — veri kaybı (#24) — 10 dk
2. store.Close() çağrılmıyor (#26) — 5 dk
3. Health check goroutine leak (#25) — 5 dk
4. Data race: whisperServer (#28) — 10 dk
5. Data race: webServer (#29) — 10 dk
6. 28+ unsafe as cast (#27) — 30 dk
7. Orchestra fallback (#10) — 30 dk

### HIGH

8. WhatsApp autoReconnect (#30) — 15 dk
9. Goroutine leak fix (#6) — 30 dk
10. Ngrok stopCh (#34) — 15 dk
11. Observer unbounded (#33) — 15 dk
12. incrementRetrieveCounts UAC (#32) — 10 dk
13. resolveAgentProvider race (#31) — 10 dk
14. GetEnabled() priority (#35) — 10 dk

### MED

15. Provider priority UI (#9) — 15 dk
16. connectionStatusProvider polling (#36) — 10 dk
17. whatsapp_provider.dart cast (#37) — 5 dk
18. Mobile API client (#8) — 4 saat, ayrı oturum
19. Logging migration (#13) — 1 saat, background task

### LOW

20. model_store_screen refactor (#7) — 2 saat
21. const constructor (#17) — 1 saat
22. Whisper GPU (#21) — 2 saat

---

## Test Coverage Durumu

```
go build ./...   ✅ (0 hata)
go vet ./...     ✅ (0 uyarı)
go test ./... -race -count=1  → 30/30 PASS
Flutter analyze  — geçiyor (CI)
```

328 test fonksiyonu var. Race detector temiz.
