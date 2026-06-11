# Known Issues & Technical Risks (Exhaustive Audit)

This document tracks all identified bugs, architectural limitations, and edge cases in the Memo project, updated after a deep codebase audit on 2026-06-03.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🟡 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- 🔵 **Low** — cosmetic, minor improvement, or edge case
- ⚪ **Info** — design note, risk, or observation

---

## 🔴 Critical

### C1. Data race on `a.syncManager` — ✅ Fixed (K16)
- **File:** `app.go:1880-1890`, `app.go:1669-1671`
- **Detail:** `UpdateSyncSettings` assigns `a.syncManager` **without any lock**. Meanwhile, `memorySaveWorker` goroutine reads `a.syncManager`. This is a **data race** — concurrent read and write of a pointer without synchronization.
- **Risk:** Panic if `syncManager` is read as nil.
- **Fix (K16):** `getSyncManager()` helper copies pointer under Lock/RLock; all callers use the copy.

### C2. `a.store` assigned without `storeMu` at startup — ✅ Fixed
- **File:** `app.go:189`
- **Detail:** `a.store = store` is assigned without holding `storeMu`. After startup, `reinitMemoryStore` modifies it with `storeMu.Lock()`. The initial write is unsynchronized.
- **Fix:** Assignment wrapped in `a.storeMu.Lock()`/`Unlock()`.

### C6. OAuth server leak + `authWg` race — ✅ Fixed (K19)
- **File:** `internal/cloudsync/drive.go:97-185`
- **Detail:**
  - **Leak:** Calling `StartAuthFlow()` twice leaves the first HTTP server running indefinitely.
  - **Race:** `dc.authWg.Add(1)` called without `dc.mu`. If flows interleave, `Done()` decrements below zero → **`sync.WaitGroup` panic**.
- **Risk:** Crash or resource leak.
- **Fix (K19):** `authSrv` field added; new flow closes old server first. `authDone` flag + `authWg` reset prevents duplicate Done panic.

### C9. Legacy Gob migration path index inconsistency — ✅ Fixed (K20)
- **File:** `internal/memory/store.go` (legacy migration path)
- **Detail:** The legacy `.gob` migration path has an index inconsistency issue.
- **Fix (K20):** File is deleted first; index is only updated on success. Index ID found via file hash without reading the file.

### C10. `UpdateSyncSettings` orphaned syncManager goroutines — ✅ Fixed (K16)
- **File:** `app.go:1880`
- **Detail:** Old `a.syncManager` may have an in-flight backup goroutine. Its `count` atomic and `inFlight` flag become orphaned.
- **Risk:** Orphaned goroutines, inconsistent state.
- **Fix (K16):** Old manager set to nil under `syncMu.Lock()`, new one created. Goroutine naturally terminates.

### C11. Flutter: `context` used after `Navigator.pop()` — ✅ Fixed (K21)
- **File:** `screens/model_store_screen.dart:1497-1498`, `widgets/model_config_dialog.dart:103-111`
- **Detail:** `ScaffoldMessenger.of(context)` called after dialog is dismissed. Context may be invalid after pop.
- **Risk:** Crash or invisible SnackBar.
- **Fix (K21):** `ScaffoldMessenger` reference captured before pop.

### C12. Flutter: Context menu async context use — ✅ Fixed (K21)
- **File:** `widgets/chat_message_list.dart:182-188`, `widgets/chat_sidebar.dart:385-391`
- **Detail:** `showMenu().then()` callback may fire after widget is disposed. `_showEditDialog()` / `_showDeleteConfirm()` use `context`.
- **Risk:** Crash after widget disposal.
- **Fix (K21):** `if (!mounted) return;` guards added.

### C13. Flutter: `TextEditingController` created in `build()` — never disposed — ✅ Fixed (K21)
- **File:** `widgets/settings_dialog.dart:1678`
- **Detail:** `_ParamIntInput.build()` creates a new `TextEditingController` every frame. None are disposed.
- **Risk:** Memory leak on every parameter change.
- **Fix (K21):** `_ParamIntInput` converted to StatefulWidget; controller created in initState, disposed in dispose.

