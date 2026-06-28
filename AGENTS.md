# AGENTS.md — Memo

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync. Designed for offline desktop use with optional API fallback.

---

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | Go | 1.26 |
| Frontend | Flutter | 3.10+ |
| State | Riverpod | 2.4 |
| HTTP | Dio | 5.4 |
| Markdown | flutter_markdown | 0.6 |
| Vector DB | SQLite + sqlite-vec | — |
| SQLite driver | mattn/go-sqlite3 | — |

---

## Architecture

**Two-process decoupled:** Go backend (headless REST API, port `:8090`) + Flutter desktop UI. Communication is plain HTTP/JSON + SSE streaming — no TLS.

**Bridge pattern:** `AppBridge` interface in `internal/webserver/bridge.go` decouples HTTP handlers from the `App` orchestrator. `FullBridge` extends it for Flutter-specific endpoints.

### Module Map

| Directory | Responsibility | Key Files |
|-----------|---------------|-----------|
| `internal/app/` | Central orchestrator (25 files) | `app.go`, `llm.go`, `chat.go`, `learning.go`, `whatsapp.go` |
| `internal/webserver/` | REST API (~45 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go`, `installer.go`, `gpu.go` |
| `internal/memory/` | Vector store (SQLite + sqlite-vec) | `store.go`, `embedder.go` |
| `internal/database/` | SQLite connection management | `sqlite.go`, `vec_register.go` |
| `internal/provider/` | External LLM providers (7 types) | `provider.go`, `router.go`, `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go`, `llamacpp.go` |
| `internal/orchestra/` | Multi-model orchestration | `conductor.go`, `roles.go`, `types.go` |
| `internal/agent/` | Agent / tool execution sandbox | `executor.go`, `pipeline.go`, `sandbox.go`, `permissions.go`, `tools.go`, `tools/` |
| `internal/cloudsync/` | Google Drive E2E encrypted backup | `drive.go`, `crypto.go`, `sync_manager.go` |
| `internal/sessions/` | Chat session persistence | `sessions.go` |
| `internal/modelstore/` | HuggingFace model search/download | `modelstore.go` |
| `internal/identity/` | System prompt & persona | `identity.go`, `styles.go` |
| `internal/config/` | Configuration management | `config.go` |
| `internal/api/` | llama.cpp / OpenAI-compatible API client | `client.go`, `streaming.go`, `types.go` |
| `internal/calendar/` | Calendar event store + reminder loop | `store.go`, `reminder.go`, `event.go`, `bridge.go` |
| `internal/intent/` | Intent extraction pipeline | `extractor.go`, `filter.go`, `result.go`, `decider_factory.go` |
| `internal/proactive/` | Proactive suggestion engine | `engine.go`, `decision.go`, `matcher.go`, `pending.go`, `prompt.go`, `feedback.go` |
| `internal/observer/` | Usage pattern analyzer | `analyzer.go`, `recorder.go`, `store.go`, `pattern.go` |
| `internal/whatsapp/` | WhatsApp bridge (whatsmeow) | `client.go`, `store.go` |
| `internal/whisper/` | Speech-to-text (whisper.cpp) | `whisper.go`, platform-specific files |
| `internal/skill/` | Skill system (plugin-like) | `manager.go`, `loader.go`, `types.go` |
| `internal/ngrok/` | ngrok tunnel integration | `installer.go`, `manager.go` |
| `internal/tunnel/` | Tailscale embedded tunnel (tsnet) | `tailscale.go` |
| `internal/truncate/` | Token-aware context truncation | `tokens.go` |
| `internal/models/` | Shared data types | `memory.go` |
| `internal/lora/` | LoRA adapter building (embryonic) | `build/` (cmake artifacts) |

### Flutter Entrypoint

`frontend/lib/main.dart` → `AppShell` (NavRail + ChatScreen / ModelStoreScreen / WhatsAppScreen). State management via Riverpod `AsyncNotifierProvider`. API client is a singleton via `Provider<MemoApiClient>`.

---

## LLM Routing (Priority Order)

Defined in `internal/app/llm.go` `callLLMStream()`:

1. **Orchestra mode** — multi-model workflow (if `orchestraConductor.Config().Enabled`)
2. **External provider** — `provider.Router` with fallback chain (if `activeProvider` is set)
3. **Local llama.cpp** — `api.Client` pointed at local `llama-server`

---

## Config & Data Layout

| Path | Purpose |
|------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API, learning, calendar) |
| `data/memory/` | SQLite + vec0 vector store |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `data/whatsapp/` | WhatsApp SQLite message store + whatsmeow session |
| `data/calendar/` | Calendar events SQLite DB |
| `data/permissions.json` | Agent tool permission policies |
| `.env` | Optional environment overrides (OAuth creds, API keys) |
| `binaries/` | Platform-specific binaries (llama-server, vec0 extension) |

---

## Development

### Quick Start

```bash
# Terminal 1 — Backend
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

### Build

```bash
go build -o memo .                                    # backend binary
cd frontend && flutter build linux --release         # frontend binary
./build_releases.sh                                  # dist packages (tar.gz, AppImage, deb)
```

### Testing

```bash
go test ./...                          # all backend tests
cd frontend && flutter test            # all frontend tests
cd frontend && flutter analyze         # lint
```

Ad-hoc API smoke tests: `dart run test_api_all.dart` (requires running backend).

CI: GitHub Actions runs Go vet/test/build + Flutter analyze/test on every push/PR.

---

## Known Pitfalls & Technical Debt

### Data Races
- `a.client` reassigned in `StartLocalModel` / `StopLocalModel` — `clientMu` exists but concurrency risk on streaming requests during model swap.
- `providerRouter` reassignment during active streams — same pattern as above.

### Memory / Vector Store
- Full rebuild on every startup (`LoadCache` is O(N), no incremental index).
- Embedding model must be started separately (config-driven auto-start on model load).

### Security
- ~~Provider API keys encrypted with hardcoded fallback key~~ → fixed: random key generated via `crypto/rand`, persisted to `data/machine.key` (0600).
- ~~Cloud sync encryption falls back to hardware ID when passphrase is empty~~ → documented behavior, machine.key now provides better fallback.
- ~~No request body size limits~~ → fixed: 50MB `limitBodyMiddleware` on all handlers.
- ~~X-Forwarded-For trusted in rate limiter~~ → fixed: removed header trust, rate limiter uses r.RemoteAddr directly.
- ~~File upload MIME spoofing~~ → fixed: MIME detected from file content via http.DetectContentType, not client header.
- ~~Import path traversal weak~~ → fixed: filepath.Rel validation ensures extracted files stay within data directories.

### Provider / Agent / Orchestra
- **~~`provider.Priority` field exists but unused by router~~** → fixed: router sorts by priority, frontend dialog now exposes priority field.
- Orchestra bypasses `provider.Router` — creates providers directly, but now has fallback chain via `tryFallbackProviders`.
- ~~Agent pipeline has no timeout per tool call~~ → fixed: 60s `DefaultToolTimeout`.
- ~~**No test files for `orchestra/` package** (~800 lines untested)~~ → 48 tests passing with `-race`. `provider/`, `agent/`, and `orchestra/` all have tests.
- **Agent frontend UI (permission dialog, tool call cards, activity panel, agent screen) — fully implemented.**

### Flutter
- ~~`settings_dialog.dart` is 4391 lines~~ → split into 15 focused files under `settings/tabs/`.
- `model_store_screen.dart` is 2469 lines — should be split into components.
- Widespread missing `const` constructors.
- ~~connectionStatusProvider polling runs forever~~ → still autoDispose but polling loop is acceptable for status checks.

### Flutter / Mobile
- Mobile API client (`mobile/lib/core/api_client.dart`) missing most backend endpoints.

### WhatsApp
- ~~QR code polling never stops~~ → adaptive: 2s during QR wait, 15s heartbeat when connected.
- ~~`handleHistorySync` only fires on first pairing~~ → uses `INSERT OR IGNORE`, safe on reconnects.
- ~~WhatsApp store no serialized writes~~ → fixed: `sync.Mutex` on `SaveMessage` and `SaveContact`.
- ~~WhatsApp client init not mutex-protected~~ → fixed: `waMu` mutex protects initialization and lifecycle.

### Other
- ~~Config file written with `0644`~~ → fixed: `config.Save()` uses `0600`. Agent permissions/backup also `0600`.
- ~~`app.go` stores `context.Context` in struct field~~ → `lifecycleCtx` is for goroutine lifecycle only, not request-scoped. All request methods accept `ctx` as parameter. Correct pattern.
- `skill.DangerLevel` and `agent.DangerLevel` are separate named types — compile-time type mismatch.
- ~~`config/config.yaml` has hardcoded `active_provider: openai`~~ → fixed: empty string default.
- ~~Shutdown does not cancel lifecycle context~~ → fixed: lifecycleCancel added, all goroutines stopped on shutdown.
- ~~Cloud backup missing WAL checkpoint~~ → fixed: PRAGMA wal_checkpoint(TRUNCATE) before archive.
- ~~Memory store write operations used read lock~~ → fixed: SaveExplicitMemory/DeleteExplicitMemory use write lock.
- ~~Sessions init not mutex-protected~~ → fixed: sessionsMu.Lock() added.
- ~~orchestraConductor read without mutex~~ → fixed: providerMu.RLock() added.
- ~~proactiveDecide swallows LLM errors~~ → fixed: empty/error responses now return proper errors.
- ~~Nil client dereference when no model loaded~~ → fixed: nil guards added in callLLMStream and callLLM.
- ~~Silent error discards in memory/selfclone/whatsapp~~ → fixed: errors now logged instead of silently ignored.
- CI: GitHub Actions runs Go vet/test/build + Flutter analyze/test on every push/PR.
- **Rate limiting** — token-bucket per-IP (100 req/s) on all handlers via `rateLimitMiddleware`.
- **Structured logging** — `internal/logx` wraps `log/slog` with levels; `webserver/server.go` migrated as example. Remaining packages still use `log.Printf` (gradual migration).
- **API versioning** — flat `/api/` prefix, no versioning strategy.

---

## Düzeltilen Buglar (2026-06-28)

Aşağıda bulunan ve düzeltilen hataların basitçe özeti:

### Veri Kaybı / Bozulma
- **Memory silme/kaydetme kilidi yanlıştı** — Aynı anda iki kişi memory ekler veya silerse veritabanı bozulabiliyordu. Write lock ile düzeltildi.
- **Cloud backup eksik kalabiliyordu** — SQLite WAL modda veriler ana dosyaya yazılmadan önce yedek alınıyordu. Artık önce checkpoint çalıştırılıyor, yedek tam oluyor.
- **Uygulama kapatılınca arka plan işlemleri durmuyordu** — Proactive öneriler, takvim hatırlatmaları, WhatsApp dinleme gibi işlemler kapanışta devam ediyordu. Artık lifecycle context iptal ediliyor, her şey duruyor.

### Eşzamanlılık (Aynı Anda Erişim)
- **WhatsApp çift bağlantı riski** — WhatsApp başlatılırken mutex kullanılmıyordu. İki çağrının aynı anda gelmesi durumunda iki ayrı bağlantı oluşabiliyordu, mesajlar kaybolabiliyordu.
- **Sessions başlatma kilitlenmemişti** — Oturum yöneticisi mutex olmadan atanıyordu, eşzamanlı erişim panic'e neden olabiliyordu.
- **Orchestra motoru kilitsiz okunuyordu** — Mod geçişlerinde data race oluşabiliyordu.

### Dosya Yükleme / Güvenlik
- **Görseller tanınamıyordu** — Dosya yüklerken görsel olup olmadığını anlamak için dosya yolunun sonuna bakılıyordu ama temp dosya isimleri rastgele olduğu için hiçbir zaman eşleşmiyordu. Artık dosya içeriğinden MIME tespit ediliyor.
- **Rate limit aşılabilir LAN'da** — X-Forwarded-For header'ı güvenilir olduğu varsayılıyordu. Saldırgan rastgele IP ile limiti aşabiliyordu. Artık sadece gerçek IP kullanılıyor.
- **Import ile dosya dışı yollara yazılabilir** — .memo dosyası import edilirken path traversal kontrolü zayıftı. filepath.Rel ile doğrulama eklendi.

### Ön Yüz (Flutter)
- **Takvim ekranı çökebiliyordu** — Backend'den hatalı tarih gelirse tüm takvim ekranı kırılıyordu. Try-catch ile güvenli hale getirildi.
- **WhatsApp konuşmada metin kaybı** — Streaming sırasında metin eklemeleri atomik değildi, bazı kelimeler kaybolabiliyordu. Accumulator ile düzeltildi.
- **Dosya gönderiminde eski durum kalıyordu** — Dosya gönderildikten sonra agent event'leri temizlenmiyordu, eski durum rozetleri görünüyordu.
- **Çalışma animasyonu yanlış zamanda görünüyordu** — Metin akarken de "Memo çalışıyor" noktaları görünüyordu. Sadece boşken gösteriliyor artık.
- **Proactive öneri hataları yutuluyordu** — LLM hatalı cevap verdiğinde bile öneri olarak kaydediliyordu. Artık hatalar raporlanıyor.
- **Takvim dialog hafıza sızıntısı** — TextController'lar dialog kapatılınca temizlenmiyordu.
- **Versiyon kontrolü yanlış uyarı** — Backend erişilemezse eski versiyon dönüyordu, yanlış "güncelleme var" bildirimi geliyordu.
- **WhatsApp QR ekranı compile hatası** — Fazladan parantez nedeniyle QR eşleştirme ekranı hiç açılmıyordu.
- **Güvensiz tip dönüşümleri** — Backend response'u beklenmedik formattaysa `as List`/`as Map` cast'leri crash oluşturuyordu. 5 farklı noktada `is` kontrolü eklendi.
- **Agent ekranı sessizce başarısız oluyordu** — `createAgentChat` hatası kullanıcıya gösterilmiyordu. Try-catch ve snackbar eklendi.
- **Skill dialog Windows uyumsuzluğu** — Unix path hint'ı Windows'ta gösteriliyordu. Platform algılama eklendi.

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the codebase (intentional for target users).
- CGO required: `CGO_ENABLED=1 go build/test/run`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.0-beta** (Go 1.26, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
