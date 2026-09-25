# Memo Roadmap

This is a living snapshot of what's actively planned past the current
release, not a commitment to dates or a final feature list. Items move,
get reshaped, or get dropped as real usage informs them. See
[`versinNote/`](../versinNote/) for what's actually shipped in each past
release, and the repo's `BUG_REPORT.md` for open bugs/design gaps found in
live testing.

> **Updated for v4.5.0** (this doc previously described a roadmap for
> "past v4.4.0" — the desktop mascot and Code Mode's Plan/Auto/Build
> sub-modes have since shipped. The items below reflect what's actually
> next, not what was next a release ago.)

## Shipped this cycle (v4.0.0 → v4.5.0), for context

- **v4.0.0** — real time-awareness in the system prompt ("how long since
  the last message"), WhatsApp third-party conversation takeover.
- **v4.3.0** — Live Mode v2: native audio-to-audio voice (Google Live /
  OpenAI Realtime), delegate/standalone modes, barge-in, ElevenLabs +
  custom engines; Telegram added as a second messaging bridge.
- **v4.4.0** — the Self-Driving task loop: `Task.md` schema,
  planner/executor mode with plan approval, sub-agent orchestration
  (coder + parallel analyzer/reviewer/test-runner), live in-chat task
  activity, escalation/retry/provider-lock hardening, real tool-calling
  for the Claude and Gemini providers (previously entirely missing), a
  new Anthropic-compatible custom provider type, an OpenAI-compatible
  Developer Gateway sibling, and an experimental "gemini-sub" provider
  (sign in with a personal Google account, Beta).
- **v4.5.0 (this branch)** — the desktop mascot: a second, always-on-top
  window (same process) that reflects Memo's live activity across every
  channel, with two selectable skins and idle animation; Code Mode split
  into three cycled sub-modes (Plan/Auto/Build) with per-mode system
  prompts and auto-permission chaining into Build; both Developer
  Gateway endpoints (Anthropic- and OpenAI-compatible) now key-enforced
  for non-loopback callers; imported skills no longer auto-activate;
  clearer Live Mode failure messages and a fixed HuggingFace avatar
  404 spam in the Model Store.

## Near-term — open items from live testing (see `BUG_REPORT.md`)

These were found running the Self-Driving loop against real tasks, not
hypothetical:

- **BUG-PLAN9** — a ready plan can only be approved from the Tasks tab,
  not inline in the chat that launched it.
- **BUG-PLAN10** — logged when the chat model had no tool to read a
  *running* task's real status and fabricated a confident, wrong "it's
  broken" narrative instead. A `get_task_status` tool + anti-fabrication
  prompt was added afterward (`def5ac1c`) that looks like the fix, but it
  hasn't been live-verified yet — worth confirming with a real "how's the
  task going" before treating this as closed.
- **BUG-PLAN11** — a plan's step count can grow via escalation (a stuck
  step splits into sub-steps); different screens compute progress
  differently and show different item/step numbers for the same list.
- **BUG-PLAN12** — a task's live activity (step started/done, sub-agent
  turn, escalation) only shows in the Tasks tab, not as a lightweight
  stream in the chat that launched it.
- **BUG-THINK1** — Claude's extended thinking is requested (spends real
  tokens) whenever an effort level is picked, but the response's
  `"thinking"` content block is parsed nowhere in the backend — the
  frontend already has a full collapsible "thinking" UI, it's just never
  fed. Medium priority (doesn't break anything, just wastes a paid-for
  feature for users who opted into effort levels).

## Mobile

**Done (2026-09-26): there is one Flutter client.** The separate `mobile/`
project is retired; `frontend/` now has `android/` and `ios/` targets and is
what ships to a phone, so feature parity is structural rather than something
to audit. Both targets are built in CI (`build-android.yml`,
`build-ios.yml`) — Android as a debug APK, iOS unsigned, since this project
has no macOS or iOS hardware and no signing certificates.

What that leaves open:

- **On-device verification** — CI proves the targets compile and link,
  nothing more. Unproven on real hardware: first-run mic permission,
  `record`'s WAV path on Android, notification delivery and its survival
  across a reboot, `just_audio` playback plus voice-mode barge-in, the
  Android save/share dialog, the back button, keyboard insets, and the
  narrow layout as a whole. iOS needs a Mac and an iPhone, neither of which
  this setup has.
- **Store publishing** — signing keys, Play Console / App Store accounts,
  and a release pipeline that has an APK/IPA slot at all (today's has
  none). The final store bundle identifier is still an open decision.
- **Live Mode's native realtime engines on mobile** — `live_pcm_player.dart`
  streams PCM into a long-lived sink and is Linux-only (it already threw on
  macOS and Windows before mobile existed). Phones fall back to the discrete
  transcribe/synthesize loop instead.
- **Reminders for events added out of band** — an event the LLM adds while
  the calendar tab was never opened is not armed until it is. Closing that
  needs a real event stream, not a poller.

## Platform Reach

- **arm64 Docker image** — the current image is amd64-only.
- **Official CasaOS App Store listing.**
- **Real-hardware verification** — the ARM build and Docker image have
  only ever been verified in CI/sandboxes, never on an actual Raspberry
  Pi or NAS.
- **Package manager distribution** *(nice-to-have)* — Homebrew tap,
  winget/Chocolatey.

## Memo Swarm

`internal/swarm/` — distributed inference across multiple machines,
currently Beta. Maturing it needs to start from actual usage friction
(the host/join flow, room codes) rather than a guessed feature list.

## Computer Use (not yet scheduled)

The user's own framing: a Claude-Code-computer-use-like system that can
directly drive the keyboard/mouse. Deliberately last in line — the
biggest, riskiest item on any list here:

- The current agent (`internal/agent/`) is sandboxed to files/commands
  with a danger-level permission system; keyboard/mouse control is a
  completely different security surface (access to everything on screen).
- Needs a per-platform implementation (Linux X11/Wayland, Windows, macOS
  Accessibility API) — ongoing maintenance, not a one-time build.
- Needs its own, likely stricter permission model (per-action
  confirmation, a persistent "Memo is in control" indicator).
- Planned as its own release, after the rest of the 4.x line settles and
  real user feedback on the Self-Driving loop comes in.

## Backlog, not yet sequenced

- Structural cleanup: `handlers_flutter.go` and `memory/store.go` are
  both large, area-based-split candidates.
- Account-scoped data isolation for self-hosted multi-user (each data
  layer would need an `account_id` — a deep change, not started).
