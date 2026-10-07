# Features Catalog

Complete feature-by-feature listing of Memo. Full detail: `docs/FEATURES.md`.

---

## 🧠 Intelligence & Memory

| Feature | Status | Description |
|---------|--------|-------------|
| Persistent RAG | ✅ | Automatic vectorization of every interaction (hybrid vector + FTS5 keyword search) |
| Contextual Recall | ✅ | Top-K similarity search before each response |
| Infinite Context | ✅ | Long-term memory independent of model window limits |
| Cross-Mode | ✅ | External provider chat + local embedding simultaneously |
| Incognito Mode | ✅ | Ephemeral sessions, zero persistence |
| Import Memory From Another AI | ✅ (v3.3.3) | Settings → Import Memory: paste another AI's summary of you, Memo breaks it into atomic facts + a communication-style summary |
| Memory context token budget capped | ✅ (v3.3.4) | Prompt-injected memory block capped at 4096 tokens; fixed a 4-5x local-generation slowdown when memory was on |
| Self-Insight (`/insight`) | ✅ (v3.3.3) | Ask directly or via a weekly Routine — looks back over mood/memory for real patterns, says so if there isn't enough to go on |

## ⏰ Routines & Proactive Learning

| Feature | Status | Description |
|---------|--------|-------------|
| Routines (scheduled automations) | ✅ (v3.3.3) | Sidebar → Routines: plain-language scheduling, simple prompt or full agent run, desktop + mobile (mobile fires as real local notifications) |
| Per-device timezone + auto-resync | ✅ (v3.3.3) | Routines fire in your device's own timezone, resynced on every (re)connect |
| Proactive ambient nudges | ✅ (v3.3.3) | On by default (subtle); habit detection, suggestion banner (Yes / Not now / Stop asking); off under Incognito |
| Minimal Mode granular toggles | ✅ (v3.3.3) | Persona, capability disclosures, passive-feature disclosures, and proactive learning can each be re-enabled independently while Minimal Mode is otherwise on |
| See [[Proactive Learning and Calendar]] | | |

## 🏭 Model Management (The Factory)

| Feature | Status | Description |
|---------|--------|-------------|
| Local llama-server | ✅ | High-performance GGUF inference |
| Dedicated Embedding Server | ✅ | Separate server, no chat performance impact |
| HuggingFace Search | ✅ | In-app model browser |
| Background Download Manager | ✅ | Real-time progress, start/stop/update |
| System Diagnostics | ✅ | NVIDIA/AMD VRAM detection, compatibility badges |

## 🔌 External Providers

| Provider | Status | Auth |
|----------|--------|------|
| OpenAI | ✅ | API key |
| Google Gemini | ✅ | API key |
| xAI Grok | ✅ | API key |
| Anthropic Claude | ✅ | API key |
| OpenRouter | ✅ | API key |
| Groq | ✅ | API key |
| Ollama | ✅ | URL |
| Custom (OpenAI-compatible) | ✅ | Base URL |
| Custom (Anthropic-compatible) | ✅ (new this branch) | Base URL — for a proxy that speaks Anthropic's Messages API shape instead of OpenAI's |
| OpenCode Zen | ✅ (v3.3.3) | API key — pay-as-you-go, some models free; free-sorted model browser (v3.9.0) |
| OpenCode Go | ✅ (v3.3.3) | API key — subscription-based |
| Kilo Code | ✅ (v3.9.0) | API key — app.kilo.ai, pay-as-you-go, some models free, live model browser with free models sorted to the top |
| Cline | ✅ (v4.6.0) | API key — api.cline.bot, OpenAI-compatible, per-model free/paid catalog; Add Provider groups providers with a free tier under a green badge and offers a **Pick a free model** button (OpenRouter, Kilo Code, OpenCode Zen, Cline) |
| Claude Code (CLI) | ✅ Beta (v3.3.4) | Shells out to the locally installed `claude` CLI, per-chat, real background job |
| Codex (CLI) | ✅ Beta (v3.3.4) | Shells out to the locally installed `codex` CLI, per-chat, real background job |
| Subscriptions | ✅ (v4.6.0) | Sign in once with an Antigravity, Claude or Codex account (via the CLIProxyAPI helper bundled in Memo); every model that account offers shows in the model selector, `/model` and the local `/v1` gateway — no separate API key. Replaced the Beta gemini-sub / claude-sub providers |

