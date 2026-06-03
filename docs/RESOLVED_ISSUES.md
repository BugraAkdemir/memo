# Resolved Issues

This document lists all 61 identified bugs that have been fixed in the Memo project.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🟡 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- 🔵 **Low** — cosmetic, minor improvement, or edge case

---

## 🔴 Critical

### C1. Orphaned SSE Streams — LLM Continues After Client Disconnect
- **File:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`, `app.go`, `internal/webserver/bridge.go`
- **Fix:** Added `ctx context.Context` parameter to `AppBridge`/`FullBridge` stream methods. `handleSendStream` now passes `r.Context()` through the entire LLM call chain (→ `SendMessageStream` → `callLLMStream` → `ChatCompletionStream`). When the client disconnects, context cancellation terminates the LLM HTTP request and the goroutine exits naturally.

### C2. Engine Mode / Config Update Resets All Llama Settings to Zero
- **File:** `app.go:1148-1151`
- **Fix:** `UpdateLlamaConfig` now performs a field-by-field merge instead of replacing the entire struct. Only non-zero values from the incoming config overwrite the current config. A partial JSON body like `{"engine_mode": "cpu"}` only updates the `EngineMode` field, preserving all other settings (port, ctx_size, binary_path, etc.).

### C3. Arbitrary File Read via `/api/image`
- **File:** `app.go:865-872` (GetImageBase64), `internal/webserver/handlers_flutter.go:214-226` (handleImage)
- **Fix:** Dual-layer path validation. Layer 1 (handler): `..` traversal and absolute paths rejected. Layer 2 (`GetImageBase64`): `filepath.Abs` + `EvalSymlinks` resolution + `data/` directory prefix whitelist. Symlink attacks also prevented.

### C4. Remote Access Server — No Authentication, Wide-Open CORS
- **File:** `internal/webserver/server.go:65-176`, `app.go:155-157`, `app.go:1048-1063`
- **Fix:** Remote access has been completely disabled in v3.0.0. `Start()` returns an error immediately. `SetRemoteAccess` refuses to enable it. CORS wildcard replaced with `Origin` echo. Will be re-implemented properly in a future version.

### C5. `a.client` Reassignment Without Synchronization
- **File:** `app.go`
- **Fix:** Added `clientMu sync.RWMutex` to `App` struct. All writes use `Lock/Unlock`, all reads use `RLock/RUnlock` with pointer copy. Both `a.client` and `a.embeddingClient` are fully protected.

### C6. `saveMemoryAsync` RLock→Lock Pattern (Deadlock Risk)
- **File:** `app.go`
- **Fix:** Replaced the lock-then-goroutine pattern with a channel-based worker. `saveMemoryAsync` now sends a `saveTask` to a buffered channel (cap 64) and returns immediately. A single `memorySaveWorker` goroutine processes tasks sequentially, acquiring `storeMu.Lock()` directly — no RLock→Lock transition, no deadlock risk.

### C7. UI Thread Performance — AnimationController Per Message
- **File:** `frontend/lib/widgets/chat_message_list.dart`
- **Fix:** Removed all entry animations from `_MessageBubble`. `SingleTickerProviderStateMixin`, `AnimationController`, `FadeTransition`, and `SlideTransition` have been eliminated entirely. Bubbles render directly with no frame callbacks.

---

### C8. Stream Cancel on Chat Switch
- **File:** `frontend/lib/providers/chat_provider.dart`
- **Fix:** Added `messagesProvider.notifier.stopStreaming()` call in `switchTo()`. Switching chats now cancels the previous HTTP request.

### C9. Incognito Toggle Race
- **File:** `app.go`
- **Fix:** Added `incognitoMu sync.RWMutex`. All read points use `RLock`/`RUnlock`, all write points use `Lock`/`Unlock`. Zero data race risk on concurrent incognito toggle + message send.

### C10. `processSSEStream` Watcher Goroutine Leak
- **File:** `internal/api/streaming.go:49-53`
- **Fix:** Added `context.WithCancel` child context + `defer cancel()`. When the function returns, `cancel()` is called and the watcher goroutine exits via `watchCtx.Done()`. Goroutine pool no longer exhaustible.

### C11. `callLLMStream` Blocks on Full Channel
- **File:** `app.go`
- **Fix:** Added `trySend()` helper: `select { case outCh <- chunk: case <-ctx.Done(): }`. The sender unblocks on context cancellation instead of blocking forever on a full channel.

### C12. `memorySaveWorker` Never Exits on Shutdown
- **File:** `app.go:309`
- **Fix:** Added `close(a.memorySaveCh)` in `shutdown()`. The goroutine exits its `range` loop when the channel is closed. Goroutine leak on every shutdown eliminated.

### C13. Concurrent `writeIndexFile` Index Corruption
- **File:** `internal/memory/store.go:357-358`
- **Fix:** Changed `go s.writeIndexFile(cp)` → synchronous `s.writeIndexFile(s.index)`. Already under write lock, no need for async. Memory index is always consistent.

---

## 🟠 High

### H1. Goroutine Leak on SSE Disconnection
- **File:** `internal/api/streaming.go`, `internal/api/client.go`
- **Fix:** `processSSEStream` now accepts a `ctx context.Context` parameter. A watcher goroutine closes the response body on context cancellation, unblocking the scanner and terminating the goroutine naturally. `ChatCompletionStream` passes the context to `processSSEStream`.

### H2. Config File World-Readable (`0644`) Containing Secrets
- **File:** `internal/config/config.go:178`
- **Fix:** Changed `0644` → `0600` in `os.WriteFile` call. Config file is now only readable by the owner.

### H3. Weak Key Derivation for Sync Encryption
- **File:** `internal/cloudsync/crypto.go`, `sync_manager.go`
- **Fix:** `encrypt`/`decrypt` now use PBKDF2 (600,000 iterations) with a random 16-byte salt per encryption. The salt is prepended to the ciphertext. Legacy SHA-256 format is kept as a fallback in `decrypt` for backward compatibility.

### H4. Hardcoded Fallback Encryption Key
- **File:** `internal/cloudsync/sync_manager.go`
- **Fix:** In `New()`, when the passphrase is empty, a persistent random UUID is stored in `persistDir/.machine-id` and used as the encryption passphrase. Each machine gets a unique key on first run.

### H5. `buildMessages` Mutates Session History Permanently
- **File:** `app.go:1358`
- **Fix:** Added a defensive copy in `buildMessages`: `history = append([]api.Message{}, history...)`. Mutations on the local copy never affect session data. System prompt is injected once per request without accumulation.

### H6. `hash2hex` — Only 4 Bytes of SHA-256 (Collision Risk)
- **File:** `internal/memory/store.go:342-344`
- **Fix:** Changed `hash[:4]` → `hash[:8]` (8 bytes / 16 hex chars). Collision probability reduced from 50% to negligible levels.

### H7. `monitor()` Goroutine Access to `s.cmd` Outside Lock
- **File:** `internal/llama/llama.go:271-302`
- **Fix:** Nil check is now performed inside the lock and copied to a local variable (`cmd := s.cmd`). `Wait()` is called on the local copy. Even if `Stop()` sets `s.cmd = nil`, `monitor()` operates on its own copy — nil-pointer panic is no longer possible.

### H8. Temp File Leak on Download Non-Cancellation Errors
- **File:** `internal/modelstore/modelstore.go:237-243`
- **Fix:** Changed conditional `os.Remove(tmpPath)` (`if ctx.Err() != nil`) to unconditional `defer os.Remove(tmpPath)`. Temp file is now cleaned up on every exit path — context cancellation, HTTP error, or network timeout.

### H9. File Descriptor Leak in `extractTarGzToBin`
- **File:** `internal/llama/installer.go:433,437`
- **Fix:** Extracted file-write logic into a separate `extractFile()` helper function that uses `defer out.Close()`. This guarantees the file descriptor is always closed, even on `io.Copy` errors. Manual `out.Close()` calls removed.

### H10. `nvidia-smi` Errors Silently Ignored → 0 VRAM → 0 GPU Layers
- **File:** `internal/llama/gpu.go:62-101`
- **Fix:** All `nvidia-smi` call errors are now logged via `log.Printf`: binary not found, name query failure, VRAM query failure, and VRAM value parse failure. Users can now see why GPU detection failed in the logs.

### H11. OAuth `authDone` Channel Race
- **File:** `internal/cloudsync/drive.go`
- **Fix:** Replaced `chan struct{}` + `authDoneClosed bool` with `sync.WaitGroup`. `StartAuthFlow` calls `Add(1)`, `closeAuthDoneLocked` calls `Done()`, `WaitForAuth` uses a goroutine to `Wait()` on the group. No channel swap means no race.

### H12. `Shutdown(context.Background())` Can Block Indefinitely
- **File:** `internal/webserver/server.go:286`
- **Fix:** Replaced `context.Background()` with `context.WithTimeout(10*time.Second)`. Server shutdown now times out after 10 seconds if a handler is stuck.

### H13. Session ID Truncated to 8 Hex Chars
- **File:** `internal/sessions/sessions.go:68`
- **Fix:** Changed `uuid.New().String()[:8]` → `uuid.New().String()`. Full UUID (36 chars) used as session ID. Collision probability is now astronomically low.

### H14. Download Polling Stream Runs Forever
- **File:** `frontend/lib/providers/models_provider.dart:66-79`
- **Fix:** Added `if (!progress.active) break;` — the loop exits when the download is idle/complete. Also breaks on error. Provider disposal automatically cancels via Dart's async* cancellation mechanism.

### H15. Backend Error Handling: Connection Error Shows "Installed"
- **File:** `frontend/lib/providers/models_provider.dart:97-104`
- **Fix:** Removed the `connectionError` special case that returned `true` ("installed"). All errors now return `false` — when the backend is unreachable, the user sees the installer and can trigger setup.

---

## 🟡 Medium

### M1. Background Errors Never Reach the UI (Broken Event System)
- **File:** `app.go`, `internal/webserver`
- **Fix:** `eventRing` (64 event ring buffer) implemented. `emitEvent` writes events to the ring buffer and logs them. `GET /api/events` endpoint added for frontend to poll background events.

### M2. Session Files World-Readable (`0644`)
- **File:** `internal/sessions/sessions.go:236`
- **Fix:** Changed `0644` → `0600`.

### M3. `save()` Errors Silently Discarded in Session Manager
- **File:** `internal/sessions/sessions.go:75,155`
- **Fix:** `save()` errors are now logged via `log.Printf`.

### M4. `loadAll()` Silently Skips Corrupted Session Files
- **File:** `internal/sessions/sessions.go:252-258`
- **Fix:** Read and decode errors are now logged via `log.Printf`.

### M5. SSE `[DONE]` Chunk Missing `FinishReason`
- **File:** `internal/api/streaming.go:73`
- **Fix:** Added `FinishReason: "stop"` to the `[DONE]` chunk.

### M6. Synchronous Blocking Writes on Main Path
- **File:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Fix:** Session saves now write to disk in a goroutine (async). Memory writes were already async via `memorySaveWorker` channel.

### M7. `LoadCache` Performance — O(N) Startup Time
- **File:** `internal/memory/store.go`
- **Fix:** Single-file index (`memory_index.gob`) replaces per-file scan. Startup reads one file instead of N. Index is maintained incrementally on save/delete.

### M8. Brute-Force O(N) Vector Search
- **File:** `internal/memory/retriever.go`, `store.go`
- **Fix:** Added pre-computed L2 norm (`MemoryIndex.Norm`) so `cosineSimilarityFast` skips norm recalculation per item ~ 2x faster. Parallel worker search already in place.

### M9. `killByPort` Depends on `lsof` / `fuser`
- **File:** `internal/llama/llama.go`, `process_unix.go`, `process_windows.go`
- **Fix:** `Server.portPid` field added — PID tracked at process start. `Stop()` kills tracked PID first via `killPID()`, falls back to `lsof`/`fuser`. `lsof`/`fuser`/`netstat` errors are now logged via `log.Printf`.

### M10. Hardcoded Windows Audio Device GUID
- **File:** `app.go:737-775`
- **Fix:** Hardcoded GUID removed. `getDefaultDshowDevice()` enumerates DirectShow devices via ffmpeg and uses the first microphone found. Falls back to `"default"` if none found.

### M11. Linux GPU Detection via Sysfs Fragile
- **File:** `internal/llama/gpu.go`
- **Fix:** `detectAMDLspci()` added — tries `lspci` first (container-friendly), falls back to sysfs. Removed `bash` dependency; uses `filepath.Glob` + `os.ReadFile`.

### M12. Auto-Scroll Yanks to Bottom When Reading History
- **File:** `frontend/lib/widgets/chat_message_list.dart:91-103`
- **Fix:** `_isNearBottom()` check added — auto-scroll skipped if user is more than 50px from the bottom.

### M13. Export Chat Silently Swallows Errors
- **File:** `frontend/lib/screens/chat_screen.dart:203`
- **Fix:** Empty `catch (_) {}` replaced with error SnackBar.

---

## 🔵 Low

### L1. Config Load Failure Silently Falls Back to Defaults
- **File:** `app.go:168-171`
- **Fix:** `a.emitEvent("config_load_error", err.Error())` notifies frontend via event ring.

### L2. Memory Store / Session Manager Init Failures Silently Disabled
- **File:** `app.go:183-193`
- **Fix:** `a.emitEvent("memory_store_error", ...)` / `a.emitEvent("sessions_manager_error", ...)` on failure.

### L3. `os.Executable()` Error Ignored Leading to Empty Path
- **File:** `app.go:916`
- **Fix:** `exePath, _` → `exePath, err`, falls back to `os.Args[0]` on error.

### L4. Token Path Empty Causes All Drive Ops to Fail
- **File:** `internal/cloudsync/drive.go:44-46`
- **Fix:** `newDriveClient` now returns `(*driveClient, error)`, validates `tokenPath` is not empty.

### L5. `NewStore` with Nil `embeddingFunc` Creates Silent Crash Path
- **File:** `internal/memory/store.go:43-45`
- **Fix:** Returns error `"memory.NewStore: embeddingFunc is nil"` if nil.

### L6. Memory Index Copies All Embeddings with `append` (2x RAM)
- **File:** `internal/memory/store.go:104-108`
- **Fix:** Direct reference to `doc.Embedding` in legacy `LoadCache` path (doc goes out of scope, no mutation risk). Copy preserved in `SaveInteraction` where chromem collection may retain reference.

### L7. `DiscordWebhook` / Action URL Writes Never Checked
- **Fix:** Feature removed from codebase; issue resolved by removal.

### L8. Type Assertion Panic in OAuth Loopback Listener
- **File:** `internal/cloudsync/drive.go:103-107`
- **Fix:** `tcpAddr, ok := ln.Addr().(*net.TCPAddr)` with error return if not TCP.

### L9. `WakeOnLan` / `Precise` File Handling
- **Fix:** Feature removed from codebase; issue resolved by removal.

### L10. No Size Limit on Model Import
- **File:** `internal/modelstore/modelstore.go:396,406-409`
- **Fix:** `maxImportSize = 50 GiB` constant; `info.Size() > maxImportSize` checked before copy.

### L11. Symlink Attack in `DeleteLocalModel`
- **File:** `internal/modelstore/modelstore.go:367-384`
- **Fix:** `filepath.EvalSymlinks` called on both `absPath` and `absModelsDir` before prefix comparison.

### L12. `TOCTOU` Race in `safePersistPath`
- **File:** `internal/memory/store.go:315-339`
- **Fix:** `filepath.EvalSymlinks` called before returning path; symlink-resolved path rechecked against prefix.

### L13. `runCmdStream` Goroutines May Outlive Function
- **File:** `internal/llama/installer.go:635-654`
- **Fix:** `sync.WaitGroup` added; `wg.Wait()` called after `cmd.Wait()` to ensure both scanners complete before return.

### L14. Chat Input `/` Command Has No Visual Indicator
- **File:** `frontend/lib/widgets/chat_input.dart:203`
- **Fix:** `hintText` now includes `' (/)'` suffix.

### L15. `FocusNode` Created on Every Build
- **File:** `frontend/lib/widgets/chat_input.dart:19-21,39-42`
- **Fix:** `_kbFocusNode` stored in state, disposed in `dispose()`.

### L16. Stale Prompt Text in Settings (Not Updated When Data Changes)
- **File:** `frontend/lib/widgets/settings_dialog.dart:321-323`
- **Fix:** `_initialized` flag removed; controller updated whenever `_controller.text != prompt`.

### L17. Error State Shows Only Icon — No Error Message
- **File:** `frontend/lib/screens/chat_screen.dart:67-70`
- **Fix:** `'$e'` replaces the generic `'connection_error'` text.

### L18. Model Stop Buttons Fire Without Awaiting
- **File:** `frontend/lib/screens/model_store_screen.dart:732-751, 804-827`
- **Fix:** `onPressed` is now `async` with `try`/`catch`, shows SnackBar on error.

### L19. Cloud Sync and Remote Access Tabs Show "Under Construction"
- **File:** `frontend/lib/widgets/settings_dialog.dart:788-958, 959-990`
- **Fix:** Cloud Sync tab: auth status, OAuth connect, sync now, disconnect UI. Remote Access tab: shows "disabled in v3.0.0" notice.

### L20. Setup Wizard Uses `$name` Literal Instead of Interpolation
- **File:** `frontend/lib/widgets/setup_wizard_view.dart:87`
- **Fix:** `\$name` → `$name` (Dart string interpolation).

---

> **Last updated:** 2026-06-03  
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend  
> **Total fixes:** 61 (13 critical, 15 high, 13 medium, 20 low)
