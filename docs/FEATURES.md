# Memo — Comprehensive Feature Catalog

This document provides a detailed breakdown of every feature integrated into the **Memo AI Memory Shell**. From architectural persistence to sensory multimodality, here is how Memo empowers your local AI experience.

---

## 1. 🧠 Core Intelligence & Memory

### Persistent RAG (Retrieval-Augmented Generation)
Memo isn't just a chat; it's a "Second Brain."
- **Semantic Indexing**: Every interaction is automatically embedded and stored in a local vector database.
- **Hybrid Search**: Retrieval combines vector similarity with FTS5 keyword search (merged via Reciprocal Rank Fusion), so a short, exact fact isn't only found by "close enough" semantic distance.
- **Compound-Question Splitting**: A multi-topic question ("what's my name, birthday, and favorite color") is split on conjunctions so each topic gets its own search instead of being diluted into one blended embedding.
- **Contextual Recall**: Before every response, Memo performs this hybrid search to retrieve the most relevant past conversations (Top-K matching).
- **Pinned Facts (2026-07-15)**: Durable personal facts (name, birthday, pets, etc.) — whether saved via `/remember` or automatically detected from ordinary conversation — are injected into every prompt unconditionally, bypassing retrieval ranking entirely, so they're never crowded out by routine chat.
- **Infinite Context**: Long-term memory allows the AI to remember details from weeks or months ago, regardless of the current model's window.

### Model-Agnostic Engine
- **Internal Llama-Server**: Powered by `llama.cpp` for high-performance GGUF inference.
- **Dedicated Embedding Server**: A second internal server runs specifically for memory indexing, ensuring chat performance remains untouched.
- **Cross-Mode Architecture**: Use external API providers (OpenAI, Claude, Gemini) for chat while a tiny local model handles embeddings — both run independently.
- **External Provider Support**: Seamlessly connects to LM-Studio or any OpenAI-compatible local API (Port 1234/8081).

### Remote Access & Self-Hosting
- **Four Auth Modes**: `none` (explicitly opt-in, loudly warned about), `token` (per-device tokens, the default), `password` (username + argon2id-hashed password, short-lived signed session), or `token+password` (either satisfies — OR logic). Selectable per-server in Settings → Remote Access or via the `memo remote set-mode` CLI command.
- **Per-Device Tokens**: Every paired device (phone, laptop, a second desktop) gets its own token, shown once at creation and only ever stored hashed — revoke one device without rotating everyone else's. Managed from Settings, or `memo remote list-devices`/`add-device`/`revoke-device` over SSH.
- **Brute-Force Protection**: Password-mode logins are rate-limited independently of the general API rate limiter — a handful of free attempts, then exponential backoff.
- **ngrok Tunnel**: Built-in ngrok integration for accessing your Memo backend from anywhere. Auto-download, tunnel management, configurable domain and region.
- **Tailscale (out of Beta)**: One-click login (no auth key to paste), Funnel on by default, auto-reconnect after a dropped connection — available directly in Settings → Remote Access, desktop and mobile.
- **Self-Hosted Server Mode**: Run just the headless backend — no desktop app — on a Raspberry Pi, home server, or VPS, managed entirely over SSH via the `memo` CLI (`memo service install` for a systemd --user service, `memo config get/set` for config.yaml, `memo remote` for auth/devices). Native installer (`get-memo-server.sh`) or a multi-arch (amd64+arm64) Docker/CasaOS image. See [Self-Hosting](SELF_HOSTED.md).
- **Multi-Account, Admin/User Roles**: A shared self-hosted server can host more than one login — an admin plus any number of user accounts, each with their own password. Managed from Settings → Accounts or `memo remote list-accounts`/`add-account`/`delete-account` over SSH.
- **Granular Per-Account Permissions**: Seven independent toggles per account (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines) — an admin can, say, let a user chat and use Agent tools while hiding the Model Store/API Providers tabs and blocking memory writes entirely. Enforced on the backend (not just hidden in the UI), checkbox UI in Settings → Accounts.

---

## 2. 🏛️ Architecture & Persistence

