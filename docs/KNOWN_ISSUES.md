# Known Issues & Technical Risks

This document tracks all currently open bugs and technical risks in the Memo project.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🔵 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- ⚪ **Low** — cosmetic, minor improvement, or edge case
- ⚫ **Info** — design note, risk, or observation

---

## ✅ Fixed Bugs (2026-06-13)

### F1. Flutter: Empty `catch (_)` Blocks Swallowing Errors
- **Previously:** 25+ `catch (_) {}` blocks silently swallowed all errors.
- **Fix:** All catch blocks now log with `debugPrint`.
- **Impact:** Users now see error messages when operations fail.

### F2. WhatsApp SSE Handler Not Monitoring `ctx.Done()`
- **Previously:** `for chunk := range streamCh` without ctx.Done() caused goroutine leaks.
- **Fix:** Changed to `select { case <-ctx.Done(): return; ... }`.
- **Impact:** No more orphaned goroutines on client disconnect.

### F3. Agent Permission Revoke Sending Wrong ID
- **Previously:** `revoke(p.argsHash)` sent wrong identifier.
- **Fix:** Added `id` field, changed to `revoke(p.id)`.
- **Impact:** Permission revocations now actually work.

### F4. Provider Connection Test Silently Returning `false`
- **Previously:** `catch (_) { return false; }` hid error details.
- **Fix:** Return type changed to `Map` with `error` field.
- **Impact:** Users see why provider test fails.

### F5. WhatsApp Stream: No Error Handling or Cancellation
- **Previously:** No `try/catch` or `CancelToken` on WhatsApp stream.
- **Fix:** Added error handling and cancellation support.
- **Impact:** WhatsApp no longer crashes on network errors.

### F6. Chat Stream Deadlock After Error
- **Previously:** `_stopped` never reset on error, locking streaming permanently.
- **Fix:** Added `_stopped = false` in catch block.
- **Impact:** Streaming recovers after errors.

### F7. Active Provider Not Visible in Settings UI
- **Previously:** No indicator of which provider is active.
- **Fix:** Added `ACTIVE` badge and green border to active provider card.
- **Impact:** Users can see which provider is answering.

### F8. Model/Embedding Status Infinite Polling
- **Previously:** `modelStatusProvider` and `embeddingStatusProvider` polled forever.
- **Fix:** Added `autoDispose` — stops when screen is closed.
- **Impact:** 34k+ unnecessary HTTP requests/day eliminated.

### F9. `handleSendFileStream`: Temp File Leak + MIME Panic
- **Previously:** Temp file never cleaned up; `mimeType[:5]` panicked on short MIME.
- **Fix:** Added `defer os.Remove`, changed to `strings.HasPrefix`.
- **Impact:** No temp file leak. No crash on custom MIME types.

### F10. Connection Status Provider No Error Logging
- **Previously:** `catch (_) { yield false; }` silently swallowed errors.
- **Fix:** Added `debugPrint` to catch block.
- **Impact:** Connection errors are now visible in logs.

### F11. HTTP Request Body Size Limits (CR02)
- **Previously:** Global limit was 10MB, breaking file uploads >10MB.
- **Fix:** Increased limit to 50MB to match file upload handlers.
- **Impact:** File uploads up to 50MB now work.

### F12. Agent Sandbox: Shell Interpreters Blacklisted (CR04)
- **Previously:** `sh`, `bash`, `zsh`, `dash` not blocked — sandbox escape via `sh -c "rm -rf /"`.
- **Fix:** Added shell interpreters to command blacklist.
- **Impact:** Sandbox can no longer be escaped via shell interpreters.

### F13. `callLLMStream` Goroutine Leak on Client Disconnect (H14)
- **Previously:** `for chunk := range ch` blocked until provider timeout (300s).
- **Fix:** Converted both provider and local model loops to `select` with `ctx.Done()`.
- **Impact:** Goroutine exits immediately when client disconnects.

### F14. Data Races on Provider Router, Active Provider, Sessions (CR03)
- **Previously:** Unprotected reads/writes on `providerRouter`, `activeProvider`, `sessions`, `syncManager`.
- **Fix:** Added mutex guards (`providerMu`, `sessionsMu`) and fixed TOCTOU race in sync functions.
- **Impact:** No more data races on concurrent reconfiguration.

### F15. SSE Stream Stall Timeout in Provider (H06)
- **Previously:** `await for` in `chat_provider.dart` blocked indefinitely on stalled streams.
- **Fix:** Added 60s `.timeout()` on stream consumption.
- **Impact:** Stalled streams now show timeout error instead of hanging forever.

