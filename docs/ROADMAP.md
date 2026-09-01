# Memo Roadmap

This is a living snapshot of what's actively planned past the current
release, not a commitment to dates or a final feature list. Items move,
get reshaped, or get dropped as real usage informs them. See
[`versinNote/`](../versinNote/) for what's actually shipped in each past
release, and the repo's `BUG_REPORT.md` for open bugs/design gaps found in
live testing.

> **Updated for v4.4.0** (this doc previously described a roadmap for
> "past v3.3.4" — most of that has since shipped: real time-awareness and
> WhatsApp third-party takeover in v4.0.0, Live Mode v2 in v4.3.0. The
> items below reflect what's actually next, not what was next a year ago.)

## Shipped this cycle (v4.0.0 → v4.4.0), for context

- **v4.0.0** — real time-awareness in the system prompt ("how long since
  the last message"), WhatsApp third-party conversation takeover.
- **v4.3.0** — Live Mode v2: native audio-to-audio voice (Google Live /
  OpenAI Realtime), delegate/standalone modes, barge-in, ElevenLabs +
  custom engines.
- **v4.4.0 (this branch)** — the Self-Driving task loop: `Task.md`
  schema, planner/executor mode with plan approval, sub-agent
  orchestration (coder + parallel analyzer/reviewer/test-runner), live
  in-chat task activity, escalation/retry/provider-lock hardening, and
  real tool-calling for the Claude and Gemini providers (previously
  entirely missing) plus a new Anthropic-compatible custom provider type.

## Near-term — open items from live testing (see `BUG_REPORT.md`)

These were found running the Self-Driving loop against real tasks, not
hypothetical:

- **BUG-PLAN9** — a ready plan can only be approved from the Tasks tab,
  not inline in the chat that launched it.
- **BUG-PLAN10** — the chat model has no tool to read a *running* task's
  real status, so if asked "how's the task going" while a tool call fails,
  it can fabricate a confident, wrong "it's broken" narrative instead of
  saying "I can't see that, check the Tasks tab."
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

`mobile/` (a separate, smaller Flutter project — 26 dart files vs.
`frontend/`'s 190) is still actively developed as of this pass (see recent
`fix(mobile)`/`feat(mobile)` commits) rather than merged into `frontend/`
— an earlier internal plan to retire it in favor of adding Android/iOS
targets to `frontend/` has not (yet) happened; don't assume it has without
checking `git log -- mobile/` first.

- **iOS build verification in CI** — `mobile/ios/` has a full Xcode
  project scaffold, but nothing currently builds it in CI.
- **Remote backend connection** — bringing the same "Backend URL + Token"
  flow the desktop client has to `mobile/`.
- **Feature parity audit against desktop** — agent mode, the memory view,
  and other desktop-only surfaces need an explicit pass to decide what's
  actually missing on mobile vs. intentionally left out.

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
