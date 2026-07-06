---
name: memo-release
description: Use when releasing a new version of Memo, bumping the version number, building distribution packages (tar.gz, AppImage, deb, exe, dmg), writing release notes, or publishing artifacts to download.bugradev.com. Also use when asked "release çıkar", "versiyon yükselt", "sürüm atla", or when the version banner / update check needs a new version published.
---

# Memo Release Procedure

## Overview

A Memo release touches **seven version locations, three build scripts, two
upload targets** — and history shows the manual ones get missed: the v3.1.2
bump shipped with `installer.iss` still saying 3.1.1, and both READMEs kept
pointing at the v3.1.1 changelog. This skill exists so no step is skipped.

**Core principle: the release is not done until every phase below is checked
off and committed. Partial releases (bumped but not uploaded, uploaded but
version.json not updated) leave users on broken update paths.**

Work through the phases in order. Commit after each phase (see Commit
Discipline at the bottom — it is not optional).

## Phase 1 — Version bump (manual edits)

The build scripts read the master `version` file automatically, but four
files hardcode the version and MUST be hand-edited. Let `NEW` be the new
version (e.g. `3.1.3`):

| # | File | What to change |
|---|------|----------------|
| 1 | `version` | Single line `V<NEW>`, **no trailing newline** (it is `//go:embed`-ed into the binary via `embed.go` and served at `/api/version`) |
| 2 | `installer.iss:8` | `#define MyAppVersion "<NEW>"` — drives Windows AppVersion and `Memo-Setup-v<NEW>.exe` filename. **This is the one that was forgotten in v3.1.2.** |
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

> **Open Beta** · <Month Day, Year> · [Download](https://memo.bugradev.com)
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

## Phase 3 — Build

All artifacts land in `build_output/dist/` with **versioned** filenames.

| Platform | Command | Artifacts |
|----------|---------|-----------|
| Linux (this machine) | `./build_releases.sh` | `Memo-linux-x64-v<NEW>.tar.gz`, `.AppImage` (deb off by default) |
| Windows | `build_releases.bat` on a Windows machine | `Memo-Setup-v<NEW>.exe` via Inno Setup (`iscc`); falls back to `.zip` if iscc missing — **the .exe is required, the zip fallback is NOT a release artifact** |
| macOS | `./build_releases.sh` on a Mac (needs Xcode) | `Memo-macos-<arch>-v<NEW>.zip` — **this zip is what gets published as `memo-mac.zip`** |
| macOS (alt) | `./macrelease.sh` on a Mac | `Memo-macos-<arch>-v<NEW>.tar.gz`, `.dmg` — for manual/DMG distribution only; do NOT upload the tar.gz as `memo-mac.zip` (`get-memo.sh` unzips, extension must really be zip) |

Before building: verification suite must be green (`CGO_ENABLED=1 go test
./... -race`, `go build ./...`, `flutter analyze lib/`, `flutter test` —
see AGENTS.md). Never build a release from a red tree.

Sanity-check each artifact after build: the tar.gz/zip contains
`run_memo.sh`/`run_memo.bat` AND `launch.vbs` staging output where
applicable, and `./memo --version`-equivalent (`curl localhost:8090/api/version`
after launch) reports `<NEW>`.

## Phase 4 — Publish (two separate targets, BOTH required)

**Target A — download server (`download.bugradev.com`):** upload with
**generic names** (the installers fetch fixed URLs — rename on upload):

| Built file | Upload as |
|------------|-----------|
| `Memo-linux-x64-v<NEW>.tar.gz` | `memo.tar.gz` |
| `Memo-macos-<arch>-v<NEW>.zip` (from `build_releases.sh`, not the macrelease.sh tar.gz) | `memo-mac.zip` |
| `Memo-Setup-v<NEW>.exe` | `memo.exe` |
| `get-memo.sh` / `get-memo.ps1` / `update.sh` / `uninstall.sh` | same names (only if changed this release) |

**Target B — update beacon:** bump `version` field in `version.json` on
`version-zeta.vercel.app`. This is what `CheckLatestVersion()`
(`internal/app/version.go`) polls; installed apps show the update banner
ONLY after this changes. Upload artifacts FIRST, beacon LAST — the moment
the beacon changes, users start downloading.

Post-publish smoke test:

```bash
curl -fsSL https://download.bugradev.com/memo.tar.gz | tar tz | head -3
curl -fsSL https://version-zeta.vercel.app/version.json   # must show <NEW>
```

## Phase 5 — Close out

- Tag: `git tag v<NEW> && git push --tags` (if remote configured).
- Append a handoff.md entry: what shipped, artifact checksums if computed,
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
| Uploading versioned filenames as-is | `get-memo.sh` 404s — it fetches `memo.tar.gz`, not `Memo-linux-x64-v*.tar.gz` | Phase 4 rename table |
| Updating version.json before uploads finish | Update banner points users at stale/missing artifacts | Phase 4 ordering rule |
| Skipping the Windows `.exe` because iscc missing | `get-memo.ps1` serves an outdated installer | Phase 3 — zip fallback is not a release |
| Bumping pubspec.yaml versions | Meaningless churn — they are independent | Phase 1 "do NOT touch" |
