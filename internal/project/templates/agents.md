# AGENTS.md — <project name>

> Project-specific facts. Keep this file short and true. If something here is
> wrong, fix it.

## Purpose
<One or two sentences: what this project is and who it is for.>

## Structure
| Path | What it owns |
|------|-------------|
| `<path>` | `<responsibility>` |

Entry points: `<how it starts / is built>`.

## Commands
| Task | Command | Verified |
|------|---------|----------|
| Install | `<cmd>` | yes/no |
| Run | `<cmd>` | yes/no |
| Build | `<cmd>` | yes/no |
| Test | `<cmd>` | yes/no |
| Lint / typecheck | `<cmd>` | yes/no |

## Verification before "done"
<The exact commands that must pass for a change to count as done, and any
checks specific to this project. Known harmless warnings: <list or none>.>

## Conventions
<Only conventions this project ALREADY follows and that are easy to get
wrong: naming, error handling, layering, formatting, test style. Observed,
not invented.>

## Protected / do-not-touch
<Generated files, vendored code, secrets, anything that must not be edited.>

## Project rules (set by the user)
- Commit policy: <ask first / commit when verified / never>
- Branching: <direct to main / feature branches / PRs>
- Language of code and comments: <...>
- Other hard rules and why they exist: <...>

## Known pitfalls
> Living log. Describe symptom and root cause for the next reader.
> When fixed: `~~old text~~ → fixed <date> (<commit>): <what changed>`.
> When wrong or outdated: `→ stale: <what is true>`.

### <area>
- <pitfall>