Router features: fallback chain, auto-disable after 3 failures, health check goroutine. **Claude and Gemini tool-calling** (previously entirely missing on both) fixed this branch — see [[External Providers]] for detail.

## 🧑‍💻 Developer Tools

| Feature | Status | Description |
|---------|--------|-------------|
| Usage Stats | ✅ | Settings → Stats: token/speed/model breakdown, 30-day chart (fl_chart) |
| Developer API Gateway | ✅ | Its own screen in the sidebar (not inside Settings): Anthropic-compatible (`ANTHROPIC_BASE_URL`, for Claude Code) **and** OpenAI-compatible (`/v1/models`, `/v1/chat/completions`) endpoints, live request/response log — see [[Developer API Gateway]] |
| Both gateway endpoints require their API key for non-loopback callers | ✅ (v4.5.0 security fix) | Previously the OpenAI-compatible pair could be reached with no credential at all when remote access was on and the key requirement left off |

## 🐝 Memo Swarm (Beta)

| Feature | Status | Description |
|---------|--------|-------------|
| Host / Join room | ✅ Beta | Sidebar → Swarm; multi-PC via room code |
| rpc-server (Join) | ✅ Beta | Joiners do not need the model file |
| llama-server `--rpc` (Host) | ✅ Beta | Model on host; layers split across machines (llama.cpp RPC) |
| Beta master switch | ✅ | Settings → **Beta Features** (moved out of Remote Access) |
| macOS | ❌ | UI hidden; RPC binary not packaged |

Plain-language guide: [[Memo Swarm]].

## 🧰 Agent Mode (Tool Calling)

