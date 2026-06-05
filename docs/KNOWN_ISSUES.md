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

### C1. Data race on `a.syncManager`
- **File:** `app.go:1880-1890`, `app.go:1669-1671`
- **Detail:** `UpdateSyncSettings` assigns `a.syncManager` **without any lock**. Meanwhile, `memorySaveWorker` goroutine reads `a.syncManager`. This is a **data race** — concurrent read and write of a pointer without synchronization.
- **Risk:** Panic if `syncManager` is read as nil.

### C2. `a.store` assigned without `storeMu` at startup
- **File:** `app.go:189`
- **Detail:** `a.store = store` is assigned without holding `storeMu`. After startup, `reinitMemoryStore` modifies it with `storeMu.Lock()`. The initial write is unsynchronized.

### C6. OAuth server leak + `authWg` race
- **File:** `internal/cloudsync/drive.go:97-185`
- **Detail:**
  - **Leak:** Calling `StartAuthFlow()` twice leaves the first HTTP server running indefinitely.
  - **Race:** `dc.authWg.Add(1)` called without `dc.mu`. If flows interleave, `Done()` decrements below zero → **`sync.WaitGroup` panic**.
- **Risk:** Crash or resource leak.

### C9. `DeleteGobFile` index inconsistency on error
- **File:** `internal/memory/store.go:267-269`
- **Detail:** If `readDocument` succeeds but `os.Remove` fails, the **index entry is silently not removed**. The index references a deleted file.
- **Risk:** Memory index bloat, dead references.

### C10. `UpdateSyncSettings` orphaned syncManager goroutines
- **File:** `app.go:1880`
- **Detail:** Old `a.syncManager` may have an in-flight backup goroutine. Its `count` atomic and `inFlight` flag become orphaned.
- **Risk:** Orphaned goroutines, inconsistent state.

### C11. Flutter: `context` used after `Navigator.pop()`
- **File:** `screens/model_store_screen.dart:1497-1498`, `widgets/model_config_dialog.dart:103-111`
- **Detail:** `ScaffoldMessenger.of(context)` called after dialog is dismissed. Context may be invalid after pop.
- **Risk:** Crash or invisible SnackBar.

### C12. Flutter: Context menu async context use
- **File:** `widgets/chat_message_list.dart:182-188`, `widgets/chat_sidebar.dart:385-391`
- **Detail:** `showMenu().then()` callback may fire after widget is disposed. `_showEditDialog()` / `_showDeleteConfirm()` use `context`.
- **Risk:** Crash after widget disposal.

### C13. Flutter: `TextEditingController` created in `build()` — never disposed
- **File:** `widgets/settings_dialog.dart:1678`
- **Detail:** `_ParamIntInput.build()` creates a new `TextEditingController` every frame. None are disposed.
- **Risk:** Memory leak on every parameter change.

### C14. Flutter: `setState` after async without `mounted` check
- **File:** `widgets/llama_installer_view.dart:53-56`
- **Detail:** Async operation followed by `setState` without `mounted` check. Crashes if widget disposed.
- **Risk:** Crash.

### C15. Flutter: `FocusNode.requestFocus()` after async without `mounted` check
- **File:** `widgets/chat_input.dart:65`
- **Detail:** `sendMessage` async call followed by `_focusNode.requestFocus()` without `mounted` check.
- **Risk:** Crash.

---

## 🟠 High

### H1. OAuth callback duplicate `Done` panic
- **File:** `internal/cloudsync/drive.go:156-174`
- **Detail:** Callback goroutine calls `dc.authWg.Done()`. If invoked twice (HTTP replay), `Done` panics.
- **Risk:** Crash.

### H2. `callLLMStream` goroutine runs 5 minutes after client disconnect
- **File:** `app.go:487-554`, `handlers_flutter.go:44-63`
- **Detail:** HTTP client disconnects → `handleSendStream` returns, but `callLLMStream` goroutine continues until 300s context timeout.
- **Risk:** GPU/CPU wasted for 5 minutes.

### H3. Concurrent `AddMessage` calls may reorder messages
- **File:** `app.go:360-378`, `app.go:487-554`
- **Detail:** `SendMessage` and `SendMessageStream` goroutine call `a.sessions.AddMessage` concurrently. Mutex prevents corruption but message order may interleave.

### H4. `isAuthenticated` has no timeout
- **File:** `internal/cloudsync/drive.go:70-93`
- **Detail:** `TokenSource` and `Token()` use `context.Background()`. If OAuth server hangs, call blocks forever.

### H5. Nil `embeddingClient` causes nil pointer panic
- **File:** `app.go:1455`, `embedder.go:13-21`
- **Detail:** `StopEmbeddingModel` sets `embeddingClient = nil`. `NewEmbeddingFunc` → `client.CreateEmbedding` on nil client panics.
- **Risk:** Crash.

