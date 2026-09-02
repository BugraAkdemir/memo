# Self-Driving Memo v4.5.0 — Planner/Executor Mode

Status: implemented on `feat/self-driving-memo-v4.4.0` (2026-08-30). Sibling of
`2026-08-29-self-driving-memo-design.md` (the v4.4.0 task loop this extends).

## Problem

The v4.4.0 loop runs every Task.md item through one long-lived agent chat
(`buildTaskLoopRunWorker` → `SendMessageStreamTo`). That chat accumulates every
tool transcript and file dump, so on a long list the model's context fills,
quality degrades, and it hallucinates progress ("devam 38": a weak model
claimed it wrote a file it never wrote).

## Design

Split the loop across a **role boundary**, per-list opt-in via
`TaskList.Mode == "planlayıcı"` (worker mode is unchanged and remains the
default):

- **Planner** — one bounded read-only agent turn decomposes the items into a
  DAG of small, self-verifying **steps** (`Plan`). Writes `Plan.md` next to the
  Task.md; unless auto-approve, the list parks in `awaiting-plan-approval`.
- **Executor (coder)** — a *fresh ephemeral* `NewSubAgentExecutor` turn per
  step (`[minimal system + repo rules + running state doc + this one step]`),
  so its context never grows. This designs out the "compact at 85%" problem —
  there is no long-lived executor. The only long-lived artifact is a bounded
  `state.md`, compacted by one planner call when it exceeds
  `HandoffStateMaxTokens`.
- **Verifier** — deterministic acceptance checks (`command` / `grep`) run by
  the engine in the project dir; fuzzy checks go to one `callLLMForReview`
  judgement (a verifier error is logged, treated as pass — never a hard block).
- **Escalation valve** — after `MaxExecutorAttempts` a stuck step is re-planned
  by a targeted cloud call (`escalateStep`) and its replacement steps spliced
  in (`Plan.ReplaceStep`, then `Plan.Normalize`). A network error parks the
  list in `waiting-escalation` with the failure in `Plan.PendingEscalation`;
  the retry scheduler resumes it (re-arming while still offline). This is the
  "plan online, execute offline" guarantee.

**Role → model resolution** (`resolveRoleModels`): each role independently, in
order — `Task.md` header (`# planlayıcı:` / `# kodlayıcı:` / `# doğrulayıcı:`)
→ an `AGENTS.md` machine line `<!-- memo:taskloop planlayıcı=… kodlayıcı=… -->`
→ the Settings default. An unresolved role falls back to the active provider.
`persistRoleChoiceToRules` writes the AGENTS.md line so a choice isn't asked
twice. (Interactive elicitation from `start_self_driving_task` is a follow-up.)

## Key components

| Area | Where |
|------|-------|
| Plan model, `RenderPlanMd`/`ParsePlanMd(Text)`, `ReadySteps`, `ReplaceStep`, `Normalize` (cycle/dep check) | `internal/taskloop/plan.go` |
| Store: `Mode`, `SavePlan`/`GetPlan`/`SetStepStatus`/`IncrementStepAttempts`, `SaveState`/`GetState`, statuses `awaiting-plan-approval` / `waiting-escalation` | `internal/taskloop/store.go` |
| Engine: `run()` mode branch, `runPlanExec` / `ApprovePlan` / `executePlan` (parallel ready-set, cap `MaxParallelSteps`) / escalation / `ApplyConfig`+`planCfg`/`execCfg` snapshots | `internal/taskloop/engine.go` |
| Gauge fields (`Mode`, `PlanSteps*`, `StateDoc*`) | `internal/taskloop/runtime.go` |
| Planner turn / step runner + acceptance checks / escalator / role config / settings | `internal/app/tasklist_{planner,stepexec,escalate,roleconfig,settings}.go` |
| Task.md schema (single source) + `create_task_md`/`edit_task_md` tools | `internal/taskloop/schema.go`, `internal/agent/tools/taskmd_tools.go`, `internal/app/taskmd_tool.go` |
| REST: `GET/PUT /api/taskloop/settings`, `GET/PUT /api/tasklists/{id}/plan`, `POST /api/tasklists/{id}/approve-plan`, `mode` on `POST /api/tasklists` | `internal/webserver/{server,handlers_flutter,bridge}.go` |
| Flutter: Settings tab controls, create-dialog mode picker, detail-screen plan approval + gauge | `frontend/lib/widgets/settings/tabs/taskloop_tab.dart`, `frontend/lib/screens/{tasks_screen,task_detail_screen}.dart`, `frontend/lib/providers/taskloop_settings_provider.dart` |

## Config (`config.TaskLoopConfig`)

`PlannerModel` / `CoderModel` / `VerifierModel` (empty = ask), `StepGranularity`
(`hybrid`), `AutoApprovePlan` (false), `TaskMemory` (false),
`MaxExecutorAttempts` (3), `MaxParallelSteps` (3), `HandoffStateMaxTokens`
(2000). All runtime-mutable via `Engine.ApplyConfig`.

## Known v1 limitations (follow-ups)

- Verifier + state compactor use `callLLMForReview` (the active provider), not
  the resolved verifier model.
- `start_self_driving_task` does not yet run the interactive "local or cloud?"
  elicitation — it relies on Task.md headers / AGENTS.md / Settings.
- Parallel steps trust the planner's DAG to keep independent steps
  non-conflicting on the filesystem.
- `callLLMForReview` is known-broken with a local model (`model 'local-model'
  not found`, pre-existing since 2026-07-05) — fuzzy checks / compaction need
  an external provider active.
