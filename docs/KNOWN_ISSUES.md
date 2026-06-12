# Known Issues & Technical Risks

This document tracks all currently open bugs and technical risks in the Memo project.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🟡 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- 🔵 **Low** — cosmetic, minor improvement, or edge case
- ⚪ **Info** — design note, risk, or observation

---

## 🟠 High

### H13. Provider Priority Field Unused
- **File:** `internal/provider/config.go`, `router.go:40-55`
- **Detail:** `ProviderConfig.Priority` field exists but `getActiveEntries()` returns providers in insertion order (Go map iteration), not by priority.

### H14. Active Provider Not Visible in Provider Settings UI
- **File:** `frontend/lib/widgets/settings_dialog.dart:199-281`
- **Detail:** The `_ProvidersTab` shows provider cards but no indicator of which provider is currently active. User must navigate elsewhere to see active status.

### H15. No Agent API Methods in Frontend ApiClient
- **File:** `frontend/lib/core/api_client.dart`
- **Detail:** Backend has fully working agent endpoints but frontend `api_client.dart` has no methods to call them. Agent mode cannot be toggled from UI.

### H16. Download Progress Polling Never Stops
- **File:** `frontend/lib/providers/models_provider.dart:69-81`
- **Detail:** `downloadProgressProvider` has an infinite `while (true)` loop. It polls `/api/models/download/progress` every 1 second for the **entire app lifetime**, even when no download is active. 1 HTTP request/second forever.
- **Risk:** Wasted CPU/network, battery drain on laptops.

---

## 🟡 Medium

### M15. Orchestra Config Has No Validation
- **File:** `internal/orchestra/conductor.go:115-120`
- **Detail:** `UpdateConfig` accepts any role configuration. An invalid chief model or missing role model causes runtime error during execution rather than at config time.

### M16. Agent Pipeline No Timeout Per Tool Call
- **File:** `internal/agent/pipeline.go:120-150`
- **Detail:** Individual tool executions have no timeout. A hanging `run_command` blocks the entire pipeline indefinitely (sandbox has 60s timeout but pipeline doesn't enforce it).

### M17. Agent Audit Log Limited to 1000 Entries
- **File:** `internal/agent/executor.go:40-45`
- **Detail:** `logEntries` slice is capped at 1000. Old entries are silently dropped. No rotation or persistence.

### M18. 19 Empty `catch (_)` Blocks Swallow Errors
- **File:** Across `frontend/lib/` — `providers/chat_provider.dart`, `providers/models_provider.dart`, `providers/agent_provider.dart`, `providers/whatsapp_provider.dart`, `providers/orchestra_provider.dart`, `providers/provider_provider.dart`, `widgets/chat_input.dart`, `widgets/agent/permission_dialog.dart`, `widgets/agent/agent_chat_card.dart`, `widgets/setup_wizard_view.dart`
- **Detail:** `catch (_) {}` silently swallows all errors. When a provider call fails, the user sees no error message — the app silently does nothing.
- **Risk:** Users have no feedback when operations fail (config save, model listing, agent toggle, etc.).

---

## ⚪ Info / Observations

### I1. `App.ctx` stored in struct (anti-pattern)
- **File:** `app.go:227`
- **Note:** Per Go `context` documentation, contexts should be passed explicitly, not stored in structs.

### I2. Flutter: L10n uses custom listener pattern instead of Riverpod
- **File:** `core/l10n.dart:8`
- **Note:** Two parallel notification systems for locale.

### I3. Flutter: Hardcoded Turkish strings bypass L10n
- Various widgets still contain hardcoded Turkish text.

### I4. Missing `const` constructors (widespread)
- Across entire Flutter codebase.

### I5. No Test Files for Provider/Agent/Orchestra
- **File:** `internal/provider/`, `internal/agent/`, `internal/orchestra/`
- **Note:** Zero unit tests exist for the three new packages (~4150 lines of production code).

### I6. Legacy GOB Format (migrated to SQLite)
- **File:** `internal/memory/store.go` (legacy migration path)

### I7. Single-File-Per-Interaction Design
- **File:** `internal/memory/store.go`

### I8. Filepath.Walk Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`

### I9. Embedding Client Stale Reference After Reinit
- **File:** `app.go:148-149`, `app.go:124-125`

### I10. Model Auto-Classification via Filename
- **File:** `internal/modelstore/modelstore.go:58-64`

### I11. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`

### I12. Llama Server Stderr Mixed with App Logs
- **File:** `internal/llama/llama.go:118-119`

---

> **Last updated:** 2026-06-12
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend
> **Open bugs:** 7 (🟠4, 🟡3)
> **Observations:** 12