### C14. Flutter: `setState` after async without `mounted` check — ✅ Fixed (K21)
- **File:** `widgets/llama_installer_view.dart:53-56`
- **Detail:** Async operation followed by `setState` without `mounted` check. Crashes if widget disposed.
- **Risk:** Crash.
- **Fix (K21):** `if (!mounted) return;` before all async setState calls.

### C15. Flutter: `FocusNode.requestFocus()` after async without `mounted` check — ✅ Fixed (K21)
- **File:** `widgets/chat_input.dart:65`
- **Detail:** `sendMessage` async call followed by `_focusNode.requestFocus()` without `mounted` check.
- **Risk:** Crash.
- **Fix (K21):** `if (!mounted) return;` before `requestFocus`.

---

## 🟠 High

### H1. OAuth callback duplicate `Done` panic — ✅ Fixed (K19)
- **File:** `internal/cloudsync/drive.go:156-174`
- **Detail:** Callback goroutine calls `dc.authWg.Done()`. If invoked twice (HTTP replay), `Done` panics.
- **Risk:** Crash.
- **Fix (K19):** `authDone` flag prevents duplicate Done calls.

### H2. `callLLMStream` goroutine runs 5 minutes after client disconnect — ✅ Partially fixed (K11+K17)
- **File:** `app.go:487-554`, `handlers_flutter.go:44-63`
- **Detail:** HTTP client disconnects → `handleSendStream` returns, but `callLLMStream` goroutine continues until 300s context timeout.
- **Risk:** GPU/CPU wasted for 5 minutes.
- **Fix (K11+K17):** `trySend()` no longer blocks on context cancellation; all `ch <-` sends in `processSSEStream` guarded by `select`. No more goroutine leak or channel block.

### H3. Concurrent `AddMessage` calls may reorder messages — ✅ Fixed (K22)
- **File:** `app.go:360-378`, `app.go:487-554`
- **Detail:** `SendMessage` and `SendMessageStream` goroutine call `a.sessions.AddMessage` concurrently. Mutex prevents corruption but message order may interleave.
- **Fix (K22):** Per-session mutex (`sessionSendMu`) added; new messages wait until prior stream finishes.

### H4. `isAuthenticated` has no timeout — ✅ Fixed (K19)
- **File:** `internal/cloudsync/drive.go:70-93`
- **Detail:** `TokenSource` and `Token()` use `context.Background()`. If OAuth server hangs, call blocks forever.
- **Fix (K19):** 10-second context timeout added.

### H5. Nil `embeddingClient` causes nil pointer panic — ✅ Fixed (K23)
- **File:** `app.go:1455`, `embedder.go:13-21`
- **Detail:** `StopEmbeddingModel` sets `embeddingClient = nil`. `NewEmbeddingFunc` → `client.CreateEmbedding` on nil client panics.
- **Risk:** Crash.
- **Fix (K23):** `CheckEmbeddingHealth` takes a safe copy under lock; nil store not assigned on init.

### H6. Memory silently disabled on startup error — ✅ Fixed (K23)
- **File:** `app.go:185-189`
- **Detail:** `NewStore` failure sets `a.store = nil`. Subsequent operations silently return nil. Log is written but no UI notification.
- **Fix (K23):** Nil store is not assigned; `retrieveMemory` and `saveMemorySync` send events.

### H7. Flutter: `Future.delayed` never cancelled — ✅ Fixed (K21)
- **File:** `providers/chat_provider.dart:220-222`
- **Detail:** Every `sendMessage()` creates a 2s delayed timer. Multiple rapid messages create multiple timers that all fire eventually. Not cleaned up on dispose.
- **Fix (K21):** Replaced with `Timer`; cancelled in `ref.onDispose`.