### H6. Memory silently disabled on startup error
- **File:** `app.go:185-189`
- **Detail:** `NewStore` failure sets `a.store = nil`. Subsequent operations silently return nil. Log is written but no UI notification.

### H7. Flutter: `Future.delayed` never cancelled
- **File:** `providers/chat_provider.dart:220-222`
- **Detail:** Every `sendMessage()` creates a 2s delayed timer. Multiple rapid messages create multiple timers that all fire eventually. Not cleaned up on dispose.

### H8. Flutter: Async methods not awaited
- **File:** `widgets/chat_sidebar.dart:113-114, 119-125`, `widgets/settings_dialog.dart:498-508`
- **Detail:** `switchTo(id)`, `delete(id)`, `save()` return Futures but are not awaited. Errors silently swallowed.

### H9. Flutter: Side-effect mutation in `build()`
- **File:** `screens/chat_screen.dart:120-125`
- **Detail:** `whenData()` callbacks mutate `title` local. First build always shows "New Chat". Data arrival fixes it after one extra frame.

### H10. Flutter: Stale closure in context menu
- **File:** `widgets/chat_sidebar.dart:36-42`
- **Detail:** `isIncognito` captured at build time. If incognito toggled since, closure uses stale value.

---

## 🟡 Medium

### M1. `retrieveMemory` uses `context.Background()` instead of request context
- **File:** `app.go:1591`
- **Detail:** User switching chats or cancelling cannot cancel memory retrieval.

### M2. `callLLM` uses `context.Background()` instead of request context
- **File:** `app.go:1608`
- **Detail:** 120s LLM call cannot be cancelled by the user.

### M3. Path traversal Layer 1 check is weak
- **File:** `internal/webserver/handlers_flutter.go:254`
- **Detail:** `strings.Contains(path, "..")` can be bypassed with URL-encoded `..`. Layer 2 (`GetImageBase64`) with `filepath.EvalSymlinks` is the real security.

### M4. No request body size limits on most HTTP handlers
- **File:** `internal/webserver/server.go`, `handlers_flutter.go`
- **Detail:** Only `handleTranscribe` uses `MaxBytesReader`. Others accept unlimited bodies. DoS vector.

### M5. Temp files written to app directory instead of system temp
- **File:** `internal/llama/installer.go:192`
- **Detail:** Multi-GB model downloads go to `i.BaseDir` ("data/") instead of `os.TempDir()`. Could fill disk partition.

### M6. `syncManager.Increment()` vs `TriggerNow` race — double backup
- **File:** `internal/cloudsync/sync_manager.go:108-152`
- **Detail:** `Increment` checks `m.inFlight` under `m.mu`, then releases lock before starting pipeline. `TriggerNow` may interleave and see `inFlight = false`. Result: **two concurrent backups**.

### M7. GitHub API calls have no timeout
- **File:** `internal/llama/installer.go:238, 325`
- **Detail:** `http.DefaultClient` used without timeout. GitHub API hanging blocks forever.

### M8. No zip bomb protection in `restoreZip`
- **File:** `internal/cloudsync/sync_manager.go:355-472`
- **Detail:** Decompressed zip size is not limited. A compromised backup can write arbitrary data to disk.

### M9. Flutter: `_init()` called from constructor — state briefly wrong
- **File:** `providers/settings_provider.dart:94-96, 283-285, 307-309, 331-333`
- **Detail:** `StateNotifier` constructors call `_init()` (async). Initial state is hardcoded default, later overwritten by SharedPreferences. Brief period of wrong values.

### M10. Flutter: `ref.listen` registered on every rebuild
- **File:** `screens/chat_screen.dart:41-48`, `screens/model_store_screen.dart:1015-1021`
- **Detail:** New listener added on every rebuild. Riverpod deduplicates but overhead remains.

### M11. Flutter: `_ModelParametersCardState` local state never updated
- **File:** `widgets/settings_dialog.dart:1517-1523`
- **Detail:** If `llamaSettingsProvider` changes externally, local state ignores it. UI becomes stale.

### M12. Flutter: `MarkdownStyleSheet` rebuilt every frame
- **File:** `widgets/chat_message_list.dart:9-37`
- **Detail:** `_buildMarkdownStyleSheet(context)` called in every `_MessageBubble.build()`. No caching.

---

## 🔵 Low

### L1. `saveToken` errors silently ignored
- **File:** `internal/cloudsync/drive.go:89, 174, 224, 256`
- **Detail:** `_ = dc.saveToken(t)` — token save failure silently swallowed. Token lost on restart.

### L2. `writeJSON` ignores encode errors
- **File:** `internal/webserver/server.go:462`
- **Detail:** `json.NewEncoder(w).Encode(v)` error not checked. Silent failure on connection close or serialization error.