### SQLite + sqlite-vec Persistence
- **Unified Storage**: Vector embeddings and metadata live in the same SQLite database.
- **ANN Indexing**: `vec0` virtual table provides approximate nearest neighbor search with O(log N) query time.
- **ACID Compliance**: Built-in transaction support ensures atomic writes and data integrity.
- **Go Fallback**: If vec0 extension is unavailable, brute-force cosine similarity fallback.

### Privacy & Local Isolation
- **100% Offline**: No data ever leaves your computer. No telemetry, no logs, no cloud dependencies.
- **Encrypted Local Storage**: Your mind stays on your hardware.
- **AES-256-GCM Encryption**: API keys encrypted with machine-derived key.

### Backup & Restore (.memo)
- **Full Export**: `GET /api/export` — zip archive of sessions, config, providers, orchestra, memory, WhatsApp data, **plus calendar events, learned habits, routines, task lists, agent tool permissions, installed skills, and `machine.key`** (previously missing — without `machine.key`, a restored backup left every provider API key permanently undecryptable). Models excluded by default (own toggle).
- **Full Import**: `POST /api/import` — restore from .memo zip. Optional model inclusion.
- **Wipe All Data**: `POST /api/wipe` — double-confirmation dialog, config file persists. Now reliably closes every internal database (memory, stats, calendar, mood, WhatsApp) before removing files, fixing a Windows-only failure.

### Mobile Companion App
- **Thin Client**: Android/iOS Flutter app connecting over LAN or remote tunnel.
- **Zero Processing**: All AI stays on desktop — mobile is a secure remote viewport.
- **Features**: Chat (SSE streaming), settings (provider/model control), session management.
- **Planned (v3.3.0)**: Full feature parity, biometric auth, offline queue, voice input.

---

## 3. 🏭 Model Management (The Factory)

### Integrated Hugging Face Search
- **Direct Repository Access**: Search for models on Hugging Face directly within the app.
- **Repo ID Support**: Paste any Hugging Face GGUF repo ID to fetch available files instantly.

### System Diagnostics
- **VRAM & GPU Check**: Auto-detection of available NVIDIA/AMD VRAM.
- **Compatibility Badge**: Flags models as "GPU Compatible" or warns about "Insufficient VRAM" before you download.

### Background Download Manager
- **Parallel Downloading**: Several GGUF downloads can now run at once (previously a second download was rejected outright), with combined progress in the engine status bar and a rough time estimate, not just a bare percentage.
- **Lifecycle Control**: One-click Start, Stop, and Update for all local models.
- **Hardware-Matched First-Run Suggestion**: Setup reads your RAM/GPU and recommends a matching chat + memory model pair, with one button to start both downloading.
- **Safer Context Sizing**: The context-size field now reads the model's real maximum context straight from the GGUF file and won't let the slider exceed it (previously free-text, could crash the engine).
- **Accurate Capability Badges**: Tool-calling/code badges are now derived from the model's actual chat template and tags instead of a hardcoded list of "known" families.
- **Plain-Language Errors & Tooltips**: Raw errors like `llama: server failed to become ready within 120s` are now short and actionable; hardware-fit and quantization badges have hover tooltips explaining what they mean.
- **Discover Filters**: Tools/Vision/Code/Embedding/Size filters now combine with OR (not AND) and are grouped into multi-select dropdowns with an "N filters active · clear" indicator.

---

## 4. ⚡ Interaction & User Experience

### Streaming Responses
- **Token-by-Token Rendering**: Watch the AI "type" its responses in real-time.
- **Thinking State**: A pulsing "Memo is thinking..." status provides visual feedback before the first token arrives.
- **Cursor UI**: A blinking terminal-style cursor (`▊`) follows the stream.

