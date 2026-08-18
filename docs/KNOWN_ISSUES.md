# Known Issues & Technical Risks

> **Scope note (2026-08-05):** This document is a frozen snapshot of a full-codebase audit from 2026-07-04 (re-verified against source as of that date, see the note at the bottom). It predates the v3.1.2 → v3.3.3 → v3.3.4 line of work. Current, actively-maintained bug tracking lives in [`BUG_REPORT.md`](../BUG_REPORT.md) (repo root) — **0 open bugs at every severity as of 2026-08-05**. Three items below have since been specifically re-checked and updated in place (H06, H11, H12/BUG-L4); the rest of this file's Medium/Low/Info sections were **not** re-verified after 2026-07-04 and may contain further false positives or already-fixed items — treat it as historical background, not a current punch list.

This document tracks all currently open bugs and technical risks in the Memo project.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🔵 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- ⚪ **Low** — cosmetic, minor improvement, or edge case
- ⚫ **Info** — design note, risk, or observation

---

## ✅ Fixed Bugs

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

### F18. WebServer CORS Reflecting Any Origin (K03)
- **Previously:** `corsMiddleware` reflected any incoming `Origin` header directly in the response without whitelist check.
- **Fix:** Added whitelist allowing only `localhost`, `127.0.0.1`, `::1` origins.
- **Impact:** Malicious websites can no longer access the local API through the user's browser.

### F19. Orchestra `safeProgress` Deadlock (K02)
- **Previously:** `safeProgress` held mutex while calling `fn(up)`, which blocked when writing to a full channel, creating a deadlock that locked all goroutines.
- **Fix:** Removed `progressMu` from conductor; `safeProgress` now calls `fn(up)` directly. `fullBuf` safety uses local `sync.Mutex` in `app.go`.
- **Impact:** Orchestra no longer freezes on slow/disconnected clients.

### F20. Orchestra: Wrong User Message in Multi-Turn Chats (K07)
- **Previously:** `userPrompt` extraction always took the first user message instead of the last.
- **Fix:** Reversed loops in `callAgentWithOrchestra`, `callLLMStream`, `callLLM` to find the last user message.
- **Impact:** Multi-turn chats now use the correct question.

