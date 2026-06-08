# AGENTS.md — Memo

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync. Designed for offline desktop use with optional API fallback.

---

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | Go | 1.25 |
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
| `app.go` | Central orchestrator | `NewApp()`, `callLLMStream()`, `buildMessages()` |
| `internal/webserver/` | REST API (~35 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go`, `installer.go`, `gpu.go` |
| `internal/memory/` | Vector store (SQLite + sqlite-vec) | `store.go`, `retriever.go`, `embedder.go` |
| `internal/database/` | SQLite connection management | `sqlite.go`, `vec_register.go` |
| `internal/provider/` | External LLM providers | `provider.go`, `router.go`, `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go` |
| `internal/orchestra/` | Multi-model orchestration | `conductor.go`, `roles.go` |
| `internal/cloudsync/` | Google Drive E2E encrypted backup | `drive.go`, `crypto.go`, `sync_manager.go` |
| `internal/sessions/` | Chat session persistence | `sessions.go` |
| `internal/modelstore/` | HuggingFace model search/download | `modelstore.go` |
| `internal/identity/` | System prompt & persona | `identity.go`, `styles.go` |
| `internal/config/` | Configuration management | `config.go` |
| `internal/api/` | llama.cpp API client | `client.go` |
| `internal/agent/` | Agent / tool execution sandbox | `sandbox.go`, `tools/command.go` |

### Flutter Entrypoint

`frontend/lib/main.dart` → `AppShell` (NavRail + ChatScreen / ModelStoreScreen). State management via Riverpod `AsyncNotifierProvider`. API client is a singleton via `Provider<MemoApiClient>`.

---

## LLM Routing (Priority Order)

Defined in `app.go` `callLLMStream()`:

1. **Orchestra mode** — multi-model workflow (if `orchestraConductor.Config().Enabled`)
2. **External provider** — `provider.Router` with fallback chain (if `activeProvider` is set)
3. **Local llama.cpp** — `api.Client` pointed at local `llama-server`

---

## Config & Data Layout

| Path | Purpose |
|------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API) |
| `data/memory/` | SQLite + vec0 vector store |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `data/whatsapp/` | WhatsApp SQLite message store + whatsmeow session |
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

No CI pipeline or pre-commit hooks configured.

---

## Known Pitfalls & Technical Debt

### Data Races
- `a.store` and `a.syncManager` pointer writes on startup/reconfigure are not mutex-guarded.
- `a.client` reassigned in `StartLocalModel` / `StopLocalModel` without synchronization.

### Memory / Vector Store
- Full rebuild on every startup (`LoadCache` is O(N), no incremental index).
- Embedding model must be started separately (config-driven auto-start on model load).

### SSE / Goroutines
- SSE handler doesn't monitor `request.Context().Done()` → orphaned goroutines on disconnect.
- `callLLMStream` goroutine persists 5 minutes after client disconnect (300s context timeout).
- `WhatsAppChatStream` has the same pattern (no context cancellation monitoring).

### WhatsApp
- QR code polling: HTTP polling for QR never stops even after successful pairing.
- `handleHistorySync` only fires on first pairing; subsequent reconnects won't get history.
- WhatsApp session reconnection not tested (whatsmeow auto-reconnect behavior unknown).

### Security
- No request body size limits on most handlers (DoS vector).
- Config file written with `0644` permissions (world-readable).
- `app.go` stores `context.Context` in struct field (`App.ctx`) — violates Go best practice.

### Flutter
- Widespread missing `const` constructors; empty `catch (_)` blocks; hardcoded Turkish strings (no L10n layer).
- `AnimationController` created per message bubble → severe jank at 50+ messages.
- Download polling loop never cancels (runs entire app lifetime).

### UI
- Cloud sync / remote access frontend tabs are "under construction" (backend ready, UI not built).

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the codebase (intentional for target users).
- CGO required: `CGO_ENABLED=1 go build/test/run`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.0-beta** (Go 1.25, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
