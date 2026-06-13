# Known Issues & Technical Risks

This document tracks all currently open bugs and technical risks in the Memo project.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🔵 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- ⚪ **Low** — cosmetic, minor improvement, or edge case
- ⚫ **Info** — design note, risk, or observation

---

## 🔴 Critical

### CR01. TOCTOU Symlink Race in `DeleteLocalModel`
- **File:** `internal/modelstore/modelstore.go:421-458`
- **Detail:** Time window between `filepath.EvalSymlinks` (line 431) and `os.Remove` (line 443). An attacker with write access to the models directory could replace the resolved path with a symlink to an arbitrary file between resolution and removal. The `strings.HasPrefix` guard (line 439) mitigates simple attacks but a determined attacker can exploit this race window.
- **Risk:** Arbitrary file deletion outside the models directory.
- **Category:** Security

### CR02. No Request Body Size Limits on Any HTTP Handler
- **File:** `internal/webserver/handlers_flutter.go` (~35 endpoints)
- **Detail:** Every HTTP handler reads `r.Body` without `http.MaxBytesReader` or any size limit. A client can send multi-gigabyte payloads, exhausting all server memory.
- **Risk:** Memory exhaustion DoS attack.
- **Category:** Security

### CR03. Data Race: `a.store` / `a.syncManager` / `a.client` Pointer Writes
- **File:** `app.go:143-186` (App struct), `app.go:240-263` (startup assignments), `handlers_flutter.go` (reconfigure handlers)
- **Detail:** `a.store`, `a.syncManager`, and `a.client` fields are reassigned on startup and reconfigure without proper mutex guards (`clientMu` exists for `a.client` but `store` and `syncManager` lack protection). Concurrent requests reading these pointers could observe partially-initialized or nil values.
- **Risk:** Crash or data corruption on concurrent access during reconfiguration.
- **Category:** Race Condition

### CR04. Agent Sandbox: Shell Interpreters Not Blacklisted
- **File:** `internal/agent/tools/command.go:23-44` (blacklist regexes), `internal/agent/permissions.go` (permission system)
- **Detail:** The command blacklist blocks dangerous tools like `rm`, `dd`, `mkfs`, `chmod`, `sudo` but does NOT block shell interpreters (`sh`, `bash`, `zsh`, `dash`). An attacker can call `sh -c "rm -rf /"` to bypass the `rm` blacklist entirely.
- **Risk:** Complete sandbox escape via shell interpreters.
- **Category:** Security

### CR05. Flutter: 25+ Empty `catch (_)` Blocks Swallow All Errors
- **File:** Across `frontend/lib/` — `providers/chat_provider.dart`, `providers/models_provider.dart`, `providers/agent_provider.dart`, `providers/whatsapp_provider.dart`, `providers/orchestra_provider.dart`, `providers/provider_provider.dart`, `providers/version_provider.dart`, `widgets/chat_input.dart`, `widgets/agent/permission_dialog.dart`, `widgets/agent/agent_chat_card.dart`, `widgets/setup_wizard_view.dart`, `widgets/agent/permission_history.dart`
- **Detail:** 25+ `catch (_) {}` blocks silently swallow ALL errors including `DioException`, `SocketException`, `FormatException`, `TypeError`. When a provider call fails, the user sees no error message — the app silently does nothing. Previously reported as 19; actual count is 25+.
- **Risk:** Users have zero feedback when operations fail (config save, model listing, agent toggle, provider test, WhatsApp toggle, etc.).

### CR06. WhatsApp SSE Handler Does Not Monitor `ctx.Done()`
- **File:** `internal/webserver/handlers_flutter.go:1311-1349` (`handleWhatsAppChatStream`)
- **Detail:** Unlike `handleSendStream` and `handleSendFileStream`, `handleWhatsAppChatStream` does NOT monitor `ctx.Done()` via a `select` statement. The `for chunk := range streamCh` loop blocks until the channel is closed. When the client disconnects, the goroutine (and underlying `WhatsAppChatStream`) continues running for the full 300s context timeout.
- **Risk:** Orphaned goroutines per disconnected client (~5 min each). DoS vector.
- **Category:** Resource Leak

### CR07. Sandbox: No Path Traversal Protection in Tool Arguments
- **File:** `internal/agent/sandbox.go:70-94` (`ValidatePath`), `internal/agent/tools/file.go`, `internal/agent/tools/edit.go`, `internal/agent/tools/search.go`
- **Detail:** The sandbox validates the working directory using `strings.HasPrefix` but does NOT validate file paths in tool arguments (`read_file`, `write_file`, `edit_file`, etc.). A tool like `write_file` given a relative path like `../../etc/passwd` can escape the sandbox directory.
- **Risk:** Sandbox escape via relative paths in tool arguments.
- **Category:** Security

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

### H06. Flutter: SSE Stream Has No Stall/Idle Timeout
- **File:** `frontend/lib/core/api_client.dart:38-87`, `600-654`
- **Detail:** `sendMessageStream` and `sendFileStream` have no idle/stall timeout. If the server stops sending data, the `await for` blocks indefinitely. The only safeguard is the 120s `receiveTimeout` at the Dio level, but a stalled stream that never errors will hold the connection forever.