### F21. Skill `Install()`: Manifest Name Path Traversal (H25) + `os.Stat` Error Swallowing (M25)
- **Previously:** `def.Manifest.Name` from YAML was used in `filepath.Join` without validation; `os.Stat` errors were swallowed.
- **Fix:** `validateSkillName()` rejects `/`, `\`, `..` in names; `filepath.Abs` comparison ensures target is within `SkillsDir()`; proper error handling for `os.Stat`.
- **Impact:** Malicious SKILL.md can no longer traverse the filesystem.

### F22. Skill `Remove()`: Path Traversal (K05)
- **Previously:** `os.RemoveAll(def.Path)` without verifying `def.Path` is within `SkillsDir()`.
- **Fix:** Added `filepath.Abs` check to ensure root is inside `SkillsDir()`.
- **Impact:** Corrupted skill install can no longer delete app files via `Remove`.

### F23. Skill `copyDir`: Symlink Following (H24) + No Size Limit (M26 rollback)
- **Previously:** Symlinks were followed via `os.ReadFile` allowing copying of sensitive system files; no size limit; partial copy left directories behind.
- **Fix:** Symlinks skipped (`entry.Type()&os.ModeSymlink != 0`); 10MB per-file limit added; `os.RemoveAll(targetDir)` rollback on failure.
- **Impact:** Malicious skills can no longer copy system files or cause memory DoS.

### F24. `/skill:on <name>` Was Deleting Other Skills (H26)
- **Previously:** `SetActive([]string{name})` completely replaced the active set.
- **Fix:** Appends to the current active list; shows informative message if already active.
- **Impact:** Activating a new skill no longer removes existing active skills.

### F25. `handleActiveProvider` / `handleOrchestraConfig` Nil Guard (H14)
- **Previously:** No `s.fullBridge == nil` check, causing server panic on nil bridge.
- **Fix:** Added `if s.fullBridge == nil` guard at the start of both handlers.
- **Impact:** Test environments or partial init no longer crash the server.

### F26. `createBackup()` Errors Were Silently Swallowed (H16)
- **Previously:** `createBackup(fullPath)` return value was ignored in all tools in `file.go` and `edit.go`.
- **Fix:** All `WriteFile`, `DeleteFile`, `EditFile`, `InsertLine`, `DeleteLines` functions now check and abort on backup error.
- **Impact:** Disk full or permission errors no longer allow unbacked writes.

### F27. `run_command` CWD Traversal Check Bypass (H17)
- **Previously:** Failed `EvalSymlinks` completely bypassed CWD traversal check.
- **Fix:** Distinguishes `IsNotExist` from other errors; validates `filepath.Clean` result for non-existent dirs; rejects other errors.
- **Impact:** Agent can no longer bypass sandbox CWD check even with non-existent directory paths.

### F28. `/api/image` Handler Path Traversal (CR01)
- **Previously:** URL-encoded path (`%2Fetc%2Fpasswd`) could bypass `IsAbs` check.
- **Fix:** `IsAbs` check now runs on URL-decoded value; `..` and absolute path checks after decode; `filepath.Clean` normalization added; subdirectory whitelist (`data/images`, `data/avatars`, `data/attachments`) enforced.
- **Impact:** Arbitrary file reads no longer possible; only allowed directories are accessible.

### F29. `callLLM` `activeProvider`/`providerRouter` Data Race (CR02)
- **Previously:** Non-streaming `callLLM` path read `activeProvider` and `providerRouter` without `providerMu`.
- **Fix:** Added `a.providerMu.RLock()`/`RUnlock()`, captured to local variables — matches `callLLMStream` pattern.
- **Impact:** Data race risk on concurrent provider reconfiguration eliminated.

### F30. Skill `SetActive` Tool Registration Logic (H14)
- **Previously:** Newly activated skills had tools unregistered via `UnregisterTool` instead of `RegisterTool`.
- **Fix:** Changed `UnregisterTool` to `RegisterTool`. Removed redundant second registration block.
- **Impact:** Activating skills now correctly registers their tools.

### F31. Cloud Sync `WaitGroup` Leak (H15)
- **Previously:** Second auth flow reset `WaitGroup`, leaving old `WaitForAuth` callers blocked forever.
- **Fix:** Handled WaitGroup lifecycle properly.
- **Impact:** Cloud sync no longer hangs on second auth flow.

### F32. SQLite Context Cancellation (H16)
- **Previously:** `execWrite` used `db.ctx` (background) instead of caller's context.
- **Fix:** `execWrite` now accepts `context.Context` and uses caller's context for `BeginTx`.
- **Impact:** SQLite write operations can now be cancelled via caller context.

### F33. Flutter AgentEventBus StreamController Leak (H17)
- **Previously:** `StreamController` was never disposed.
- **Fix:** Added `ref.onDispose(() => bus.dispose())`.
- **Impact:** Agent event bus memory leak fixed.

### F34. Mobile Chat Stream Subscription Leak (H18)
- **Previously:** `StreamSubscription` never stored/cancelled.
- **Fix:** Added `_streamSubscription` field; cancels old subscription on re-send; cleans up on dispose.
- **Impact:** No more orphaned stream subscriptions on message re-send.

### F35. Agent Permission Channel Blockage (M21)
- **Previously:** `HandlePermissionResponse` send blocked forever if `waitFn` had returned.
- **Fix:** Added `select` with 1s timeout — drops response if channel full.
- **Impact:** HTTP handler goroutine no longer leaks on permission timeout.

### F36. Memory Store `COUNT(*)` Error Handling (M22)
- **Previously:** `Scan` error silently discarded.
- **Fix:** Added error check and logging.
- **Impact:** Database errors now visible in logs.

### F37. API Streaming `body.Close()` Double-Close (M23)
- **Previously:** Race between deferred `body.Close()` and watcher goroutine.
- **Fix:** `sync.Once` ensures `body.Close()` is called only once.
- **Impact:** Rare double-close panic risk eliminated.

### F38. Agent Screen Permission Dialog Bypass (M24)
- **Previously:** Back button could dismiss permission dialog.
- **Fix:** Added `PopScope(canPop: false)`.
- **Impact:** Permission dialog can no longer be bypassed via back button.

### F39. Model Store Dio Instance Leak (M25)
- **Previously:** `_loadReadme()`, `_loadMoreModels()` each created separate `Dio()` instances.
- **Fix:** Now uses shared `apiClientProvider.dio`.
- **Impact:** Redundant Dio instances and inconsistent timeouts eliminated.

### F40. Settings `didUpdateWidget` Overwrites Input (M28)
- **Previously:** `_controller.text` overwritten on every `didUpdateWidget`.
- **Fix:** Added `_isEditing` flag; no update when field has focus.
- **Impact:** User input no longer lost on external updates.

### F41. Provider Config Dialog Double-Submit (L17)
- **Previously:** `_save()` had no saving guard or loading indicator.
- **Fix:** Added `_isSaving` flag; disabled button with spinner during save.
- **Impact:** Double-submit and "app froze" UX eliminated.

### F42. `memorySaveWorker` Context (L13)
- **Previously:** Used `context.Background()` instead of app context.
- **Fix:** Changed to `a.ctx` — now cancellable on shutdown.
- **Impact:** Memory saves stop promptly on shutdown.

### F43. `ExportData` File Handle Leak (L12)
- **Previously:** `defer f.Close()` accumulated inside loops.
- **Fix:** Explicit `f.Close()` closes each file immediately.
- **Impact:** File descriptor exhaustion risk eliminated on large exports.

---

## 🔴 Critical

*No open critical issues as of 2026-07-04 — see F44/F45 below.*

### ~~CR01. Provider API Keys Encrypted with Weak Machine-Derived Key~~ ✅ FIXED
- ~~`internal/provider/config.go:374-402`~~
- ~~`defaultMachineKey()` derived AES-256 key from `/etc/machine-id`/registry GUID/IOPlatformUUID — not secret, plus a hardcoded fallback key visible in source.~~
- **Fix:** `defaultMachineKey()` now generates a random key via `crypto/rand` on first run and persists it to `data/machine.key` (mode `0600`), independent of any hardware identifier.

### ~~CR02. Cloud Sync Encryption Falls Back to Hardware ID When Passphrase Empty~~ ✅ FIXED
- ~~`internal/cloudsync/crypto.go:27-38, 67-87`~~
- ~~`encrypt()` and `deriveKey()` silently fell back to `hardwareID()` (not secret) when passphrase was empty.~~
- **Fix:** `encrypt()` (crypto.go:34-36) now returns an error immediately if passphrase is empty — no silent weak-key fallback on the write path. `deriveKey()`'s hardware-ID path is retained only for *decrypting* pre-v3.0.0 backups (documented legacy fallback in `decrypt()`), not for new encryption.

---

## 🟠 High

### ~~H01. Provider Priority Field Unused~~ ✅ FIXED
- ~~`ProviderConfig.Priority` existed but `getActiveEntries()` ignored it, using insertion order.~~
- **Fix:** `router.go` (lines 65, 223) now sorts entries by `cfg.Priority` before use; the provider config dialog exposes the field in the UI.

### ~~H02. No Agent API Methods in Frontend ApiClient~~ ✅ FIXED
- ~~`api_client.dart` had no methods for the backend's agent endpoints.~~
- **Fix:** `api_client.dart` now has a full Agent Mode section (`getAgentEnabled`, `setAgentEnabled`, `handleAgentPermission`, `getAgentPermissions`, `revokeAgentPermission`, `clearAgentPermissions`, `undoAgentEdit`, `getAgentAutoPermission`, `setAgentAutoPermission`) plus a complete agent screen/permission-dialog UI.

### ~~H03. Download Progress Polling Never Stops~~ ✅ FIXED
- ~~`downloadProgressProvider` polled every 1s forever, even with no active download.~~
- **Fix:** `models_provider.dart` — `downloadProgressProvider` is now `StreamProvider.autoDispose` with an adaptive interval (1s while a download is active, 4s while idle), and stops entirely when no screen is listening.

### ~~H04. ngrok Binary Download Has No Integrity Check~~ ✅ PARTIALLY FIXED (2026-08-18)
- ~~`internal/ngrok/installer.go:34-91`~~
- ~~The ngrok binary is downloaded over HTTPS but no SHA256 checksum verification is performed. The download URL (`bin.ngrok.com/c/bNyj1mQVY4c/`) contains a static API key. If the CDN is compromised or the download is intercepted, a malicious binary could be installed.~~
- **Fix:** `verifyArchiveMagic` rejects a response whose leading bytes don't match the archive format its extension promises (gzip magic for `.tgz`, `PK` for `.zip`) — catches a real CDN failure mode where an error page is served with a 200 status instead of the binary. **Not fully closed:** a real SHA256 pin isn't hardcoded — ngrok doesn't publish checksums for this rolling "stable" shortlink, and the content changes on every release, so a fixed hash would break the next update. Full tamper-resistance against a compromised CDN/origin (HTTPS already covers MITM) needs pinning this repo to one specific ngrok version with a maintainer-verified hash, bumped by hand on every update — a real trade-off against "always installs latest stable," left as a deliberate, documented decision rather than made unilaterally.

### ~~H05. WhatsApp SQL LIKE Injection in `SearchMessages`~~ ✅ FIXED (2026-08-18)
- ~~`internal/whatsapp/store.go:107-138`~~
- ~~User input is wrapped with `"%" + query + "%"` for a LIKE pattern. If `query` contains `_` (single-character wildcard in SQL LIKE), it matches unintended rows. Example: query `"test_"` would match `"test1"`, `"testX"`, etc.~~
- **Fix:** `escapeLikePattern` escapes `\`, `%`, `_` before substring-wrapping the query, paired with `ESCAPE '\'` on the LIKE clause.

