# Agent Templates

Reusable, project-agnostic versions of the three files this project uses to
keep a coding agent (Claude Code or similar) reliable across long,
multi-session work:

| File | Purpose |
|------|---------|
| `AGENTS.example.md` → `AGENTS.md` | Project map, working rules, verification commands, and a running "Known Pitfalls" log. Read at the start of every session. |
| `handoff.example.md` → `handoff.md` | Chronological session log — newest entry on top. Written at the end of every session so context survives to the next one. |
| `BUG_REPORT.example.md` → `BUG_REPORT.md` | The list of bugs that are open *right now*. Fixed bugs are deleted from it, not archived here — their permanent record is git history and `AGENTS.md`. |

## Using these in a new project

1. Copy all three files into the new project's root, stripping the
   `.example` suffix (`AGENTS.example.md` → `AGENTS.md`, etc.).
2. Fill in every `<...>` placeholder — tech stack, module map, commands,
   commit conventions, gotchas specific to that codebase.
3. Delete any section that doesn't apply rather than leaving it as an empty
   placeholder (e.g. no mobile client → delete the mobile section).
4. Keep the **process** sections close to verbatim — the Agent Working
   Rules, the verification-before-"done" discipline, the
   `~~struck~~ → fixed` convention in Known Pitfalls, and the "prepend, one
   entry per session, always end with Next Session" rule for the handoff
   log. These aren't project facts; they're what makes the other two files
   trustworthy after months of sessions instead of just rotting into stale
   claims nobody re-verifies.

These three templates were extracted from this project's own
`AGENTS.md`/`handoff.md`/`BUG_REPORT.md` after they proved themselves over
many real sessions — the structure isn't theoretical, it's what this project
actually uses.
