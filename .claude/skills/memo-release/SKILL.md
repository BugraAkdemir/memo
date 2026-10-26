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
  "components": { "llama": "b11669", "whisper": "1.9.5", "cliproxy": "v8.0.43" }
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

## Engine release (llama.cpp, whisper.cpp, CLIProxyAPI) — no new Memo

Engines ship on their own: installed apps (and the installer scripts, see step 7) pick up a newer
engine from the beacon without a Memo release. Do this when upstream moved (the scheduled
"Upstream watch" workflow opens an `upstream-notice` issue per CLIProxyAPI release) or when an engine
fix cannot wait. Order matters — **the beacon is always last**, and nothing here is announced until
step 6. Every R2 write below is user-visible: confirm with the person before the first one.

**0. Look first.** `gh release list -R router-for-me/CLIProxyAPI`, `ggml-org/llama.cpp`,
`ggml-org/whisper.cpp`; read the notes between the pinned and the new version. Pinned versions live in
`internal/cliproxy/PINNED.txt` (CLIProxyAPI) and `internal/components/ENGINES.txt` (llama/whisper,
per folder). llama.cpp now publishes stable `v0.N.0` releases whose only asset is `nightly-tag.txt`
naming the `bNNNN` build; **Memo's tooling speaks `bNNNN`**, so use that tag (`gh release download
v0.7.0 -R ggml-org/llama.cpp -p nightly-tag.txt -O -`).

**1. CLIProxyAPI.** Edit `PINNED.txt`: `version`, then per asset the archive digest (`gh api
repos/router-for-me/CLIProxyAPI/releases/tags/<v> -q '.assets[]|"\(.name) \(.digest)"'`) and the digest
of the EXTRACTED binary (download, extract, `sha256sum`). Then `scripts/vendor_cliproxy.sh` (lays out
`binaries/…/cliproxy*` and refuses a digest mismatch) and `scripts/verify_cliproxy.sh binaries
linux/cliproxy linux/cliproxy-arm64 windows/cliproxy darwin/cliproxy`. `go test ./internal/cliproxy`.
Test the REAL binary before R2 is touched: serve `binaries/` with `python3 -m http.server`, point `r2Base`
in `internal/cliproxy/upstream_test.go` at it, run `UPSTREAM_DEEP=1 go test -tags upstream
./internal/cliproxy -run RealBinary`, then `git checkout` the test file. Then R2 (step 5), commit
`PINNED.txt` and push it **right after** R2 is updated (the builds check R2 against the pin).

**2. llama.cpp.** `scripts/vendor_llama.sh -t bNNNN linux/cpu linux/amd linux/cpu-arm64` stages the new
engine into `binaries-staged/` without touching `binaries/` and refuses a folder where a helper would lose a
library. NVIDIA stays on its old build unless CUDA builds Memo can use are published (check
`gh release view bNNNN -R ggml-org/llama.cpp`); folders not published are simply not offered.
Windows/macOS folders have no version marker (`ENGINES.txt` lists only proven Linux folders) — do not
publish them from this machine. **Test the staged `llama-server` with a real model** (Phi-3 chat +
nomic embedding in `data/models/`, the flags from `internal/llama/llama.go`), not just `--version`.
Remove stale leftovers from the staged folder (a `llama-new/` directory from an old attempt), then
`rclone sync binaries-staged/linux/<f> binaries/linux/<f>`.
`linux/cpu` is staged WITH `libggml-vulkan.so` (the script fetches it from the same tag's `ubuntu-vulkan-x64`
asset and checks its SHA-256): it is what lets the cpu engine use an integrated/unsupported GPU for prompt
processing (~2x), see AGENTS.md "Local-model speed". After staging, `LD_LIBRARY_PATH=<folder> <folder>/llama-server
--list-devices` must print a `Vulkan0:` line on a machine with a Vulkan driver, and the folder must still
start `whisper-server` (it passes `--no-gpu` itself). Never add the plugin to `linux/amd`.

**3. whisper.cpp** lives in the SAME `linux/cpu` folder, so it is built to NOT depend on llama's ggml:
clone the tag, then `cmake -DBUILD_SHARED_LIBS=OFF -DGGML_NATIVE=OFF -DGGML_AVX2=ON -DGGML_FMA=ON
-DGGML_F16C=ON -DGGML_BMI2=OFF -DGGML_AVX512=OFF -DGGML_OPENMP=OFF -DWHISPER_BUILD_EXAMPLES=ON
-DCMAKE_EXE_LINKER_FLAGS="-static-libstdc++ -static-libgcc"` and build `whisper-server`. Check
`objdump -T whisper-server | grep -o 'GLIBC_[0-9.]*' | sort -Vu | tail -1`: it must not need more than the
old build (2.38). A host with a newer glibc leaks new symbol versions — 2026-10 `sqrtf@GLIBC_2.43`; pin it
with `-include <file with __asm__(".symver sqrtf,sqrtf@GLIBC_2.2.5");>` in `CMAKE_C_FLAGS/CXX_FLAGS`.
Copy it over `binaries/linux/cpu/whisper-server`, delete `libwhisper.so.1`, and **transcribe a real
wav from inside the folder** (`whisper-server --model ggml-small.bin …`, POST `/inference`): a static
whisper-server still `load_backend`s the folder's `libggml-*.so`, so test it where it will run.

**4. Declare and stamp.** Edit `internal/components/ENGINES.txt` (one `llama`/`whisper` line per set of
folders; add an "Evidence" comment), then `scripts/stamp_engine_versions.sh stamp --from
internal/components/ENGINES.txt binaries` and the same with `check`. `go test ./internal/components
./internal/app`. If the engine prints its version differently, fix `llamaReportsBuild`/`verifyEngine`
in `internal/app/update.go` FIRST: apps older than that fix cannot verify the new engine, which is what
`--requires-app` (step 5) is for. (b11669 prints `version: 0.7.0-dev (build 11669, …)`; apps before the
fix look for `version: 11669` and would roll every update back.)

**5. R2 — back up, then `sync`, never `copy`.** `rclone copy` overwrites but never deletes, so an old
`libggml*.so.0.13.1` would keep shipping inside the bundle. First back the old folders up inside the
bucket (server-side, no download): `rclone copy memo-r2:memo/binaries/linux/<f>
memo-r2:memo/binaries-backup-<oldtag>/linux/<f>`. Then `rclone sync --dry-run --checksum
binaries/linux/<f> memo-r2:memo/binaries/linux/<f>` (read the delete list), then the real
`rclone sync --checksum`. This is the folder the NEXT Memo release bundles. Then publish the
update components: `scripts/publish_component.sh --dry-run --requires-app <A> <component> <version>
<os/folder>=<dir> …` and the same without `--dry-run`. `<A>` is the first app version that can verify and
run the engine — 4.7.0 for CLIProxyAPI; 4.7.1 for llama b11669 and for whisper 1.9.5 (they share
`linux/cpu`, so BOTH markers are inside the archive and both components point at it). The script refuses
a folder whose marker is not the version, packs with symlinks followed (the app refuses links), signs
with `~/.config/memo-release/update-signing.key`, verifies the signature with the key in
`internal/updater/pubkey.go`, uploads archives first and `manifest.json` last. It announces nothing yet.

**6. Beacon — last.** In `/home/bugra/Documents/version/version.json` set
`"components": {"llama": "b11669", "whisper": "1.9.5", "cliproxy": "v8.0.43"}` (list only what you
published; leave `version` alone for an engine-only release), commit, push, then
`curl -fsSL https://version-zeta.vercel.app/version.json`. Apps whose version is below `requires_app`
see reason `needs_newer_app` and are offered nothing; apps with a missing folder in the manifest see
`not_for_this_machine`. Ask the person to close the matching `upstream-notice` issues (they stay open
otherwise).

