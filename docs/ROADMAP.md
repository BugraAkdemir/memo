# Memo Roadmap

Strategic vision and release plan based on full codebase audit (2026-06-02).

---

## v3.0.0 — "Solidify" (Current Target)

**Theme:** Stability, security, and performance hardening. No new features — fix every known issue from the audit.

### Security (P0)
- [ ] `/api/image` arbitrary file read — restrict to app data directory
- [ ] Remote access: add JWT/session authentication, disable wildcard CORS on `0.0.0.0`
- [ ] Config file (`config.yaml`) permissions: `0644` → `0600`
- [ ] Session files permissions: `0644` → `0600`
- [ ] Modelstore `DeleteLocalModel` — add symlink attack protection (`filepath.EvalSymlinks`)
- [ ] Modelstore `ImportLocalModel` — add file size limit
- [ ] Weak KDF (`sha256.Sum256`) → PBKDF2/argon2id for sync encryption
- [ ] Hardcoded fallback encryption key — remove or require explicit passphrase

### Concurrency & Resource Leaks (P0)
- [ ] SSE stream context wiring — cancel LLM call on client disconnect
- [ ] `a.client` / `a.embeddingClient` — guard all reads/writes with mutex
- [ ] `saveMemoryAsync` RLock→Lock pattern — rewrite with channel-based worker
- [ ] `monitor()` goroutine — move `s.cmd` nil check + `Wait()` inside lock
- [ ] `Shutdown(context.Background())` → `WithTimeout`
- [ ] OAuth `authDone` channel race — use `sync.WaitGroup` or shared channel

### Critical Bug Fixes (P0)
- [ ] Config patch on engine mode update: partial JSON merges, doesn't zero out fields
- [ ] `buildMessages` mutates session history — copy slice before injecting system prompt
- [ ] `hash2hex` 4-byte collision → use at least 8 bytes
- [ ] Session ID truncated to 8 hex chars → use full UUID or 16+ chars

### Frontend Performance & UX (P1)
- [ ] `AnimationController` per message → remove entry animations; use lightweight rendering
- [ ] Download polling loop (`models_provider.dart`) — cancel on completion / dispose
- [ ] Auto-scroll yanks to top when reading history — only scroll if near bottom
- [ ] Error state (chat_screen) — show actual error message, not just icon
- [ ] Chat export — surface errors to user instead of silent `catch (_) {}`
- [ ] Model stop buttons — `await` API call, revert UI on failure

### Task.md Backlog (P1)
- [ ] SSE stream token rebuild optimization (Bölüm 2)
- [ ] Incognito toggle race condition fix (Bölüm 3)
- [ ] Stream cancellation on chat switch (Bölüm 4)
- [ ] Error messages shown as snackbar, not chat bubbles (Bölüm 5)
- [ ] Double-send prevention (Bölüm 6)
- [ ] Timestamp: `HH:mm` → `HH:mm:ss` (Bölüm 7)
- [ ] Export chat: file picker save dialog (Bölüm 8)
- [ ] Delete confirmations for chat, memory, model (Bölüm 9)
- [ ] Empty message guard (Bölüm 10)

### Quality of Life (P1)
- [ ] Background errors reach UI — implement event polling or SSE status endpoint
- [ ] Session `save()` errors — propagate instead of silent discard
- [ ] `loadAll()` — log skipped corrupted session files
- [ ] SSE `[DONE]` — include `finish_reason` field
- [ ] Temp file leak on download error — always clean up `.downloading` files
- [ ] `extractTarGzToBin` file descriptor leak — use `defer out.Close()`
- [ ] `nvidia-smi` error handling — detect failure and warn user
- [ ] KillByPort `lsof`/`fuser` dependency — track PIDs directly
- [ ] Hardcoded Windows audio GUID — enumerate or use default device
- [ ] Linux GPU detection — add `lspci` fallback

---

## v4.0.0 — "Refresh"

**Theme:** Architectural improvements, UI overhaul, missing frontend features.

### Storage Overhaul
- [ ] Migrate memory store from `.gob` files to SQLite (CGO-free: `modernc.org/sqlite`)
- [ ] One-time migration script for existing `.gob` data
- [ ] O(N) brute-force vector search → add ANN index (SQLite FTS5 + vector extension or dedicated vector DB)
- [ ] Lazy loading / pagination for `LoadCache`

### UI/UX Overhaul
- [ ] Custom design system (brand identity, custom icons beyond Material)
- [ ] Markdown rendering for user messages (currently plain `SelectableText`)
- [ ] Model store visual refresh (cover images, badges, search)
- [ ] Smooth, performant animations (no more per-message AnimationController)
- [ ] Cloud Sync settings UI tab (backend ready, frontend shows "under construction")
- [ ] Remote Access settings UI tab (backend ready, frontend shows "under construction")
- [ ] `/` command visual hint in chat input
- [ ] System prompt editor — live sync with backend changes

### Reliability
- [ ] Config validation at startup — fail loudly instead of silent defaults
- [ ] Memory store / session init failures — show blocking error, don't silently disable
- [ ] Event system: implement SSE endpoint for background status reporting
- [ ] `os.Executable()` error handling
- [ ] STT startup: validate dependencies (`ffmpeg`, `sox`, `arecord`) before recording

---

## v5.0.0 — "Evolve"

**Theme:** New capabilities, ecosystem, and autonomy features from the original vision.

### Enhanced Intelligence
- [ ] Dynamic Top-K selection based on query complexity
- [ ] Cross-session reasoning — synthesize information across chat sessions
- [ ] Knowledge Graph — visualize semantic links between memories (Obsidian-style graph view)
- [ ] Advanced reranking with a secondary model

### Ecosystem
- [ ] Plugin system — Go plugins for custom tools (web search, calculator, code exec)
- [ ] Mobile companion app — secure tunnel to local memory
- [ ] Import/Export wizards — Notion, Obsidian, Google Keep

### Autonomy
- [ ] Autonomous memory pruning — AI-driven cleanup of redundant/contradictory memories
- [ ] Self-improving system prompt — learns from user feedback
- [ ] Multi-user support with isolation

---

> **Legend:** P0 = must-have for v3.0, P1 = should-have for v3.0  
> **Full issue reference:** [KNOWN_ISSUES.md](./KNOWN_ISSUES.md) (55 issues, 7 critical, 15 high, 13 medium, 20 low, 8 info)
