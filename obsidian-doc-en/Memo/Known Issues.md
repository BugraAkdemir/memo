# Known Issues & Technical Risks

Exhaustive bug audit from the full codebase review (2026-06-03). Full detail: `docs/KNOWN_ISSUES.md`.

**Status**: 54 total bugs identified → 46 fixed, 8 still open.

---

## 🔴 Critical (All Fixed)

| ID | Issue | Fix |
|----|-------|-----|
| C1 | Data race on `a.syncManager` — concurrent read/write | `getSyncManager()` helper with Lock/RLock (K16) |
| C2 | `a.store` assigned without `storeMu` at startup | Wrapped in Lock/Unlock |
| C6 | OAuth server leak + `authWg` race on duplicate `StartAuthFlow()` | `authSrv` field, `authDone` flag (K19) |
| C9 | Legacy Gob migration index inconsistency | Delete-first strategy, hash-based ID (K20) |
| C10 | Orphaned syncManager goroutines on config update | Old manager set nil under lock (K16) |
| C11 | Flutter: context used after `Navigator.pop()` | ScaffoldMessenger captured before pop (K21) |
| C12 | Flutter: Context menu async context use | `if (!mounted) return;` guards (K21) |
| C13 | Flutter: TextEditingController created in build(), never disposed | StatefulWidget + dispose (K21) |
| C14 | Flutter: setState after async without mounted check | Guard added (K21) |
| C15 | Flutter: FocusNode.requestFocus() after async without mounted check | Guard added (K21) |
| C16 | Orchestra bypasses Provider Router — no fallback | Providers must be registered in Router |
| C17 | Agent pipeline no streaming — blocks UI | ChatCompletionStream with progress updates |

## 🟠 High (4 still open)

**Fixed:**
- H1: OAuth callback duplicate Done panic (K19)
- H2: callLLMStream goroutine runs 5min after disconnect (K11+K17)
- H3: Concurrent AddMessage reorders messages (K22)
- H4: isAuthenticated has no timeout (K19)
- H5: Nil embeddingClient causes panic (K23)
- H6: Memory silently disabled on startup error (K23)
- H7-H10: Various Flutter issues (K21)
- H11: Ngrok early return — config not saved
- H12: Ngrok crash not shown in UI

**Still Open:**
- H13: Provider Priority field unused (`router.go:40-55`)
- H14: Active provider not visible in settings UI
- H15: No agent API methods in frontend ApiClient
- H16: Agent permission dialog not implemented

## 🟡 Medium (3 still open)

**Fixed:** M1-M14 (context, path traversal, body size limits, temp files, race conditions, GitHub timeouts, zip bomb, Flutter issues, ngrok API/install)

**Still Open:**
- M15: Orchestra config has no validation
- M16: Agent pipeline no timeout per tool call
- M17: Agent audit log limited to 1000 entries

## 🔵 Low (1 still open)

**Fixed:** L1-L12 (error logging, permissions, process cleanup, Flutter const constructors, connection polling, L10n strings, ngrok UI)

**Still Open:**
- L7: Flutter missing `const` constructors (widespread)

## ⚪ Info / Observations

- I1: Legacy GOB format (migrated to SQLite)
- I2: Single-file-per-interaction design
- I3: Filepath.Walk error swallowing
- I4: Embedding client stale reference after reinit
- I5: Model auto-classification via filename
- I6: `unsanitizePath` can inject `/` from `__`
- I7: Llama server stderr mixed with app logs
- I8: `App.ctx` stored in struct (anti-pattern)
- I9: Flutter L10n uses custom listener instead of Riverpod
- I10: Flutter hardcoded Turkish strings
- I11: No test files for provider/agent/orchestra
