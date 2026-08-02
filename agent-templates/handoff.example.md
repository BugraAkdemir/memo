# handoff.md — how this file works

`handoff.md` is the running session log a coding agent reads at the **start**
of every session and writes to at the **end** of every session. It is what
lets context survive across sessions and across different models — the agent
should never have to reconstruct "what was I doing" from git log alone.

**Rules for this file:**

1. **Newest entry always goes on top.** Prepend, never append — the top of
   the file is always "what happened most recently and what's pending."
2. **One entry per session**, using the exact heading format below. If a
   session has multiple distinct legs (e.g. resumed later the same day),
   either add a new dated entry or clearly mark a sub-section — don't merge
   unrelated work into one undifferentiated blob.
3. **Every entry ends with a "Next Session" section.** This is the single
   most important part of the entry — it's the actual handoff. Be specific:
   name files, name the exact next step, name what was *deliberately left
   out of scope* and why (so it isn't silently forgotten or redone).
4. **Verification results are pasted, not summarized.** "Tests pass" is not
   verification; the actual command and its actual output (or a faithful
   excerpt) is.
5. This file is allowed to grow long. Don't prune old entries — they're the
   permanent record. If it gets unwieldy to read top-to-bottom, that's a sign
   to `grep` for a module/date, not a sign to delete history.
6. Fixed bugs get their permanent record in `AGENTS.md`'s Known Pitfalls
   section (with the `~~struck~~ → fixed` convention) and in git history —
   this file's job is session narrative and handoff state, not a bug
   database. Open bugs live in `BUG_REPORT.md`.

Below are two example entries showing the expected shape — a bug-fix session
and a feature session. Delete them once the project has its own real history.

---

# Handoff — <YYYY-MM-DD> (Session <N>) — <One-line title of what this session did>

## Summary

<2-5 sentences: what problem or request kicked off this session, what
approach was taken, and the headline outcome. Write this for someone with
zero memory of the conversation — no "as discussed," no unexplained
shorthand.>

**Commit status:** `<commit-hash>`, `<commit-hash>` — <one clause per commit,
or "not yet committed, pending user confirmation" if that's the actual
state. Never leave this ambiguous; the next session needs to know exactly
what's safely on disk in git vs. what's still uncommitted working-tree state.>

---

## What Was Done

### 1. <Sub-task title>

**Root cause:** <if this was a bug fix, the actual mechanism — not just the
symptom. "It was broken" is not a root cause; "X read a shared field without
holding Y's lock, causing Z under concurrent access" is.>

| File | Change |
|------|--------|
| `<path>` | <what changed and why> |
| `<path>` | <what changed and why> |

**Deliberately not done / out of scope:** <anything a thorough pass would
also touch but was consciously left alone this session, and why — scope
creep avoided, a decision that needs the user's input first, a fix that's
"good enough" but not complete. This is as important to record as what
*was* done — it prevents the next session from either redoing the analysis
or assuming something is finished when it isn't.>

### 2. <Sub-task title, if the session had more than one>

<Same shape as above.>

---

## Verification

```bash
$ <exact command run>
<exact or faithfully excerpted output>
```

```bash
$ <exact command run>
<exact or faithfully excerpted output>
```

<Explicitly state anything that was NOT verified and why — e.g. "not tested
against a real <external system>, no such environment available here" or
"unit-tested only; no live device/browser verification in this environment."
An honest gap here is far more useful to the next session than a claim of
completeness that isn't true.>

---

## Next Session

1. <The single most important next step, named specifically — file, function,
   or plan item, not just a vague area.>
2. <Anything flagged above as "deliberately not done" that the user should
   be asked about or that should be picked up next.>
3. <Any known risk or fragile assumption introduced this session that the
   next session should keep an eye on.>

---

# Handoff — <YYYY-MM-DD> (Session <N-1>) — <Title of the prior session>

## Summary

<Same shape as above — this is here purely to illustrate that entries stack
chronologically, newest on top. A real project will have many of these
accumulating over time.>

## Next Session

1. <...>
