# Memo — User Guide

Memo is a privacy-first AI assistant that runs on your own computer. It can
chat like any assistant, but it can also act: read and write files, run
commands, work through a multi-step checklist unattended, talk to you by
voice, watch your WhatsApp/Telegram, and remember what matters across
conversations — all without your data ever having to leave your machine,
unless you explicitly connect a cloud model.

This guide walks through actually using it, task by task. For the deep
technical reference (architecture, file-by-file internals), see the
`obsidian-doc-en/` vault instead — this file stays practical.

---

## 1. Getting Memo running

1. **Install and launch.** The first run walks you through a short setup
   wizard:
   - **Language & Theme** — Turkish/English, Light/Dark/System.
   - **Assistant Persona** — pick a starting style (Normal, Fun, Formal,
     Technical, Creative, Kanka/"buddy", or write your own system prompt
     from scratch). You can change this any time from Settings later.
   - **Model recommendation** — Memo looks at your hardware (RAM, GPU) and
     suggests a local chat model + a small memory/embedding model, with a
     one-click "download these" button, or you can skip straight to
     **connecting an external API provider** instead.
   - **Starting preferences** — two independent toggles: *Proactive
     Learning* (Memo may notice a habit and suggest something, entirely
     local, nothing saved while it's off) and *Minimal Mode* (strips
     personality/mood/passive-feature instructions from every prompt, for
     weaker hardware or a leaner context budget).
   - **System check** — a live checklist (backend connection, local
     models, "ready to chat") so you know exactly what's working before
     you start. It's fine if local models show a warning here — you can
     still chat immediately using an external provider, or come back to
     the Model Store later.
2. If your GPU needs a specific llama.cpp build, Memo detects it and offers
   a one-click install — or you can skip this and add a cloud provider
   instead, no local model required at all.
3. First screen after setup is a "what do you want to do" picker: **Chat**,
   **Agent**, **Orchestra**, **WhatsApp**, **Calendar** — each opens
   straight into that mode.

## 2. Basic chat

- **New chat:** the `+` button top-left of the chat list.
- **Memory:** Memo saves what you tell it and retrieves it automatically on
  later turns (a hybrid vector + keyword search over everything you've
  said) — ask about something from a few days ago and it'll actually
  remember, no manual "remind me" needed.
- **Attach a file or image:** the `+`/paperclip icon in the composer, or
  type `@` to mention a specific file by name from the current project
  folder.
- **`/insight`:** ask Memo to look back over your recent mood/memory
  history for a real pattern — it says so honestly if there isn't enough
  signal yet, rather than inventing one.
- **Quick model switch:** click the model/provider pill in the chat's own
  top bar instead of opening full Settings.
- **Incognito Mode:** a toggle for a fully ephemeral session — nothing
  gets written to memory.

## 3. Connecting a model

You have two independent paths, and you can use both at once (local for
memory embeddings, cloud for chat, for example):

### Local models (Settings → sidebar → Model Store)
- Search Hugging Face for a GGUF model right inside the app.
- Memo shows a compatibility badge based on your detected RAM/VRAM before
  you download.
- Background download manager with real progress, pause/resume.
- Hit "Start Model" once downloaded — a local `llama-server` process
  starts and Memo talks to it directly, no data leaves your machine.

### External providers (Settings → API Providers)
Click "Add Provider" and pick a type. As of this release, 12 provider
types plus 2 CLI-based ones are supported:

| Type | Notes |
|---|---|
| OpenAI | Bearer-token API key |
| Google Gemini | API-key query param; tool-calling supported |
| Anthropic Claude | `x-api-key` header; tool-calling supported |
| xAI Grok, Groq, OpenRouter, Ollama | OpenAI-compatible wire format |
| Custom | any OpenAI-compatible endpoint — your own proxy, LM Studio, vLLM |
| Custom (Anthropic-compatible) | any Anthropic Messages-API-shaped endpoint — for a proxy that doesn't speak OpenAI's format |
| OpenCode Zen / OpenCode Go / Kilo Code | live model-list gateways; free models are sorted to the top with a green checkmark |
| Claude Code CLI / Codex CLI (beta) | runs your locally installed `claude`/`codex` CLI as the chat provider instead of calling an API — per-chat, its own session, no memory/identity injected |

Every provider except the CLI ones lets you pick a model from a **live,
fetched-on-the-spot list** — nothing is a hardcoded guess. API keys are
encrypted at rest (AES-256-GCM, machine-bound key).

If a provider fails repeatedly, Memo's router auto-disables it and
falls back to the next enabled one; a background health check
re-enables it once it recovers.