### ~~H06. Flutter: Global Style Cache (`_styleCache`) Memory Leak~~ ✅ RE-CHECKED, NOT A REAL LEAK
- ~~`frontend/lib/widgets/chat_message_list.dart:13`~~
- ~~`_styleCache` is a global mutable `Map` that is never cleared. It grows indefinitely with every unique combination of theme brightness and accent color visited.~~
- **Re-verified (per handoff.md):** the key space is bounded to 2 (light/dark brightness) since `MemoTheme.accent` is a constant, not a variable the user changes — the map can never grow past 2 entries. False positive, not touched.

### ~~H07. Flutter: `connectionStatusProvider` Infinite Polling~~ ✅ FIXED (duplicate of M31)
- ~~Ran a `while(true)` 30s polling loop for the app's entire lifetime, no `autoDispose`.~~
- **Fix:** `chat_provider.dart:687` — `connectionStatusProvider` is now `StreamProvider.autoDispose<bool>`.

### ~~H08. Orchestra Config Has No Validation~~ ✅ FIXED
- ~~`UpdateConfig` accepted any role configuration without validation.~~
- **Fix:** `conductor.go` `UpdateConfig` now runs `cfg.Sanitize()` before storing the config.

### ~~H09. Agent Pipeline No Timeout Per Tool Call~~ ✅ FIXED
- ~~Individual tool executions had no timeout enforced by the pipeline.~~
- **Fix:** `pipeline.go` sets `toolTimeout: 120 * time.Second` and wraps each tool call in a derived context. **2026-07-04 follow-up:** this budget was being silently truncated to 60s by `tools/command.go`'s own hard-coded `DefaultToolTimeout` — fixed so `run_command`/`search_files` now honor the caller's deadline and only fall back to 60s when the caller sets none (see Resolved Issues).

