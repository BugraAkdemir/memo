# 🧩 Chat Sub-agents

> **Packages:** `internal/subagents/` (manager, isolated git copies, status), `internal/agent/tools/subagents.go`, bridged into `internal/app/`
> **API:** `/api/subagents`, `/api/subagents/settings`, `/api/subagents/{id}` and `/api/subagents/{id}/{send|cancel|close|merge|resume}`
> Full page: `docs/CHAT_SUBAGENTS.md`. Not the same thing as the sub-agents inside the [[Self-Driving Task Loop]].

In an ordinary [[Agent Mode]] chat Memo can hand a bounded job to a **sub-agent**, when it decides delegating helps or when you ask. Sub-agents work in the background while you keep talking to the main agent.

- **Focused:** a sub-agent gets its task, the coding rules and only the skills picked for it — never your personal memory, persona or the rest of the conversation. A read-only one can look at files and search the web; a coding one can also edit files and run commands, and your permission rules still apply.
- **Parallel writers don't collide:** each works in its own separate copy of the project; results are checked and merged one at a time, and a conflict is kept for you instead of discarded.
- **You can watch and steer:** cards in the chat show the task, live output and tool calls, and accept a follow-up. Finished ones move to a history list.
- **Results always arrive:** messages, attachments and sub-agent results go through one durable queue per chat; a result counts as delivered only once the main agent has replied to it. An interrupted review is picked up again.
- **Same model as the turn that started them:** provider, model and effort are inherited; if the provider was removed or a local model stopped, the sub-agent fails clearly instead of switching quietly.
- **Concurrency:** Settings → Chat sub-agents (1–16, default 3), separate from Self-Driving.

Related: [[Project Folders]], [[Agent Mode]].
