# Desktop startup and process lifetime — 11 November 2026

Status: investigation and proposed direction; no production behavior changed. The reported Windows 60-second first launch / 40-second later launch has not been profiled on Windows. Code findings below do not establish how much of that time each component consumes.

## Startup findings

| Finding | Evidence | Proposed correction |
| --- | --- | --- |
| Installer already places packaged engines in the app directory, then first launch copies them again to the user's `.memo/binaries`. | `installer.iss:50` recursively installs the staged tree; `.github/workflows/build-windows.yml:230` copies the installed binaries before starting the frontend. The home-copy launcher exists in history at `5e4f97cc`. | Use the installed bundle as the immutable baseline and data-directory component updates as overrides. Remove the redundant full-tree copy after testing resolvers, portable mode, updates and architecture aliases. Moving the same duplicate copy into setup would retain duplication. The original author's rationale is not documented by this investigation. |
| Legacy home directory and current Windows data layout differ. | Launcher changes cwd to `.memo`; `internal/config/config.go:45` selects ProgramData on Windows unless explicitly overridden. `internal/llama/llama.go:752` and `internal/whisper/whisper.go:580` search cwd before executable roots, after update overrides; `internal/cliproxy/locate.go:52` searches executable roots before cwd. | Specify one installed/portable layout and consistent engine resolution; avoid an old home copy shadowing an updated installed engine. Do not delete user files blindly. |
| Hidden tabs are mounted from startup. | `frontend/lib/screens/app_shell.dart:413` builds an IndexedStack of screens; comments document persistent mounting/state and stable indices. Calendar starts loading/subscribes after its first frame, Routines loads from initState, Developer watches gateway providers from build. | Create a tab on its first visit, retain its state thereafter. Gate screen-specific polling; keep app-wide chat/permission/notification behavior available. |
| Fonts can use network at runtime. | `frontend/lib/core/theme.dart:222` uses GoogleFonts Inter/Schibsted Grotesk; `frontend/pubspec.yaml` declares no font files. Locked google_fonts 6.3.3 defaults to runtime fetching and checks assets/cache before HTTP. It returns fallback styling while loading asynchronously. | Bundle the used styles/weights and licenses, disable runtime fetching, validate offline startup and typography. HTTP blocking the first window was NOT demonstrated. |

The empty initial model-search query returns an empty result in `frontend/lib/providers/models_provider.dart:166`; do not describe startup as necessarily fetching HF model details. A model detail is not selected initially.

Windows launcher also sleeps one second before backend start and two after it on its start-backend path. These fixed waits and the first-copy path precede frontend launch, so cannot alone explain the reported later frontend delay. `frontend/lib/main.dart:52` awaits preferences and notification initialization before runApp. `frontend/windows/runner/flutter_window.cpp:27` registers plugins and shows the window on the next Flutter frame. Native/plugin/preferences/first-frame durations remain unmeasured.

## Why processes can remain

1. **Closing and hiding differ.** `frontend/lib/providers/settings_provider.dart:1138` defaults Windows to minimize-to-tray unless a saved choice overrides it. `frontend/lib/core/tray_controller.dart:120` hides on X in that mode. This intentionally retains backend/engines. Tray Quit unregisters the GUI and destroys its window.
2. **The GUI and backend have separate lifetimes.** Windows PowerShell starts them separately, waits for the frontend, then attempts shutdown only for the backend it started. The backend supports other clients, including CLI. `internal/app/clients.go` uses 30-second GUI heartbeats, a 90-second stale limit and 15-second sweeps; a missed unregister is therefore not immediate shutdown, and auto-shutdown requires its activation and no remaining client. `main.go:439` also supports a detached CLI backend.
3. **Windows containment already exists, but at the backend boundary.** `internal/procjob/procjob_windows.go` uses KILL_ON_JOB_CLOSE for engines, with best-effort assignment after Start. If assignment succeeds, backend exit terminates those job members. UI exit alone does not close the job. Assignment failure is logged and currently allows the process to continue. Updaters and intentionally opened external applications are excluded by design.
4. **Linux/macOS launchers can skip normal cleanup on UI failure.** `.github/workflows/build-linux.yml:173` and its arm64 counterpart, plus `.github/workflows/build-macos.yml:142`, use `set -e` and place shutdown after the foreground GUI without an exit cleanup trap. A nonzero GUI exit can skip shutdown. Their backend invocations do not enable auto-shutdown. Process groups used by engine Stop paths are not an automatic parent-death mechanism; `internal/procjob/procjob_other.go` is a no-op.
5. **Cleanup ordering has a fallback risk.** `internal/app/app.go:1006` allows 15 seconds for shutdown before forced exit. Earlier task/background/storage cleanup precedes parallel engine stops. If earlier cleanup hangs, forced exit can occur before engine Stop. This is a possible source path, not a reproduced user incident; Windows job containment helps only for successfully bound children.

Current launchers also contain process-name-based kill paths. A replacement must track owned handles/session identities instead of killing every matching executable and potentially affecting another Memo client/service.

## Recommended application model

One package, one branded entry point and one desktop-session owner; keep engines as managed child processes. Existing Go and Flutter can stay. Separate PIDs are compatible with one application. A job object controls lifetime, not the exact grouping shown by Task Manager. Process-detail views may continue to list helpers separately.

Proposed ownership tree (not the current implementation):

```text
Memo desktop-session owner
  UI
  owned backend
    on-demand llama / whisper / subscriptions helpers
```