### ~~H10. Agent Audit Log Limited to 1000 Entries~~ ✅ FIXED (2026-08-18)
- ~~`internal/agent/executor.go:40-45`~~
- ~~`logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.~~
- **Fix:** `logEvent` now also appends each entry as one JSON line to `config.DataDir()/agent-audit.jsonl` (0600, append-only, best-effort). The in-memory slice stays capped at 1000 — it's now just a fast "recent" cache, not the only copy.

### ~~H11. Mobile API Client Missing Most Backend Endpoints~~ ✅ RE-CHECKED, NO LONGER TRUE
- ~~`mobile/lib/core/api_client.dart`~~
- ~~Mobile API client lacks: `sendFileStream`, `exportChat`, `generateTitle`, `updateMessage`, `deleteMessage`, `getSystemPrompt`, memory settings, model search/download, WhatsApp, sync, remote access, backup/restore, recording, image endpoints.~~
- **Re-verified (per handoff.md):** grep-counted against the backend's route table — 111 of 118 endpoints now exist on mobile. The remaining 7 are either brand-new client-registry endpoints mobile has no use for, or CLI-management endpoints (mobile has no CLI). Removed from `BUG_REPORT.md`.

### ~~H12. Data Race on `a.client` During `StartLocalModel`/`StopLocalModel`~~ ✅ RE-CHECKED — NOT A RACE, NARROWER ISSUE FIXED AS BUG-L4
- ~~`app.go` (`StartLocalModel`, `StopLocalModel`)~~
- ~~`a.client` (llama.cpp API client) is reassigned during model start/stop. `clientMu` exists but concurrent streaming requests using the old client while it is being swapped could fail or observe inconsistent state.~~
- **Re-verified (per handoff.md):** both the read and write sides of `a.client`/`providerRouter` reassignment are correctly guarded by `clientMu`/`providerMu` — this was never a data race. The real, narrower residual risk (a stream copies the client to a local variable under lock and holds it for the duration of the stream; if a model/provider swap happens mid-stream, that stream keeps talking to the old client) was tracked as **BUG-L4** and fixed: swap-caused failures now surface a clear error message instead of a raw "connection refused" (commit `07930f4`).

### ~~H13. Flutter: Model/Embedding Status Providers Infinite Polling~~ ✅ FIXED
- ~~`frontend/lib/providers/models_provider.dart:34-54`~~
- ~~`modelStatusProvider` and `embeddingStatusProvider` run infinite `while(true)` polling loops every 5 seconds for the entire app lifetime. Added `autoDispose` — now stops when model store screen is closed.~~

### ~~H14. Skill `SetActive()` Unregisters Tools Instead of Registering~~ ✅ FIXED
- ~~Newly activated skills had their tools unregistered instead of registered.~~
- **Fix:** `manager.go` `SetActive()` now correctly calls `UnregisterTool()` only for skills being deactivated and `RegisterTool()` only for skills newly activated.

### H15. Cloud Sync `WaitForAuth` Blocks Forever on Second Call — PARTIALLY FIXED, downgraded to 🔵 Medium
- **File:** `internal/cloudsync/drive.go:103-113, 209-221, 493-499`
- **2026-07-04 update:** An `authDone` guard and a previous-auth-server shutdown were added, closing the original panic/hang paths. However `WaitForAuth` still spawns a goroutine that reads `dc.authWg` without holding `dc.mu` (line ~212), while `StartAuthFlow` reassigns the same field under lock — a genuine (if narrow) data race if a second auth flow starts while a previous `WaitForAuth` call is still in flight. Low real-world likelihood for a single-user desktop app, but not fully closed.
- **Category:** Concurrency (residual race, not a guaranteed hang anymore)

### ~~H16. SQLite Writes Ignore Caller Context Cancellation~~ ✅ FIXED
- ~~`execWrite` used `db.ctx` (background context) instead of the caller's context.~~
- **Fix:** `sqlite.go` `execWrite` now takes `ctx context.Context` as a parameter and calls `db.sql.BeginTx(ctx, nil)` with the caller's context.

### ~~H17. Flutter `AgentEventBus` StreamController Never Disposed~~ ✅ FIXED
- ~~`StreamController<AgentEvent>.broadcast()` was never closed.~~
- **Fix:** `agent_provider.dart` now calls `ref.onDispose(() => bus.dispose())`.

### ~~H18. Mobile Chat Stream Subscription Leaks on Re-send~~ ✅ FIXED
- ~~`StreamSubscription` from `sendMessageStream().listen(...)` was never stored or cancelled.~~
- **Fix:** `mobile/lib/providers/chat_provider.dart` now stores it in `_streamSubscription`, cancelling the previous one before starting a new stream and on dispose.

### H19. WhatsApp Screen Polling Never Stops in IndexedStack — PARTIALLY FIXED, downgraded to ⚪ Low
- **File:** `frontend/lib/screens/whatsapp_screen.dart`, `frontend/lib/providers/whatsapp_provider.dart:95-107`, `frontend/lib/screens/app_shell.dart:159`
- **2026-07-04 update:** The original fast-forever QR polling is fixed — interval is now adaptive (2-4s while connecting, 15s heartbeat once connected). `WhatsAppScreen` is still kept mounted inside `app_shell.dart`'s `IndexedStack`, so `dispose()`/`stopPolling()` still won't fire until the app closes — but at a 15s heartbeat this is a minor, accepted cost rather than a battery/network drain.
- **Category:** Performance (reduced from infinite fast polling to a cheap heartbeat)

### ~~H20. `handleImage` Can Read All Files Under `dataDir`~~ ✅ FIXED (duplicate of F28)
- ~~Allowed any relative path under `dataDir`, e.g. `data/providers.json`.~~
- **Fix:** `handlers_flutter.go` `handleImage` now enforces a subdirectory whitelist (`data/images`, `data/avatars`, `data/attachments` only) in addition to URL-decode + `..`/absolute-path checks. Already recorded as fixed under F28 above — this was a leftover duplicate entry.

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

### ~~M10. No Tests for Provider/Agent/Orchestra Packages~~ ✅ FIXED
- ~~Zero unit tests existed for the three packages.~~
- **Fix:** 13 test files now exist across the three packages (`router_test.go`, `backup_test.go`, `permissions_test.go`, `tools/edit_test.go`, `tools/selfclone_test.go`, and 7 files under `orchestra/`), matching AGENTS.md's "48 tests passing with `-race`" note.

### ~~M11. Orchestra Config File Written with 0644 Permissions~~ ✅ FIXED
- ~~Orchestra config JSON file was world-readable.~~
- **Fix:** `conductor.go:141` now writes with `os.WriteFile(filePath, data, 0600)`.

### ~~M12. Agent Permissions File Written with 0644 Permissions~~ ✅ FIXED
- ~~`permissions.json` was world-readable.~~
- **Fix:** `permissions.go:240` now writes with `0600`.

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

### M21. Agent Permission Channel Send Blocks HTTP Handler Forever
- **File:** `internal/agent/executor.go:145`
- **Detail:** `resCh := make(chan PermissionPolicy, 1)` is created in `waitFn` and sent to in `HandlePermissionResponse` (line 194: `req.ResCh <- policy`). If `waitFn` has already returned (context cancellation or timeout), no goroutine reads from the channel. Buffer is 1, but `waitFn` already exited the `select` — the send blocks forever. Since `HandlePermissionResponse` runs in an HTTP handler goroutine, this goroutine leaks.
- **Risk:** After permission timeout + user response, HTTP handler goroutine blocks permanently.
- **Category:** Concurrency (goroutine leak)

### M22. Memory Store `COUNT(*)` Query Error Silently Ignored
- **File:** `internal/memory/store.go` (~line 530)
- **Detail:** `s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM memories").Scan(&stats.Count)` — the error from `Scan` is discarded. If the database is closed or corrupt, `stats.Count` remains 0 (zero value) with no error reported.
- **Risk:** UI shows "0 memories" when DB is broken — silent data loss indication.
- **Category:** Error handling

