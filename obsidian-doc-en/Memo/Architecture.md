# Architecture — Memo v3.3.4 (in development; last released v3.3.3)

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
              │  │160+ routes│ │ Engine │  │  Engine   │  │
              │  │SSE stream│  │(25+ files)│ │Observer→  │  │
              │  └──────────┘  └───┬────┘  │Analyzer→Act│  │
              │                    │        └───────────┘  │
              │  ┌────────┐┌──────┴──────┐┌─────────────┐  │
              │  │ Memory ││  Providers  ││    Agent    │  │
              │  │SQLite+ ││ 13 types    ││  Pipeline   │  │
              │  │vec0    ││  Router     ││  19 tools   │  │
              │  └────────┘└─────────────┘└─────────────┘  │
              │                                            │
              │  Llama · WhatsApp · Calendar · Orchestra    │
              │  CloudSync · Whisper · Skills · Mood · TTS  │
              │  ModelStore · Intent · ngrok · Tunnel       │
              │  Routine · Stats · Swarm · AgentCLI         │
              │  AnthropicAPI (Dev Gateway) · GGUF · Models │
              │  Sessions · Config · Truncate · Logx        │
              └────────────────────────────────────────────┘
```

## Module Map (41 packages)

| Directory | Responsibility |
|-----------|---------------|
| `internal/app/` | Central orchestrator |
| `internal/webserver/` | REST API (160+ routes), SSE streaming |
| `internal/memory/` | Vector store — SQLite + sqlite-vec + FTS5 hybrid search, embedder |
| `internal/provider/` | External LLM providers — 13 types, router, fallback |
| `internal/agent/` | Agent pipeline, sandbox, permissions, 19 built-in tools |
| `internal/agentcli/` | Claude Code CLI / Codex CLI as chat providers (Beta, v3.3.4) — subprocess-based, registers into `provider` via `RegisterConstructor` |
| `internal/anthropicapi/` | Developer API Gateway wire-format translation (Anthropic ⇄ internal) |
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
| `internal/taskloop/` | Task list subsystem |
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
7. **Developer Gateway** — External tool (e.g. Claude Code via `ANTHROPIC_BASE_URL`) → `POST /v1/messages` → `internal/anthropicapi` translates to Memo's internal format → routes to local model or a configured provider → translates the response back to Anthropic's shape
8. **CLI providers** — Chat with Claude Code/Codex CLI selected → Memo shells out to the installed CLI as a subprocess tied to the chat, independent of the app's global stream lock, survives switching chats/closing the window
