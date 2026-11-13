# Desktop tray and lifecycle plan

11 November 2026. User authorized implementation after the investigation in
`docs/DESKTOP_STARTUP_LIFECYCLE_REVIEW.md`. Keep X-to-tray behavior; Quit is
separate. This session is limited to the first two items, each verified before
moving on. No UI framework rewrite or literal single-PID migration.

- [x] 1. Guard desktop hiding on successful icon/menu setup; retry failed setup,
  minimize to the taskbar with a TR/EN explanation if unavailable, serialize
  close events and test pending/failing/recovered tray initialization.
- [x] 2. Make Windows native tray results trustworthy: initialize structures,
  check image/shell errors, restore after Explorer restart, retain main-window
  ownership when a mascot window registers plugins, and verify native code in CI.
- [x] 3. Measure installed Windows startup stages, then normalize engine bundle
  resolution and remove the redundant first-launch copy/readiness sleeps. *(Startup timing instrumentation merged; engine resolution normalized to override -> exeDir -> parent -> cwd with macOS arch suffix selection; Windows redundant `binaries/` copy and blind startup sleeps removed. Cold/warm installed Windows capture runs on a Windows machine via `StartupTiming`.)*
- [x] 4. Lazy-create/cache tabs and bundle fonts; verify offline startup and
  retained screen/global notification behavior. *(Bundled Inter + Schibsted Grotesk static TTFs & OFLs under `frontend/assets/fonts/` with `allowRuntimeFetching = false`; deferred secondary `AppShell` tabs via `_visitedTabs` cache; extracted `CalendarReminderSync` + `CalendarReminderBridge` so mobile OS reminders reconcile from launch without mounting `CalendarScreen`.)*
- [x] 5. Implement owned desktop sessions and graceful/bounded termination across
  Windows/Linux/macOS, preserving shared CLI/headless backends and updater/external
  app exceptions. Native crash/forced-kill acceptance is required per platform. *(Implemented `--desktop-session` + `--owned` + stdin control pipe EOF supervisor in `internal/procjob/session.go` and `main.go`, `EnsureSelf()` + `BREAKAWAY_OK` Job Object and Unix `-pgid` tree cleanup, `DetachedAttr()` on `memo-updater`/`open_app`/`browseropen`, bounded pre-engine shutdown phase `shutdownPreEngineTimeout` in `internal/app/app.go`, and updated Windows/Linux/macOS launchers removing `pkill`/`taskkill` and blind sleeps. Native crash/forced-kill acceptance still required on physical Windows/macOS devices.)*

Native installed Windows reproduction remains required even if automated checks
pass. Do not equate a method-channel fixture with Explorer/taskbar acceptance.

Item 1 final local verification: 12 pass, 1 Windows-only scenario skipped;
focused analyze reports no issues; UI literal scan has no matches. Full remote
CI and native Windows verification follow item 2; this is not native acceptance.

Linux/macOS X-to-tray as the universal default is deferred until native host
availability is validated. Upstream Linux set_icon reports success without
proving a StatusNotifier host displays it. Keep existing defaults/saved choices
in this first Windows-focused change; do not create inaccessible hidden apps.

Item 2 source is implemented in 3b3fca12 (including earlier checkpoints) and
merged to `main` in `d626728b`; Windows native CI run `43099165002` and
`Build Windows` (`43321361484`) passed. Actual installed
Windows/Explorer/overflow/mascot acceptance is still open after automated checks.