### M23. API Streaming `body.Close()` Double-Close Race Condition
- **File:** `internal/api/streaming.go:47, 55`
- **Detail:** `defer body.Close()` on line 47 runs when `processSSEStream` returns. The watcher goroutine (line 53-56) also calls `body.Close()` on context cancellation. Double-close on an `io.ReadCloser` may cause panic depending on implementation.
- **Risk:** Rare panic on HTTP response body double-close.
- **Category:** Concurrency (race condition)

### M24. Agent Screen Permission Dialog Bypassable via Back Button
- **File:** `frontend/lib/screens/agent_screen.dart:178`
- **Detail:** The agent permission dialog is shown with `barrierDismissible: false` but without `PopScope(canPop: false)`. The desktop chat screen correctly sets `canPop: false` but the agent screen lacks this guard.
- **Risk:** User can dismiss the dialog via back button, causing the agent pipeline to block forever waiting for a response.
- **Category:** UX

### M25. Model Store Creates 3+ Separate `Dio()` Instances per View
- **File:** `frontend/lib/screens/model_store_screen.dart:850, 1204, 1232`
- **Detail:** `_loadFiles()` uses the shared `apiClientProvider` Dio, but `_loadReadme()`, `_loadMoreModels()`, and `_AuthorAvatarState._resolve()` each create their own `Dio()` instance with different timeouts and configurations.
- **Risk:** Unnecessary resource usage; no connection pooling; inconsistent timeout behavior.
- **Category:** Performance