### Live Mode — Hands-Free Voice (Beta)
- A small voice icon next to the chat input box (after enabling Settings → Beta Features) — not a separate sidebar tab. Listens, auto-detects when you start/stop speaking, transcribes locally, sends it as a normal chat message, and speaks the reply back.
- **Local by default**: on-device transcription + local **Piper** TTS, so nothing has to leave the machine. An optional external OpenAI TTS provider can be configured instead; local Piper is always the fallback.
- **Offline voice picker**: download a small curated set of Piper voices (Turkish/English), switch instantly, no restart.
- **One-directional barge-in**: speak again while Memo is talking and it stops to listen instead of talking over you.
- **Known limitation**: no echo cancellation yet — speakers (vs. headphones) can occasionally make Memo mistake its own voice for an interruption.

### @ File-Mention
- Type `@` in any chat's message box to search for and reference a file by name — useful for pointing agent mode at a specific file without typing the full path.

### Incognito Mode
- **Zero-Persistence**: A secure toggle that disables all memory saving and history logging for sensitive sessions.
- **Volatile Context**: Context exists only within that specific session and is wiped upon closing.

### Performance HUD
- **Real-time Stats**: Hover over the timestamp to see generation speed (tok/s), total tokens, and precise duration metrics.

### WhatsApp Integration
- **QR Pairing**: Full WhatsApp Web multi-device pairing via QR code displayed in-app.
- **Bidirectional Messaging**: Send and receive messages with contact-aware display.
- **Contact Resolution**: Phonebook sync, push names, fallback to phone number.
- **Whitelist File Transfer**: Trusted contacts can request files from whitelisted directories.
- **Agent Tools**: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`.
- **Dedicated Chat Mode**: Isolated executor and tool registry for WhatsApp-only interactions.
- **Self-Chat Assistant**: Message your own WhatsApp number (the number paired for QR login) and Memo replies as a full assistant — chat, memory, and agent tools all reachable from your phone's WhatsApp app without opening Memo itself.
- **Routines via Chat**: Ask in plain language ("her sabah 8'de bana hava durumunu hatırlat") and Memo creates, lists, or cancels a routine directly from the conversation, using the same `create_routine`/`list_routines`/`cancel_routine` agent tools available in-app.
- **`/auto-perm`**: A self-chat slash command that flips tool-call permission prompts to auto-allow for that conversation, so routine/agent actions triggered from chat don't stall waiting for a desktop click that isn't coming.
- **Local Storage**: All WhatsApp messages stored in an isolated SQLite database.

### Telegram Integration
- **Bot Pairing**: Connect a Telegram bot token (from `@BotFather`) in Settings → Telegram; Memo long-polls the Bot API for messages once configured.
- **Owner Lock**: Since anyone who finds a bot's username can message it, Memo locks in whoever messages first as the bot's permanent owner and silently ignores everyone else afterward — the entire access-control boundary for this integration.
- **Assistant Chat**: Once linked, the owner gets a full assistant — chat, memory, and agent tools — the same capability as the WhatsApp self-chat path, on Telegram instead.
- **Routines via Chat**: The same `create_routine`/`list_routines`/`cancel_routine` tool flow works from a Telegram conversation.
- **Local Storage**: Telegram messages stored in their own isolated SQLite database, independent of WhatsApp's.

---

## 5. 🔌 External Provider Support

### Multi-Provider Architecture
Memo connects to external LLM APIs alongside local models:
- **Supported Providers (15 `ProviderType` values):** OpenAI, Google Gemini, xAI Grok, Anthropic Claude, OpenRouter, Groq, Ollama, a generic **Custom** (any OpenAI-compatible endpoint), a new **Custom (Anthropic-compatible)** (any Anthropic Messages API-shaped endpoint — e.g. your own proxy, for when it doesn't speak OpenAI's format), plus **OpenCode Zen** (pay-as-you-go, some models free), **OpenCode Go** (subscription), and **Kilo Code** (app.kilo.ai — pay-as-you-go, some models free) — the three gateways let you pick from a live model list instead of typing a model name by hand, with free models sorted to the top and marked with a green checkmark.
- **Claude and Gemini now support real tool-calling** (previously entirely missing on both — an agent/task-loop turn on either provider silently couldn't use tools at all). Both round-trip single and parallel tool calls correctly per each vendor's own wire format.
- **Claude Code / Codex CLI as chat providers (beta):** instead of an API call, Memo shells out to a locally installed `claude`/`codex` CLI. Per-chat (not app-wide), runs as a real untimed background job, uses the CLI's own no-prompt permission mode, and its own `/` slash commands surface in Memo's command popup. No memory/identity context is sent — the CLI manages its own session.
- **Provider Interface:** Common `Provider` interface with `ChatCompletion`, `ChatCompletionStream`, `ListModels`
- **Fallback Chain:** Router tries providers in order; auto-disables after 3 consecutive failures; health-check re-enables on recovery

### Encrypted Key Management
- **AES-256-GCM Encryption:** API keys encrypted with machine-derived key (`/etc/machine-id`)
- **Key Storage:** `data/providers.json` with encrypted key values
- **Test Connection:** Built-in test button validates connectivity before saving

### Frontend Provider UI
- **API Providers Tab:** Settings tab for adding/editing providers
- **Configuration Dialog:** Provider type selector, API key input (masked), base URL, model dropdown
- **Active Provider Selection:** Choose which provider is active for chat

---

## 6. 🧠 Agent Mode (Tool Calling)

### Tool Execution Engine
Memo acts as an AI agent with full computer control:
- **27 Built-in Tools** (verified against `registerBuiltins()`, `internal/agent/tools.go` — up from an earlier "22"): file I/O (`read_file`, `write_file`, `edit_file`, `insert_line`, `delete_lines`, `delete_file`, `list_directory`, `get_file_info`, `search_files`, `change_directory`), `run_command`, `read_env`, `web_search`, `fetch_page`, `self_clone`, `configure_provider`, `get_calendar_events`, task-loop control (`get_task_status`, `pause_task`, `resume_task`, `create_task_md`, `edit_task_md`, `start_self_driving_task` — see §6.5 below), routines (`create_routine`, `list_routines`, `cancel_routine`), `share_file`. WhatsApp's 4 tools (`whatsapp_send`/`search`/`latest`/`messages`) live in a *separate* scoped registry, not this main one.
- **Skill tools now actually execute.** A skill's `SKILL.md` can define a `command:` field, wired into the exact same tool pipeline and permission-prompt UI as built-in tools — previously this was declaration-only and never ran anything.
- **Tool Registry:** Thread-safe registry with JSON Schema parameter definitions
- **Danger Level System:** `safe` (auto-allowed), `medium` (prompt user), `dangerous` (prompt + delay)

### Permission System
- **6 Policy Types:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever
- **Session Persistence:** Permissions stored in `data/permissions.json`
- **Arg Hashing:** SHA-256 hashing for permission matching

### Security Sandbox
- **Path Traversal Protection:** Symlink resolution, `..` blocking, project root confinement
- **Command Blacklist:** 43 dangerous patterns blocked (`rm -rf /`, `sudo`, fork bombs, etc. — grown from an earlier "23")
- **Rate Limiting:** 30 tool calls/minute, 5s cooldown per command

### Agent Pipeline
- **LLM ↔ Tool Loop:** Sends user message + tool definitions to LLM, executes tool calls, feeds results back, loops until final response (max 40 iterations, verified against `pipeline.go`'s `maxIters`)
- **Event Streaming:** Tool execution events streamed to frontend via SSE
- **Audit Log:** Last 1000 tool executions logged with timestamps

> **Note:** Agent frontend UI (permission dialogs, tool call cards, mode toggle) shipped some time ago and is fully live — the toggle sits directly in Chat's top bar next to the web-search toggle, no separate Agent-only screen needed.

---

## 6.5 🚗 Self-Driving Task Loop (new in v4.4.0)

An unattended, multi-step task runner built on top of Agent Mode — you hand it a checklist, it works through it on its own.

- **`Task.md` schema.** A plain-Markdown checklist (`- [ ]` items) with optional `# key: value` headers controlling mode (`worker` or `planlayıcı`/planner), notification verbosity, per-role model pinning, memory, provider lock/roaming (`# sağlayıcı: sabit|otomatik|<name>`), and plan auto-approval. Created/edited via the `create_task_md`/`edit_task_md` tools, or started from an existing file via `start_self_driving_task` (also reachable from the Tasks tab by pointing at a `Task.md` path).
- **Planner/executor mode.** For `# mod: planlayıcı`, a planning turn first produces a `Plan.md` (steps, acceptance checks, a DAG of dependencies) that the user approves — either from the Tasks tab's plan-approval card, or automatically with `# onay: otomatik`.
- **Sub-agent orchestration.** A large or clearly-parallel item can split into up to 3 sub-agents: exactly one write-capable `coder` runs first, then up to 3 read-only `analyzer`/`reviewer`/`test-runner` sub-agents run in true parallel and their results feed a chief review.
- **Live activity in the task card.** Tool calls, sub-agent turns (`[coder]`/`[analyzer]`/…), "model is generating" during long silent LLM calls, and slow-tool "starting…" lines stream into a live in-app card as the loop runs.
- **Resilience, not silent failure.** A busy chat queues and retries instead of killing the task instantly; a rate-limited provider waits and resumes from the same item, never restarts the list; a transient fault gets an escalating retry (5 then 10 minutes) before the item is parked for the user; an auth/config fault parks the list in a waiting-user state rather than looping forever. Every terminal state notifies (chat message + push), by design.
- **Pause/resume from chat.** `pause_task`/`resume_task` let the model itself pause a running task and resume it from the same step, carrying forward whatever the user typed while it was paused.
- **Known open gaps** (see `BUG_REPORT.md`, `BUG-PLAN9`/`10`/`11`/`12`): plan approval is Tasks-tab-only for now (not inline in chat), the chat model can't yet read a *running* task's live status (risk of a confidently wrong "it's broken" narrative if asked mid-run), and step/item counters can disagree across screens after an escalation splits a step.