An explicitly attached shared/headless backend needs a different contract: closing one GUI must not silently terminate another client's service. Tray hide remains a distinct operation from Quit. Updater handoff and user-opened external applications remain explicit exceptions.

| Platform | Proposed lifetime boundary | Important limit |
| --- | --- | --- |
| Windows | Extend existing Job Object/session ownership, monitor UI exit, graceful shutdown followed by bounded termination of owned children. Resolve start-to-assignment races and define assignment-failure behavior. | An existing engine-only job does not bind GUI and backend lifetimes. Visual grouping is separate. |
| Linux | Owned process groups plus a supervisor/liveness channel; consider a systemd user scope/cgroup where available. Define cleanup when owner or UI dies, including forced termination. | A shell trap or process group alone does not cover SIGKILL. Existing llama code avoids Pdeathsig because Go parent-thread lifetime caused premature engine death; do not blindly restore it. |
| macOS | Place helpers in the app bundle and supervise the owned process tree using process-exit monitoring/liveness. Preserve signing/update paths. | A `.app` bundle alone does not terminate its helpers; native crash/Force Quit behavior needs testing. |

A literal single PID is a different, much larger integration: host Flutter and backend together and replace llama/whisper/sidecar executable boundaries with in-process interfaces where feasible. It changes ABI, update and crash-isolation boundaries. Merely embedding Flutter does not merge the other executables. It is not required to solve the reported lifecycle problem.

## Suggested implementation order and acceptance

1. Instrument launcher entry, frontend spawn/native entry, plugin registration, Dart entry, preferences completion, first build/raster and usable UI. Compare cold/warm/offline installed Windows runs. Establish the actual 40–60-second critical path.
2. Normalize bundled-engine layout and remove redundant copy; test fresh install, upgrade, portable mode, existing `.memo` files and component overrides. Replace fixed sleeps with readiness observation where appropriate.
3. Lazy-create/cache screens; bundle fonts. Check first frame offline, navigation state, auth recovery, global notifications and absence of hidden-tab-only startup calls.
4. Implement desktop session ownership in small platform checkpoints. Native tests must cover X with both preferences, Quit, UI crash/forced kill, backend crash/forced kill, launcher death, active chat/STT/login, shared CLI client, updater and external-app exclusions. Assert no owned orphan and no unrelated process termination; keep storage flush bounded.

These are proposed steps, not completed checklist items. Start with at most one or two scoped items per implementation session.

## Verification and evidence limits

Graph-first Verify discovery with exact source fallback; project ready, generation recorded during investigation as `2026-11-11T20:17:13Z`. Cited candidate production paths were coverage-checked; no recorded gaps for the checked paths except untracked `installer.iss`, which was read directly. A clean coverage result is not proof of completeness. Graph caller edges were not treated as exhaustive where source contradicted them.

Bounded local checks, each once:

```text
GOMAXPROCS=2 CGO_ENABLED=1 go test -p 1 -tags sqlite_fts5 ./internal/app -race -run '^(TestClientRegistry_|TestShutdown_)' -count=1
ok  memo/internal/app  1.500s

flutter test --no-pub --concurrency=1 test/providers/minimize_to_tray_default_test.dart
+3: All tests passed!
```

These validate existing registry/shutdown and preference behavior; they do not validate real native Windows/macOS process exit. No full suite rerun for this documentation-only investigation. Existing unrelated full-CI failures remain documented in `docs/MMPROJ_WINDOWS_STARTUP_REVIEW.md`.

Official references:

- [Windows Job Objects](https://learn.microsoft.com/en-us/windows/win32/procthread/job-objects): kill-on-job-close and ownership constraints.
- [systemd.kill](https://www.freedesktop.org/software/systemd/man/latest/systemd.kill.html): control-group termination where systemd is available.
- [Apple bundle layout](https://developer.apple.com/documentation/bundleresources/placing-content-in-a-bundle) and [process-exit notification](https://developer.apple.com/documentation/dispatch/dispatch_proc_exit).
- [Linux PR_SET_PDEATHSIG](https://www.man7.org/linux/man-pages/man2/PR_SET_PDEATHSIG.2const.html): parent-thread semantics and limits.
- [Google Fonts package](https://pub.dev/packages/google_fonts): asset bundling and runtime fetching.
- [Flutter custom embedders](https://github.com/flutter/flutter/blob/master/docs/engine/Custom-Flutter-Engine-Embedders.md): low-level hosting interface, not automatic integration of external services.

## User clarification: Windows X leaves no visible tray icon

The user reports that X removes the window, backend/helpers remain, and no Memo tray application is visible. This is not accepted as successful minimize-to-tray behavior. Whether the frontend process remains, whether Explorer puts the icon in overflow, the installed build version and native tray initialization results have not been established.

Source inspection of `frontend/lib/core/tray_controller.dart` finds that `_init` enables close interception before awaiting `trayManager.setIcon`; there is no local initialization error handler. `_trayReady` gates menu rebuilding but is not checked by `onWindowClose` before hiding. Therefore failed/pending tray setup can leave a hide path active without established tray readiness. This is a concrete code weakness, not proof that it caused this device's missing icon. The current source already selects `memo.ico` on Windows; do not assume the older PNG issue explains the user's installed build.

Acceptance requirement: X must only hide when a usable tray interaction is established; initialization failure needs a recoverable fallback that keeps Memo reachable. Successful plugin initialization alone may not prove Explorer still exposes the icon, so test actual installed Windows behavior and Explorer restart as well as initialization errors. Full Quit must terminate owned helpers separately from hide. No production fix in this investigation.
