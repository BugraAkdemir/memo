# 🚗 Self-Driving Task Loop

> **Package:** `internal/taskloop/` (engine, schema, plan, retry, notify, sub-agent orchestration), bridged into `internal/app/` via `tasklist*.go` and `task_*.go`
> **Introduced:** v4.4.0 — sub-mode chaining with [[Agent Mode|Code Mode]] added in v4.5.0
> **Data:** `data/tasklists/` (one task list's state per entry)
> **API endpoints:** `/api/tasklists`, `/api/tasklists/{id}`, `/api/tasklists/{id}/plan`, `/api/tasklists/{id}/approve-plan`, `/api/tasks/running`, `/api/tasks/events`, `/api/tasks/{id}/{pause,resume,cancel,skip,inject}`, `/api/taskloop/settings`

An unattended, multi-step task runner built on top of [[Agent Mode]] — you hand it a checklist, and it works through it on its own, turn after turn, without you babysitting each step.

---

## `Task.md`: the schema

A plain-Markdown checklist of `- [ ]` items, with optional `# key: value` headers controlling behavior:

- **Mode** — `worker` (default) or `planlayıcı` (planner: produces a `Plan.md` first, see below)
- **Notification verbosity**
- **Per-role model pinning** — planner/coder/verifier roles can each be pinned to a specific model
- **Provider lock/roaming** — `# sağlayıcı: sabit` (default — one provider for the whole run) or `# sağlayıcı: otomatik` (roam across enabled providers)
- **Plan auto-approval** — `# onay: otomatik`

`TaskMdSchemaDoc()` (`internal/taskloop/schema.go`) is the single human/LLM-facing definition of this format — kept in sync with the real parser (`ParseTaskMd`, `taskmd.go`) via a dedicated sync test.

A `Task.md` is created/edited via the `create_task_md`/`edit_task_md` agent tools (see [[Agent Mode]]), or an existing file is started via the `start_self_driving_task` tool — also reachable from the Tasks tab by pointing it at a `Task.md` path.

---

## Planner/executor mode

For `# mod: planlayıcı`, a planning turn runs first and produces a `Plan.md` — concrete, individually-verifiable steps, acceptance checks, and a dependency DAG (`internal/taskloop/plan.go`). The user approves it either from the Tasks tab's plan-approval card, or automatically with `# onay: otomatik`.

This is a different mechanism from [[Agent Mode|Code Mode's Plan sub-mode]] — Code Mode's Plan is a single-chat, single-turn planning gear you flip on and off; the task loop's planner mode is a property of an entire multi-step `Task.md` run. They can be combined (a task list item can itself trigger a Code Mode Plan turn), but neither implies the other.

---

## Sub-agent orchestration

A large or clearly-parallel item can split into up to 3 sub-agents (`internal/taskloop/subagent.go`, `SubAgentOrchestrator.Spawn`):

- Exactly **one** write-capable `coder` sub-agent runs first.
- Then up to **3** read-only `analyzer`/`reviewer`/`test-runner` sub-agents run in true parallel.
- Their results feed a chief review that decides whether the item is actually done.

---

## Live activity

Tool calls, sub-agent turns (`[coder]`/`[analyzer]`/`[reviewer]`/`[test-runner]`), a "model is generating" indicator during long silent LLM calls, and slow-tool "starting…" lines all stream into a live in-app card as the loop runs — in the Tasks tab's task detail screen, and reflected by the [[Desktop Mascot]]'s activity signal.

---

## Resilience, not silent failure

The engine (`internal/taskloop/engine.go`) classifies a worker-turn error via `providerFailKind` (`provider_err.go`) and reacts accordingly:

- A **busy chat** queues and retries instead of killing the task instantly (`chat_locks.go`'s `withChatLockWait` in `internal/app/`).
- A **rate-limited provider** waits and resumes from the same item — it never restarts the list.
- A **transient fault** gets an escalating retry: 5 minutes, then 10, before the item is parked for the user (`RetryScheduler`, `retry.go`).
- An **auth/config fault** parks the whole list in a waiting-user state rather than looping forever.
- Every terminal state notifies — a chat message and a push notification — by design (`NotifyBus`, `notify.go`).

---

## Pause/resume from chat

The `pause_task`/`resume_task` agent tools let the model itself pause a running task and resume it from the same step — whatever the user typed while it was paused is carried forward into the next step.

---

## Provider lock and self-heal

A running task list carries its **own** `agent.Executor` + provider `Router` snapshot (`taskRunConfig` on the request context, `internal/app/tasklist_run.go`) — separate from the app's global active-provider state. `callAgentStream` uses this snapshot whenever it's present. Self-heal (`healTaskProvider`, `tasklist_selfheal.go`) and planning-time self-configuration mutate **that snapshot only**, never `a.activeProviderName` — so a task list's provider decisions never leak into or get clobbered by whatever the user is doing in a normal chat at the same time.

---

## Known open gaps

See `BUG_REPORT.md`, `BUG-PLAN9`/`10`/`11`/`12`:

- Plan approval is Tasks-tab-only for now — not inline in the chat that launched it.
- The chat model can't yet read a *running* task's live status reliably enough to avoid a confidently-wrong "it's broken" narrative if asked mid-run (a `get_task_status` tool + anti-fabrication prompt was added as a likely fix, not yet live-verified).
- A plan's step count can grow via escalation (a stuck step splits into sub-steps); different screens can compute progress differently and show different item/step numbers for the same list.
- A task's live activity only surfaces in the Tasks tab, not as a lightweight stream in the chat that launched it.

---

### Linked Notes:
- [[Agent Mode]] — The tool-calling pipeline this builds on, and Code Mode's related (but distinct) Plan sub-mode
- [[Desktop Mascot]] — Reflects task-loop activity in real time
- [[Architecture]] — System integration
- [[API Documentation]] — Task loop endpoint details