## 4. Agent Mode — giving Memo hands

Turn on the Agent toggle (top bar of Chat, or the dedicated Agent tab) and
Memo gets access to a real toolset — 27 built-in tools as of this release:

- **Files:** `read_file`, `write_file`, `edit_file`, `insert_line`,
  `delete_lines`, `delete_file`, `list_directory`, `get_file_info`,
  `search_files`, `change_directory`
- **System:** `run_command` (sandboxed, timeout + a large blocklist of
  destructive patterns), `read_env`
- **Web:** `web_search`, `fetch_page` (reads a URL's full content, not
  just a search snippet)
- **Calendar & routines:** `get_calendar_events`, `create_routine`,
  `list_routines`, `cancel_routine`
- **Self:** `self_clone`, `configure_provider`, `share_file`
- **Task loop control:** `get_task_status`, `pause_task`, `resume_task`,
  `create_task_md`, `edit_task_md`, `start_self_driving_task` — see §5

WhatsApp's 4 tools (`whatsapp_send`/`search`/`latest`/`messages`) live in a
separate, WhatsApp-only tool set, not the general registry above.

**Every tool call goes through a permission check** before it runs, based
on a danger level (`safe` — auto-allowed, `medium`/`dangerous` — asks you).
You can allow once, allow for the session, or allow forever per tool —
forever-allows persist to disk and are manageable from Settings.

## 5. The Self-Driving Task Loop — hand it a checklist and walk away

This is the headline feature of this release: instead of one chat turn,
Memo can work through a whole checklist across many turns, unattended,
recovering from the kinds of errors that would previously have just
stopped it.

### The `Task.md` format

A plain checklist file. Ask Memo to write one for you in chat (it uses the
`create_task_md` tool), or write it by hand:

```markdown
# bildirim: önemli
# mod: planlayıcı
# sağlayıcı: sabit
# onay: otomatik

Build a small local blog: signup, salted-hash login, a session cookie,
and a page to write + list posts. Keep it simple, no framework needed.

- [ ] User signup with a hashed password
- [ ] Login that sets a session cookie
- [ ] A larger piece [parallel] — write + list posts
  - [ ] POST endpoint to create a post
  - [ ] GET endpoint that lists posts

---
Any free notes here are ignored by the parser.
```

Header reference (all optional, any order):

| Header | Values | Meaning |
|---|---|---|
| `# bildirim:` | `sadece-bitince` \| `önemli` (default) \| `her-şey` | how chatty notifications are |
| `# mod:` | `worker` (default) \| `planlayıcı` | worker runs each item in one turn; planlayıcı plans first, you approve, then it executes step by step |
| `# sağlayıcı:` | `sabit` (default) \| `otomatik` \| `<provider name>` | sabit locks the task to whatever provider was active when it started (waits/retries/parks on failure, never silently switches); otomatik allows switching to another enabled provider on failure; a name pins it to one specific provider |
| `# planlayıcı:`, `# kodlayıcı:`, `# doğrulayıcı:` | a model name | pin a specific model to the planner/coder/verifier role |
| `# hafıza:` | `açık` \| `kapalı` (default) | whether the task's own turns get memory/RAG context |
| `# onay:` | `otomatik` | skip the plan-approval gate (planlayıcı mode only) |

Put `[parallel]` anywhere in an item's text to let it fan out to
sub-agents automatically.

### Two ways it runs

- **Worker mode** — each checkbox item is one agent turn: do it, get
  reviewed, check it off, move on.
- **Planner/executor mode** (`# mod: planlayıcı`) — a planning turn
  produces a `Plan.md` (concrete steps, acceptance checks, dependencies)
  that you approve — either from the Tasks tab's approval card, or
  automatically with `# onay: otomatik`. Once approved, steps execute one
  at a time, each checked against its acceptance criteria before moving on.

### Sub-agents, for real parallel work

An item marked `[parallel]` (or one the planner judges large enough) can
split into up to 3 sub-agents: exactly one write-capable **coder** runs
first and alone, then up to 3 read-only **analyzer/reviewer/test-runner**
sub-agents run genuinely at the same time. Their combined output feeds a
chief review before the item is marked done. Sub-agents get no long-term
memory or persona — just the raw model and whatever skills you've enabled
for the task.

### Watching and controlling it

- **Tasks tab** shows every running/paused/done list, with a live
  activity feed: tool calls, `[coder]`/`[analyzer]`-tagged sub-agent
  turns, a "model is generating" line during long silent calls, and
  "starting…" lines for slow tools before they finish.