---

## 7. 🎵 Orchestra Mode (Multi-Model Orchestration)

### Concept
Multiple AI models collaborate as a team:
1. **Chief Model** analyzes user request, breaks into subtasks
2. **Expert Roles** execute tasks in parallel (frontend, backend, bug_fixer, etc.)
3. **Chief Model** synthesizes results into a single coherent response

### Built-in Roles
| Role | Default Model | Purpose |
|------|-------------|---------|
| Planner | Claude | Software architecture, task decomposition |
| Frontend | Grok | UI development |
| Backend | GPT-4o | API/server logic |
| Bug Fixer | Gemini | Debugging, root cause analysis |
| Reviewer | Claude | Code quality review |
| Security | GPT-4o | Security auditing |
| DevOps | Grok | Infrastructure/deploy |
| General | GPT-4o | General-purpose fallback |

### Execution Model
- **Parallel Tasks:** Independent tasks run concurrently (goroutines + WaitGroup)
- **Sequential Tasks:** Dependency resolution via `depends_on` field
- **Retry:** Rate-limit aware retry with exponential backoff (up to 3 retries)
- **Streaming:** Progress updates streamed per phase (plan → execute → synthesize)

### Frontend Controls
- **Settings Tab:** Enable/disable, configure chief model, assign models to roles
- **Config Dialog:** Role editor with model selection, system prompt editing, custom role support
- **Slash Command:** `/orchestra on`, `/orchestra off`, `/orchestra config`, `/orchestra status`

