# Resolved Issues

61 documented fixes from the full codebase audit. Full detail: `docs/RESOLVED_ISSUES.md`.

---

## 🔴 Critical (13 fixes)

| ID | Issue | Approach |
|----|-------|----------|
| C1 | Orphaned SSE streams — LLM continues after client disconnect | Context propagation through entire call chain |
| C2 | Engine mode / config update resets all llama settings | Field-by-field merge instead of struct replacement |
| C3 | Arbitrary file read via `/api/image` | Dual-layer path validation + symlink protection |
| C4 | Remote access — no auth, wide-open CORS | Completely disabled in v3.0.0 |
| C5 | `a.client` reassignment without synchronization | `clientMu sync.RWMutex` added |
| C6 | `saveMemoryAsync` RLock→Lock pattern (deadlock risk) | Channel-based worker replaces lock+goroutine |
| C7 | UI thread performance — AnimationController per message | All entry animations removed from bubbles |
| C8 | Stream cancel on chat switch | `stopStreaming()` called in `switchTo()` |
| C9 | Incognito toggle race | `incognitoMu sync.RWMutex` added |
| C10 | `processSSEStream` watcher goroutine leak | `context.WithCancel` + `defer cancel()` |
| C11 | `callLLMStream` blocks on full channel | `trySend()` helper with select |
| C12 | `memorySaveWorker` never exits on shutdown | `close(a.memorySaveCh)` in shutdown |
| C13 | Concurrent `writeIndexFile` index corruption | Synchronous write (already under lock) |

## 🟠 High (15 fixes)

| ID | Issue | Approach |
|----|-------|----------|
| H1 | Goroutine leak on SSE disconnection | Context cancellation unblocks scanner |
| H2 | Config file world-readable (`0644`) | Changed to `0600` |
| H3 | Weak key derivation for sync encryption | PBKDF2 (600K iterations) + random salt |
| H4 | Hardcoded fallback encryption key | Persistent UUID per machine |
| H5 | `buildMessages` mutates session history permanently | Defensive copy before mutation |
| H6 | `hash2hex` — only 4 bytes of SHA-256 | Changed to 8 bytes |
| H7 | `monitor()` goroutine accesses `s.cmd` outside lock | Local copy under lock |
| H8 | Temp file leak on download non-cancellation errors | Unconditional `defer os.Remove` |
| H9 | File descriptor leak in `extractTarGzToBin` | `extractFile()` helper with `defer out.Close()` |
| H10 | `nvidia-smi` errors silently ignored | All errors logged via `log.Printf` |
| H11 | OAuth `authDone` channel race | `sync.WaitGroup` replaces channel |
| H12 | `Shutdown(context.Background())` blocks indefinitely | 10s timeout context |
| H13 | Session ID truncated to 8 hex chars | Full UUID (36 chars) |
| H14 | Download polling stream runs forever | `if (!progress.active) break;` |
| H15 | Backend error shows "Installed" on connection error | All errors return false |

## 🟡 Medium (13 fixes)

| ID | Issue | Approach |
|----|-------|----------|
| M1 | Background errors never reach UI | `eventRing` (64 event ring buffer) |
| M2 | Session files world-readable (`0644`) | Changed to `0600` |
| M3 | `save()` errors silently discarded | Errors logged via `log.Printf` |
| M4 | `loadAll()` skips corrupted session files | Errors logged |
| M5 | SSE `[DONE]` chunk missing `FinishReason` | `FinishReason: "stop"` added |
| M6 | Synchronous blocking writes on main path | Async goroutine for session saves |
| M7 | `LoadCache` performance — O(N) startup | SQLite index replaces per-file scan |
| M8 | Brute-force O(N) vector search | Pre-computed L2 norm, parallel workers |
| M9 | `killByPort` depends on `lsof`/`fuser` | PID tracking + fallback |
| M10 | Hardcoded Windows audio device GUID | Enumeration via ffmpeg |
| M11 | Linux GPU detection via sysfs fragile | `detectAMDLspci()` added |
| M12 | Auto-scroll yanks to bottom when reading history | `_isNearBottom()` check |
| M13 | Export chat silently swallows errors | Error SnackBar added |

## 🔵 Low (20 fixes)

| ID | Issue |
|----|-------|
| L1 | Config load failure silently falls back to defaults |
| L2 | Memory store / session init failures silently disabled |
| L3 | `os.Executable()` error ignored leading to empty path |
| L4 | Token path empty causes all drive ops to fail |
| L5 | `NewStore` with nil `embeddingFunc` creates silent crash path |
| L6 | Memory index copies all embeddings with `append` (2x RAM) |
| L7 | Discord webhook / action URL writes never checked (removed) |
| L8 | Type assertion panic in OAuth loopback listener |
| L9 | WakeOnLan / Precise file handling (removed) |
| L10 | No size limit on model import (50 GiB limit added) |
| L11 | Symlink attack in `DeleteLocalModel` |
| L12 | TOCTOU race in `safePersistPath` |
| L13 | `runCmdStream` goroutines may outlive function |
| L14 | Chat input `/` command has no visual indicator |
| L15 | `FocusNode` created on every build |
| L16 | Stale prompt text in settings |
| L17 | Error state shows only icon — no error message |
| L18 | Model stop buttons fire without awaiting |
| L19 | Cloud sync and remote access tabs "under construction" |
| L20 | Setup wizard uses `$name` literal instead of interpolation |