- **From chat:** ask "how's the task going" — the model has a
  `get_task_status` tool for exactly this (phase, step N/M, current item,
  elapsed time) and is instructed not to guess when it hasn't called it.
  Say "pause"/"devam" (resume) to control it — `pause_task`/`resume_task`
  carry forward anything you typed while paused into the next step.
- **The composer locks** while a task is running in that chat, so there's
  no ambiguity about who's driving.

### What happens when something goes wrong

This loop is built to **never fail silently**:

- A busy chat queues and retries rather than killing the task instantly.
- A rate limit parks the list, waits, and resumes from the exact item it
  was on — never restarts from the beginning.
- A transient fault (timeout, 5xx) gets an escalating wait-and-retry (5
  minutes, then 10) on the same provider before finally parking the item.
- An auth/config fault parks the whole list in a waiting-for-you state
  instead of retrying forever.
- Every terminal state — done, stuck, parked, failed — sends you a
  notification (chat message and push), not just a silent status flag.

## 6. Orchestra Mode — multiple models as a team

An alternative to Agent Mode for work that benefits from splitting across
specialized models: a chief plans and assigns sub-tasks to expert roles
(planner, frontend, backend, bug_fixer, reviewer, security, devops,
general), runs them (in parallel where there's no dependency, sequential
where `depends_on` says so), and synthesizes the results. Configure roles
and their models from Settings → Orchestra. Note it's a separate execution
path from the Self-Driving task loop above — Orchestra bypasses the
provider fallback router and creates its own provider connections
directly.

## 7. Routines — scheduled automation

Sidebar → Routines. Describe what you want in plain language — "every
morning at 8, summarize my calendar and send it to me" — and Memo figures
out the timing, content, and delivery channel from that sentence alone. A
routine can be a simple scheduled message or a full tool-using agent run.
Fires in your device's own timezone, resynced on every reconnect.

## 8. WhatsApp & Telegram

Settings → WhatsApp/Telegram to pair (QR code for WhatsApp, bot token for
Telegram). Once paired:
- **Self-chat assistant** — message yourself and get a full assistant
  back, same as the desktop chat.
- **Third-party takeover** — Memo can step into a conversation with
  someone else on your behalf, either announcing itself or impersonating
  your writing style (your call, made explicit per-conversation).
- Routines and (on WhatsApp) 4 dedicated agent tools work the same way as
  in desktop chat.

## 9. Live Mode — real-time voice

The voice icon next to the chat input opens a full-screen call: pick
Google Live or OpenAI Realtime as the engine in Settings first, paste its
API key, choose a voice. This is native audio-to-audio — the model
actually hears your tone and pauses, not a transcribe-then-read relay.

- **Delegate mode** (default): the live model's only "tool" is handing
  real work to your main chat model, then narrating the result back in
  its own voice.
- **Standalone mode**: the live model gets your full agent toolset
  directly — faster, but dangerous tool calls get a spoken permission
  prompt instead of a screen dialog.
- You can interrupt it mid-sentence (barge-in sensitivity is configurable),
  and the transcript is kept in your normal chat history afterward.

## 10. Settings you'll actually touch

Settings is a searchable rail, not a wall of tabs — type a few letters of
what you're after. Worth knowing about specifically:

- **System Prompt / Gizli Mod Prompt** — how Memo talks to you, and a
  separate prompt for Incognito sessions.
- **Bellek (Memory)** — turn memory off entirely, or manage what's stored.
- **Öğrenme (Learning)** — the proactive-nudge settings from the setup
  wizard, revisited any time.
- **Cloud Sync** — Google Drive backup of your data, end-to-end encrypted.
- **Uzaktan Erişim (Remote Access)** — expose Memo to other devices
  (Tailscale or ngrok tunnel), with token/password auth required.
- **Hafızayı İçe Aktar (Import Memory)** — paste another AI's summary of
  you and Memo breaks it into atomic facts.

## 11. Self-hosting

Memo runs standalone on a home server, NAS, or Raspberry Pi (Docker image,
CasaOS, or the plain binary). Point another device at it with a Backend
URL + Token, same flow the desktop and mobile clients both use. See
`docs/SELF_HOSTED.md` for the setup script and reverse-proxy notes.

## 12. If something's not working

- Check `docs/TROUBLESHOOTING.md` (or the Turkish `docs/tr/TROUBLESHOOTING.md`) first.
- `BUG_REPORT.md` at the repo root lists currently-open, known issues —
  worth a quick check before assuming you've found a new bug.
- For a stuck/misbehaving local backend, `memo --kill` followed by a
  relaunch clears most transient state issues.