### H07. Flutter: `sendWhatsAppChatStream` Has No Error Handling
- **File:** `frontend/lib/core/api_client.dart:846-864`
- **Detail:** `sendWhatsAppChatStream` has no `try/catch` or `DioException` handling at all. Any network error during WhatsApp streaming results in an unhandled exception.

### H08. Flutter: `sendWhatsAppChatStream` Missing `CancelToken` Parameter
- **File:** `frontend/lib/core/api_client.dart:846-864`
- **Detail:** Unlike `sendMessageStream` and `sendFileStream`, the WhatsApp stream method has no `CancelToken` parameter. WhatsApp streaming cannot be cancelled from the client side.

### H09. Flutter: Agent Permission Revoke API Mismatch
- **File:** `frontend/lib/widgets/agent/permission_history.dart:83-88`, `frontend/lib/providers/agent_provider.dart:58-62`
- **Detail:** `revoke(p.argsHash)` may send `argsHash` instead of the `id` parameter expected by the backend API (`DELETE /api/agent/permissions?id=...`). The expected identifier is ambiguous. Error is silently swallowed (empty catch).
- **Risk:** Permission revocations silently fail; user believes permissions are revoked when they are not.

### H10. Flutter: `testProvider()` Silently Returns `false`
- **File:** `frontend/lib/providers/provider_provider.dart:36-41`
- **Detail:** `testProvider()` returns `false` on ANY error with an empty catch block. Users testing provider connections get no feedback on WHY it failed (wrong key, network error, server error).

### H11. Flutter: Global Style Cache (`_styleCache`) Memory Leak
- **File:** `frontend/lib/widgets/chat_message_list.dart:13`
- **Detail:** `_styleCache` is a global mutable `Map` that is never cleared. It grows indefinitely with every unique combination of theme brightness and accent color visited. A memory leak proportional to theme configurations visited.

### H12. Flutter: `chat_provider.sendMessage()` Permanent Stream Deadlock
- **File:** `frontend/lib/providers/chat_provider.dart:260-263, 296`
- **Detail:** The `sendMessage()` catch block (line 296) does NOT call `_stopped = false`. If a stream errors after being stopped, `_stopped` remains `true`, permanently preventing all future messages from using the streaming path.
- **Risk:** Agent/streaming mode permanently locks; restart required.

### H13. Flutter: `connectionStatusProvider` Infinite Polling
- **File:** `frontend/lib/providers/chat_provider.dart:429-438`
- **Detail:** `connectionStatusProvider` runs a `while(true)` polling loop every 30 seconds for the entire app lifetime. No `autoDispose` variant used. Runs even when sidebar is hidden.

### H14. `handleSendFileStream`: Temp File Cleanup Issues
- **File:** `internal/webserver/handlers_flutter.go:72-159`
- **Detail:** Temp file created with `os.CreateTemp` has multiple issues:
  - Line 98-104: File closed twice (`defer tmpFile.Close()` + `tmpFile.Close()`)
  - No `defer os.Remove(tmpFilePath)` — temp file leaks on error
  - Line 65-66: No error check after `io.Copy` (partial upload silently succeeds)

### H15. Orchestra Config Has No Validation
- **File:** `internal/orchestra/conductor.go:120` (`UpdateConfig`)
- **Detail:** `UpdateConfig` accepts any role configuration without validation. An invalid chief model or missing role model causes runtime error during execution rather than at config time.

