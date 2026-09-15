# Architecture — Memo v4.5.0

> **Updated for v4.5.0.** Since the v3.3.4 baseline this page was last fully written for, four more releases shipped: v4.0.0 (real time-awareness, WhatsApp takeover), v4.3.0 (Live Mode v2 native audio, Telegram bridge), v4.4.0 (the Self-Driving task loop's major expansion, real Claude/Gemini tool-calling, the OpenAI-compatible Developer Gateway sibling, gemini-sub), and v4.5.0 (the desktop mascot, Code Mode's Plan/Auto/Build sub-modes). The module map and data-flow list below have been updated to match; the ASCII diagram and per-module descriptions elsewhere on this page may still read slightly foundational — see [[Self-Driving Task Loop]] and [[Desktop Mascot]] for the two newest subsystems in full depth.

## Overview

Memo is a **two-process, local-first application**. The Go backend and Flutter frontend run as separate processes communicating over plain HTTP (localhost:8090) via REST + SSE streaming.

## Process Architecture

```
Flutter Desktop (Linux/Windows/macOS)     Flutter Mobile (Android/iOS)
         │                                    │
         │  REST + SSE (:8090)                 │  LAN / ngrok / Tailscale
         └──────────────┬─────────────────────┘
                        │
              ┌─────────┴──────────────────────────────────┐
              │            Go Backend (41 packages)         │
              │  (every background task panic-recovered,    │
              │   v3.3.4 — a crash in one is contained,      │
              │   not fatal to the whole process)            │
              │                                            │
              │  ┌──────────┐  ┌────────┐  ┌───────────┐  │
              │  │Web Server│  │  App   │  │ Proactive │  │
              │  │180+ routes│ │ Engine │  │  Engine   │  │
              │  │SSE stream│  │(40+ files)│ │Observer→  │  │
              │  └──────────┘  └───┬────┘  │Analyzer→Act│  │
              │                    │        └───────────┘  │
              │  ┌────────┐┌──────┴──────┐┌─────────────┐  │
              │  │ Memory ││  Providers  ││    Agent    │  │
              │  │SQLite+ ││ 16 types    ││  Pipeline   │  │
              │  │vec0+FTS││  Router     ││  27 tools   │  │
              │  └────────┘└─────────────┘└─────────────┘  │
              │                                            │
              │  Llama · WhatsApp · Telegram · Calendar     │
              │  Orchestra · TaskLoop · LiveMode · CloudSync│
              │  Whisper · STT · TTS · Skills · Mood        │
              │  ModelStore · Intent · ngrok · Tunnel       │
              │  Routine · Stats · Swarm · AgentCLI         │
              │  GeminiSub · RemoteAuth · BrowserEngine     │
              │  AnthropicAPI/OpenAIAPI (Dev Gateway)       │
              │  GGUF · Models · Sessions · Config · Logx   │
              └────────────────────────────────────────────┘
```

## Module Map (40+ packages)

