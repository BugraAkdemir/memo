# Architecture — Memo v3.1.1

## Overview

Memo is a **two-process, local-first application**. The Go backend and Flutter frontend run as separate processes communicating over plain HTTP (localhost:8090) via REST + SSE streaming.

## Process Architecture

```
Flutter Desktop (Linux/Windows)     Flutter Mobile (Android/iOS)
         │                                    │
         │  REST + SSE (:8090)                 │  LAN / ngrok tunnel
         └──────────────┬─────────────────────┘
                        │
              ┌─────────┴──────────────────────────────────┐
              │            Go Backend (29 packages)         │
              │                                            │
              │  ┌──────────┐  ┌────────┐  ┌───────────┐  │
              │  │Web Server│  │  App   │  │ Proactive │  │
              │  │~90 routes│  │ Engine │  │  Engine   │  │
              │  │SSE stream│  │(25 files)│  │Observer→  │  │
              │  └──────────┘  └───┬────┘  │Analyzer→Act│  │
              │                    │        └───────────┘  │
              │  ┌────────┐┌──────┴──────┐┌─────────────┐  │
              │  │ Memory ││  Providers  ││    Agent    │  │
              │  │SQLite+ ││  8 types    ││  Pipeline   │  │
              │  │vec0    ││  Router     ││  8 tools    │  │
              │  └────────┘└─────────────┘└─────────────┘  │
              │                                            │
              │  Llama · WhatsApp · Calendar · Orchestra    │
              │  CloudSync · Whisper · Skills · Mood        │
              │  ModelStore · Intent · ngrok · Tunnel       │
              │  Sessions · Config · Truncate · Logx        │
              └────────────────────────────────────────────┘
```

## Module Map (29 packages)

| Directory | Responsibility |
|-----------|---------------|
| `internal/app/` | Central orchestrator (25 files) |
| `internal/webserver/` | REST API (~90 endpoints), SSE streaming |
| `internal/memory/` | Vector store — SQLite + sqlite-vec, embedder |
| `internal/provider/` | External LLM providers — 8 types, router, fallback |
| `internal/agent/` | Agent pipeline, sandbox, permissions, 8 tools |
| `internal/orchestra/` | Multi-model conductor, 8 roles, parallel execution |
| `internal/llama/` | llama.cpp subprocess lifecycle, GPU detection |
| `internal/whatsapp/` | WhatsApp bridge — whatsmeow client + store |
| `internal/calendar/` | Event store, reminder loop, intent bridge |
| `internal/cloudsync/` | Google Drive E2E encrypted backup |
| `internal/modelstore/` | HuggingFace model search and download |
| `internal/sessions/` | Chat session JSON persistence |
| `internal/config/` | YAML configuration management |
| `internal/database/` | SQLite connection + vec0 extension registration |
| `internal/api/` | OpenAI-compatible API client + SSE streaming |
| `internal/identity/` | System prompt, persona, incognito prompt |
| `internal/intent/` | Intent extraction pipeline (chat → calendar events) |
| `internal/proactive/` | Proactive suggestion engine |
| `internal/observer/` | Usage pattern analyzer (circular statistics) |
| `internal/skill/` | Skill system — load, manage, inject instructions |
| `internal/mood/` | Stochastic emotion engine + self-interest protocol |
| `internal/whisper/` | Speech-to-text via whisper.cpp |
| `internal/ngrok/` | ngrok tunnel manager (auto-restart on crash) |
| `internal/tunnel/` | Tailscale embedded tunnel (tsnet) |
| `internal/truncate/` | Token-aware context truncation |
| `internal/logx/` | Structured logging (slog wrapper with levels) |
| `internal/websearch/` | DuckDuckGo HTML scraping |

## Data Flow

1. **Chat** — User → Flutter → POST /api/send/stream → App.buildMessages() → LLM → SSE stream → Flutter render
2. **Memory** — User + assistant messages → embed → SQLite vec0 → retrieve on next query → inject into system prompt
3. **Agent** — User request → Agent Pipeline → LLM tool call → Permission dialog → Tool execution → Result feedback → Loop
4. **Proactive** — Observer records timestamps → Analyzer detects patterns → Chief LLM decides action → Notify/Suggest/Auto-execute
5. **Calendar** — Message text → Keyword filter → LLM intent extraction → Store event → Reminder loop fires notification
