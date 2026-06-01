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

### C1. Orphaned SSE Streams — LLM Continues After Client Disconnect
- **File:** `internal/webserver/handlers_flutter.go:39-61`, `internal/api/streaming.go`
- **Issue:** When a client disconnects during SSE streaming, the `request.Context().Done()` channel is not monitored. The LLM backend goroutine continues running `ChatCompletionStream` until the model finishes (up to 300s). The context passed to the LLM API (`streamCtx`) is derived from the HTTP request context but is not properly cascaded to the llama.cpp subprocess.
- **Impact:** GPU/CPU resources are wasted on every disconnected stream. On low-end hardware, an orphaned stream blocks new requests because llama.cpp is single-threaded for generation.
- **Fix:** Wire `request.Context()` into the LLM call chain; add a `select` on `ctx.Done()` in the SSE write loop.

### C2. Engine Mode / Config Update Resets All Llama Settings to Zero
- **File:** `frontend/lib/providers/settings_provider.dart:63-73` ←→ `internal/webserver/handlers_flutter.go:667-685`, `app.go:1148-1151`
- **Issue:** The frontend sends a partial JSON body (e.g. `{"engine_mode": "cpu"}`) to the `/api/config/llama` endpoint. The Go handler unmarshals this into `config.LlamaConfig{}` — any field absent from JSON is zero-valued (empty string, 0, false). Then `UpdateLlamaConfig` does `a.cfg.Llama = cfg`, replacing the **entire** struct. All existing settings (binary path, port, context size, GPU layers, model path) are silently erased.
- **Impact:** After changing engine mode, the user must reconfigure every llama setting. The app may fail to start any model until all fields are manually re-entered.
- **Fix:** Implement a merge/patch strategy: only overwrite fields present in the JSON body, or make the handler return current values for omitted fields.

### C3. Arbitrary File Read via `/api/image`
- **File:** `app.go:866-871` (GetImageBase64), `internal/webserver/handlers_flutter.go:215-227` (handleImage)
- **Issue:** The `/api/image?path=...` endpoint accepts an arbitrary file path, reads the file, and returns it as base64. No path sanitization beyond a basic `filepath.Clean`. An attacker can read `/etc/passwd`, `~/.ssh/id_rsa`, or any world-readable file.
- **Impact:** Full local file disclosure. On remote-access-enabled instances (0.0.0.0 binding), this is remotely exploitable.
- **Fix:** Restrict reads to the app's data directory and/or a whitelist of allowed paths.

### C4. Remote Access Server — No Authentication, Wide-Open CORS
- **File:** `internal/webserver/server.go:144`, `internal/webserver/server.go:507` + related handlers
- **Issue:** The remote access mode (`Start()`) binds to `0.0.0.0:<port>`, sets `Access-Control-Allow-Origin: *`, and has no authentication on any endpoint. The only "protection" is a plaintext passphrase sent on every request with no session/token mechanism. Full chat history, model control, file access, and memory manipulation is available to anyone on the network.
- **Impact:** Complete remote compromise of the application and its data.
- **Fix:** Implement proper authentication (JWT/session token), HTTPS enforcement, and disable wildcard CORS on remote bind.

### C5. `a.client` Reassignment Without Synchronization
- **File:** `app.go:1106`, `app.go:1123`, `app.go:1216`, `app.go:1228`
- **Issue:** `a.client` and `a.embeddingClient` are reassigned (new LLM endpoint, stop model sets to nil) without any mutex. Concurrent calls to `ChatCompletion`, `ChatCompletionStream`, or `CreateEmbedding` can read a half-assigned or nil client pointer.
- **Impact:** Potential nil-pointer panic in production under concurrent request load. Random `connection refused` errors when client is being swapped.
- **Fix:** Guard all client reads/writes with `a.mu` (or a dedicated `clientMu`).