### L3. `config.Save` doesn't report validation errors
- **File:** `internal/config/config.go:170`
- **Detail:** `cfg.validate()` returns no error, silently corrects invalid values. User doesn't know their input was ignored.

### L4. STT binary world-executable
- **File:** `app.go:289`
- **Detail:** STT binary written to temp dir with 0755 permissions. Other users can read it.

### L5. STT process group not cleaned
- **File:** `app.go:308-311`
- **Detail:** Only main process killed, child processes become orphans.

### L6. `Setpgid` not set for child processes
- **File:** `internal/llama/sysproc_linux.go:12-16`
- **Detail:** Without `Setpgid`, `Pdeathsig` only kills direct child, not descendants.

### L7. Flutter: Missing `const` constructors (widespread)
- **Across entire codebase:**
  - `AppShell()`, `ChatSidebar()`, `ChatInput()`, `WelcomeView()`, countless `SizedBox()`, `Padding()`, `Text()`, `Icon()` calls without `const`.

### L8. Flutter: Empty `catch (_)` blocks swallow errors
- **File:** `core/api_client.dart:68`, `providers/settings_provider.dart:214, 221, 229, 239, 268`
- **Detail:** Errors silently swallowed, no user notification.

### L9. Flutter: `connectionStatusProvider` queries only once
- **File:** `providers/chat_provider.dart:301-303`
- **Detail:** `FutureProvider` runs once. If backend goes down later, status indicator stays green.

### L10. Flutter: Hardcoded Turkish strings bypass L10n system
- **File:** `screens/model_store_screen.dart:58-59, 183, 197-198, 330, 340, 369, 403, 464, 507, 519, 536, 796, 888, 1478, 1501, 1533`
- **Detail:** Turkish text appears in English UI.

---

## 🔴 Critical (Provider/Agent/Orchestra)

### C16. Orchestra Bypasses Provider Router — No Fallback
- **File:** `internal/orchestra/conductor.go:510-540`
- **Detail:** `createProviderForType` creates providers directly via factory, bypassing `provider.Router`. If a provider fails, there is no fallback chain — the task fails immediately.
- **Risk:** Orchestra task failure on transient provider errors.

### C17. Agent Pipeline No Streaming — Blocks UI
- **File:** `internal/agent/pipeline.go:85-180`
- **Detail:** `Pipeline.Run()` uses non-streaming `ChatCompletion` for tool calls. User sees no response until the entire tool-calling loop completes (potentially minutes).
- **Risk:** Poor UX, perceived hang.

## 🟠 High (Provider/Agent/Orchestra)

### H11. Provider Priority Field Unused
- **File:** `internal/provider/config.go`, `router.go:40-55`
- **Detail:** `ProviderConfig.Priority` field exists but `getActiveEntries()` returns providers in insertion order, not by priority.

### H12. Active Provider Not Visible in Provider Settings UI
- **File:** `frontend/lib/widgets/settings_dialog.dart:199-281`
- **Detail:** The `_ProvidersTab` shows provider cards but no indicator of which provider is currently active. User must navigate elsewhere to see active status.

### H13. No Agent API Methods in Frontend ApiClient
- **File:** `frontend/lib/core/api_client.dart`
- **Detail:** Backend has fully working agent endpoints but frontend `api_client.dart` has no methods to call them. Agent mode cannot be toggled from UI.

### H14. Agent Permission Dialog Not Implemented
- **File:** Not created
- **Detail:** Backend emits `EventPermissionRequest` events but there is no frontend dialog to allow/deny tool calls. Agent pipeline blocks forever waiting for permission response.

## 🟡 Medium (Provider/Agent/Orchestra)

### M13. Orchestra Config Has No Validation
- **File:** `internal/orchestra/conductor.go:115-120`
- **Detail:** `UpdateConfig` accepts any role configuration. An invalid chief model or missing role model causes runtime error during execution rather than at config time.

### M14. Agent Pipeline No Timeout Per Tool Call
- **File:** `internal/agent/pipeline.go:120-150`
- **Detail:** Individual tool executions have no timeout. A hanging `run_command` blocks the entire pipeline indefinitely (sandbox has 60s timeout but pipeline doesn't enforce it).

### M15. Agent Audit Log Limited to 1000 Entries
- **File:** `internal/agent/executor.go:40-45`
- **Detail:** `logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.

---

## ⚪ Info / Observations

### I1. GOB Encoding vs. Forward Compatibility
- **File:** `internal/memory/store.go:302-306`

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

> **Last updated:** 2026-06-05
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend
> **Total bugs:** 54 (🔴12, 🟠14, 🟡15, 🔵10) + 11 observations
