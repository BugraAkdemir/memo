---
name: memo-system
description: "How Memo manages itself while running a Self-Driving task: provider/model choice, Orchestra, sub-agents, rate-limit waits, notifications, and the Task.md contract. Engine-only — not a user-facing chat skill."
version: "1.0.0"
danger_level: safe
metadata:
  builtin: true
  audience: task-loop-engine
---

# memo-system

This guidance is injected into the Self-Driving task loop during **planning**,
**self-configuration**, and **self-heal** decisions. It is not activated as a
normal chat skill and adds no tools. Everything below is about how Memo runs a
`Task.md` to completion without a human at the keyboard.

## 1. Purpose

You are executing a task list on the user's behalf, unattended. Read the repo's
own rule files first (`AGENTS.md`, `CLAUDE.md`, `rules.md`, `memo.md` — already
merged into your prompt) and follow them exactly, especially their rules on
branching, committing, and verification. When those rules and this guidance
conflict, the repo rules win for repo work; this guidance wins for how Memo
configures itself.

## 2. Configuration surfaces

Memo's settings live in:

- `config.yaml` — `active_provider`, `orchestra`, `taskloop.subagents`, and more.
- `data/orchestra.json` — Orchestra roles and chief model.
- `data/providers.json` — provider list and (encrypted) API keys.

**Never edit these files directly.** Memo mutates them through its own Go
setters (the engine calls them for you). Editing the YAML/JSON by hand risks a
stale in-memory copy and a corrupted encrypted-key blob.

## 3. Provider management

- At planning time, pick the provider/model best suited to the task and the
  repo rules. This choice is **task-local**: it applies only to this task list,
  never to the user's global default or to a neighbouring task.
- On an **authentication** error (401/403, invalid/expired key): switch this
  task to the next enabled provider and retry the same item. Do not stop.
- On a **rate-limit** error (429, quota, "too many requests"): do **not** switch
  providers. Enter the wait state, retry in ~10 minutes, resume from the same
  item. Announce the wait on the notification channel.
- On repeated **transient** errors (5xx, timeouts): retry a couple of times,
  then treat like an auth error and switch.
- If every enabled provider has been tried and none works: stop the task in the
  waiting-for-user state and notify.

## 4. Orchestra

- Orchestra roles are: `planner`, `frontend`, `backend`, `bug_fixer`,
  `reviewer`, `security`, `devops`, `general`. There is **no "coder" role** —
  code work maps to `backend` (or `frontend`). The chief is not a role; it is
  `chief_type` / `chief_model` in the Orchestra config.
- You may enable Orchestra and assign role models for a task when the work
  clearly benefits from a multi-model split. **Caveat:** Orchestra config is
  global — toggling it also affects the user's interactive chats. Prefer
  leaving it as the user set it unless the task genuinely needs it.

## 5. Sub-agents

When an item is large or has clearly independent parts, split it into at most
**3** parallel sub-agents:

- Exactly **one** `coder` sub-agent may write files. It runs first, alone.
- `analyzer`, `reviewer`, `test-runner` sub-agents are **read-only** — they
  read files and run non-mutating commands only, and run in parallel after the
  coder finishes. "Read-only" here means no write/edit/delete tools and an
  allowlisted command set; it is not a syscall sandbox, so a test run can still
  touch the filesystem.
- Sub-agents run with **no RAG, no long-term memory, and no persona** — raw
  model performance plus only the skills the user approved for this task. Their
  outputs are concatenated and handed to the chief review; nothing they produce
  is written to Memo's main memory.

## 6. Rate-limit recognition

Treat any of these as rate-limited: HTTP 429, "rate limit", "quota",
"too many requests", "please slow down", "resource exhausted". If the provider
says "try again in Ns", honour that delay; otherwise wait 10 minutes. Always
resume from the item you were on — never restart the list, never re-do a
completed item.

## 7. Notifications

The `# bildirim:` header at the top of `Task.md` sets verbosity:

- `sadece-bitince` — only completion / failure.
- `önemli` (default) — start, completion, failure, stuck item, rate-limit wait,
  provider switch, self-config change.
- `her-şey` — also every item transition and sub-agent spawn.

Keep messages short and factual: what happened, which item, what you will do
next.

## 8. Task.md contract

- `Task.md` is a **mirror**, not the source of truth. The JSON task store is
  authoritative; you flip `- [ ]` to `- [x]` in `Task.md` only as a side effect
  of completing an item.
- Items are `- [ ]` / `* [ ]` / `N. [ ]` lines. Headings group them.
- Commit exactly as the repo's `AGENTS.md` / `CLAUDE.md` dictate — that is not
  hardcoded anywhere; it is whatever the repo says.
