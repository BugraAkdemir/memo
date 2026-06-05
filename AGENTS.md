# AGENTS.md — Memo (Local LLM Memory Shell)

## Quick start

```bash
# Backend (terminal 1)
go run . --port 8090

# Frontend (terminal 2)
cd frontend && flutter run -d linux
```

## Development commands

```bash
go build -o memo .                    # build backend
cd frontend && flutter build linux --release  # build frontend
./package_linux.sh                    # full portable release
./build_releases.sh                   # dist packages (tar.gz, AppImage, deb)
```

## Testing

**Go backend** (standard `go test`, no framework):
```bash
go test ./internal/llama/...          # single package
go test ./internal/memory/...
go test ./internal/sessions/...
go test ./internal/api/...
go test ./internal/modelstore/...
go test ./internal/cloudsync/...
go test ./internal/identity/...
go test ./...                         # all backend tests
```
No CI config, no required services, no integration test prerequisites.

**Flutter frontend** (standard `flutter_test`):
```bash
cd frontend && flutter test            # all (currently placeholder + model tests)
cd frontend && flutter analyze         # lint (flutter_lints preset)
```
Ad-hoc API smoke tests: `dart run test_api_all.dart` (requires running backend).

## Architecture

**Two-process decoupled:** Go backend (headless REST API on `:8090`) + Flutter desktop UI (Riverpod state, Dio HTTP/SSE). Communication is plain HTTP/JSON + SSE streaming — no TLS.

**Bridge pattern:** `AppBridge` interface in `internal/webserver/bridge.go` decouples HTTP layer from the `App` orchestrator (`app.go`, ~2316 lines). `FullBridge` extends it for Flutter-specific handlers.

**Module ownership:**

| Directory | Role | Key files |
|-----------|------|-----------|
| `app.go` | Central orchestrator (everything routes through here) | `NewApp()`, `callLLMStream()`, `buildMessages()` |
| `internal/webserver/` | REST API (~35 endpoints) | `server.go` (router), `handlers_flutter.go` (handlers), `bridge.go` (interface) |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go` (start/stop/monitor), `installer.go` (auto-download), `gpu.go` (nvidia/amd/metal detect) |
| `internal/memory/` | Vector store (chromem-go + .gob files) | `store.go` (CRUD), `retriever.go` (cosine search), `embedder.go` |
| `internal/provider/` | External LLM providers | `provider.go` (interface), `router.go` (fallback), `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go` |
| `internal/orchestra/` | Multi-model orchestration | `conductor.go` (workflow), `roles.go` (6 expert roles) |
| `internal/cloudsync/` | Google Drive E2E encrypted backup | `drive.go` (OAuth), `crypto.go` (AES-256-GCM), `sync_manager.go` |
| `internal/sessions/` | Chat session persistence | `sessions.go` (JSON CRUD + auto-title) |
| `internal/modelstore/` | HuggingFace model search/download | `modelstore.go` |
| `internal/identity/` | System prompt & persona | `identity.go`, `styles.go` |

**Flutter entrypoint:** `frontend/lib/main.dart` → `AppShell` (NavRail + ChatScreen / ModelStoreScreen).

**Flutter state:** Riverpod `AsyncNotifierProvider` patterns. API client is a singleton via `Provider<MemoApiClient>`.

## LLM routing (priority order, in `callLLMStream`)

1. **Orchestra mode** (if `orchestraConductor.Config().Enabled`) → multi-model workflow
2. **External provider** (if `activeProvider` is set) → `provider.Router` with fallback chain
3. **Local llama.cpp** → `api.Client` pointed at local `llama-server`

## Config & data

| File/Dir | Purpose |
|----------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API) |
| `data/memory/` | `.gob` vector store files (one per interaction) |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `.env` | Optional env overrides (sync OAuth creds, API keys) |

## Known pitfalls

- **Two-terminal dev required:** backend and frontend are separate processes
- **`a.store` and `a.syncManager` have data races** on startup/reconfigure (unlocked pointer writes)
- **`a.client` reassigned without mutex** in `StartLocalModel` / `StopLocalModel`
- **Memory store rebuilds from scratch on every startup** (O(N) `LoadCache` — no incremental index)
- **SSE handler doesn't monitor request `Context().Done()`** → orphaned goroutines when client disconnects
- **`callLLMStream` goroutine runs 5 min after client disconnect** (300s context timeout)
- **Embedding model must be started separately** for RAG to function (auto-start on model load if memory enabled)
- **No request body size limits** on most handlers (DoS vector)
- **Cloud sync / remote access frontend tabs are "under construction"** (backend ready, UI not built)
- **Config written with 0644 permissions** (world-readable)
- **No CI pipeline or pre-commit hooks configured**

## Code style notes

- Go backend uses `http.ServeMux` (gorilla/mux removed), no external router dependency
- Turkish error messages mixed with English across the codebase (intentional for target users)
- `app.go` stores `context.Context` in struct (`App.ctx`) — explicit context passing preferred per Go docs
- Flutter has widespread missing `const` constructors, empty `catch (_)` blocks, and hardcoded Turkish strings bypassing L10n
- `AnimationController` created per message bubble (severe jank at 50+ messages)
- Download polling loop never cancels (runs entire app lifetime)

## Version

Current: v2.0.0-beta (Go 1.25, Flutter 3.10+, `flutter_riverpod` 2.4, `dio` 5.4, `flutter_markdown` 0.6, `chromem-go` 0.7)