---

## 8. 👁️ Multimodality & Senses

### Vision Support (Multimodal)
- **Image Integration**: Drag-and-drop or upload images for analysis (requires a multimodal-capable GGUF like Llava or Moondream).
- **Base64 Processing**: Local, secure image encoding.

### File Contextualization
- **Document Indexing**: Attach code files (.go, .js, .py) or documents (.md, .txt) to give the AI massive instant context for a specific task.

### Local STT (Speech-to-Text)
- **Offline Transcription**: Record voice messages directly in the app.
- **Bundled Engine**: Uses a localized environment (Vosk/Whisper equivalent) for zero-latency, private transcription.

---

## 9. ⏰ Routines & Proactive Intelligence

### Routines (Scheduled Automations)
- Describe a task and a schedule in plain language; Memo turns it into a routine that fires on schedule as a simple prompt or a full tool-using agent run.
- **Create from chat, not just the Routines tab**: ask for a routine in plain language from a normal chat, or from the WhatsApp/Telegram self-chat assistant, and the `create_routine`/`list_routines`/`cancel_routine` agent tools handle it — no need to open the dedicated Routines screen.
- A routine always has full agent + web-search tool access when it fires, regardless of how it was created — an earlier bug tied that access to a one-shot classification made at creation time, so it could silently "turn off" later; fixed to be unconditional.
- Works on **desktop and mobile** — mobile delivers real, pre-scheduled local notifications so reminders arrive even if the app isn't open.
- Fires in **your own device's timezone** (captured at creation, resynced on every reconnect), so travel/DST corrects itself instead of staying frozen.

