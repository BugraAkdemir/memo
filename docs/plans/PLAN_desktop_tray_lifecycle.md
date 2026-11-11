# Desktop tray and lifecycle plan

11 November 2026. User authorized implementation after the investigation in
`docs/DESKTOP_STARTUP_LIFECYCLE_REVIEW.md`. Keep X-to-tray behavior; Quit is
separate. This session is limited to the first two items, each verified before
moving on. No UI framework rewrite or literal single-PID migration.

- [x] 1. Guard desktop hiding on successful icon/menu setup; retry failed setup,
  minimize to the taskbar with a TR/EN explanation if unavailable, serialize
  close events and test pending/failing/recovered tray initialization.
- [ ] 2. Make Windows native tray results trustworthy: initialize structures,
  check image/shell errors, restore after Explorer restart, retain main-window
  ownership when a mascot window registers plugins, and verify native code in CI.
- [ ] 3. Measure installed Windows startup stages, then normalize engine bundle
  resolution and remove the redundant first-launch copy/readiness sleeps.
- [ ] 4. Lazy-create/cache tabs and bundle fonts; verify offline startup and
  retained screen/global notification behavior.
- [ ] 5. Implement owned desktop sessions and graceful/bounded termination across
  Windows/Linux/macOS, preserving shared CLI/headless backends and updater/external
  app exceptions. Native crash/forced-kill acceptance is required per platform.

Native installed Windows reproduction remains required even if automated checks
pass. Do not equate a method-channel fixture with Explorer/taskbar acceptance.

Item 1 local verification: 7 tray widget scenarios + 3 preference tests pass;
focused analyze reports no issues; UI literal scan has no matches. Full remote
CI and native Windows verification follow item 2; this is not native acceptance.

Linux/macOS X-to-tray as the universal default is deferred until native host
availability is validated. Upstream Linux set_icon reports success without
proving a StatusNotifier host displays it. Keep existing defaults/saved choices
in this first Windows-focused change; do not create inaccessible hidden apps.