| Feature | Status |
|---------|--------|
| 40 built-in tools in the main registry (counted from `NewRegistry()`, up from an earlier "27"): file/edit/command/search/calendar/routines/web-search/fetch-page/provider-config/self-clone/task-loop-control, `save_code_plan`, `open_app`, the seven `browser_*` tools and WhatsApp's four | ✅ |
| `open_app` — "open Spotify / the browser" launches a desktop app on Windows/macOS/Linux | ✅ (v4.6.0) — Medium danger level; Linux names resolved through Desktop Entries |
| Live interactive browser panel — Memo (or you, by hand) drives an isolated Chromium tab next to the chat | ✅ (v4.6.0) — `browser_*` tools, `browser_frame` SSE chunks, screenshots never enter chat history, only `http(s)` accepted |
| Skills active per chat | ✅ (v4.6.0) — a new chat starts with none; Settings › Skills only lists/installs/removes |
| WhatsApp's 4 tools | ✅ — in the main registry and also in a separate scoped registry |
| `create_routine`/`list_routines`/`cancel_routine` | ✅ (v3.9.0) — usable from normal chat or the WhatsApp/Telegram self-chat assistant |
| Skill tools actually executable | ✅ (v3.3.3) — a skill's `SKILL.md` `command:` field now runs through the same tool pipeline and permission UI |
| 3-tier danger level | ✅ |
| 6 permission policies | ✅ |
| Execution sandbox | ✅ |
| Rate limiting (30 calls/min) | ✅ |
| Command blacklist (hardened) | ✅ — plus a symlink sandbox-escape fix (v3.3.3) |
| Audit trail (1000 entries) | ✅ |
| Agent frontend UI (permission dialog, toggle in Chat's top bar) | ✅ |

See [[Agent Mode]] for the full tool list.

## 🛠️ Code Mode: Plan / Auto / Build (v4.5.0)

| Feature | Status |
|---------|--------|
| Three sub-modes cycled with Ctrl+Tab (or the engine-strip chip) | ✅ |
| Plan — investigates, writes a saved plan, touches no files | ✅ |
| Auto — today's familiar confirm-before-edit flow | ✅ |
| Build — edits + `run_command` run without waiting | ✅ |
| Per-sub-mode editable system prompt (Settings) | ✅ |
| Auto-permission chains a finished plan straight into Build, same reply | ✅ |
| Local model + Plan mode + auto-permission chaining | ⚠️ — code-reviewed correct fallback behavior, not yet live-verified against a real local model |

See [[Agent Mode]] §Code Mode for detail.

## 🐾 Desktop Mascot (v4.5.0)

| Feature | Status |
|---------|--------|
| Always-on-top second window, same process | ✅ |
| Reflects live activity across chat/WhatsApp/Telegram/task-loop | ✅ |
| Plain-language status bubble | ✅ |
| Idle animation (blink/breathe/wave) | ✅ |
| Two selectable skins with live preview | ✅ |
| Speaks along in Live Mode | ✅ |
| Genuinely always-on-top on Wayland (forced XWayland) | ✅ |

See [[Desktop Mascot]] for detail.

## 🚗 Self-Driving Task Loop (v4.4.0)

| Feature | Status |
|---------|--------|
| `Task.md` checkbox-item schema + `# key: value` headers (mode, notify, provider lock, memory, plan auto-approve) | ✅ |
| `create_task_md`/`edit_task_md`/`start_self_driving_task`/`get_task_status`/`pause_task`/`resume_task` tools | ✅ |
| Planner/executor mode with `Plan.md` + plan-approval gate | ✅ |
| Sub-agent orchestration (1 write-capable `coder` then up to 3 parallel read-only `analyzer`/`reviewer`/`test-runner`) | ✅ |
| Live in-chat/in-card activity (tool calls, sub-agent turns, "model is generating") | ✅ |
| Busy-chat queueing, rate-limit wait-and-resume, transient-fault escalating retry, auth-fault park-for-user | ✅ |
| Never fails silently (chat message + push on every terminal state) | ✅ |
| Plan approval reachable from chat (not just the Tasks tab) | ❌ — `BUG-PLAN9`, open |
| Chat model can read a *running* task's live status without guessing | ⚠️ — `get_task_status` tool exists and looks like the fix for `BUG-PLAN10`, but not live-verified yet |
| Consistent step/item counters across screens after an escalation split | ❌ — `BUG-PLAN11`, open |

See `docs/FEATURES.md` §6.5 and the repo's `BUG_REPORT.md` for detail.

## 🎵 Orchestra Mode (Multi-Model)

| Feature | Status |
|---------|--------|
| 8 expert roles | ✅ |
| Plan → Execute → Synthesize | ✅ |
| Parallel task execution | ✅ |
| Sequential with `depends_on` | ✅ |
| SSE progress streaming | ✅ |
| Exponential backoff retry | ✅ |

## 💬 WhatsApp Integration

| Feature | Status |
|---------|--------|
| QR pairing | ✅ |
| Bidirectional messaging | ✅ |
| Contact name resolution | ✅ |
| Whitelist file transfer | ✅ |
| 4 agent tools | ✅ |
| Local SQLite storage | ✅ |
| Self-chat assistant (message yourself, get a full assistant back) | ✅ (v3.9.0) |
| Routines via chat (`create_routine`/`list_routines`/`cancel_routine`) | ✅ (v3.9.0) |
| `/auto-perm` slash command | ✅ (v3.9.0) |

## ✈️ Telegram Integration (v3.9.0)

| Feature | Status |
|---------|--------|
| Bot token pairing (`@BotFather`) | ✅ |
| Owner lock (first sender becomes permanent owner) | ✅ |
| Self-chat assistant | ✅ |
| Routines via chat | ✅ |
| Local SQLite storage, isolated from WhatsApp's | ✅ |
| `telegram_send` agent tool | ❌ — no equivalent to WhatsApp's send/search/latest/messages tools yet |

See [[Telegram Integration]].

## 🔐 Remote Access & Backup

| Feature | Status |
|---------|--------|
| ngrok tunnel | ✅ |
| Tailscale tunnel | ✅ — graduated out of Beta (v3.3.4): one-click login (no auth key needed), Funnel on by default, auto-reconnect |
| Token auth (`X-Memo-Token`) — now required on remote access | ✅ (v3.3.3 security fix) |
| Multi-account, admin/user roles | ✅ (v3.5.5, Faz 5.1) — Settings → Accounts or `memo remote add-account` |
| Granular per-account permissions (7 checkboxes) | ✅ (v3.9.0, Faz 5.1.1) — enforced server-side, not just hidden in the UI |
| `.memo` export/import — now actually complete | ✅ (v3.3.3) — calendar, habits, routines, task lists, agent permissions, skills, and `machine.key` are all included; see [[Backup & Restore]] |
| Full wipe (Delete All Data, fixed on Windows) | ✅ (v3.3.4 fix) |
| Google Drive E2E sync | ✅ |
| AES-256-GCM encryption | ✅ |
| Developer API Gateway | ✅ (v3.3.3) — see [[Developer API Gateway]] |

## 🎨 UI & UX

| Feature | Status |
|---------|--------|
| Streaming SSE responses | ✅ — ended by silence (300 s) not a fixed total; 10 s `heartbeat` chunks (v4.6.0) |
| Context ring + popover (used / window, make-up, free space, auto-compact buffer, Subscriptions usage limits) | ✅ (v4.6.0) — `GET /api/context`; the provider's real count when reported, else a `~` estimate |
| Auto-compact at 90% of the window, every provider (local, API, Subscriptions) | ✅ (v4.6.0) — whole prompt incl. tool schema; also triggers on the provider's own previous count |
| Readable provider errors (safety, allowance, rate limit, key, model, context, timeout, outage, network) | ✅ (v4.6.0) — sentence in the UI language + `(HTTP n)`; raw text logged |
| Live progress line with elapsed timer and a 30 s "no word from the server" warning | ✅ (v4.6.0) |
| Sidebar spinner on every chat still generating (background tasks, WhatsApp/Telegram, other tabs) | ✅ (v4.6.0) |
| Model-picker panel (search, foldable vendor sections, logos, remaining-allowance badges) | ✅ (v4.6.0) |
| Usage-limit card with countdown and automatic continue | ✅ (v4.6.0) — Subscriptions; in memory only, works while Memo is open |
| Memory tab: Known Facts / Conversation History with edit and selective delete | ✅ (v4.6.0) |
| Usage Stats: Prompt Cache panel | ✅ (v4.6.0) — "not reported" when a provider sends no cache figure |
| Markdown rendering | ✅ |
| Image attach (vision) | ✅ |
| File context attach | ✅ |
| Edit/delete/export messages | ✅ |
| `@` file-mention in chat | ✅ (v3.3.4) |
| Quick model-switcher pill in chat top bar | ✅ (v3.3.4) |
| Agent mode toggle in chat top bar | ✅ (v3.3.3) — next to the web-search toggle |
| Incognito toggle | ✅ |
| Minimal Mode | ✅ (v3.3.3) — strips personality/mood/web-search instructions for max local performance |
| Setup wizard (6 personas) | ✅ |
| Multi-language (TR/EN) | ✅ |
| Settings reorganized into a searchable rail | ✅ (v3.3.4) — replaces ~20 flat tabs |
| Greige theme, Material 3 | ✅ |
| Memo on a phone | ✅ — the same `frontend/` client, built for Android/iOS (the separate mobile app was retired in 2026-09); OS-level calendar reminders |
| Dark mode | ✅ |

## 🎵 Voice & Multimodal

| Feature | Status |
|---------|--------|
| Local STT | ✅ |
| Live Mode v2 — native audio-to-audio (Google Live / OpenAI Realtime) | ✅ (v4.3.0) — replaced the old Whisper→LLM→Piper relay entirely; full-screen call UI, delegate mode (main model does real work, live model narrates) or standalone mode (live model gets the full agent toolset directly), configurable barge-in sensitivity, transcript kept in chat history, feeds long-term memory — see [[Multimodal Capabilities (Vision and Voice)]] |
| ElevenLabs / Custom voice engines | ✅ (v4.3.0) |
| Image upload (multimodal GGUF) | ✅ |
| Document indexing | ✅ |