### Proactive Learning & Ambient Nudges
- Memo notices usage patterns (a stated habit, or something you tend to do at a certain time) and can bring it up on its own — on by default at a subtle level.
- A directly-stated habit ("I code every night around 9") is trusted immediately; a passively-observed pattern needs to show up statistically first.
- A nudge can appear woven into a normal reply, or as a desktop suggestion banner (Yes / Not now / Stop asking).
- Fully disabled in Incognito mode, and under Minimal Mode unless specifically re-enabled.

### Self-Insight (`/insight`)
- Ask directly, or let a weekly Routine ask, and Memo looks back over recent mood/memory history for a real pattern — explicitly instructed not to invent one if there isn't enough signal.

### Minimal Mode (Settings → General)
- Strips personality/mood/web-search instructions from every prompt for people who want their local model running with as little overhead as possible; with memory also off, nothing extra is added beyond the typed message.
- Persona/system-prompt, capability disclosures, passive-feature disclosures, and proactive learning can each be independently re-enabled even while Minimal Mode is otherwise on.

### Memo's Own Identity
- Asking who built Memo, what it's for, or what it stands for now gets a real, grounded answer instead of a guess — this only surfaces when asked, and doesn't change day-to-day behavior or depend on which persona was picked.

---

## 10. 🛠️ Developer & Power-User Features

### Developer API Gateway (Sidebar → Developer)
- A local Anthropic-compatible API endpoint, so tools that only support that wire format (most notably **Claude Code**, via `ANTHROPIC_BASE_URL`) can run against Memo's local model or any configured provider/key.
- Model selection via a `type/model-id` format (`local/qwen2.5`, `openai/gpt-4o`, ...). Full agentic tool calling for openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go providers.
- Optional API-key requirement (shares Remote Access's token), optional memory integration, live request log.

### Memo Swarm (Beta)
- Pool several PCs' compute (Settings → Beta Features → Swarm) to run one GGUF model too large for a single machine's RAM/VRAM — one Host holds the model file, others Join with a room code and lend compute via llama.cpp's `rpc-server`.
- Goal is capacity, not speed. Not available on macOS yet.

### Usage Stats (Settings → Stats)
- KPI cards (total requests, input/output tokens, avg tok/s, most-used model), a 30-day stacked daily-usage chart, and a per-model breakdown — recorded for every completed turn (local, agent, orchestra, or external provider) except in Incognito mode.

### Import Memory From Another AI (Settings)
- Paste a structured description from another AI assistant (ChatGPT, Gemini, Claude, ...) and Memo breaks it into atomic facts saved the same way `/remember` does, plus a communication-style summary folded into its own system prompt.

### Report a Bug (Settings)
- Prefills a GitHub issue in your browser (with an optional attachment of your last 10 background error events) — nothing is sent anywhere until you review and submit it yourself on GitHub.

### Settings, Reorganized
- Settings moved from ~20 flat tabs into a searchable, grouped rail with a search box.

---

## 🎨 Design Philosophy: "Greige" Minimalism
- **Focus-First UI**: Minimalist color palette to reduce cognitive load.
- **Responsive Layout**: Designed for both desktop-wide and mobile-narrow views.
- **Onboarding Wizard**: A guided setup for name, persona, and initial diagnostics.

---
**Built by Buğra.**
*Control your AI. Own your Memory.*
