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

### Provider / Agent / Orchestra
- **`provider.Priority` field exists but unused by router** — config field defined, sort logic present but not wired.
- Orchestra bypasses `provider.Router` — creates providers directly, no fallback chain.
- ~~Agent pipeline has no timeout per tool call~~ → fixed: 60s `DefaultToolTimeout`.
- **No test files for `orchestra/` package** (~800 lines untested). `provider/` and `agent/` now have tests.
- **Agent frontend UI (permission dialog, tool call cards) not yet fully implemented.**

### Flutter
- ~~`settings_dialog.dart` is 4391 lines~~ → split into 15 focused files under `settings/tabs/`.
- `model_store_screen.dart` is 2469 lines — should be split into components.
- Widespread missing `const` constructors.
- `connectionStatusProvider` and download progress polling run forever.

### Flutter / Mobile
- Mobile API client (`mobile/lib/core/api_client.dart`) missing most backend endpoints.

### WhatsApp
- ~~QR code polling never stops~~ → adaptive: 2s during QR wait, 15s heartbeat when connected.
- ~~`handleHistorySync` only fires on first pairing~~ → uses `INSERT OR IGNORE`, safe on reconnects.
- ~~WhatsApp store no serialized writes~~ → fixed: `sync.Mutex` on `SaveMessage` and `SaveContact`.

### Other
- ~~Config file written with `0644`~~ → fixed: `config.Save()` uses `0600`. Agent permissions/backup also `0600`.
- ~~`app.go` stores `context.Context` in struct field~~ → `lifecycleCtx` is for goroutine lifecycle only, not request-scoped. All request methods accept `ctx` as parameter. Correct pattern.
- `skill.DangerLevel` and `agent.DangerLevel` are separate named types — compile-time type mismatch.
- ~~`config/config.yaml` has hardcoded `active_provider: openai`~~ → fixed: empty string default.
CI: GitHub Actions runs Go vet/test/build + Flutter analyze/test on every push/PR.
- ~~**No CI pipeline**~~ → fixed: GitHub Actions for Go + Flutter.
- **Rate limiting** — token-bucket per-IP (100 req/s) on all handlers via `rateLimitMiddleware`.
- **Structured logging** — `internal/logx` wraps `log/slog` with levels; `webserver/server.go` migrated as example. Remaining packages still use `log.Printf` (gradual migration).
- **API versioning** — flat `/api/` prefix, no versioning strategy.

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the codebase (intentional for target users).
- CGO required: `CGO_ENABLED=1 go build/test/run`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.0-beta** (Go 1.26, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
