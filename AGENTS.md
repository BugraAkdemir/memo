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
| `internal/replcli/` | Terminal REPL client — talks to the REST API the same way Flutter does | `client.go`, `repl.go`, `sse.go`, `agent_event.go`, `clients_client.go`, `commands.go`, `editor.go`, `keys.go`, `models_client.go`, `remote_client.go`, `sessions_client.go`, `tasklist_client.go` |
| `internal/webserver/` | REST API (~45 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go`, `installer.go`, `gpu.go` |
| `internal/memory/` | Vector store (SQLite + sqlite-vec) | `store.go`, `embedder.go` |
| `internal/database/` | SQLite connection management | `sqlite.go`, `vec_register.go` |
| `internal/provider/` | External LLM providers (9 types) | `provider.go`, `router.go`, `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go`, `llamacpp.go`, `opencode_zen.go`, `opencode_go.go` |
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
# Interactive terminal chat (starts the backend if needed, opens a REPL)
go run .

# Headless backend only, to pair with the Flutter frontend
go run . --headless --port 8090
cd frontend && flutter run -d linux
```

`memo` auto-detects whether stdin is an interactive terminal: interactive →
opens the terminal REPL (agent mode on by default, see `internal/replcli/`);
non-interactive or `--headless` → today's headless server, unchanged. If a
backend is already listening on the port, a second `memo` invocation attaches
to it instead of trying to bind again.

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

### Release

Never release from memory — use the **memo-release skill**
(`.claude/skills/memo-release/SKILL.md`). It carries the full checklist:
seven version locations, EN+TR release notes, per-platform build commands,
the versioned→generic artifact rename for `download.bugradev.com`, and the
`version.json` update beacon that must be bumped LAST.

---

## Known Pitfalls & Technical Debt

### Data Races
- ~~`a.client`/`providerRouter` reassignment during active streams — data race~~ → verified 2026-07-10: not actually a race, both reads (`internal/app/llm.go:714-716,965-967`) and writes (`internal/app/llama.go`, `providers.go`) are correctly `clientMu`/`providerMu`-locked. The real, narrower risk that survives: a stream copies the client into a local var under lock at request start, then holds it for the stream's whole duration — if the model/provider is swapped mid-stream, that in-flight request keeps talking to the now-stopped/replaced client instead of the new one (fails with a connection error, doesn't corrupt anything). See BUG-L4 in `BUG_REPORT.md`.

### Memory / Vector Store
- ~~Full rebuild on every startup (`LoadCache` is O(N), no incremental index)~~ → stale note: `LoadCache` doesn't exist in the current codebase (`internal/memory/store.go`'s `NewStore`/`initSchema` don't do a startup full-scan — this refers to an older architecture, before the hybrid vector+FTS rework). Removed as a claim; re-audit from scratch if startup performance on a large memory store is ever reported as an issue.
- ~~`chunkText` sized chunks by word count (`strings.Fields`), a poor proxy for token count~~ → fixed 2026-07-12: `internal/memory/chunker.go` now sizes chunks by `truncate.EstimateTokens` (the same char/3 heuristic used elsewhere for context budgeting), so a message short by word count but made of long tokens (URLs, code, agglutinative Turkish) can no longer slip through as a single unsplit, over-budget chunk. Note: `chunkText` is only used by `SaveInteraction` to chunk a single chat turn (user message + reply) before embedding — Memo has no document/file-upload → RAG ingestion pipeline, so heading-aware/semantic splitting (relevant for chunking uploaded documents) doesn't apply to this codebase's actual usage.
- Embedding model must be started separately (config-driven auto-start on model load).