### H8. Flutter: Async methods not awaited — ✅ Fixed (K21)
- **File:** `widgets/chat_sidebar.dart:113-114, 119-125`, `widgets/settings_dialog.dart:498-508`
- **Detail:** `switchTo(id)`, `delete(id)`, `save()` return Futures but are not awaited. Errors silently swallowed.
- **Fix (K21):** Wrapped with `unawaited()` or `await` added.

### H9. Flutter: Side-effect mutation in `build()` — ✅ Fixed (K21)
- **File:** `screens/chat_screen.dart:120-125`
- **Detail:** `whenData()` callbacks mutate `title` local. First build always shows "New Chat". Data arrival fixes it after one extra frame.
- **Fix (K21):** `ref.listen` with mounted check added.

### H10. Flutter: Stale closure in context menu — ✅ Fixed (K21)
- **File:** `widgets/chat_sidebar.dart:36-42`
- **Detail:** `isIncognito` captured at build time. If incognito toggled since, closure uses stale value.
- **Fix (K21):** `if (!mounted) return;` guard added.

### H11. Ngrok `SetRemoteAccess` early return — config not saved ✅ Fixed
- **File:** `app.go:1918`
- **Detail:** `SetNgrokMode` writes token to memory then calls `SetRemoteAccess`. `SetRemoteAccess` has `enabled == a.remoteAccessEnabled && port == port` early return. Ngrok never starts, token not written to config.
- **Risk:** Ngrok silently fails; user sees "ngrok tunnel started" but no URL.
- **Fix:** Added ngrok mode check: `a.cfg.RemoteAccess.NgrokMode == (a.ngrokServer != nil)`.

### H12. Ngrok subprocess crash not shown in UI ✅ Fixed
- **File:** `internal/ngrok/manager.go:53-87`, `app.go:1899-1909`
- **Detail:** Ngrok subprocess starts asynchronously. Auth error causes process crash but `pollPublicURL` only times out. No error shown to user.
- **Risk:** User thinks ngrok works with wrong token.
- **Fix:** `cmd.Wait()` goroutine captures exit error. `LastError()` method exposed via `RemoteAccessStatus.NgrokError`. Displayed as red box in frontend.

---

## 🟡 Medium

### M1. `retrieveMemory` uses `context.Background()` instead of request context — ✅ Fixed (K23)
- **File:** `app.go:1591`
- **Detail:** User switching chats or cancelling cannot cancel memory retrieval.
- **Fix (K23):** `ctx context.Context` parameter added, derived from callers.

### M2. `callLLM` uses `context.Background()` instead of request context — ✅ Fixed (K23)
- **File:** `app.go:1608`
- **Detail:** 120s LLM call cannot be cancelled by the user.
- **Fix (K23):** `ctx context.Context` parameter added, derived from callers.

### M3. Path traversal Layer 1 check is weak ✅ Fixed
- **File:** `internal/webserver/handlers_flutter.go:344`
- **Detail:** `strings.Contains(path, "..")` can be bypassed with URL-encoded `..` (`%2e%2e`). Layer 2 (`GetImageBase64`) with `filepath.EvalSymlinks` is the real security.
- **Fix:** `url.QueryUnescape` decodes before `..` check. `%2e%2e` bypass closed.

### M4. No request body size limits on most HTTP handlers — ✅ Fixed (K18)
- **File:** `internal/webserver/server.go`, `handlers_flutter.go`
- **Detail:** Only `handleTranscribe` uses `MaxBytesReader`. Others accept unlimited bodies. DoS vector.
- **Fix (K18):** `limitBodyMiddleware` applies 10MB limit to all handlers.

### M5. Temp files written to app directory instead of system temp ✅ Fixed
- **File:** `internal/llama/installer.go:192`
- **Detail:** Multi-GB model downloads go to `i.BaseDir` ("data/") instead of `os.TempDir()`. Could fill disk partition.
- **Fix:** Changed to `os.TempDir()`.

### M6. `syncManager.Increment()` vs `TriggerNow` race — double backup — ✅ Fixed (K24)
- **File:** `internal/cloudsync/sync_manager.go:108-152`
- **Detail:** `Increment` checks `m.inFlight` under `m.mu`, then releases lock before starting pipeline. `TriggerNow` may interleave and see `inFlight = false`. Result: **two concurrent backups**.
- **Fix (K24):** `scheduleMu` mutex prevents race between Increment and TriggerNow.