| Directory | Responsibility |
|-----------|---------------|
| `internal/app/` | Central orchestrator |
| `internal/webserver/` | REST API (180+ routes), SSE streaming |
| `internal/memory/` | Vector store — SQLite + sqlite-vec + FTS5 hybrid search, embedder |
| `internal/provider/` | External LLM providers — 16 types, router, fallback |
| `internal/agent/` | Agent pipeline, sandbox, permissions, 27 built-in tools, Code Mode sub-mode tool set (`save_code_plan`) |
| `internal/taskloop/` | Self-Driving task loop engine — `Task.md` schema, planner/executor, sub-agent orchestration, escalating retry — see [[Self-Driving Task Loop]] |
| `internal/agentcli/` | Claude Code CLI / Codex CLI as chat providers (Beta) — subprocess-based, registers into `provider` via `RegisterConstructor` |
| `internal/geminisub/` | "gemini-sub" — personal Google account sign-in, Gemini via Code Assist (Beta) |
| `internal/anthropicapi/`, `internal/openaiapi/` | Developer API Gateway wire-format translation — Anthropic- and OpenAI-compatible, both key-enforced for non-loopback callers (v4.5.0) |
| `internal/livemode/` | Live Mode v2 engine — Google Live / OpenAI Realtime session management, reconnect, transcript, delegate mode |
| `internal/telegram/` | Telegram bot bridge — mirrors `internal/whatsapp/`'s shape, isolated SQLite store |
| `internal/remoteauth/` | Remote access auth core — password hashing, brute-force lockout, per-device tokens, JWT session tokens |
| `internal/browserengine/` | Optional headless-browser rendering for JS-heavy pages `internal/websearch` alone can't read |
| `internal/stt/` | Live Mode speech-to-text provider routing (local whisper.cpp, ElevenLabs, custom) |
| `internal/orchestra/` | Multi-model conductor, 8 roles, parallel execution |
| `internal/llama/` | llama.cpp subprocess lifecycle, GPU detection, RPC (Swarm) support |
| `internal/whatsapp/` | WhatsApp bridge — whatsmeow client + store |
| `internal/calendar/` | Event store, reminder loop, intent bridge |
| `internal/routine/` | Scheduled automations (Routines) — per-device timezone, prompt or agent run |
| `internal/cloudsync/` | Google Drive E2E encrypted backup |
| `internal/modelstore/` | HuggingFace model search and download |
| `internal/models/` | Local model metadata/registry helpers |
| `internal/gguf/` | GGUF file inspection (real max context, chat template/tag-based capability badges) |
| `internal/sessions/` | Chat session JSON persistence (incl. CLI provider/session/workdir fields) |
| `internal/config/` | YAML configuration management |
| `internal/database/` | SQLite connection + vec0 extension registration |
| `internal/api/` | OpenAI-compatible API client + SSE streaming |
| `internal/identity/` | System prompt, persona, incognito prompt, self-identity disclosure |
| `internal/intent/` | Intent extraction pipeline (chat → calendar events) |
| `internal/proactive/` | Proactive suggestion engine, ambient nudges |
| `internal/observer/` | Usage pattern analyzer (circular statistics) |
| `internal/skill/` | Skill system — load, manage, inject instructions, execute `command:` tools |
| `internal/mood/` | Stochastic emotion engine + self-interest protocol |
| `internal/whisper/` | Speech-to-text via whisper.cpp |
| `internal/tts/` | Text-to-speech — local Piper by default, optional external provider (Live Mode) |
| `internal/swarm/` | Memo Swarm — multi-PC room/worker orchestration over llama.cpp RPC (Beta) |
| `internal/stats/` | Usage stats recording (tokens, speed, per-model breakdown) |
| `internal/ngrok/` | ngrok tunnel manager (auto-restart on crash) |
| `internal/tunnel/` | Tailscale embedded tunnel (tsnet) — one-click login, auto-reconnect |
| `internal/truncate/` | Token-aware context truncation |
| `internal/logx/` | Structured logging (slog wrapper with levels) |
| `internal/websearch/` | DuckDuckGo HTML scraping |
| `internal/shutdown/` | Coordinated graceful shutdown |
| `internal/fileutil/`, `internal/jsonutil/` | Shared file/JSON helpers |
| `internal/browseropen/` | Cross-platform "open in browser" helper |
| `internal/replcli/` | Terminal CLI (`memo-cli`) — themes, `/theme` picker, Shift+Tab auto-approve |

## Data Flow

1. **Chat** — User → Flutter → POST /api/send/stream → App.buildMessages() → LLM → SSE stream → Flutter render
2. **Memory** — User + assistant messages → embed → SQLite vec0 + FTS5 hybrid search → retrieve on next query → inject into system prompt (capped at 4096 tokens as of v3.3.4)
3. **Agent** — User request → Agent Pipeline → LLM tool call → Permission dialog → Tool execution → Result feedback → Loop
4. **Proactive** — Observer records timestamps → Analyzer detects patterns → Chief LLM decides action → Notify/Suggest/Auto-execute (suggestion banner on desktop)
5. **Calendar** — Message text → Keyword filter → LLM intent extraction → Store event → Reminder loop fires notification
6. **Routines** — User describes a schedule in plain language → parsed into a routine → fires in the device's own timezone → simple prompt or full agent run → notification (mobile: real pre-scheduled local notification)
7. **Developer Gateway** — External tool (e.g. Claude Code via `ANTHROPIC_BASE_URL`, or any OpenAI-SDK tool) → `POST /v1/messages` or `/v1/chat/completions` → `internal/anthropicapi`/`internal/openaiapi` translates to Memo's internal format → routes to local model or a configured provider → translates the response back
8. **CLI providers** — Chat with Claude Code/Codex CLI selected → Memo shells out to the installed CLI as a subprocess tied to the chat, independent of the app's global stream lock, survives switching chats/closing the window
9. **Self-Driving task loop** — `Task.md` checklist → `internal/taskloop` engine works items sequentially (optionally via a planner turn first) → each item runs through the same Agent Pipeline, its own provider/executor snapshot, with up to 3 parallel sub-agents on large items → live activity streamed to the Tasks tab and the desktop mascot → terminal state always notifies
10. **Code Mode sub-modes** — `Session.CodeSubMode` (Plan/Auto/Build) resolves which system prompt and tool auto-approve set an agent turn gets; Plan's output goes through the dedicated `save_code_plan` tool to `data/plans/`, never through the project-sandboxed `write_file`
11. **Desktop Mascot** — every agent/task-loop/plain-chat turn updates one app-wide activity signal (`internal/app/activity.go`) → polled by the mascot's separate window (`mascot_main.dart`) and by the terminal REPL's spinner
12. **Live Mode v2** — audio in/out streams natively through `internal/livemode`'s Google Live / OpenAI Realtime session, refreshing memory context mid-conversation, with the mascot animating along