### M26. `_delayedRefreshTimer` Can Fire After Provider Disposal
- **File:** `frontend/lib/providers/chat_provider.dart:314-317`
- **Detail:** A `Timer` is set for 2 seconds after a message is sent. `ref.onDispose` cancels it, but between `stopStreaming()` and `onDispose`, the timer could fire and call `ref.invalidate(chatListProvider)`. The `ref` may be invalid at that point.
- **Risk:** Possible `StateError` if timer fires after provider disposal.
- **Category:** Widget lifecycle

### M27. WhatsApp `_msgTimer` Polling Has No Error Backoff
- **File:** `frontend/lib/screens/whatsapp_screen.dart:46-53`
- **Detail:** Polls every 5 seconds with no backoff. If the network is down, it keeps polling at full rate.
- **Risk:** Battery drain on mobile; unnecessary network traffic.
- **Category:** Performance

### M28. Settings `didUpdateWidget` Overwrites User Input
- **File:** `frontend/lib/widgets/settings_dialog.dart:2526-2533`
- **Detail:** `_controller.text = widget.value.toString()` is called in `didUpdateWidget` without checking if the user is currently editing. If the user is typing, this overwrites their input.
- **Risk:** User input loss in parameter fields.
- **Category:** UX

### M29. Dio Stream `timeout` May Add Error After Cancellation
- **File:** `frontend/lib/providers/chat_provider.dart:227-233, 369-374`
- **Detail:** `stream.timeout(onTimeout: (sink) => sink.addError(...))` adds an error to the stream sink on timeout. However, if the `CancelToken` was cancelled before the timeout fires, the error is added after stream closure — unhandled exception.
- **Risk:** Unhandled exceptions in stream processing.
- **Category:** Concurrency