**7. The installer scripts update engines themselves.** `get-memo.sh`, `get-memo-beta.sh`,
`get-memo-server*.sh`, `get_memo_arm.sh` and `update.sh` embed the block between `# >>> memo-engine-refresh`
and `# <<< memo-engine-refresh` from `scripts/engine_refresh.sh`. After the archive is copied it reads the
same beacon, compares every Linux engine folder's marker (`LLAMA_VERSION`, `WHISPER_VERSION`, `VERSION`)
with it, downloads the published folder (SHA-256 from the manifest, no links/`..` in the archive, marker
must equal the announced version, the new binary must start) and swaps it whole, keeping the old one on any
failure. It only touches folders this machine can use (`cpu`, `cliproxy`; `amd`/`nvidia` only with that GPU;
the `-arm64` ones on arm), never a folder without a marker, and is never fatal. `MEMO_SKIP_ENGINE_REFRESH=1`
skips it. It does NOT check the Ed25519 signature (a shell script has no key) — it trusts the same HTTPS
origin as the archive it just downloaded — and ignores `requires_app` (the app that was just installed is
the newest one). **Edit `scripts/engine_refresh.sh` only**, then paste its block into all six installers;
`go test ./internal/components` (`TestInstallers_EmbedTheSameEngineRefreshBlock`) fails when one drifts.
The scripts reach users when the commit lands on `main` (CI uploads them to R2), no tag needed.

Things that went wrong before: `vendor_llama.sh` used `is_llama_file && rm` as the last command of a
`set -e` loop and silently exited when the last file was not llama's (fixed 2026-10-25); `grep -q` at the
end of a pipe under `pipefail` can report "no match" after SIGPIPE — keep listings in a variable; with the
Docker daemon off the engine is built on the host, so always check glibc symbol versions.

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