### F16. Agent Permission Cards Show Human-Readable Names
- **Previously:** Cards showed raw tool names like `write_file`, `run_command`.
- **Fix:** Added `tool_names.dart` with Turkish display names, descriptions, and icons.
- **Impact:** Users now see "Dosya Yaz" instead of `write_file`, with clear descriptions.

### F17. TOCTOU Symlink Race in `DeleteLocalModel` (CR01)
- **Previously:** Time window between `filepath.EvalSymlinks` and `os.Remove` allowed symlink replacement attack.
- **Fix:** Added re-resolution via `filepath.EvalSymlinks` + re-validation right before `os.Remove`.
- **Impact:** TOCTOU window eliminated — path is re-verified at the moment of deletion.

---

## 🔴 Critical

_None. All critical bugs have been fixed._

---

## 🟠 High

### H01. Provider Priority Field Unused
- **File:** `internal/provider/config.go`, `router.go:188-204`
- **Detail:** `ProviderConfig.Priority` field exists but `getActiveEntries()` returns providers in insertion order (Go slice append order), not by priority. The priority field is never sorted or consulted during routing.

### H02. No Agent API Methods in Frontend ApiClient
- **File:** `frontend/lib/core/api_client.dart`
- **Detail:** Backend has fully working agent endpoints but frontend `api_client.dart` has no methods to call them. Agent mode cannot be toggled from UI.

### H03. Download Progress Polling Never Stops
- **File:** `frontend/lib/providers/models_provider.dart:71-87`
- **Detail:** `downloadProgressProvider` has an infinite `while (true)` loop. It polls `/api/models/download/progress` every 1 second for the **entire app lifetime**, even when no download is active. 1 HTTP request/second forever.
- **Risk:** Wasted CPU/network, battery drain on laptops.

### H04. ngrok Binary Download Has No Integrity Check
- **File:** `internal/ngrok/installer.go:34-91`
- **Detail:** The ngrok binary is downloaded over HTTPS but no SHA256 checksum verification is performed. The download URL (`bin.ngrok.com/c/bNyj1mQVY4c/`) contains a static API key. If the CDN is compromised or the download is intercepted, a malicious binary could be installed.
- **Risk:** Arbitrary code execution via compromised ngrok binary.

### H05. WhatsApp SQL LIKE Injection in `SearchMessages`
- **File:** `internal/whatsapp/store.go:107-138`
- **Detail:** User input is wrapped with `"%" + query + "%"` for a LIKE pattern. If `query` contains `_` (single-character wildcard in SQL LIKE), it matches unintended rows. Example: query `"test_"` would match `"test1"`, `"testX"`, etc.
- **Risk:** Incorrect search results when messages contain underscores.

### H06. Flutter: Global Style Cache (`_styleCache`) Memory Leak
- **File:** `frontend/lib/widgets/chat_message_list.dart:13`
- **Detail:** `_styleCache` is a global mutable `Map` that is never cleared. It grows indefinitely with every unique combination of theme brightness and accent color visited. A memory leak proportional to theme configurations visited.

### H07. Flutter: `connectionStatusProvider` Infinite Polling
- **File:** `frontend/lib/providers/chat_provider.dart:429-438`
- **Detail:** `connectionStatusProvider` runs a `while(true)` polling loop every 30 seconds for the entire app lifetime. No `autoDispose` variant used. Runs even when sidebar is hidden.

### H08. Orchestra Config Has No Validation
- **File:** `internal/orchestra/conductor.go:120` (`UpdateConfig`)
- **Detail:** `UpdateConfig` accepts any role configuration without validation. An invalid chief model or missing role model causes runtime error during execution rather than at config time.