### M7. GitHub API calls have no timeout — ✅ Fixed (K25)
- **File:** `internal/llama/installer.go:238, 325`
- **Detail:** `http.DefaultClient` used without timeout. GitHub API hanging blocks forever.
- **Fix (K25):** 30s timeout for API calls, 5min for downloads.

### M8. No zip bomb protection in `restoreZip` ✅ Fixed
- **File:** `internal/cloudsync/sync_manager.go:363-412`
- **Detail:** Decompressed zip size is not limited. A compromised backup can write arbitrary data to disk.
- **Fix:** 100MB per file, 500MB total limit. `io.LimitReader` guards the copy.

### M9. Flutter: `_init()` called from constructor — state briefly wrong ✅ Fixed
- **File:** `providers/settings_provider.dart:94-96, 283-285, 307-309, 331-333`, `main.dart`
- **Detail:** `StateNotifier` constructors call `_init()` (async). Initial state is hardcoded default, later overwritten by SharedPreferences. Brief period of wrong values.
- **Fix:** `SharedPreferences` loaded at startup in `main()`, injected via `prefsProvider` override. All constructors start synchronously; `_init()` pattern removed entirely.

### M10. Flutter: `ref.listen` registered on every rebuild ✅ Fixed
- **File:** `screens/chat_screen.dart:41-48`, `screens/model_store_screen.dart:1015-1021`
- **Detail:** New listener added on every rebuild. Riverpod deduplicates but overhead remains.
- **Fix:** Converted `ConsumerWidget` → `ConsumerStatefulWidget`. `ref.listen` moved to `initState()`.

### M11. Flutter: `_ModelParametersCardState` local state never updated ✅ Fixed
- **File:** `widgets/settings_dialog.dart:1517-1523`
- **Detail:** If `llamaSettingsProvider` changes externally, local state ignores it. UI becomes stale.
- **Fix:** `ref.listen` tracks provider changes. `_saveVersion`/`_displayedVersion` mechanism: auto-syncs when user is not editing. Save button increments version.

### M12. Flutter: `MarkdownStyleSheet` rebuilt every frame ✅ Fixed
- **File:** `widgets/chat_message_list.dart:9-37`
- **Detail:** `_buildMarkdownStyleSheet(context)` called in every `_MessageBubble.build()`. No caching.
- **Fix:** `_styleCache` map added — reuses style sheet until theme changes.

### M13. Ngrok error field missing from API ✅ Fixed
- **File:** `app.go:1875-1910`
- **Detail:** `RemoteAccessStatus` struct had no field for ngrok error. Backend couldn't forward error to frontend.
- **Fix:** `NgrokError string \`json:"ngrok_error"\`` added, populated via `a.ngrokServer.LastError()`.

### M14. Ngrok Install doesn't check bundled `binaries/` path ✅ Fixed
- **File:** `internal/ngrok/installer.go:34-81`
- **Detail:** `Install("data")` only checks `data/binaries/<os>/ngrok`, ignores `binaries/<os>/ngrok` (bundled). Unnecessary download (~31MB).
- **Fix:** Install checks `data/binaries/` first, then bundled `binaries/` path. Uses bundled if found.

---

## 🔵 Low

### L1. `saveToken` errors silently ignored ✅ Fixed
- **File:** `internal/cloudsync/drive.go:89, 174, 224, 256`
- **Detail:** `_ = dc.saveToken(t)` — token save failure silently swallowed. Token lost on restart.
- **Fix:** Error is now logged.

### L2. `writeJSON` ignores encode errors ✅ Fixed
- **File:** `internal/webserver/server.go:537-539`
- **Detail:** `json.NewEncoder(w).Encode(v)` error not checked. Silent failure on connection close or serialization error.
- **Fix:** Error logged: `log.Printf("writeJSON error: %v", err)`.