### C6. `saveMemoryAsync` RLock→Lock Pattern (Deadlock Risk)
- **File:** `app.go:1399-1421`
- **Issue:** `saveMemoryAsync` acquires `storeMu.RLock()` at line 1394 (deferred RUnlock at 1395), then spawns a goroutine at line 1399 that tries to acquire `storeMu.Lock()` at line 1404. The function returns almost immediately, so the deferred RUnlock fires quickly, and the goroutine can proceed. However, if `saveMemoryAsync` is called while `storeMu` is already write-locked (e.g. during `reinitMemoryStore`), the RLock blocks. The goroutine then queues behind the RLock. If two goroutines call `saveMemoryAsync` concurrently, the first holds RLock, the second also takes RLock (readers don't block each other), then both goroutines try Lock() — one succeeds, the other blocks until the first finishes. This is fragile and confusing.
- **Impact:** Hard-to-debug hangs under concurrent memory save + reinit scenarios.
- **Fix:** Use a channel-based worker goroutine for async memory saves instead of the lock-then-goroutine pattern.

### C7. UI Thread Performance — AnimationController Per Message
- **File:** `frontend/lib/widgets/chat_message_list.dart:92-99`
- **Issue:** Every `_MessageBubble` creates its own `AnimationController` in `initState()`. With 100 messages, 100 animation controllers are created, each driving a frame callback. Combined with `FadeTransition` + `SlideTransition` wrapping every bubble, this causes severe jank on scroll and during streaming token updates.
- **Impact:** Choppy scrolling on chats with 50+ messages; noticeable frame drops on mid-range devices.
- **Fix:** Remove entry animations from message bubbles, or use a shared animation controller with staggered delays.

---

## 🟠 High

### H1. Goroutine Leak on SSE Disconnection
- **File:** `internal/webserver/handlers_flutter.go:39-61`
- **Issue:** The SSE handler starts a `ChatCompletionStream` LLM call but does not monitor `request.Context().Done()`. If the client disconnects (navigates away, closes tab), the goroutine calling `stream.ProcessSSEStream` and the underlying LLM request continue running until completion. Every disconnection leaks one goroutine and one HTTP connection to llama.cpp.
- **Impact:** Accumulated goroutine leaks over time; resource exhaustion on the server.
- **Fix:** Add a `select { case <-ctx.Done(): return; case ... }` pattern.

### H2. Config File World-Readable (`0644`) Containing Secrets
- **File:** `internal/config/config.go:178`
- **Issue:** `config.yaml` is written with `os.WriteFile` and `0644` permissions. The file contains `client_secret` (OAuth), `passphrase` (sync encryption), and other sensitive values. On multi-user systems, any local user can read these secrets.
- **Impact:** OAuth token theft, encrypted sync data decryption.
- **Fix:** Write with `0600` and document manual chmod for existing installations.

### H3. Weak Key Derivation for Sync Encryption
- **File:** `internal/cloudsync/crypto.go:18-23`
- **Issue:** `deriveKey` uses `sha256.Sum256([]byte("memo-sync-v1:" + passphrase))` — a single SHA-256 iteration with a fixed salt string. This is trivially brute-forceable for weak passphrases (which most users choose). Industry standard is PBKDF2, bcrypt, or argon2id with a random salt.
- **Impact:** Sync data confidentiality relies on a weak KDF.
- **Fix:** Use `golang.org/x/crypto/pbkdf2` or argon2id with a random per-user salt stored alongside the data.

### H4. Hardcoded Fallback Encryption Key
- **File:** `internal/cloudsync/crypto.go:59-62`
- **Issue:** When `hardwareID()` cannot determine a machine ID (e.g. container without hostname), it falls back to the constant string `"memo-fallback-key"`. Every machine without a passphrase or machine-id uses the **exact same** encryption key.
- **Impact:** All such machines can decrypt each other's sync data.
- **Fix:** Require an explicit passphrase, or generate a random key on first sync and store it in the config.

### H5. `buildMessages` Mutates Session History Permanently
- **File:** `app.go:1308`
- **Issue:** `buildMessages` loops over `history []api.Message` and when injecting the system prompt, does `history[i] = api.Message{Role: "user", Content: systemPrompt + "\n\n" + history[i].Content}`. Since `history` is a slice referencing `sessions.Messages` (the session's own backing array), this permanently alters the stored session. After the first request, every subsequent request prepends the system prompt again, doubling it.
- **Impact:** After 2–3 requests, the accumulated system prompt injection causes context overflow and confuses the model.
- **Fix:** Copy the slice before mutating, or build a new slice.

### H6. `hash2hex` — Only 4 Bytes of SHA-256 (Collision Risk)
- **File:** `internal/memory/store.go:342-344`
- **Issue:** `hash2hex` returns `fmt.Sprintf("%x", h.Sum(nil)[:4])` — the first 4 bytes (32 bits) of SHA-256. With ~2^16 entries, birthday paradox gives a 50% collision probability. A collision silently overwrites the existing `.gob` file, losing a memory entry.
- **Impact:** Data loss under moderate memory store usage.
- **Fix:** Use at least 8 bytes (16 hex chars) or use the full hash.

### H7. `monitor()` Goroutine Access to `s.cmd` Outside Lock
- **File:** `internal/llama/llama.go:271-302`
- **Issue:** The `monitor()` goroutine checks `s.cmd == nil` at line 272 **outside** the lock, then calls `s.cmd.Wait()` at line 276. `Stop()` sets `s.cmd = nil` inside the lock. If the process has already exited and `Wait()` returns immediately, `Stop()` can acquire the lock and nil `s.cmd` during the window between the nil check and the `Wait()` call.
- **Impact:** Rare nil-pointer panic during rapid stop/start cycles of the LLM server.
- **Fix:** Move the nil check and `Wait()` call inside the lock, or use an atomic pointer.

### H8. Temp File Leak on Download Non-Cancellation Errors
- **File:** `internal/modelstore/modelstore.go:237-243`
- **Issue:** The deferred cleanup `os.Remove(tmpPath)` only runs when `ctx.Err() != nil` (context cancelled). If the download fails with an HTTP error or network timeout, the `.downloading` temporary file remains on disk indefinitely.
- **Impact:** Accumulated partial download files waste disk space.
- **Fix:** Remove `tmpPath` unconditionally in the defer, or always on error.

### H9. File Descriptor Leak in `extractTarGzToBin`
- **File:** `internal/llama/installer.go:433,437`
- **Issue:** Inside the tar extraction loop, `out.Close()` is called manually (not deferred). If `io.Copy` fails at line 432, the `continue` at line 437 skips the close, leaking a file descriptor.
- **Impact:** FD exhaustion on installers with partially corrupted archives.
- **Fix:** Use `defer out.Close()` before each file open, or restructure the loop.

### H10. `nvidia-smi` Errors Silently Ignored → 0 VRAM → 0 GPU Layers
- **File:** `internal/llama/gpu.go:71-86`
- **Issue:** `exec.Command("nvidia-smi", ...).Output()` — the error return is silently ignored. If `nvidia-smi` fails (not installed, permission denied, or driver issue), `output` is nil/empty. Parsing empty output yields `vram = 0`, which leads to `recommendedLayers = 0` — the model runs entirely on CPU even though a GPU might be present but misconfigured.
- **Impact:** Silent CPU fallback with no user-visible warning.
- **Fix:** Check the error from `Output()` and log/propagate a meaningful message.

### H11. OAuth `authDone` Channel Race
- **File:** `internal/cloudsync/drive.go:99-103`
- **Issue:** `StartAuthFlow` creates a new `authDone` channel with `make(chan struct{})`. If `WaitForAuth` is concurrently reading the previous channel, the swap causes it to wait forever on the old (already closed) channel while the new channel is never closed.
- **Impact:** Rare hang of the OAuth flow when starting authentication rapidly.
- **Fix:** Use a `sync.WaitGroup` or a single shared channel that is reset with a mutex.

### H12. `Shutdown(context.Background())` Can Block Indefinitely
- **File:** `internal/webserver/server.go:286`
- **Issue:** `s.srv.Shutdown(context.Background())` has no timeout. If an HTTP handler is stuck (e.g. blocked on LLM response), `Shutdown` waits forever.
- **Impact:** App hangs on graceful shutdown if a stream is active.
- **Fix:** Use `context.WithTimeout(ctx, 10*time.Second)`.

### H13. Session ID Truncated to 8 Hex Chars
- **File:** `internal/sessions/sessions.go:68`
- **Issue:** `uuid.New().String()[:8]` takes only the first 8 hex characters (32 bits). With ~10^5 sessions, birthday paradox gives ~1% collision probability. A collision silently overwrites an existing session file.
- **Impact:** Chat history loss.
- **Fix:** Use the full UUID or at least 16 hex characters.

### H14. Download Polling Stream Runs Forever
- **File:** `frontend/lib/providers/models_provider.dart:66-79`
- **Issue:** The `downloadProgressProvider` stream contains a `while (true)` loop that polls the backend every 1–3 seconds. This loop is never cancelled — it runs for the entire application lifetime. Even after the download completes (and the progress dialog is dismissed), the polling continues, making unnecessary HTTP requests every 3 seconds.
- **Impact:** Unnecessary network traffic and battery drain.
- **Fix:** Cancel the stream subscription when the download completes or the provider is disposed.

### H15. Backend Error Handling: Connection Error Shows "Installed"
- **File:** `frontend/lib/providers/models_provider.dart:97-104`
- **Issue:** `llamaInstalledProvider` catches network errors: if the error is a `connectionError`, it returns `true` (showing "installed"). For other errors, it returns `false` (showing the installer). This is inverted — when the backend is unreachable (connection refused), the user sees that everything is installed and cannot trigger re-installation.
- **Impact:** User cannot trigger llama installation when the backend is genuinely unreachable or has an issue.
- **Fix:** Return `null` or a distinct error state for connection errors vs. installation status.

---

## 🟡 Medium

### M1. Background Errors Never Reach the UI (Broken Event System)
- **File:** `app.go` (emitEvent is stubbed), all call sites
- **Issue:** `emitEvent` was designed for Wails and is now a no-op for Flutter. Background errors (cloud sync failures, embedding model load failures, download progress, auto-start failures) are only written to `server.log` and never shown to the user.
- **Impact:** Silent failures; user sees no feedback when sync breaks or embedding fails.
- **Fix:** Implement a server-sent event endpoint or poll endpoint for background status.

### M2. Session Files World-Readable (`0644`)
- **File:** `internal/sessions/sessions.go:236`
- **Issue:** Chat session JSON files are written with `0644` permissions. Full chat history is readable by any local user.
- **Impact:** Privacy leak on multi-user systems.
- **Fix:** Write with `0600`.

### M3. `save()` Errors Silently Discarded in Session Manager
- **File:** `internal/sessions/sessions.go:75,155`
- **Issue:** `newSession` and `AddMessage` call `m.save(s)` but discard the returned error. Session data silently fails to persist to disk.
- **Impact:** Chat history loss under disk-full or permission-error conditions without any warning.
- **Fix:** Log the error and/or return it to the caller.

### M4. `loadAll()` Silently Skips Corrupted Session Files
- **File:** `internal/sessions/sessions.go:252-258`
- **Issue:** In `loadAll`, individual file read errors and JSON decode failures are skipped with `continue` — no error is logged, and the user never knows some sessions were lost.
- **Impact:** Silent data loss on corruption.
- **Fix:** Log each skipped file.

### M5. SSE `[DONE]` Chunk Missing `FinishReason`
- **File:** `internal/api/streaming.go:65`
- **Issue:** The `[DONE]` sentinel chunk is emitted without a `finish_reason` field. The frontend has no way to distinguish a normal completion from a truncation (max tokens) or a stop sequence.
- **Impact:** Frontend cannot show "max tokens reached" or "stopped" indicators.
- **Fix:** Send `finish_reason` in the `[DONE]` chunk.

### M6. Synchronous Blocking Writes on Main Path
- **File:** `internal/sessions/sessions.go:155`, `internal/memory/store.go:105`
- **Issue:** Every message triggers a synchronous `json.MarshalIndent` + `os.WriteFile` for sessions, and a synchronous embedding computation + gob write for memory. These block the LLM response path.
- **Impact:** Increased response latency on slow disks or during embedding computation.
- **Fix:** Buffer writes with a debounce timer / async worker.

### M7. `LoadCache` Performance — O(N) Startup Time
- **File:** `internal/memory/store.go:72-90`
- **Issue:** `LoadCache` reads every `.gob` file from disk on startup and stores all embeddings in RAM. With 10,000+ memory entries, startup time and memory usage grow linearly.
- **Impact:** Slow startup; excessive RAM usage for large memory stores.
- **Fix:** Implement pagination, lazy loading, or an on-disk index (SQLite/bolt).

### M8. Brute-Force O(N) Vector Search
- **File:** `internal/memory/retriever.go`
- **Issue:** Memory search does linear scanning of all embedding vectors in RAM. Beyond 10,000 entries, search latency degrades noticeably.
- **Fix:** Use an approximate nearest neighbor (ANN) index or a vector database.

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

### M13. Export Chat Silently Swallows Errors
- **File:** `frontend/lib/screens/chat_screen.dart:170-178`
- **Issue:** Export chat has an empty `catch (_) {}` block. If the export fails (no active chat, permission denied, write error), the user gets zero feedback.
- **Impact:** Users think export succeeded when it silently failed.
- **Fix:** Show a SnackBar or dialog on error.

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
> **Total issues:** 55 (7 critical, 15 high, 13 medium, 20 low, 8 info notes)