### H09. Agent Pipeline No Timeout Per Tool Call
- **File:** `internal/agent/pipeline.go:122-222`
- **Detail:** Individual tool executions have no timeout enforced by the pipeline. A hanging `run_command` blocks the entire pipeline indefinitely (sandbox has 60s timeout but pipeline doesn't enforce it).

### H10. Agent Audit Log Limited to 1000 Entries
- **File:** `internal/agent/executor.go:40-45`
- **Detail:** `logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.

### H11. Mobile API Client Missing Most Backend Endpoints
- **File:** `mobile/lib/core/api_client.dart`
- **Detail:** Mobile API client lacks: `sendFileStream`, `exportChat`, `generateTitle`, `updateMessage`, `deleteMessage`, `getSystemPrompt`, memory settings, model search/download, WhatsApp, sync, remote access, backup/restore, recording, image endpoints.

### H12. Data Race on `a.client` During `StartLocalModel`/`StopLocalModel`
- **File:** `app.go` (`StartLocalModel`, `StopLocalModel`)
- **Detail:** `a.client` (llama.cpp API client) is reassigned during model start/stop. `clientMu` exists but concurrent streaming requests using the old client while it is being swapped could fail or observe inconsistent state.
- **Risk:** Unexpected errors or hangs during model switching.

### ~~H13. Flutter: Model/Embedding Status Providers Infinite Polling~~ ✅ FIXED
- ~~`frontend/lib/providers/models_provider.dart:34-54`~~
- ~~`modelStatusProvider` and `embeddingStatusProvider` run infinite `while(true)` polling loops every 5 seconds for the entire app lifetime. Added `autoDispose` — now stops when model store screen is closed.~~

---

## 🔵 Medium

### M01. Flutter: `prefsProvider` Throws `UnimplementedError`
- **File:** `frontend/lib/providers/settings_provider.dart:9-11`
- **Detail:** `prefsProvider` throws `UnimplementedError()`. Intentional (overridden in `main()`) but if any code path accesses it before `main()` provides the override, the app crashes with a confusing error.

### M02. Flutter: L10n Uses Custom Listener Pattern Instead of Riverpod
- **File:** `frontend/lib/core/l10n.dart:8`
- **Detail:** Custom listener pattern for locale change notification exists in parallel with Riverpod — violates the project's single-state-management principle.

### M03. Flutter: Side-Effect in `app_shell.build()`
- **File:** `frontend/lib/screens/app_shell.dart:34-36`
- **Detail:** `_currentIndex = 0` mutable field assignment inside `build()` method. Violates Flutter's build purity rules and won't trigger `setState`.

### M04. Flutter: `IndexedStack` Keeps All Screens Alive Unnecessarily
- **File:** `frontend/lib/screens/app_shell.dart:46-57`
- **Detail:** `IndexedStack` keeps `ChatScreen`, `AgentScreen`, `ModelStoreScreen` alive simultaneously. All three providers are active at once, causing redundant network calls.

### M05. Flutter: `_StreamingBubble` Unnecessary Timestamp Rebuild
- **File:** `frontend/lib/widgets/chat_message_list.dart:496`
- **Detail:** `Text(DateTime.now().toIso8601String().substring(11, 16))` evaluates on every rebuild — updates timestamp every frame during streaming.

### M06. Flutter: `_showEditDialog` Post-Dispose Callback Risk
- **File:** `frontend/lib/widgets/chat_message_list.dart:206-236`
- **Detail:** `_showEditDialog` creates `TextEditingController` but does not cancel the dialog subscription. If widget is disposed while dialog is open (tab switch), the dialog completion callback runs after disposal.

### M07. Flutter: Chat Sidebar External Edit Race
- **File:** `frontend/lib/widgets/chat_sidebar.dart:177-181`
- **Detail:** `_editController.text` updated only when `!_isEditing`. If chat title changes externally while user is editing, the user's edits are overwritten on next `didUpdateWidget`.

### M08. Agent Permission Dialog Has No Timeout
- **File:** `frontend/lib/widgets/chat_input.dart:400-418`
- **Detail:** `_startOpenRouterOAuth` shows a loading dialog with no timeout. If the API call hangs, the dialog blocks the UI forever.

### M09. Flutter: API Keys Visible in Plaintext in Provider Config Dialog
- **File:** `frontend/lib/widgets/provider_config_dialog.dart`
- **Detail:** Provider config dialog shows API keys in `TextField`. If someone is watching the screen (screen recording, screenshot, shoulder surfing), API keys are exposed.

### M10. No Tests for Provider/Agent/Orchestra Packages
- **File:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Detail:** Zero unit tests exist for the three packages (~4700 lines of production code). (Was ~4150 lines in previous audit; grew with new additions.)

### M11. Orchestra Config File Written with 0644 Permissions
- **File:** `internal/orchestra/conductor.go:114`
- **Detail:** Orchestra config JSON file is world-readable (`0644`). While it doesn't contain API keys, it leaks configuration details.

### M12. Agent Permissions File Written with 0644 Permissions
- **File:** `internal/agent/permissions.go:229`
- **Detail:** Agent permissions file (`permissions.json`) is world-readable (`0644`).

### M13. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`
- **Detail:** Double underscores `__` in HuggingFace repo IDs are converted to `/` in file paths. A malicious repo ID (`foo__..__bar`) could cause directory traversal.

### M14. Model Auto-Classification via Filename Heuristic
- **File:** `internal/modelstore/modelstore.go:58-64`
- **Detail:** `isEmbeddingModel` searches for "embedding" substring in filename. Regular models containing "embedding" in the name are misclassified.

### M15. `filepath.Walk` Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:374-376`
- **Detail:** In `filepath.Walk` callbacks, non-nil errors are silently skipped with `return nil`.

### M16. Embedding Client Stale Reference After Reinit
- **File:** `app.go:143-165` (App struct embeddingClient), `app.go:148-149`
- **Detail:** When the embedding model is restarted, the `embeddingClient` reference is not updated. The old client reference may point to a closed connection.

### M17. Memory Store: No Size Limit on User Messages
- **File:** `internal/memory/store.go` (various insert paths)
- **Detail:** User messages of any size can be stored as memory. Embedding models typically have token limits (e.g., 512 tokens). No truncation is performed before storing.
- **Risk:** Embedding failure or oversized memory entries wasting storage.

### M18. Identity System Prompt Can Bloat with Large Memory Context
- **File:** `internal/identity/identity.go:26-49`
- **Detail:** `BuildSystemPrompt` appends all retrieved memories directly into the system prompt. With many high-similarity memories, the prompt can grow arbitrarily large, exceeding the model's context window. No truncation or size limit.

### M19. Orchestra: No Self-Referencing Role Loop Detection
- **File:** `internal/orchestra/conductor.go` (`callModel`)
- **Detail:** A role's model endpoint pointing back to the Memo app itself (or creating a loop via another service) enables infinite recursion. No cycle detection.

### M20. Cloud Sync: Interrupted Upload Leaves Partial File
- **File:** `internal/cloudsync/drive.go`
- **Detail:** If an upload is interrupted partway, the cloud destination contains a partial/corrupt file. No cleanup or partial upload abort on error.

---

## ⚪ Low

### L01. `api/streaming.go`: `scanner.Err()` Is Checked (Confirmation)
- **File:** `internal/api/streaming.go:127-129`
- **Note:** `scanner.Err()` IS checked after the scan loop. This is a cross-check, not a bug.

### L02. Provider Router: Spawned Goroutines Accumulate on Rapid Fallback
- **File:** `internal/provider/router.go`
- **Detail:** In the fallback chain, each timed-out provider spawns a goroutine. If all providers fail quickly, multiple short-lived goroutines accumulate briefly.

### L03. Sessions: No Limit on Loaded Sessions
- **File:** `internal/sessions/sessions.go`
- **Detail:** All sessions are loaded into memory at startup. Hundreds of large message histories can consume significant memory.

### L04. SQLite `MaxOpenConns` at Default (Unlimited)
- **File:** `internal/database/sqlite.go:50-53`
- **Detail:** SQLite in WAL mode can handle multiple concurrent readers, but `SetMaxOpenConns` default is 0 (unlimited). While `MaxPool` is set to 1 for writes, the underlying `sql.DB` has no connection limit.

### L05. `doDownload` Temp File Cleanup Logic Fragile
- **File:** `internal/modelstore/modelstore.go:255-262`
- **Detail:** Deferred cleanup calls `os.Remove(tmpPath)` even after `os.Rename` succeeds (tmpPath no longer exists, error silently swallowed).

### L06. `ngrok/installer.go`: Archive Extraction Overwrites Without Check
- **File:** `internal/ngrok/installer.go:93-122, 124-154`
- **Detail:** TGZ/ZIP extraction does not check if target file already exists. Concurrent downloads could corrupt files.

### L07. Flutter: `unawaited(future)` Warning
- **File:** `frontend/lib/widgets/agent/permission_dialog.dart:51`
- **Detail:** The `Future` from `handleAgentPermission` is explicitly `unawaited`. If the request fails, the error is silently lost.

### L08. Flutter: `ThemeColors.lerp()` Null-Forging Operator Risk
- **File:** `frontend/lib/core/theme.dart:65-80`
- **Detail:** `Color.lerp` uses null-forgiving `!` operator. If `other` is not null but its fields are, the `!` causes a crash.

### L09. Flutter: `version_banner.dart` `addPostFrameCallback` on Every Rebuild
- **File:** `frontend/lib/widgets/version_banner.dart:115-119`
- **Detail:** `WidgetsBinding.instance.addPostFrameCallback` called inside `build()`. Each rebuild registers a new callback. Since `_AnimatedBanner` is a `StatelessWidget`, it rebuilds on every parent rebuild.

### L10. Mobile: Default URL Leaks Home Network IP
- **File:** `mobile/lib/core/api_client.dart:68`
- **Detail:** Default URL hardcoded to `http://192.168.1.100:8090` — leaks the developer's internal network topology.

### L11. `unsanitizePath` 64-bit Integer Overflow Risk
- **File:** `internal/modelstore/modelstore.go` (relevant lines)
- **Detail:** 64-bit integer conversions for repo IDs could overflow for very large numbers.

---

## ⚫ Info / Observations

### I1. `App.ctx` Stored in Struct (Anti-pattern)
- **File:** `app.go:227`
- **Note:** Per Go `context` documentation, contexts should be passed explicitly, not stored in structs.

### I2. Legacy GOB Format (Migrated to SQLite)
- **File:** `internal/memory/store.go`
- **Note:** Memory store still supports legacy GOB file format migration path.

### I3. Single-File-Per-Interaction Design
- **File:** `internal/memory/store.go`
- **Note:** Memory store still supports the old per-interaction file model for backward compatibility.

### I4. Flutter: Hardcoded Turkish Strings Bypass L10n
- **File:** `frontend/lib/widgets/agent/permission_dialog.dart:149`, `agent_chat_card.dart:19,25,30,75,81`, `permission_history.dart`, `chat_input.dart`
- **Note:** Various widgets still contain hardcoded Turkish text, bypassing the L10n system.

### I5. Missing `const` Constructors (Widespread)
- **Note:** Across the entire Flutter codebase. Every non-const widget allocation creates a new object on every rebuild, increasing GC pressure.

### I6. Llama Server Stderr Mixed with App Logs
- **File:** `internal/llama/llama.go:125-126`
- **Note:** `s.cmd.Stderr = os.Stderr` — llama.cpp stderr is mixed with application logs, not separated.

### I7. Memory Store Full Rebuild on Every Startup
- **File:** `internal/memory/store.go`
- **Note:** `LoadCache` is O(N) with no incremental indexing. Startup time grows linearly with memory count.

### I8. Duplicate Type Definitions in `models/memory.go` and `memory/store.go`
- **File:** `internal/models/memory.go`, `internal/memory/store.go`
- **Note:** `models.MemoryResult` and `memory.MemoryResult` are duplicate type aliases serving the same purpose. Should be consolidated.

### I9. `stt_proc_windows.go`: `sttSetProcessGroup` is No-Op
- **File:** `stt_proc_windows.go:7`
- **Note:** On Windows, `sttSetProcessGroup` is empty; `sttKillProcessGroup` only kills the immediate process, not children. Orphaned child processes may remain.

### I10. Provider `defaultMachineKey()` Has Hardcoded Fallback Key
- **File:** `internal/provider/config.go:380`
- **Note:** When hardware ID cannot be obtained, hardcoded `"Mm3m0L0c4lK3y!@#$%^&*()9876543210"` is used. This means API keys can be decrypted by anyone who knows this fallback.

### I11. Cloud Sync: Hardware ID Fallback Hardcoded
- **File:** `internal/cloudsync/crypto.go:145`
- **Note:** When hardware ID cannot be obtained, `"memo-fallback-key"` is used. Sync encryption fundamentally relies on this key.

### I12. Provider `defaultMachineKey()` Executes Shell Command on macOS
- **File:** `internal/provider/config.go:371-377` (macOS path)
- **Note:** Calls `sh -c "ioreg ..."` for hardware ID — assumes `sh` binary is available, may fail in minimal environments.

### I13. `patch_interfaces.go` is an Empty Dummy File
- **File:** `patch_interfaces.go`
- **Note:** File contains only `package main` and a comment — appears to be a dummy file for a build script.

### I14. `go 1.25.0` — Unreleased Go Version
- **File:** `go.mod:4`
- **Note:** `go 1.25.0` has not been released (current stable: 1.24.x). May cause build issues.

### I15. Agent WhatsApp Tool Bypasses AppBridge
- **File:** `internal/agent/tools/whatsapp.go`
- **Note:** The WhatsApp tool accesses the WhatsApp store directly rather than through the App bridge. This bypasses access controls and audit logging in the App layer.

---

> **Last updated:** 2026-06-13
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend
> **Open bugs:** 23+ (🔴0, 🟠12, 🔵9, ⚪2)
> **Observations:** 15
> **Fixed:** 17
> **Total issues found:** 55+