### L3. `config.Save` doesn't report validation errors ✅ Fixed
- **File:** `internal/config/config.go:170-173`
- **Detail:** `cfg.validate()` returns no error, silently corrects invalid values. User doesn't know their input was ignored.
- **Fix:** `validate()` now returns `[]string` of corrected fields. Logged at call site.

### L4. STT binary world-executable ✅ Fixed
- **File:** `app.go:416`
- **Detail:** STT binary written to temp dir with 0755 permissions. Other users can read it.
- **Fix:** Changed to 0700 (owner-only execute).

### L5. STT process group not cleaned ✅ Fixed
- **File:** `app.go:422-423`, `stt_proc_unix.go`
- **Detail:** Only main process killed, child processes become orphans.
- **Fix:** STT subprocess starts with `Setpgid: true`; shutdown kills the entire process group.

### L6. `Setpgid` not set for child processes ✅ Already fixed
- **File:** `internal/llama/sysproc_linux.go:12-16`
- **Detail:** Without `Setpgid`, `Pdeathsig` only kills direct child, not descendants.
- **Status:** Code review reveals `Setpgid: true` was already set. Doc was outdated.

### L7. Flutter: Missing `const` constructors (widespread)
- **Across entire codebase:**
  - `AppShell()`, `ChatSidebar()`, `ChatInput()`, `WelcomeView()`, countless `SizedBox()`, `Padding()`, `Text()`, `Icon()` calls without `const`.

### L8. Flutter: Empty `catch (_)` blocks swallow errors ✅ Fixed
- **File:** `core/api_client.dart:68, 606`, `providers/settings_provider.dart:214, 221, 229, 239, 268`
- **Detail:** Errors silently swallowed, no user notification.
- **Fix:** `catch (_)` → `catch (e)`. Error variable is now accessible.

### L9. Flutter: `connectionStatusProvider` queries only once ✅ Fixed
- **File:** `providers/chat_provider.dart:375-385`
- **Detail:** `FutureProvider` runs once. If backend goes down later, status indicator stays green.
- **Fix:** Converted to `StreamProvider` — polls `isAlive()` every 30 seconds.

 ### L10. Flutter: Hardcoded Turkish strings bypass L10n system ✅ Fixed
- **File:** `screens/model_store_screen.dart` (priority)
- **Detail:** Turkish text appears in English UI.
- **Fix:** All hardcoded Turkish strings in `model_store_screen.dart` replaced with `L10n.t(...)` calls.

### L11. Ngrok UI "Start" button and token pre-fill missing ✅ Fixed
- **File:** `frontend/lib/widgets/settings_dialog.dart:2135-2185`
- **Detail:** Token field empty on every open; user must re-enter token and click "Start ngrok Tunnel" separately. Token saved in config but not shown.
- **Fix:** Token pre-fetched from backend. Toggle auto-starts ngrok, separate button removed. Loading state, auto-refresh timer (2s interval, 20s max) added.

### L12. Ngrok connection status not auto-refreshed ✅ Fixed
- **File:** `frontend/lib/widgets/settings_dialog.dart:2159-2184`
- **Detail:** Ngrok starts asynchronously (1-5s). Frontend queries once at toggle time, doesn't wait for URL. User must manually refresh.
- **Fix:** `_startRefreshTimer()` — `Timer.periodic` every 2s invalidates `remoteAccessProvider`, max 10 retries (20s). Timer cancelled on dispose. Token auto-saved on `onEditingComplete`.

---

## 🔴 Critical (Provider/Agent/Orchestra)

### C16. Orchestra Bypasses Provider Router — No Fallback ✅ Fixed
- **File:** `internal/orchestra/conductor.go:153-171`, `app.go:343-352`
- **Detail:** `createProviderForType` creates providers directly via factory, bypassing `provider.Router`. If a provider fails, there is no fallback chain — the task fails immediately.
- **Risk:** Orchestra task failure on transient provider errors.
- **Fix:** Orchesta provider factory now requires provider to be registered in Router. `NewProvider` fallback removed — orchestra providers must be enabled in API Providers for Router health checks and fallback to work.

