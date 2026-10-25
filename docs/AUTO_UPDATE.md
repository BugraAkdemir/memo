# In-app updates

Memo can update itself from inside the app. When it starts it asks what is new; if
there is an update a window opens with the versions, the download size and a short
note, and one click ("Update now") does the rest. Engine files (llama.cpp,
whisper.cpp, the Subscriptions connector) can be refreshed on their own, without a
new Memo release.

This page is for people who ship Memo. Design record: `PLAN_auto_update.md`.

## What the person sees

| Situation | Window |
|---|---|
| A newer Memo exists and this install can update itself | versions, size, note → **Update now** / Later / Skip this version |
| …and the release is marked required | same, **without** "Skip this version" ("Later" stays) |
| A newer Memo exists but this install cannot update itself (AppImage, deb, Docker, source checkout, a platform with no published build) | note + **Open download page** |
| Only newer engine files exist | list of components (`b9441 → b11456`), applied one at a time |
| The previous attempt failed and was rolled back | a line above the buttons saying so; another try is offered |

"Skip this version" is remembered per device; a *newer* release, or a required one,
asks again. "Later" asks again next launch.

## How an update runs

**Linux / macOS (script installs, `~/.memo`).** The backend downloads the release,
checks its SHA-256 against a signed manifest, unpacks it into `<install>/.update/`,
starts the `memo-updater` found *inside the new release* and exits. The updater waits
for the old process to be gone, renames the old app files into a backup, renames the
new ones in, relaunches Memo through its launcher script and waits for the backend to
answer with the expected version. If it does not (or the launcher dies), the new
files are removed, the old ones restored and the old Memo relaunched; the reason is
left in `update-failure.txt` and shown at the next start. `config/`, `.env` and
everything under `data/` (models, memory, sessions, providers) are never touched —
the list of replaced items is `updater.SwapItems`, which mirrors `get-memo.sh`.
Progress is logged to `<install>/update.log`.

**Windows.** The backend downloads `memo.exe` (the Inno Setup program), verifies it,
starts it through the shell (so the normal UAC prompt appears) and exits. The person
finishes in the installer; it upgrades in place (stable AppId, data in ProgramData).

