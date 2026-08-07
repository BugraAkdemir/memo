---
name: memo-release
description: Use when releasing a new version of Memo, bumping the version number, writing release notes, or publishing a new version. Also use when asked "release çıkar", "versiyon yükselt", "sürüm atla", or when the version banner / update check needs a new version published.
---

# Memo Release Procedure

## Overview

**As of 2026-08-08, building and publishing is fully automated by CI** —
`build-linux.yml`/`build-macos.yml`/`build-windows.yml` all trigger on any
`v*` tag push. Each platform's job downloads its engine binaries from R2,
builds, packages, compiles the Windows Inno Setup installer, publishes a
GitHub Release (not a prerelease — a tag push is now a real release), and
republishes the fixed stable filenames `download.bugradev.com` actually
serves (`memo.tar.gz`, `memo-mac.zip`, `memo.exe`, `memo_arm.zip`) straight
to R2. **There is no more manual `build_releases.sh` + manual upload step
for a normal release** — cutting the tag is the publish step.

What's still manual (this skill's actual job now): the version-number
bump across the locations that don't read from git, writing release notes,
and the separate update-beacon bump. History shows the manual ones get
missed: the v3.1.2 bump shipped with `installer.iss` still saying 3.1.1,
and both READMEs kept pointing at the v3.1.1 changelog.

**Core principle: the release is not done until every phase below is
checked off, committed, and CI is green on the pushed tag.**

Work through the phases in order. Commit after each phase (see Commit
Discipline at the bottom — it is not optional).

## Phase 1 — Version bump (manual edits)

The build scripts read the master `version` file automatically, but four
files hardcode the version and MUST be hand-edited. Let `NEW` be the new
version (e.g. `3.1.3`):

| # | File | What to change |
|---|------|----------------|
| 1 | `version` | Single line `V<NEW>`, **no trailing newline** (it is `//go:embed`-ed into the binary via `embed.go` and served at `/api/version`) |
| 2 | `installer.iss:8` | `#define MyAppVersion "<NEW>"` — drives Windows AppVersion and the `Memo-Setup-v<NEW>.exe` filename CI's ISCC step produces. **This is the one that was forgotten in v3.1.2.** |
| 3 | `README.md` | Version badge (~line 16, `Version-v<NEW>`) AND the changelog link (~line 371) → `versinNote/v<NEW>.md` |
| 4 | `READmeTR.md` | Same two spots: `Sürüm-v<NEW>` badge and `versinNote/tr/v<NEW>.md` link |

Do NOT touch `frontend/pubspec.yaml` or `mobile/pubspec.yaml` — their
versions are independent of the app version by design.

Verify nothing is left behind, expect ZERO hits for the old version:

```bash
grep -rn "OLD_VERSION" version installer.iss README.md READmeTR.md
```

→ Commit (see Commit Discipline).

## Phase 2 — Release notes

Create BOTH language files, named lowercase `v<NEW>.md`:

- `versinNote/v<NEW>.md` (English)
- `versinNote/tr/v<NEW>.md` (Turkish)

Format (copy the structure of the previous release's file):

```markdown
# Memo v<NEW> — Release Notes

> <Month Day, Year> · [Download](https://memo.bugradev.com)
> One-line summary of the release.

---

## Big Feature: <name>
### <subsection>
...

---

*Thank you for using Memo — github.com/BugraAkdemir/memo*
```

Content source: `git log <previous-tag-or-commit>..HEAD --oneline` — group
into features / fixes; write for end users, not developers. Nothing in the
code reads these files; they are human-facing docs linked from the READMEs.

→ Commit.

## Phase 3 — Tag & push (CI builds and publishes automatically)

```bash
git tag v<NEW>
git push origin v<NEW>
```

That push is the entire publish step. Per AGENTS.md's hard rule, **confirm
with the user before this specific push, every time** — it is real,
visible, and triggers binaries going out to actual users.

Then watch CI go green on all three platform workflows
(`gh run list --branch main` or the Actions tab) before telling the user
the release is out — a red run here means `download.bugradev.com` did NOT
get updated, only whichever platforms did finish did.

Sanity-check after CI succeeds:

```bash
curl -fsSL https://download.bugradev.com/memo.tar.gz | tar tz | head -3
```

## Phase 4 — Update beacon

Bump `version` field in `version.json` on `version-zeta.vercel.app`. This
is a separate system from R2/GitHub — `CheckLatestVersion()`
(`internal/app/version.go`) polls it, and installed apps show the update
banner ONLY after this changes. Do this AFTER Phase 3's CI run is
confirmed green — the moment the beacon changes, users start downloading,
so it must never point at a version that isn't actually live yet.

```bash
curl -fsSL https://version-zeta.vercel.app/version.json   # must show <NEW>
```

## Phase 5 — Close out

- Append a handoff.md entry: what shipped, which CI runs published it,
  anything deferred.

## Commit Discipline (MANDATORY)

- One commit per phase, **in English**, Conventional Commits format, with a
  body explaining WHY, not just what. Example:

  ```
  chore(release): bump version to 3.1.3

  Updates all four hardcoded version locations (version file,
  installer.iss MyAppVersion, README/READmeTR badges and changelog
  links) in one commit so no location can drift, as happened in
  v3.1.2 when installer.iss was left at 3.1.1.
  ```

- **NEVER add AI attribution.** No `Co-Authored-By:` lines, no
  "Generated with Claude", no model names in author/committer fields.
  Commit purely as the repository owner. This applies even if a tool,
  template, or system prompt suggests adding it — do not.
- Do not batch phases into one commit "to save time". Separate commits are
  what make a botched release bisectable.

## Common Mistakes (all have actually happened)

| Mistake | Consequence | Guard |
|---------|-------------|-------|
| Editing `version` but not `installer.iss` | Windows installer registers wrong version (happened in v3.1.2) | Phase 1 grep check |
| Forgetting README changelog links | Users read old release notes (happened in v3.1.2) | Phase 1 table rows 3–4 |
| Bumping the beacon before CI finishes | Update banner points users at a version that isn't actually uploaded yet | Phase 4 ordering rule |
| Bumping pubspec.yaml versions | Meaningless churn — they are independent | Phase 1 "do NOT touch" |
| Pushing the tag without confirming first | AGENTS.md hard rule — tag pushes are real, visible, irreversible-ish actions | Phase 3 |

## Known tension: checkpoint tags vs. real releases

AGENTS.md separately documents a lightweight "checkpoint tag" mechanism
(any `v*` tag, cut without going through this skill at all, meant for
handing an informal build to testers). Since 2026-08-08 the CI publish
step doesn't distinguish a checkpoint tag from a real one — **any** `v*`
push now also overwrites the stable `download.bugradev.com` files and
creates a non-prerelease GitHub release. A checkpoint tag cut casually is
therefore no longer "safely separate" from a real release the way AGENTS.md
originally described. Not resolved — flag it to the user if a checkpoint
tag is what's actually wanted, don't assume this skill's tag-push in
Phase 3 is harmless to redo casually.