### C17. Agent Pipeline No Streaming — Blocks UI ✅ Fixed
- **File:** `internal/orchestra/conductor.go:414-418`
- **Detail:** `executeSingleTask` uses non-streaming `ChatCompletion` for tool calls. User sees no response until the entire tool-calling loop completes (potentially minutes).
- **Risk:** Poor UX, perceived hang.
- **Fix:** `executeSingleTask` now uses `ChatCompletionStream` when `onProgress` is set, sending `ProgressTaskChunk` updates per token. Falls back to non-streaming on failure.

## 🟠 High (Provider/Agent/Orchestra)

### H13. Provider Priority Field Unused
- **File:** `internal/provider/config.go`, `router.go:40-55`
- **Detail:** `ProviderConfig.Priority` field exists but `getActiveEntries()` returns providers in insertion order, not by priority.

### H14. Active Provider Not Visible in Provider Settings UI
- **File:** `frontend/lib/widgets/settings_dialog.dart:199-281`
- **Detail:** The `_ProvidersTab` shows provider cards but no indicator of which provider is currently active. User must navigate elsewhere to see active status.

### H15. No Agent API Methods in Frontend ApiClient
- **File:** `frontend/lib/core/api_client.dart`
- **Detail:** Backend has fully working agent endpoints but frontend `api_client.dart` has no methods to call them. Agent mode cannot be toggled from UI.

### H16. Agent Permission Dialog Not Implemented
- **File:** Not created
- **Detail:** Backend emits `EventPermissionRequest` events but there is no frontend dialog to allow/deny tool calls. Agent pipeline blocks forever waiting for permission response.

## 🟡 Medium (Provider/Agent/Orchestra)

### M15. Orchestra Config Has No Validation
- **File:** `internal/orchestra/conductor.go:115-120`
- **Detail:** `UpdateConfig` accepts any role configuration. An invalid chief model or missing role model causes runtime error during execution rather than at config time.

### M16. Agent Pipeline No Timeout Per Tool Call
- **File:** `internal/agent/pipeline.go:120-150`
- **Detail:** Individual tool executions have no timeout. A hanging `run_command` blocks the entire pipeline indefinitely (sandbox has 60s timeout but pipeline doesn't enforce it).

### M17. Agent Audit Log Limited to 1000 Entries
- **File:** `internal/agent/executor.go:40-45`
- **Detail:** `logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.

---

## ⚪ Info / Observations

### I1. Legacy GOB Format (migrated to SQLite)
- **File:** `internal/memory/store.go` (legacy migration path)

### I2. Single-File-Per-Interaction Design
- **File:** `internal/memory/store.go`

### I3. Filepath.Walk Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`

### I4. Embedding Client Stale Reference After Reinit
- **File:** `app.go:148-149`, `app.go:124-125`

### I5. Model Auto-Classification via Filename
- **File:** `internal/modelstore/modelstore.go:58-64`

### I6. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`

### I7. Llama Server Stderr Mixed with App Logs
- **File:** `internal/llama/llama.go:118-119`

### I8. `App.ctx` stored in struct (anti-pattern)
- **File:** `app.go:100`
- **Note:** Per Go `context` documentation, contexts should be passed explicitly, not stored in structs.

### I9. Flutter: L10n uses custom listener pattern instead of Riverpod
- **File:** `core/l10n.dart:8`
- **Note:** Two parallel notification systems for locale.

### I10. Flutter: Hardcoded Turkish strings bypass L10n
- See L10 for details.

### I11. No Test Files for Provider/Agent/Orchestra
- **File:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Note:** Zero unit tests exist for the three new packages (~4150 lines of production code).

---

> **Last updated:** 2026-06-11
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend
> **Total bugs:** 54 (🔴12, 🟠14, 🟡15, 🔵10) + 11 observations
> **Fixed:** 46 (34 original + 12 more)
> **Still open:** 8 (🔴0, 🟠4, 🟡3, 🔵1)