### H16. Agent Pipeline No Timeout Per Tool Call
- **File:** `internal/agent/pipeline.go:122-222`
- **Detail:** Individual tool executions have no timeout enforced by the pipeline. A hanging `run_command` blocks the entire pipeline indefinitely (sandbox has 60s timeout but pipeline doesn't enforce it).

### H17. Agent Audit Log Limited to 1000 Entries
- **File:** `internal/agent/executor.go:40-45`
- **Detail:** `logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.

### H18. Mobile API Client Missing Most Backend Endpoints
- **File:** `mobile/lib/core/api_client.dart`
- **Detail:** Mobile API client lacks: `sendFileStream`, `exportChat`, `generateTitle`, `updateMessage`, `deleteMessage`, `getSystemPrompt`, memory settings, model search/download, WhatsApp, sync, remote access, backup/restore, recording, image endpoints.

### H19. Data Race on `a.client` During `StartLocalModel`/`StopLocalModel`
- **File:** `app.go` (`StartLocalModel`, `StopLocalModel`)
- **Detail:** `a.client` (llama.cpp API client) is reassigned during model start/stop. `clientMu` exists but concurrent streaming requests using the old client while it is being swapped could fail or observe inconsistent state.
- **Risk:** Unexpected errors or hangs during model switching.

### H20. `callLLMStream` Goroutine Persists 5 Min After Client Disconnect
- **File:** `app.go:931-1146`
- **Detail:** When the client disconnects, the HTTP handler returns but the goroutine inside `callLLMStream` continues running until the 300s context timeout fires. Orphaned goroutines accumulate.
- **Risk:** ~5 min goroutine leak per disconnected client.

### H21. Flutter: Model/Embedding Status Providers Infinite Polling
- **File:** `frontend/lib/providers/models_provider.dart:34-54`
- **Detail:** `modelStatusProvider` and `embeddingStatusProvider` run infinite `while(true)` polling loops every 5 seconds for the entire app lifetime. No `autoDispose`. Runs even when Model Store screen is not visible.

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

### M09. Active Provider Not Visible in Provider Settings UI
- **File:** `frontend/lib/widgets/settings_dialog.dart ~199-281`
- **Detail:** The `_ProvidersTab` shows provider cards but no indicator of which provider is currently active. User must navigate elsewhere to see active status.

### M10. Flutter: API Keys Visible in Plaintext in Provider Config Dialog
- **File:** `frontend/lib/widgets/provider_config_dialog.dart`
- **Detail:** Provider config dialog shows API keys in `TextField`. If someone is watching the screen (screen recording, screenshot, shoulder surfing), API keys are exposed.

### M11. No Tests for Provider/Agent/Orchestra Packages
- **File:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Detail:** Zero unit tests exist for the three packages (~4700 lines of production code). (Was ~4150 lines in previous audit; grew with new additions.)

### M12. Orchestra Config File Written with 0644 Permissions
- **File:** `internal/orchestra/conductor.go:114`
- **Detail:** Orchestra config JSON file is world-readable (`0644`). While it doesn't contain API keys, it leaks configuration details.

### M13. Agent Permissions File Written with 0644 Permissions
- **File:** `internal/agent/permissions.go:229`
- **Detail:** Agent permissions file (`permissions.json`) is world-readable (`0644`).

### M14. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`
- **Detail:** Double underscores `__` in HuggingFace repo IDs are converted to `/` in file paths. A malicious repo ID (`foo__..__bar`) could cause directory traversal.

### M15. Model Auto-Classification via Filename Heuristic
- **File:** `internal/modelstore/modelstore.go:58-64`
- **Detail:** `isEmbeddingModel` searches for "embedding" substring in filename. Regular models containing "embedding" in the name are misclassified.

### M16. `filepath.Walk` Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:374-376`
- **Detail:** In `filepath.Walk` callbacks, non-nil errors are silently skipped with `return nil`.

### M17. Embedding Client Stale Reference After Reinit
- **File:** `app.go:143-165` (App struct embeddingClient), `app.go:148-149`
- **Detail:** When the embedding model is restarted, the `embeddingClient` reference is not updated. The old client reference may point to a closed connection.

### M18. Memory Store: No Size Limit on User Messages
- **File:** `internal/memory/store.go` (various insert paths)
- **Detail:** User messages of any size can be stored as memory. Embedding models typically have token limits (e.g., 512 tokens). No truncation is performed before storing.
- **Risk:** Embedding failure or oversized memory entries wasting storage.

### M19. Identity System Prompt Can Bloat with Large Memory Context
- **File:** `internal/identity/identity.go:26-49`
- **Detail:** `BuildSystemPrompt` appends all retrieved memories directly into the system prompt. With many high-similarity memories, the prompt can grow arbitrarily large, exceeding the model's context window. No truncation or size limit.

### M20. Orchestra: No Self-Referencing Role Loop Detection
- **File:** `internal/orchestra/conductor.go` (`callModel`)
- **Detail:** A role's model endpoint pointing back to the Memo app itself (or creating a loop via another service) enables infinite recursion. No cycle detection.

### M21. Cloud Sync: Interrupted Upload Leaves Partial File
- **File:** `internal/cloudsync/drive.go`
- **Detail:** If an upload is interrupted partway, the cloud destination contains a partial/corrupt file. No cleanup or partial upload abort on error.

### M22. `handleSendFileStream`: Temp File Not Cleaned Up
- **File:** `internal/webserver/handlers_flutter.go:72-159`
- **Detail:** Temp file created with `os.CreateTemp` (`/tmp/memo_web_*`) is never deleted (`defer os.Remove` is missing). Each file upload leaks a temp file.

### M23. `handleSendFileStream`: Double File Close
- **File:** `internal/webserver/handlers_flutter.go:98-105`
- **Detail:** Temp file is closed twice — once via `defer tmpFile.Close()`, once via explicit `tmpFile.Close()`. The deferred `Close()` on an already-closed file returns `os.ErrClosed` which is ignored.

### M24. `handleSendFileStream`: Potential Slice Panic on Short MIME Type
- **File:** `internal/webserver/handlers_flutter.go:110`
- **Detail:** `mimeType[:5]` slicing panics if `mimeType` is shorter than 5 characters (empty MIME type, or a custom type < 5 chars).

### M25. `handleSendFileStream`: Filename Retrieved From Closed File
- **File:** `internal/webserver/handlers_flutter.go:104-105`
- **Detail:** `tmpFile.Name()` is called at line 104 AFTER `tmpFile.Close()` at line 105. Works on Linux but is non-portable behavior.

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
> **Open bugs:** 45+ (🔴7, 🟠21, 🔵15, ⚪2)
> **Observations:** 15
> **Total issues found:** 60+
