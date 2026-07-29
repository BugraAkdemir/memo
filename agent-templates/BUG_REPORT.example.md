# BUG_REPORT.md — <Project Name> Open Bug List

> **Purpose:** the list of bugs that are genuinely open right now and block a
> stable release. **Fixed bugs are not kept here** — once fixed, delete the
> entry entirely. Its permanent record is git history (the commit message)
> and, if it's the kind of finding worth a future agent knowing about even
> after it's fixed, `AGENTS.md`'s Known Pitfalls section (`~~struck~~ →
> fixed` convention). This file's only job is: *what's still broken, right
> now, today.*
>
> **Last updated:** <YYYY-MM-DD> — <one-line pointer to the most recent
> change to this list, e.g. "TD-2 closed (`<hash>`), see below" or "3 new
> findings from this session's review, see HIGH section.">

---

## Summary

| Severity | Open |
|----------|------|
| 🔴 CRITICAL | `<N>` |
| 🟠 HIGH | `<N>` |
| 🟡 MEDIUM | `<N>` |
| 🟢 LOW | `<N>` |
| 🔧 TECH DEBT | `<N>` |
| **TOTAL** | **`<N>`** |

**Severity guide** (keep this consistent across sessions/agents so the table
above stays meaningfully comparable over time):

- **CRITICAL** — data loss/corruption, security vulnerability, or the app is
  unusable for its primary purpose. Blocks any release.
- **HIGH** — a real feature is broken or produces wrong results for a common
  case. Should block the next release unless explicitly deferred.
- **MEDIUM** — a real bug, but narrow (edge case, cosmetic-but-confusing,
  workaround exists).
- **LOW** — minor/cosmetic, no workaround needed, or affects a rarely-used
  path.
- **TECH DEBT** — not a bug a user would notice today, but a maintainability
  or scalability risk (missing tests on a critical path, an architecture
  decision that will need revisiting before the next big feature, etc.).

---

## Open Bugs

### 🔴 CRITICAL

- **`<ID>`** `<one-line title>` — `<path/to/file.ext>`
  <2-4 sentences: exact reproduction, root cause if known, current impact.
  If a fix is in progress or a fix attempt was tried and reverted, say so
  and why it didn't work — that's valuable information for whoever picks
  this up next, so the same dead end isn't re-explored from scratch.>

### 🟠 HIGH

- **`<ID>`** `<one-line title>` — `<path/to/file.ext>`
  <same shape as above.>

### 🟡 MEDIUM

- **`<ID>`** `<one-line title>` — `<path/to/file.ext>`

### 🟢 LOW

- **`<ID>`** `<one-line title>` — `<path/to/file.ext>`

### 🔧 TECH DEBT

- **`<ID>`** `<one-line title>` — `<path/to/file.ext>`
  <What the actual future risk is — e.g. "no tests on this ~800-line
  package," "this works today but won't scale past N," "this is a conscious
  shortcut, here's the real fix it's standing in for.">

---

## Residual (not a fix, just a thing to keep tracking)

<Anything that was addressed but not to 100% completion — e.g. "L10n pass
closed the main dialogs; a handful of low-traffic screens are still
unaudited" or "streaming race class fixed for the two known paths; sweep for
any others if a similar symptom is reported again.">

---

## Recently Closed (optional — delete this section if the team prefers pure
## git-log-as-record with zero duplication here)

> Some teams like a short, time-boxed "closed this week" list here for
> stand-up/changelog purposes even though the permanent record is git log +
> AGENTS.md. If you keep this section, prune it aggressively — it should
> never become a second copy of full project history.

- `<ID>` `<one-line what it was>` → fixed `<date>` (`<commit-hash>`)

---

*A bug fixed in this list should be deleted from it entirely, not marked
struck-through — `git log` and commit messages are the permanent record for
this file's purpose. (Contrast with `AGENTS.md`'s Known Pitfalls section,
which deliberately does keep struck-through history — the two files have
different jobs: this one is "what's open right now," that one is "what have
we learned about this codebase over time.")*