### M30. Flutter Model Status Polling Runs Forever Due to Engine Strip
- **File:** `frontend/lib/providers/models_provider.dart:35-59`
- **Detail:** Despite using `StreamProvider.autoDispose`, the engine strip widget at the bottom of the app always watches `modelStatusProvider`, keeping it alive permanently. `autoDispose` never triggers — polling every 5s continues forever.
- **Risk:** Continuous HTTP requests for the entire app lifetime.
- **Category:** Performance (infinite polling)

### ~~M31. Flutter `connectionStatusProvider` 30s Polling Never Stops (Duplicate of H07)~~ ✅ FIXED
- See H07 above — `connectionStatusProvider` is now `StreamProvider.autoDispose`.

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

### L12. `ExportData` File Handles Not Closed on Error
- **File:** `app.go:3551-3608`
- **Detail:** The `addFile` helper opens files with `os.Open` but `defer f.Close()` is inside a loop — all deferred `Close` calls accumulate and only run when `addFile` returns. Walk callbacks have the same issue.
- **Risk:** File descriptor exhaustion with large exports.

### L13. `memorySaveWorker` Uses `context.Background()` Instead of App Context
- **File:** `app.go:3441`
- **Detail:** `memorySaveWorker` calls `a.saveMemorySync(context.Background(), ...)` with a fresh background context instead of `a.ctx`. Ignores app-level cancellation.
- **Risk:** Memory saves continue briefly after shutdown.

### L14. `Server.Stop()` Uses `context.Background()` for Shutdown
- **File:** `internal/webserver/server.go:269`
- **Detail:** `srv.Shutdown(context.Background())` uses background context with no timeout. If the server is overloaded, `Shutdown` could block forever.
- **Risk:** Server shutdown hangs on active connections.

### L15. Flutter: `_delayedRefreshTimer` Provider Dispose StateError Risk
- **File:** `frontend/lib/providers/chat_provider.dart:314-317`
- **Detail:** Timer fires 2s after message send. `ref.onDispose` cancels it, but between `stopStreaming()` and `onDispose` the timer could call `ref.invalidate` on an invalid ref.

### L16. Flutter: `contextMenu` Position Wrong After Scroll
- **File:** `frontend/lib/widgets/chat_message_list.dart:302-304`
- **Detail:** `_tapPosition` stored as `details.localPosition` but `showMenu` expects global coordinates. `localToGlobal` conversion may produce wrong position if widget scrolled between down and tap events.

### L17. Flutter: `provider_config_dialog.dart` No `_saving` Guard
- **File:** `frontend/lib/widgets/provider_config_dialog.dart`
- **Detail:** `_save()` lacks double-submission protection. User can tap "Save" multiple times. No loading indicator during save.

### L18. Flutter: `unawaited(future)` Warnings (Multiple Locations)
- **File:** `frontend/lib/widgets/agent/permission_dialog.dart:51`, `frontend/lib/providers/agent_provider.dart`
- **Detail:** `Future` from `handleAgentPermission` is `unawaited`. Request failure silently lost.

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

### I16. Orchestra Conductor Hardcoded 300s Timeout
- **File:** `internal/orchestra/conductor.go:249, 478, 657`
- **Note:** All sub-operations use the same hardcoded 300s timeout. These should be configurable.

### I17. `memory/store.go` Latency Log Message Incorrect
- **File:** `internal/memory/store.go:269-275`
- **Note:** `time.Since(embedStart.Add(writeDur)).Milliseconds()` is wrong — `embedStart.Add(writeDur)` gives a time earlier than the correct value, making `time.Since` larger than actual. Logging bug only.

