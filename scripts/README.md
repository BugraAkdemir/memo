# scripts/

All of Memo's shell/PowerShell/batch scripts, moved out of the repo root
(2026-08-09) to keep it uncluttered. **Every script here assumes it's run
from the repo root**, e.g. `./scripts/build_releases.sh` — none of them
`cd` relative to their own file location, so running them from anywhere
else won't find `config/`, `frontend/`, `data/`, etc.

None of these are invoked by CI (GitHub Actions builds everything inline,
see `.github/workflows/`) — they're for local/manual use only.

---

## Release & packaging (maintainer use, from repo root)

| Script | What it does |
|---|---|
| `build_releases.sh` | The main Linux release builder — backend + Flutter frontend, produces `.tar.gz` / `.AppImage` / `.deb`. Run: `./scripts/build_releases.sh` |
| `build_releases.bat` | Same, for Windows (Inno Setup installer + zip). Run from `cmd`/PowerShell: `.\scripts\build_releases.bat` |
| `build_releases_arm.sh` | Same as `build_releases.sh`, targeting Linux arm64 (Raspberry Pi). Needs `binaries/linux/cpu-arm64/` populated first — run `download_binaries.sh` before it if that's empty. |
| `package_linux.sh` | Older, narrower Linux packager (backend + frontend + a generated `run_memo.sh`, no `.AppImage`/`.deb`). `build_releases.sh` superseded most of what this did — kept for a lighter-weight local test build. |
| `package_windows.sh` | Windows counterpart to `package_linux.sh` — cross-compiles the backend, no installer, just a folder. |
| `macrelease.sh` | macOS packaging — `.app` bundle, `.tar.gz`/`.dmg`. Needs Xcode + cgo, so realistically only runs on an actual Mac. |
| `download_binaries.sh` | Pulls the llama.cpp `llama-server` + `vec0` binaries (Linux CPU/NVIDIA/AMD) that get bundled into a release. Run once before the packaging scripts if `binaries/` is empty. |
| `patch.sh` | A one-off `sed` patch from a specific past refactor (three `App` method return types → `interface{}`). Not a general-purpose tool — kept for reference, not something you'd normally run today. |

## End-user installers (published to `download.bugradev.com`)

These are what `curl -fsSL https://download.bugradev.com/<name> | bash`
actually downloads and runs on someone else's machine — **not** meant to
be run from a repo checkout. They're tracked here as the source of truth,
but changing one doesn't take effect for end users until it's re-uploaded
to R2 by hand (no CI step publishes these — see `docs/SELF_HOSTED.md`).

| Script | Installs | Channel |
|---|---|---|
| `get-memo.sh` | Desktop app + backend, Linux/macOS | Stable |
| `get-memo-beta.sh` | Same, beta build (updated on every push to `main`) | Beta |
| `get-memo.ps1` | Desktop app + backend, Windows | Stable |
| `get-memo-beta.ps1` | Same, beta | Beta |
| `get_memo_arm.sh` | Desktop app + backend, Linux arm64 (Raspberry Pi) | Stable |
| `get-memo-server.sh` | **Server only** (backend + CLI + engine, no desktop app) — for self-hosting on a Pi/home server/VPS | Stable |
| `get-memo-server-beta.sh` | Same, beta — currently the only way to try self-hosting features not in a tagged release yet | Beta |
| `update.sh` | Updates an existing install in place, data/config preserved | — |
| `uninstall.sh` | Removes a desktop+backend install (offers to back up memory first) | — |
| `uninstall-arm.sh` | Same, also detects and cleans up a Docker-based arm64 install | — |

## Local dev convenience

| Script | What it does |
|---|---|
| `run_memo.sh` | Starts the backend and the Flutter frontend together for local development. **Has a hardcoded absolute path** (this developer's machine) — edit it before using on a different checkout, or just run the two commands from `AGENTS.md`'s Quick Start yourself. |
| `install.sh` | Installs a build produced by `package_linux.sh` (`build_output/memo-linux-x64/`) into `~/.local/share/MemoApp` with a desktop entry. Older/narrower than `get-memo.sh` — mainly useful for testing a local build without going through the R2-hosted installer. |
