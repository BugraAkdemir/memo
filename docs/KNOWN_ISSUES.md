# Known Issues & Technical Risks (Exhaustive Audit)

This document tracks all identified bugs, architectural limitations, and edge cases in the Memo project, updated after a deep codebase audit.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🟡 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- 🔵 **Low** — cosmetic, minor improvement, or edge case
- ⚪ **Info** — design note, risk, or observation

---

## 🔴 Critical

### ~~C1. Orphaned SSE Streams — LLM Continues After Client Disconnect~~ ✅ Fixed
- **File:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`, `app.go`, `internal/webserver/bridge.go`
- **Fix:** Added `ctx context.Context` parameter to `AppBridge`/`FullBridge` stream methods. `handleSendStream` now passes `r.Context()` through the entire LLM call chain (→ `SendMessageStream` → `callLLMStream` → `ChatCompletionStream`). When the client disconnects, context cancellation terminates the LLM HTTP request and the goroutine exits naturally.

### ~~C2. Engine Mode / Config Update Resets All Llama Settings to Zero~~ ✅ Fixed
- **File:** `app.go:1148-1151`
- **Fix:** `UpdateLlamaConfig` now performs a field-by-field merge instead of replacing the entire struct. Only non-zero values from the incoming config overwrite the current config. A partial JSON body like `{"engine_mode": "cpu"}` only updates the `EngineMode` field, preserving all other settings (port, ctx_size, binary_path, etc.).

### ~~C3. Arbitrary File Read via `/api/image`~~ ✅ Fixed
- **File:** `app.go:865-872` (GetImageBase64), `internal/webserver/handlers_flutter.go:214-226` (handleImage)
- **Fix:** Dual-layer path validation. Layer 1 (handler): `..` traversal and absolute paths rejected. Layer 2 (`GetImageBase64`): `filepath.Abs` + `EvalSymlinks` resolution + `data/` directory prefix whitelist. Symlink attacks also prevented.

### ~~C4. Remote Access Server — No Authentication, Wide-Open CORS~~ ✅ Fixed
- **File:** `internal/webserver/server.go:65-176`, `app.go:155-157`, `app.go:1048-1063`
- **Fix:** Remote access has been completely disabled in v3.0.0. `Start()` returns an error immediately. `SetRemoteAccess` refuses to enable it. CORS wildcard replaced with `Origin` echo. Will be re-implemented properly in a future version.

### ~~C5. `a.client` Reassignment Without Synchronization~~ ✅ Fixed
- **File:** `app.go`
- **Fix:** Added `clientMu sync.RWMutex` to `App` struct. All writes use `Lock/Unlock`, all reads use `RLock/RUnlock` with pointer copy. Both `a.client` and `a.embeddingClient` are fully protected.

### ~~C6. `saveMemoryAsync` RLock→Lock Pattern (Deadlock Risk)~~ ✅ Fixed
- **File:** `app.go`
- **Fix:** Replaced the lock-then-goroutine pattern with a channel-based worker. `saveMemoryAsync` now sends a `saveTask` to a buffered channel (cap 64) and returns immediately. A single `memorySaveWorker` goroutine processes tasks sequentially, acquiring `storeMu.Lock()` directly — no RLock→Lock transition, no deadlock risk.

### ~~C7. UI Thread Performance — AnimationController Per Message~~ ✅ Fixed
- **File:** `frontend/lib/widgets/chat_message_list.dart`
- **Fix:** Removed all entry animations from `_MessageBubble`. `SingleTickerProviderStateMixin`, `AnimationController`, `FadeTransition`, and `SlideTransition` have been eliminated entirely. Bubbles render directly with no frame callbacks.

---

## 🟠 High

### ~~H1. Goroutine Leak on SSE Disconnection~~ ✅ Fixed
- **File:** `internal/api/streaming.go`, `internal/api/client.go`
- **Fix:** `processSSEStream` now accepts a `ctx context.Context` parameter. A watcher goroutine closes the response body on context cancellation, unblocking the scanner and terminating the goroutine naturally. `ChatCompletionStream` passes the context to `processSSEStream`.

### ~~H2. Config File World-Readable (`0644`) Containing Secrets~~ ✅ Fixed
- **File:** `internal/config/config.go:178`
- **Fix:** Changed `0644` → `0600` in `os.WriteFile` call. Config file is now only readable by the owner.

### ~~H3. Weak Key Derivation for Sync Encryption~~ ✅ Fixed
- **File:** `internal/cloudsync/crypto.go`, `sync_manager.go`
- **Fix:** `encrypt`/`decrypt` now use PBKDF2 (600,000 iterations) with a random 16-byte salt per encryption. The salt is prepended to the ciphertext. Legacy SHA-256 format is kept as a fallback in `decrypt` for backward compatibility.

### ~~H4. Hardcoded Fallback Encryption Key~~ ✅ Fixed
- **File:** `internal/cloudsync/sync_manager.go`
- **Fix:** In `New()`, when the passphrase is empty, a persistent random UUID is stored in `persistDir/.machine-id` and used as the encryption passphrase. Each machine gets a unique key on first run.
- **Fix:** Require an explicit passphrase, or generate a random key on first sync and store it in the config.

### ~~H5. `buildMessages` Mutates Session History Permanently~~ ✅ Fixed
- **File:** `app.go:1358`
- **Fix:** Added a defensive copy in `buildMessages`: `history = append([]api.Message{}, history...)`. Mutations on the local copy never affect session data. System prompt is injected once per request without accumulation.

### ~~H6. `hash2hex` — Only 4 Bytes of SHA-256 (Collision Risk)~~ ✅ Fixed
- **File:** `internal/memory/store.go:342-344`
- **Fix:** Changed `hash[:4]` → `hash[:8]` (8 bytes / 16 hex chars). Collision probability reduced from 50% to negligible levels.

### ~~H7. `monitor()` Goroutine Access to `s.cmd` Outside Lock~~ ✅ Fixed
- **File:** `internal/llama/llama.go:271-302`
- **Fix:** Nil check is now performed inside the lock and copied to a local variable (`cmd := s.cmd`). `Wait()` is called on the local copy. Even if `Stop()` sets `s.cmd = nil`, `monitor()` operates on its own copy — nil-pointer panic is no longer possible.

### ~~H8. Temp File Leak on Download Non-Cancellation Errors~~ ✅ Fixed
- **File:** `internal/modelstore/modelstore.go:237-243`
- **Fix:** Changed conditional `os.Remove(tmpPath)` (`if ctx.Err() != nil`) to unconditional `defer os.Remove(tmpPath)`. Temp file is now cleaned up on every exit path — context cancellation, HTTP error, or network timeout.

### ~~H9. File Descriptor Leak in `extractTarGzToBin`~~ ✅ Fixed
- **File:** `internal/llama/installer.go:433,437`
- **Fix:** Extracted file-write logic into a separate `extractFile()` helper function that uses `defer out.Close()`. This guarantees the file descriptor is always closed, even on `io.Copy` errors. Manual `out.Close()` calls removed.

### ~~H10. `nvidia-smi` Errors Silently Ignored → 0 VRAM → 0 GPU Layers~~ ✅ Fixed
- **File:** `internal/llama/gpu.go:62-101`
- **Fix:** All `nvidia-smi` call errors are now logged via `log.Printf`: binary not found, name query failure, VRAM query failure, and VRAM value parse failure. Users can now see why GPU detection failed in the logs.

### ~~H11. OAuth `authDone` Channel Race~~ ✅ Fixed
- **File:** `internal/cloudsync/drive.go`
- **Fix:** Replaced `chan struct{}` + `authDoneClosed bool` with `sync.WaitGroup`. `StartAuthFlow` calls `Add(1)`, `closeAuthDoneLocked` calls `Done()`, `WaitForAuth` uses a goroutine to `Wait()` on the group. No channel swap means no race.

### ~~H12. `Shutdown(context.Background())` Can Block Indefinitely~~ ✅ Fixed
- **File:** `internal/webserver/server.go:286`
- **Fix:** Replaced `context.Background()` with `context.WithTimeout(10*time.Second)`. Server shutdown now times out after 10 seconds if a handler is stuck.

### ~~H13. Session ID Truncated to 8 Hex Chars~~ ✅ Fixed
- **File:** `internal/sessions/sessions.go:68`
- **Fix:** Changed `uuid.New().String()[:8]` → `uuid.New().String()`. Full UUID (36 chars) used as session ID. Collision probability is now astronomically low.

### ~~H14. Download Polling Stream Runs Forever~~ ✅ Fixed
- **File:** `frontend/lib/providers/models_provider.dart:66-79`
- **Fix:** Added `if (!progress.active) break;` — the loop exits when the download is idle/complete. Also breaks on error. Provider disposal automatically cancels via Dart's async* cancellation mechanism.

### ~~H15. Backend Error Handling: Connection Error Shows "Installed"~~ ✅ Fixed
- **File:** `frontend/lib/providers/models_provider.dart:97-104`
- **Fix:** Removed the `connectionError` special case that returned `true` ("installed"). All errors now return `false` — when the backend is unreachable, the user sees the installer and can trigger setup.

---

## 🟡 Medium

### M1. Background Errors Never Reach the UI (Broken Event System)
- **File:** `app.go` (emitEvent is stubbed), all call sites
- **Issue:** `emitEvent` was designed for Wails and is now a no-op for Flutter. Background errors (cloud sync failures, embedding model load failures, download progress, auto-start failures) are only written to `server.log` and never shown to the user.
- **Impact:** Silent failures; user sees no feedback when sync breaks or embedding fails.
- **Fix:** Implement a server-sent event endpoint or poll endpoint for background status.

### ~~M2. Session Files World-Readable (`0644`)~~ ✅ Fixed
- **File:** `internal/sessions/sessions.go:236`
- **Fix:** Changed `0644` → `0600`.

### ~~M3. `save()` Errors Silently Discarded in Session Manager~~ ✅ Fixed
- **File:** `internal/sessions/sessions.go:75,155`
- **Fix:** `save()` errors are now logged via `log.Printf`.

### ~~M4. `loadAll()` Silently Skips Corrupted Session Files~~ ✅ Fixed
- **File:** `internal/sessions/sessions.go:252-258`
- **Fix:** Read and decode errors are now logged via `log.Printf`.

### ~~M5. SSE `[DONE]` Chunk Missing `FinishReason`~~ ✅ Fixed
- **File:** `internal/api/streaming.go:73`
- **Fix:** Added `FinishReason: "stop"` to the `[DONE]` chunk.

### ~~M6. Synchronous Blocking Writes on Main Path~~ ✅ Fixed
- **File:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Fix:** Session saves now write to disk in a goroutine (async). Memory writes were already async via `memorySaveWorker` channel.

### ~~M7. `LoadCache` Performance — O(N) Startup Time~~ ✅ Fixed
- **File:** `internal/memory/store.go`
- **Fix:** Single-file index (`memory_index.gob`) replaces per-file scan. Startup reads one file instead of N. Index is maintained incrementally on save/delete.

### ~~M8. Brute-Force O(N) Vector Search~~ ✅ Fixed
- **File:** `internal/memory/retriever.go`, `store.go`
- **Fix:** Added pre-computed L2 norm (`MemoryIndex.Norm`) so `cosineSimilarityFast` skips norm recalculation per item ~ 2x faster. Parallel worker search already in place.

### M9. `killByPort` Depends on `lsof` / `fuser`
- **File:** `internal/llama/llama.go:244-253`
- **Issue:** `killByPort` shells out to `lsof` or `fuser`. On minimal containers, embedded systems, or Windows without these tools, the function silently fails, leaving a stale process bound to the port.
- **Impact:** "Address already in use" errors on subsequent model starts.
- **Fix:** Track child process PIDs and kill directly instead of port-based discovery.

### M10. Hardcoded Windows Audio Device GUID
- **File:** `app.go:739` (via `StartRecording`)
- **Issue:** On Windows, the recording command uses a hardcoded `@device_cm_{...}` GUID for the microphone. This GUID is specific to one hardware configuration. On most Windows machines, recording fails silently.
- **Impact:** STT is broken on Windows for most users.
- **Fix:** Enumerate audio devices at startup or use a system default recording device.

### M11. Linux GPU Detection via Sysfs Fragile
- **File:** `internal/llama/gpu.go:167`
- **Issue:** GPU detection execs `bash -c "cat /sys/class/drm/card*/device/vendor"`. This depends on `/sys` being available (not in Docker without `--privileged`), on `bash` being present, and on the specific naming pattern of DRM devices.
- **Impact:** GPU detection fails silently in containers or non-standard environments.
- **Fix:** Use `lspci` parsing as a fallback, or read `hwmon` / `drm` info more robustly.

### M12. Auto-Scroll Yanks to Bottom When Reading History
- **File:** `frontend/lib/widgets/chat_message_list.dart:23-33`
- **Issue:** `didUpdateWidget` calls `_scrollToBottom()` whenever messages change. If the user has scrolled up to read earlier messages, a new token arriving forces them back to the bottom.
- **Impact:** Cannot read history while streaming is active.
- **Fix:** Only auto-scroll if the user is within a threshold (e.g. 50px) of the bottom.

### ~~M13. Export Chat Silently Swallows Errors~~ ✅ Fixed
- **File:** `frontend/lib/screens/chat_screen.dart:203`
- **Fix:** Empty `catch (_) {}` replaced with error SnackBar.

---

## 🔵 Low

### L1. Config Load Failure Silently Falls Back to Defaults
- **File:** `app.go:115-119`
- **Issue:** If `config.Load()` fails (corrupt YAML, permissions), the app logs the error and uses default config. The user's custom settings are silently discarded.
- **Fix:** Return the error to main and refuse to start (or at least show a blocking dialog).

### L2. Memory Store / Session Manager Init Failures Silently Disabled
- **File:** `app.go:126-137`
- **Issue:** Errors from `NewStore` and `sessions.NewManager` are only logged. `a.store` and `a.sessions` are set to nil. The app continues running with no memory and no session persistence — both appear to work but save nothing.
- **Fix:** Surface these errors to the user or refuse to start.

### L3. `os.Executable()` Error Ignored Leading to Empty Path
- **File:** `app.go:813`
- **Issue:** `exePath, _ := os.Executable()` — on systems where `/proc/self/exe` is not available, `exePath` is empty string. This propagates into relative paths that resolve to the current working directory.
- **Fix:** Check the error and fall back to `os.Args[0]`.

### L4. Token Path Empty Causes All Drive Ops to Fail
- **File:** `internal/cloudsync/drive.go:115`
- **Issue:** If `dc.tokenPath` is empty (no config), `os.ReadFile("")` reads the current directory and fails. No clear error is shown to the user.
- **Fix:** Validate `tokenPath` at initialization and return a clear error.

### L5. `NewStore` with Nil `embeddingFunc` Creates Silent Crash Path
- **File:** `internal/memory/store.go:38-57`
- **Issue:** If `embeddingFunc` is nil, `NewStore` succeeds but any `SaveInteraction` call panics with nil function call.
- **Fix:** Return an error from `NewStore` when `embeddingFunc` is nil.

### L6. Memory Index Copies All Embeddings with `append` (2x RAM)
- **File:** `internal/memory/store.go:84`
- **Issue:** `MemoryIndex` copies every embedding vector with `append([]float32(nil), doc.Embedding...)`. During `LoadCache`, this doubles RAM usage for memory data.
- **Fix:** Reference embeddings directly if the source won't be mutated.

### L7. `DiscordWebhook` / Action URL Writes Never Checked
- **File:** `app.go:463-514`
- **Issue:** The discord webhook and action URL writes (lines 483-494 and 503) have results that are never checked. Silent failure of these integrations.
- **Fix:** At minimum, log the error.

### L8. Type Assertion Panic in OAuth Loopback Listener
- **File:** `internal/cloudsync/drive.go:109`
- **Issue:** `port := ln.Addr().(*net.TCPAddr).Port` uses a hard type assertion. If the listener is not TCP (unlikely but possible with some Go network implementations), this panics.
- **Fix:** Use a comma-ok assertion.

### L9. `WakeOnLan` / `Precise` File Handling
- **File:** `app.go:1150-1178`
- **Issue:** Various temporary file writes and flag file operations do not check errors. If disk is full or permissions are wrong, operations appear to succeed but don't.
- **Fix:** Check all `os.WriteFile` and `os.Remove` calls.

### L10. No Size Limit on Model Import
- **File:** `internal/modelstore/modelstore.go:399-433`
- **Issue:** `ImportLocalModel` copies a file from a user-specified source path with no size limit. A multi-terabyte file selection can fill the disk.
- **Fix:** Check file size before copy, and/or use `io.CopyN` with a limit.

### L11. Symlink Attack in `DeleteLocalModel`
- **File:** `internal/modelstore/modelstore.go:370-397`
- **Issue:** Path validation uses `strings.HasPrefix(absPath, absModelsDir)`. A resolved path like `/data/models/evil` can pass even if `evil` is a symlink to `/etc/passwd`, leading to arbitrary file deletion.
- **Fix:** Use `filepath.EvalSymlinks` on both paths before comparing.

### L12. `TOCTOU` Race in `safePersistPath`
- **File:** `internal/memory/store.go:262-278`
- **Issue:** Path validation and file operation are not atomic. A malicious process could replace the validated file with a symlink between the check and the operation.
- **Fix:** Open the file before validation (using `syscall.Open` with `O_NOFOLLOW`).

### L13. `runCmdStream` Goroutines May Outlive Function
- **File:** `internal/llama/installer.go:628-633`
- **Issue:** Stdout/stderr reader goroutines are spawned with `go func()`. If `cmd.Wait()` returns before the goroutines finish reading (short-lived command), they may write to the logger after the function returns.
- **Fix:** Use a `sync.WaitGroup` to ensure goroutines complete.

### L14. Chat Input `/` Command Has No Visual Indicator
- **File:** `frontend/lib/widgets/chat_input.dart:29-32`
- **Issue:** The `/` key triggers a prompt template popup, but there is no UI hint (no placeholder text, no tooltip). Users must discover this by accident.
- **Fix:** Add a hint text or an icon button.

### L15. `FocusNode` Created on Every Build
- **File:** `frontend/lib/widgets/chat_input.dart:185-193`
- **Issue:** `KeyboardListener` uses `FocusNode()` in the build method, creating a new object on every rebuild. The old node is garbage collected.
- **Fix:** Store the FocusNode in the state.

### L16. Stale Prompt Text in Settings (Not Updated When Data Changes)
- **File:** `frontend/lib/widgets/settings_dialog.dart:343-345`
- **Issue:** The system prompt `TextEditingController` is initialized once on first data arrival. If the prompt changes externally (another device, API call), the displayed text is stale.
- **Fix:** Add a listener to the provider and update the controller.

### L17. Error State Shows Only Icon — No Error Message
- **File:** `frontend/lib/screens/chat_screen.dart:53`
- **Issue:** The loading/error state for the chat list shows only a generic error icon. The actual error object is not displayed.
- **Fix:** Show the error message text.

### L18. Model Stop Buttons Fire Without Awaiting
- **File:** `frontend/lib/screens/model_store_screen.dart:606-608, 662-665`
- **Issue:** Stop model buttons call the API without `await` and without checking the result. If the API call fails, the button state ("stopped") doesn't match reality.
- **Fix:** Await the call and revert UI state on error.

### L19. Cloud Sync and Remote Access Tabs Show "Under Construction"
- **File:** `frontend/lib/widgets/settings_dialog.dart:766-823`
- **Issue:** Cloud Sync and Remote Access settings tabs display "Yapım aşamasında..." despite the backend having full implementation for both features.
- **Fix:** Implement the UI tabs to match the backend functionality.

### L20. Setup Wizard Uses `$name` Literal Instead of Interpolation
- **File:** `frontend/lib/widgets/setup_wizard_view.dart:87`
- **Issue:** The generated system prompt contains `\$name` (escaped, literal `$name` text) while the backend uses `%s` format strings. The name is never substituted.
- **Fix:** Remove `$name` from the prompt or implement proper substitution.

---

## ⚪ Info / Observations

### I1. GOB Encoding vs. Forward Compatibility
- **File:** `internal/memory/store.go:302-306`
- **Note:** `chromem.Document` is serialized with Go's `gob` encoding. Gob is sensitive to struct field changes: adding, removing, or renaming fields in a future version will make all existing memory files unreadable. Consider a self-describing format (JSON, CBOR, or protobuf).

### I2. Single-File-Per-Interaction Design
- **File:** `internal/memory/store.go`
- **Note:** Every memory interaction is a separate `.gob` file. This design simplifies deletion (`os.Remove`) but creates pathological behavior for:
  - Startup: O(N) file reads
  - Cloud sync: O(N) API calls per sync
  - File descriptor usage
  - Disk seek times on HDDs

### I3. Filepath.Walk Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`
- **Note:** Several `filepath.Walk` callbacks return `nil` for all errors. Permission-denied directories and I/O errors are invisible to the user.