### I18. `api/streaming.go`: `scanner.Err()` Is Checked (Cross-check)
- **File:** `internal/api/streaming.go:127-129`
- **Note:** `scanner.Err()` IS checked after the scan loop. Confirmation, not a bug.

### I19. Agent Pipeline 20 Iterations Hardcoded
- **File:** `internal/agent/pipeline.go:63/78`
- **Note:** `maxIters: 20` is hardcoded. Complex tasks requiring many tool calls may hit this limit.

### I20. Sessions Write to Disk Synchronously on Every Message
- **File:** `internal/sessions/sessions.go:185`
- **Note:** `AddMessage` calls `m.save(s)` on every invocation. For streaming responses, `finishStream` calls once per reply, which is acceptable. But there's no rate limiting or batching for rapid calls.

### I21. WhatsApp Store Has No Connection Pool
- **File:** `internal/whatsapp/store.go:20`
- **Note:** Uses `sql.Open` directly without the `database.DB` wrapper that provides serialized writes. Concurrent WhatsApp handler and agent tool writes can cause "database is locked" errors.

### I22. `skill/manager.go` SetActive: Redundant Unregister+Register Cycle
- **File:** `internal/skill/manager.go:159-166`
- **Note:** Newly activated skills have tools unregistered then immediately re-registered. The middle block incorrectly calls `UnregisterTool` instead of `RegisterTool`. Final block correctly registers. If execution is interrupted between blocks, tools remain unregistered.

### I23. `config/config.yaml` Has Hardcoded `active_provider: openai`
- **File:** `config/config.yaml:63`
- **Note:** `active_provider: openai` is committed in the config file. On server startup, this could override the user's previous choice (e.g., `claude`).

### I24. `skill.DangerLevel` and `agent.DangerLevel` Duplicate Types — Type Mismatch
- **File:** `internal/skill/types.go:7` vs `internal/agent/tools.go:13`
- **Note:** Two packages define separate `DangerLevel` named types with identical string values. `SkillTool.DangerLevel` (`skill.DangerLevel`) is incompatible with agent pipeline `agent.DangerLevel` type assertions. Code using both packages has compile-time type mismatch. A shared `internal/common` package is recommended.

---

> **Last updated:** 2026-07-04 (re-verified against current source ahead of the v3.1.1 open beta); H06/H11/H12 re-checked and closed on 2026-08-05 (see scope note at top — current bug tracking has moved to `BUG_REPORT.md`)
> **Audit scope:** Full codebase — Go backend (app.go, app_skill.go, all internal/ packages) + Flutter frontend + mobile frontend + skill system + orchestra system
> **2026-07-04 pass:** Every 🔴 Critical and 🟠 High item, plus M09-M12/M31, was individually re-checked against current source. 15 items previously listed as open (CR01, CR02, H01, H02, H03, H07, H08, H09, H14, H16, H17, H18, H20, M10, M11, M12, M31 — one High item cross-linked as a Medium duplicate) were already fixed and are now moved to ✅ Fixed. Two (H15, H19) were partially fixed and downgraded in place (High → Medium/Low) rather than closed outright. Medium items M13-M30 and all ⚪ Low / ⚫ Info items were **not** re-verified this pass and may already be stale in the same way — treat counts below as a floor, not a ceiling.
> **2026-08-05 spot-check:** H06 (style cache) and H11 (mobile API client) were false positives on re-check; H12 (data race) was also a false positive, but the real narrower issue it pointed at was tracked separately as BUG-L4 and has since been fixed. All three moved to ✅ Fixed above.
> **2026-08-18 spot-check:** H04, H05, H10 — the 3 High items this document had flagged as still-confirmed-open since 2026-07-04 — were individually re-verified against current source (all three genuinely still open, unlike the 2026-08-05 false positives) and fixed same-day. All three moved to ✅ Fixed above (H04 only partially — see its entry for the real SHA256-pin limitation left open).
> **Open bugs (as of 2026-07-04, minus the 6 above):** 🔴0, 🟠0 confirmed, 🔵~28 (incl. H15 downgrade, not fully re-verified), ⚪~19 (incl. H19 downgrade, not fully re-verified) — Medium/Low/Info not re-verified since 2026-07-04; check `BUG_REPORT.md` for current ground truth
> **Observations:** 24 (I24 spot-checked 2026-07-04, still open)
> **Fixed:** 64 (43 previous + 15 confirmed 2026-07-04 + 3 confirmed 2026-08-05 + 3 confirmed 2026-08-18)
> **Total issues found:** 108+
