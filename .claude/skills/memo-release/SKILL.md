---
name: memo-release
description: Use when releasing a new version of Memo, bumping the version number, writing release notes, or publishing a new version. Also use when asked "release çıkar", "versiyon yükselt", "sürüm atla", or when the version banner / update check needs a new version published.
---

# Memo Release Procedure

## Overview

**As of 2026-08-08, building and publishing is fully automated by CI** —
`build-linux.yml`/`build-macos.yml`/`build-windows.yml` (plus
`build-android.yml`/`build-ios.yml` since 2026-09-28) all trigger on any
`v*` tag push. Each platform's job downloads its engine binaries from R2,
builds, packages, compiles the Windows Inno Setup installer, publishes a
GitHub Release (not a prerelease — a tag push is now a real release), and
republishes the fixed stable filenames `data.memocpp.com` actually
serves (`memo.tar.gz`, `memo-mac.zip`, `memo.exe`, `memo_arm.zip`, and the
mobile `memo.apk` / `memo.ipa` (betas: `memo-beta.apk` / `memo-beta.ipa`) — unsigned, sideload-only)
straight to R2. The mobile APK/IPA take their version name from the
root `version` file at build time, so `frontend/pubspec.yaml` still needs
no bump. **There is no more manual `build_releases.sh` + manual upload step
for a normal release** — cutting the tag is the publish step.

What's still manual (this skill's actual job now): the version-number
bump across the locations that don't read from git, writing release notes,
and the separate update-beacon bump. History shows the manual ones get
missed: the v3.1.2 bump shipped with `installer.iss` still saying 3.1.1,
and both READMEs kept pointing at the v3.1.1 changelog.

Since 4.7.0 CI also **signs and publishes the in-app update manifests**
(`update/app/<X.Y.Z>/<platform>.json` + `.sig` on `data.memocpp.com`) right after each
platform's artifact is uploaded, using the `MEMO_UPDATE_SIGNING_KEY` Actions secret, and
builds `memo-updater` into the Linux/macOS releases (see `docs/AUTO_UPDATE.md`). A tag
that does not match the `version` file (a checkpoint tag) publishes no manifest.

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

Do NOT touch `frontend/pubspec.yaml` — its version is independent of the
app version by design.

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

> <Month Day, Year> · [Download](https://memocpp.com)
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
the release is out — a red run here means `data.memocpp.com` did NOT
get updated, only whichever platforms did finish did.

Sanity-check after CI succeeds — two checks, not one:

```bash
curl -fsSL https://data.memocpp.com/memo.tar.gz | tar tz | head -3
```

That alone only proves *a* tarball is being served — it does not prove
it's the one CI *just* published (R2/CDN could be serving a stale cached
copy under the same fixed filename). Confirm freshness directly with a
`HEAD` request and check `Last-Modified` against the actual time the tag
push's CI runs finished — it should be within a couple of minutes, not
hours or days old:

```bash
curl -sI https://data.memocpp.com/memo.tar.gz | grep -i last-modified
```

Then verify what installed apps will fetch (manifests + signatures + the size of each
file) — this downloads only manifests and issues HEAD requests:

```bash
scripts/verify_update_manifest.sh          # defaults to the version file; exit 1 = do NOT bump the beacon
```

Downloading the full ~750MB tarball just to peek inside it (`tar tz` on a
piped-and-cut download, or re-downloading to extract the embedded
`version`) is slow and has actually hung/failed mid-transfer before —
the `HEAD`-only freshness check above is enough and doesn't need to know
the tarball's internal layout, which isn't guaranteed stable anyway.

## Phase 4 — Update beacon

The beacon is `version.json` in the **`BugraAkdemir/version`** repo
(`/home/bugra/Documents/version`), served at `version-zeta.vercel.app/version.json`;
pushing to its `main` deploys it. Installed apps poll it and, when it names a newer
version, open the update window — so bump it **last**, only after Phase 3 is green AND
`scripts/verify_update_manifest.sh` passed. The moment it changes, users start
downloading; it must never point at a release that does not check out.

```json
{
  "version": "V<NEW>",
  "mini_version_note_tr": "kısa not (güncelleme penceresinde görünür)",
  "mini_version_note_en": "short note shown in the update window",
  "required_update": false,
  "components": { "llama": "b9441", "whisper": "1.8.6", "cliproxy": "v8.0.25" }
}
```

* `version` — the release being announced (`V4.7.0`).
* The two notes — plain text, ≤ 600 characters, shown as-is (no markup). Write what the
  person gains, not the changelog.
* `required_update: true` hides "Skip this version" ("Later" stays). Use it sparingly.
* `components` — the newest engine tags. **Keep it equal to what the release bundles**
  unless an engine release was published with `scripts/publish_component.sh`; a tag
  here with no signed manifest behind it simply offers nothing.

```bash
cd /home/bugra/Documents/version
# edit version.json, then
git add version.json && git commit -m "chore(release): announce Memo <NEW> to installed clients" && git push origin main
curl -fsSL https://version-zeta.vercel.app/version.json   # must show <NEW> (allow a minute for the deploy)
```

An engine-only release (no new Memo) changes only `components` and leaves `version`
alone, so older apps show nothing.

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
| Bumping the beacon before `verify_update_manifest.sh` passes | Apps look for a release whose manifest is missing or inconsistent and quietly fall back to the download page | Phase 4 ordering rule |
| Publishing an engine folder without its other component's marker | `linux/cpu` holds llama.cpp AND whisper.cpp; a whole-folder replace that lacks one breaks it | `scripts/publish_component.sh` refuses a folder whose marker is not the version published |
| Pushing the tag without confirming first | AGENTS.md hard rule — tag pushes are real, visible, irreversible-ish actions | Phase 3 |

## Known tension: checkpoint tags vs. real releases

AGENTS.md separately documents a lightweight "checkpoint tag" mechanism
(any `v*` tag, cut without going through this skill at all, meant for
handing an informal build to testers). Since 2026-08-08 the CI publish
step doesn't distinguish a checkpoint tag from a real one — **any** `v*`
push now also overwrites the stable `data.memocpp.com` files and
creates a non-prerelease GitHub release. A checkpoint tag cut casually is
therefore no longer "safely separate" from a real release the way AGENTS.md
originally described. Not resolved — flag it to the user if a checkpoint
tag is what's actually wanted, don't assume this skill's tag-push in
Phase 3 is harmless to redo casually.