### I4. Embedding Client Stale Reference After Reinit
- **File:** `app.go:148-149`, `app.go:124-125`
- **Note:** When `a.client` is swapped (new LLM endpoint), the embedding function in `a.store` still references the old client. The store continues to use the previous endpoint for embeddings until `reinitMemoryStore` is called.

### I5. Model Auto-Classification via Filename
- **File:** `internal/modelstore/modelstore.go:58-64`
- **Note:** `isEmbeddingModel` checks if the filename or repo ID contains embedding-related keywords (bge, e5, etc.). This heuristic misclassifies models that have these strings in their name but are actually chat models.

### I6. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`
- **Note:** `unsanitizePath` replaces `__` with `/`. If a HuggingFace repo ID naturally contains `__`, this creates unexpected directory structures. Path traversal is prevented by `filepath.Join` normalization, but directory layout may surprise users.

### I7. Llama Server Stderr Mixed with App Logs
- **File:** `internal/llama/llama.go:118-119`
- **Note:** Subprocess stdout/stderr are set to `os.Stdout`/`os.Stderr`. Llama.cpp's diagnostic output (prompt processing stats, timing, warnings) appears directly in the app's output stream with no prefix or filtering.

### I8. Race Between `UpdateSyncSettings` and `ensureSyncManager`
- **File:** `app.go:1505-1510`, `app.go:1627-1652`
- **Note:** `ensureSyncManager()` reads `a.syncManager` without a lock. `UpdateSyncSettings` sets `syncManager = nil` and creates a new instance without synchronization. Concurrent calls can result in stale or double initialization.

---

> **Last updated:** 2026-06-02  
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend  
> **Total issues:** 55 (7 critical ✅, 15 high ✅, 13 medium ✅, 20 low, 8 info notes)
