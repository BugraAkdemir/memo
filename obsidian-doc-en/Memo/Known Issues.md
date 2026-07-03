# Known Issues & Technical Risks

> Updated: July 4, 2026 — re-verified against current source ahead of the v3.1.1 open beta.

**Summary**: 14 known issues documented, 11 fixed (one this pass: polling leaks), 3 remaining. Most are design-level technical debt, not bugs. See `docs/KNOWN_ISSUES.md` and `docs/tr/BILINEN_SORUNLAR.md` for the full itemized list with code references.

---

## 🔴 Data Races

**a.client reassignment during streaming** (`clientMu` exists but streaming goroutines may hold stale reference when model stops/starts mid-stream). Same pattern for `providerRouter` reassignment.

**Status**: Known, tolerated. Local-only app makes this low-risk. Would require connection pooling to fix properly.

---

## 🟠 Memory / Vector Store

- **Full rebuild on startup** — `LoadCache` is O(N), no incremental index. Acceptable for personal-scale usage.
- **Embedding model requires manual start** — config-driven auto-start exists but must be configured.

**Status**: Design trade-off. Not broken, just not optimized for large corpuses.

---

## 🟡 Provider / Agent / Orchestra

| Issue | Status |
|-------|--------|
| `provider.Priority` field exists but unused by router | Design debt — sort logic present, not wired |
| Orchestra bypasses `provider.Router` — creates providers directly, no fallback chain | Architecture limitation |
| No test files for `orchestra/` package (~800 lines) | Coverage gap |
| Agent frontend UI (permission dialog, tool call cards) not fully implemented | Partial — basic dialog exists, streaming events rendered |

---

## 🟢 Flutter

- `model_store_screen.dart` is 2469 lines — should be split into components
- Widespread missing `const` constructors (lint warnings)
- ~~`connectionStatusProvider` and download progress polling run forever~~ ✅ fixed — both are now `autoDispose` with adaptive intervals

**Recently fixed**: `settings_dialog.dart` was split from 5013 → 15 files. ✓

---

## 🔵 Other

| Issue | Status |
|-------|--------|
| `skill.DangerLevel` and `agent.DangerLevel` separate types | Compile-time type mismatch — needs unification |
| No API versioning strategy | Flat `/api/` prefix, no `/v1/`, `/v2/` |
| Gradual logging migration | `webserver/` uses `logx`; other packages still use `log.Printf` |

---

## ✅ Recently Fixed (v3.1.0 Polish)

| Issue | Original Status | Fix |
|-------|----------------|-----|
| Hardcoded encryption key in source | Security risk | `crypto/rand` + `data/machine.key` (0600) |
| No request body size limits | DoS vector | 50MB `limitBodyMiddleware` on all handlers |
| Config files world-readable | Privacy risk | `0600` permissions on all sensitive writes |
| WhatsApp store no serialized writes | Data corruption risk | `sync.Mutex` on `SaveMessage` + `SaveContact` |
| Calendar reminder double-fire | UX bug | `ClaimPendingReminders()` atomic transaction |
| ngrok no auto-recovery | Reliability | 5s auto-restart on crash |
| QR polling never stops | UX issue | Adaptive: 2s during QR, 15s heartbeat |
| `handleHistorySync` only on first pairing | Data gap | `INSERT OR IGNORE` makes it safe on reconnects |
| Hardcoded `active_provider: openai` in config | Override bug | Changed to empty string default |
| No CI pipeline | Quality | GitHub Actions: Go + Flutter auto-test on push |