### Security
- ~~Provider API keys encrypted with hardcoded fallback key~~ → fixed: random key generated via `crypto/rand`, persisted to `data/machine.key` (0600).
- ~~Cloud sync encryption falls back to hardware ID when passphrase is empty~~ → documented behavior, machine.key now provides better fallback.
- ~~No request body size limits~~ → fixed: 50MB `limitBodyMiddleware` on all handlers.
- ~~X-Forwarded-For trusted in rate limiter~~ → fixed: removed header trust, rate limiter uses r.RemoteAddr directly.
- ~~File upload MIME spoofing~~ → fixed: MIME detected from file content via http.DetectContentType, not client header.
- ~~Import path traversal weak~~ → fixed: filepath.Rel validation ensures extracted files stay within data directories.
- ~~Remote access (LAN/ngrok) had zero authentication~~ → fixed 2026-07-11: `remoteAuthMiddleware` (`internal/webserver/server.go`) requires `X-Memo-Token`/`Authorization: Bearer <token>` on every request once the listener is bound to `0.0.0.0` (LAN mode or ngrok — both go through `SetRemoteAccess`/`SetNgrokMode`); local-only mode (127.0.0.1, the default) is unaffected. The `RemoteAccess.Token` shown in Settings was already generated but never checked anywhere before this. Known follow-up: BUG-L5 in `BUG_REPORT.md` (ngrok auto-start + app restart can leave the desktop client without the token until it's re-toggled).
- ~~Agent's `rm -rf` blacklist regex never matched its own target~~ → fixed 2026-07-11: `\brm\s+-rf\s+/\b` (and the `~`/`.` variants) relied on `\b`, but `/`, `~`, `.` are non-word characters, so no boundary ever formed at end-of-string — "rm -rf /", "rm -rf /*", "sudo rm -rf /" all sailed through unblocked while a harmless "rm -rf /home/user/foo" was flagged instead. `internal/agent/tools/command.go` now uses an explicit terminator class (end-of-string/whitespace/shell-operator) instead of `\b`.

### Provider / Agent / Orchestra
- **~~`provider.Priority` field exists but unused by router~~** → fixed: router sorts by priority, frontend dialog now exposes priority field.
- Orchestra bypasses `provider.Router` — creates providers directly, but now has fallback chain via `tryFallbackProviders`.
- ~~Agent pipeline has no timeout per tool call~~ → fixed: pipeline grants 120s per tool call. 2026-07-04: `tools/command.go`/`tools/search.go` were silently truncating that budget to 60s via their own hard-coded `DefaultToolTimeout`; now they honor the caller's deadline and only fall back to 60s when none is set.
- ~~**No test files for `orchestra/` package** (~800 lines untested)~~ → 48 tests passing with `-race`. `provider/`, `agent/`, and `orchestra/` all have tests.
- **Agent frontend UI (permission dialog, tool call cards, activity panel, agent screen) — fully implemented.**

### REPL CLI (`internal/replcli/`)
- ~~`/model-download` ran an in-terminal Hugging Face search-and-download flow whose progress loop only read from a ticker, never the keyboard — a stalled download left the whole REPL stuck with no way to cancel (not even Esc/Ctrl+C, since raw mode turns those into plain keypresses, not signals)~~ → fixed 2026-07-09: `/model-download` no longer downloads anything itself; it prints a short message and opens the desktop GUI (`cmdGui`) instead. Heavy, long-running work (model search/download with real progress bars) belongs in the GUI, not the terminal client.
- ~~`/gui` looked for the bundled `memo_flutter` binary only next to the CLI's own executable, so it never found it on a real install~~ → fixed 2026-07-09: the installed CLI binary lives one level deeper (`~/.memo/bin/memo`) than the bundled GUI (`~/.memo/memo_flutter`) — `cmdGui` now searches the exe's own directory *and* its parent (`guiSearchDirs`, same pattern as `binarySearchBasesFrom` in `internal/llama`), and runs the GUI with its own directory as `cmd.Dir` (needed for Flutter's `lib/`/`flutter_assets/`, which sit next to the binary, not next to the CLI).
- ~~`/model`/`/embedding` used the plain JSON client's blanket 10s timeout, but the backend's own model-load budget is 120–180s (`WaitReady` in `internal/app/llama.go`/`embedding.go`) — any load slower than 10s reported a false "başlatılamadı" while the backend kept loading successfully in the background~~ → fixed 2026-07-09: `StartModel`/`StartEmbedding` now run on a dedicated no-fixed-timeout client (`Client.longOpHTTP`) with an explicit deadline matching the real backend budget (185s/125s), and `startAndReport` wraps the call in the same Esc/Ctrl+C-cancellable, spinner-shown pattern `sendMessage` uses for streaming replies — cancelling only stops the CLI from waiting (the backend handler doesn't watch the request context), so a cancelled load may still finish in the background; `/models` shows the outcome.
- ~~An external SIGTERM/SIGINT during the interactive REPL left the terminal in raw mode (`stty sane`/`reset` required) because `main.go`'s signal-select branch returned without waiting for `replcli.Run()`'s goroutine, so its deferred `term.Restore` never ran~~ → fixed 2026-07-09: `main.go` captures the pre-raw terminal state via `term.GetState` before starting the REPL goroutine and restores it directly in the signal branch. (Ctrl+C typed at the keyboard was never affected — raw mode disables ISIG, so it's decoded as a plain keypress by `keys.go`, not delivered as a signal at all.)
- ~~No bracketed-paste support: every embedded newline in a pasted multi-line block decoded as a real Enter press, splitting one paste into several separately-submitted messages (and running any "/"-prefixed line inside it as a command)~~ → fixed 2026-07-09: `Run()` enables bracketed paste (`ESC[?2004h`, disabled again on exit) and `keys.go`'s `readBracketedPaste` decodes the `ESC[200~ … ESC[201~`-wrapped block as one `keyPaste`, collapsing embedded line breaks to spaces (`collapsePasteNewlines`) so the whole paste lands as a single insertable chunk at the cursor, submitted only on an actual Enter press.
- ~~A `memo` interactive session that spawned its own backend ran it *in-process* (`a.Startup`/`a.StartWebServerHTTP` inside `main()`), so exiting the REPL unconditionally shut the backend down via `defer a.Shutdown(ctx)` — if the user had opened the Flutter GUI mid-session via `/gui`, it was still relying on that exact backend, and lost it the moment the terminal exited~~ → fixed 2026-07-09, see "Backend process model" below.

### Backend process model — client reference counting (2026-07-09)

An interactive `memo` session that finds no backend running now spawns one as a **genuinely separate, detached OS process** (`spawnDetachedBackend` in `main.go`, `exec.Command(exe, "--headless", "--port", ..., "--auto-shutdown")`, `Setsid`/`CREATE_NEW_PROCESS_GROUP` via `main_unix.go`/`main_windows.go`) instead of running it in-process — the REPL is now always a pure HTTP client of the backend, whether it just spawned one or attached to an existing one (unifies what used to be two different code paths).

That backend tracks which clients are attached (`internal/app/clients.go`'s `clientRegistry`: `RegisterClient`/`HeartbeatClient`/`UnregisterClient`, exposed as `POST /api/clients/{register,heartbeat,unregister}`) and shuts itself down (self-`SIGINT`, same mechanism `/api/shutdown` uses) once the last one disconnects — **but only if `--auto-shutdown` was passed**, which only `spawnDetachedBackend` does. A plain `--headless` backend run as a standalone service (remote access, WhatsApp bridge) never arms this, regardless of which clients happen to register with it — this is the guard that keeps a persistent service safe.

Both the CLI (`internal/replcli/clients_client.go`, wired into `repl.go`'s `Run()`) and the Flutter GUI (`frontend/lib/providers/chat_provider.dart`'s `connectionStatusProvider`, which already polled every 30s — extended to register once then heartbeat each tick instead of adding a new timer) register on startup and heartbeat every ~25-30s. The CLI unregisters explicitly on a clean exit (`/exit`, Ctrl+D, double Ctrl+C — not reachable on external SIGTERM, same blocking-read limitation as the terminal-restore fix above); Flutter has no reliable window-close hook on desktop today (confirmed: no `window_manager`, no lifecycle observer wired up), so a closed GUI is only noticed once it stops heartbeating — the backend prunes a client silently after `clientStaleAfter` (90s, ~3 missed beats).

This is what makes "CLI open, then open the GUI via /gui, then close the CLI" and the reverse both safe: whichever client is left keeps the backend alive via its own heartbeat, and only when the registry drains to zero does the backend shut down — bounded, not indefinite background lingering.

Verified against the real compiled binary (not just unit tests) in this session: `--auto-shutdown` backend with two registered clients survives one leaving and self-shuts-down when the second leaves; a plain `--headless` backend (no `--auto-shutdown`) survives a client registering and unregistering exactly as before. Not yet verified: an actual `/gui`-launched Flutter window in this environment (no display), and the Windows detach path (`CREATE_NEW_PROCESS_GROUP`) — only compile-checked via `GOOS=windows go vet`.

### Identity / System Prompt (`internal/identity/`) (2026-07-10)

The default system prompt (`buildIdentityBlock`) was translated from Turkish to English — instructions generally get followed more reliably in English regardless of conversation language, and the block already tells the model to always reply in whatever language it's written to (this is independent of the setup wizard's own persona prompts in `frontend/lib/widgets/setup_wizard_view.dart`, which carry separate `prompt_tr`/`prompt_en` text per persona and override the default entirely via `CustomRole` when picked).

`buildOriginBlock` was added and is **always** appended in `BuildSystemPrompt`, independent of whether `CustomRole` (a wizard persona or a hand-written prompt) is set — it used to live inside `buildIdentityBlock`, which only runs when `CustomRole` is empty, so picking any wizard persona silently dropped "who made you" grounding entirely. Kept deliberately terse (~100 tokens, down from an initial ~260) since it only pays off on the rare message where someone actually asks — Memo's target audience runs small local models on tight context budgets, where padding a permanent tax for a rarely-used fact is the wrong trade.

`Identity.MinimalMode` (`cfg.Identity.MinimalMode`, toggle in Settings → General → "Minimal Mod") strips identity/origin/style injection from `BuildSystemPrompt` entirely, and gates the mood-directive/web-search injection in `internal/app/helpers.go`'s `buildMessages` too — memory context still goes in if `cfg.Memory.MemoryEnabled` is separately on, since MinimalMode only targets persona/mood/search injection, not memory. With both toggles off, zero extra tokens reach the model beyond the raw conversation. `MinimalMode` overrides `CustomRole` too, not just the default block — a wizard persona is also fully suppressed when it's on.

### Flutter
- ~~`settings_dialog.dart` is 4391 lines~~ → split into 15 focused files under `settings/tabs/`.
- `model_store_screen.dart` is 2612 lines (re-verified 2026-07-11) — should be split into components.
- ~~`EngineStrip`'s divider before the memory warning only checked `chatRunning || isApiProvider`, missing the case where the offline hint rendered instead (fully offline: no chat model, no API provider, embedding not running) — the offline hint and the memory warning rendered stuck together with zero gap ("Start a model●No memory model")~~ → fixed 2026-07-10: two named booleans (`firstSlotHasContent`, `secondSlotHasContent`) capture what the strip's first/second slot actually rendered, used for all three divider checks (embedding indicator, memory warning, download indicator) instead of the incomplete `chatRunning || isApiProvider` shorthand. Regression test in `engine_strip_test.dart` asserts both the divider's presence and a minimum pixel gap between the two texts.
- ~~Widespread missing `const` constructors.~~ → fixed: 116 auto-fixes via `dart fix`.
- ~~connectionStatusProvider polling runs forever~~ → still autoDispose but polling loop is acceptable for status checks.
- ~~Chat SSE client timeout (60s) shorter than backend's own generation budget (300s), no server heartbeat~~ → fixed 2026-07-04: `chat_provider.dart` now uses 300s for both regular and agent chat streams (and file-send stream), matching `llm.go`'s `context.WithTimeout(ctx, 300*time.Second)`. Previously a slow first-token (long prompt, weak/CPU hardware) past 60s aborted an in-progress, otherwise-valid generation.

### Build / Dependencies
- 2026-07-04: `frontend/pubspec.yaml` had accumulated reactive `dependency_overrides` (`jni: 0.13.0`, `path_provider_android: 2.3.1`, `jni_flutter: 1.0.1`) added to work around what was actually a **corrupted pub-cache** (`jni-1.0.0` and `path_provider_windows-2.3.0` had 0-byte/partial downloads), not a real version conflict. Removed the overrides and re-fetched the packages; Linux build is clean again. Worth remembering if a similarly "unexplained" plugin build failure shows up again — check the pub-cache contents before reaching for a version pin.

### Flutter / Mobile
- ~~Mobile API client (`mobile/lib/core/api_client.dart`) missing most backend endpoints~~ → stale: verified 2026-07-10, it covers 111 of the backend's 118 registered routes. The 7 missing are either brand-new (today's client-registry/minimal-mode endpoints, which mobile has no use for — no CLI/detached-backend concept there) or CLI-install-management endpoints (`/api/cli/*`, `/api/uninstall`) that don't apply to a mobile app either. Not a real gap anymore.

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
- **Structured logging** — `internal/logx` wraps `log/slog` with levels; all packages migrated to `logx.Printf`.
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

## Agent Working Rules (READ FIRST, EVERY SESSION)

1. **Start of session:** read this file, then `handoff.md` (top entry = last session's state and pending work).
2. **End of session:** prepend a new handoff entry to `handoff.md` (what was done, commit status, verification results, what's next). This is how context survives between sessions and models.
3. **Never claim "done" without running verification** (below) and pasting the actual results.
4. Plan files (`plan.md`, `PLAN_*.md`) contain step-by-step implementation plans — follow them in order, tick checkboxes as you complete items, don't improvise a different architecture mid-plan.
5. Work in small units: max 1–2 plan items per session, each with tests, verified green before moving on.
6. Commit messages: Conventional Commits (`fix(frontend): ...`, `feat(backend): ...`). No AI attribution / Co-Authored-By lines.

### Verification Commands (mandatory before any "done" claim)

```bash
# Backend (CGO is required — sqlite)
CGO_ENABLED=1 go build ./...
CGO_ENABLED=1 go vet ./...
CGO_ENABLED=1 go test ./... -race

# Frontend (Flutter SDK is NOT in PATH on this machine)
export PATH="$PATH:/home/bugra/Belgeler/flutter/bin"
cd frontend && flutter analyze lib/ && flutter test
```

Acceptable pre-existing noise: a few `use_build_context_synchronously` **info**-level findings in `flutter analyze`. Anything else new must be fixed.

---

## Gotchas (project-specific traps — violating these causes real, shipped bugs)

**Paths & data**
- All data paths go through `config.DataPath()`. Never hardcode `data/` — on Windows data lives in `%ProgramData%\Memo\data`.
- The installed CLI lives at `~/.memo/bin/memo` (one level deep): lookups for bundled files must also check the **parent** of the executable's directory. REPL debug log: `~/.memo/data/repl.log`.
- Windows path bugs in Flutter: always `package:path` (`p.join`, `p.basename`) — never `split('/')` or `'$a/$b'` string concat.

**Concurrency & architecture**
- SQLite writes go through `database.DB.Write()` (serialized write loop) — never call `ExecContext`/`Exec` directly on the DB; it corrupts the single-writer design.
- The app has **one global active chat** and a global agent-mode flag. `App.SendMessageStream` writes to whatever chat is currently active. Any automated caller (e.g. task loop) must `SwitchChat` under `taskloopRunMu` and restore state after. In Flutter, don't trust `activeChatIdProvider` blindly — pass explicit chat IDs.
- `a.client` and `providerRouter` are swapped at runtime (model/provider changes during active streams) — always take `clientMu`/`providerMu` before touching them.

**Streaming / SSE**
- Timeout contract: backend generation budget in `internal/app/llm.go` is **300s**; every frontend SSE consumer (`chat_provider.dart`, `chat_input.dart` WhatsApp stream, file-send stream) must use the **same 300s — as a per-call option on the streaming request**, NOT via Dio's global `receiveTimeout` (that stays at 120s for regular API calls; see commit `2abf8dd`). A 60s frontend timeout once aborted valid slow generations on CPU hardware.
- SSE `finishReason == 'agent_event'` chunks carry JSON tool events — render as badges, never as raw text; parse defensively (payload may be malformed).

**Flutter**
- `IndexedStack` keeps every screen mounted forever: any polling loop must stop itself via `VisibilityDetector` / `AppLifecycleListener` / `ref.onDispose`, or it leaks and polls in background.
- Backend JSON must be checked with `is` before casting — `as List`/`as Map` on unexpected payloads has crashed the UI in 5+ places before.
- New user-facing strings go through `frontend/lib/core/l10n.dart`.
- Unexplained plugin build failure? Check `~/.pub-cache` for 0-byte/partial package downloads **before** adding `dependency_overrides` (see 2026-07-04 note above).

**Types & misc**
- `skill.DangerLevel` and `agent.DangerLevel` are separate named types — they do not cross-assign.
- Turkish + English mixed user-facing text is intentional (target users are Turkish).
- sqlite-vec extension (`vec0.so`/`vec0.dll`) is bundled under `binaries/` — never add a runtime download for it.

---

## Known Open Work (pointers)

| Item | Where |
|------|-------|
| ~~Onboarding / launchpad UX~~ — done, archived | `docs/plans/plan.md` |
| Chat-ID refactor: kill the single-global-active-chat architecture | `docs/plans/PLAN_chatid_refactor.md` |
| `model_store_screen.dart` (2612 lines) needs splitting | Known Pitfalls above |

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the codebase (intentional for target users).
- CGO required: `CGO_ENABLED=1 go build/test/run`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.2** (open beta, 2026-07-06) (Go 1.26, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
