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
| OpenCode Zen | ✅ (v3.3.3) | API key — pay-as-you-go, some models free; free-sorted model browser (v3.9.0) |
| OpenCode Go | ✅ (v3.3.3) | API key — subscription-based |
| Kilo Code | ✅ (v3.9.0) | API key — app.kilo.ai, pay-as-you-go, some models free, live model browser with free models sorted to the top |
| Claude Code (CLI) | ✅ Beta (v3.3.4) | Shells out to the locally installed `claude` CLI, per-chat, real background job |
| Codex (CLI) | ✅ Beta (v3.3.4) | Shells out to the locally installed `codex` CLI, per-chat, real background job |

Router features: fallback chain, auto-disable after 3 failures, health check goroutine. Full detail: [[External Providers]].

## 🧑‍💻 Developer Tools (v3.3.3)

| Feature | Status | Description |
|---------|--------|-------------|
| Usage Stats | ✅ | Settings → Stats: token/speed/model breakdown, 30-day chart (fl_chart) |
| Developer API Gateway | ✅ | Its own screen in the sidebar (not inside Settings): point Claude Code (`ANTHROPIC_BASE_URL`) or any OpenAI-compatible tool at Memo's local/external model, includes a live request/response log — see [[Developer API Gateway]] |

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
| 22 built-in tools (file/edit/command/search/calendar/routines/WhatsApp/web-search/provider-config/self-clone) | ✅ |
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
| Bot token pairing (@BotFather) | ✅ |
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
| Streaming SSE responses | ✅ |
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
| Mobile companion app | ✅ — now fully localized (TR/EN), Routines support |
| Dark mode | ✅ |

## 🎵 Voice & Multimodal

| Feature | Status |
|---------|--------|
| Local STT | ✅ |
| Live Mode (hands-free voice chat) | ✅ Beta (v3.3.4) — icon next to the chat input; local Piper TTS by default, optional external OpenAI TTS, offline voice picker, one-directional barge-in; no echo cancellation yet — see [[Multimodal Capabilities (Vision and Voice)]] |
| Image upload (multimodal GGUF) | ✅ |
| Document indexing | ✅ |