**Engine components.** New files are downloaded into `<data>/components/binaries/<os>/<flavor>/`
— never into the install (on Windows that is Program Files and needs an administrator)
— and searched *before* the bundled copy. The engine using the folder is stopped,
swapped, started again and the new binary is run (`--version` must report the
manifest's build). On failure the folder is removed again, so the bundled engine is
used. Which flavor is used is still decided by the bundled tree and the engine mode:
a downloaded cpu engine can never move a machine that runs the amd engine to cpu.
After an app update that bundles the same or a newer engine, the downloaded folder is
deleted at startup.

## What can go in `version.json` (version-zeta.vercel.app)

```json
{
  "version": "V4.7.0",
  "mini_version_note_tr": "kısa not",
  "mini_version_note_en": "short note",
  "required_update": false,
  "components": { "llama": "b11456", "whisper": "1.9.0", "cliproxy": "v8.0.26" }
}
```

Releases before the updater read only `version`; the rest is additive. The beacon is
**not signed** and is only a trigger: it names which version to ask for. URLs, sizes
and SHA-256s come from signed manifests, so a taken-over beacon can withhold or
re-announce an update but cannot make Memo run a file you did not sign. `components`
values are validated (they end up in URLs); `required_update` only hides the skip
button; notes are shown as plain text, cut to 600 characters.

A component-only release does **not** change `version`, so older apps show nothing.

## Trust model

* An ed25519 public key is compiled into the app (`internal/updater/pubkey.go`). The
  private key exists only as the `MEMO_UPDATE_SIGNING_KEY` Actions secret and in
  `~/.config/memo-release/update-signing.key` on the maintainer's machine. A build with
  no key fails closed.
* The signature covers the exact manifest bytes plus a fixed context string, so it
  cannot be replayed as something else. A manifest must also be *for what was asked*
  (version, platform, component): a genuine old manifest served at a new address is
  refused. Versions are ordered as semver; a downgrade is never an update.
* Downloads must be https (loopback http only for tests), are byte-exact (compression
  disabled), size- and SHA-256-checked, and resumable. Unpacking refuses absolute
  paths, `..`, hard links, devices and links that resolve outside the target — it
  resolves *real* paths before each write, because a lexical check is beaten by
  `x/a -> ..` followed by `x/a/l -> ../../y`.
* Rotating the key means shipping a release that carries the new public key signed
  with the old one. There is no remote way to change it.

## Releasing

CI does the heavy lifting on a real release tag (`vX.Y.Z` matching the `version`
file):

1. Linux stamps `LLAMA_VERSION` / `WHISPER_VERSION` into the engine folders from
   `internal/components/ENGINES.txt` and checks them; `memo-updater` is built into the
   Linux and macOS releases.
2. After each platform's artifact is uploaded to R2, `scripts/publish_update_manifest.sh`
   signs and uploads `update/app/<X.Y.Z>/<platform>.json(.sig)`. A checkpoint tag that
   does not match `version` publishes nothing. The signature is checked against the
   compiled-in key before upload, so a mismatched secret fails the build.
3. Run `scripts/verify_update_manifest.sh` — it checks every platform's manifest,
   signature and the size of the file at its URL. **Only then** bump `version.json`:
   the beacon is what sends apps looking.

Engine-only release (no new Memo):

```bash
scripts/stamp_engine_versions.sh stamp --llama b11456 <dir>     # markers inside the folder
scripts/publish_component.sh --dry-run --requires-app 4.7.0 llama b11456 linux/cpu=<dir> …
scripts/publish_component.sh           --requires-app 4.7.0 llama b11456 linux/cpu=<dir> …
# then set "components": {"llama": "b11456"} in version.json
```

A folder is replaced **as a whole**, so publish it complete: `linux/cpu` holds llama.cpp
*and* whisper.cpp (and `memo-lora-train`) linked to the same libraries — see
`AGENTS.md` "llama.cpp upgrade traps". Folders you leave out are not updated (this is
how NVIDIA stays on an older llama.cpp). `--requires-app` keeps an old app from being
handed an engine it cannot drive (for example a changed flag).

## Limits and honest status

* **Not run on real Windows or macOS.** Windows (Setup.exe via `ShellExecute`) and
  macOS are compiled in CI only. macOS also ships no `run_memo.sh` next to the backend
  in script installs, so most macOS installs take the manual path.
* **Installer scripts** (`get-memo*.sh`, `get_memo_arm.sh`, `update.sh`) refresh older engine folders while they install: after copying `binaries/` they read the same beacon, compare each Linux engine folder's marker, download the published folder (SHA-256 from the manifest, no `..`/links, marker must match, the new binary must start) and swap it whole, keeping the old one on any failure. Only folders this machine can use (`cpu`, `cliproxy`; `amd`/`nvidia` with that GPU), never a folder without a marker, never fatal; `MEMO_SKIP_ENGINE_REFRESH=1` skips it. Source of truth: `scripts/engine_refresh.sh` (embedded verbatim in the installers, kept identical by `internal/components/installer_refresh_test.go`). It does not check the Ed25519 signature and ignores `requires_app`.
* Windows/macOS engine folders carry **no version marker** (their llama.cpp build was
  never recorded), so they get no engine updates until someone confirms the tag and
  adds it to `ENGINES.txt`.
* The first update *to* a version that contains the updater is manual (4.6.0 and
  earlier have none).
* The full archive is ~780 MB every time (engines included).
* Real GPU/model restarts after an engine update were exercised with fake engines
  only.

## Troubleshooting

* `<install>/update.log` — what the updater did. `<install>/update-failure.txt` — the
  one-line reason the last attempt was rolled back.
* `.update/` is left behind only when even the rollback failed; it may hold the only
  intact copy of the previous version — Memo refuses to start a new update until you
  have looked at it.
* Test seams (like `MEMO_TELEGRAM_API_BASE`): `MEMO_UPDATE_BEACON_URL`,
  `MEMO_UPDATE_BASE_URL`, and — only together with a loopback base URL —
  `MEMO_UPDATE_PUBKEY`.

## Endpoints

`GET /api/update/check[?force=1]` and `GET /api/update/status` are ambient;
`POST /api/update/apply {"kind":"app"}` / `{"kind":"component","component":"llama"}`
is admin-only and answers 202 at once.
