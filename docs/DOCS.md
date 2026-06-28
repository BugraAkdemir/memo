---
tags: project, ai, go, flutter, memory, rag, llm
status: Active
version: 3.1.0-beta
tech_stack: [Go 1.26, Flutter 3.10, SQLite, vec0, llama.cpp, whatsmeow]
category: [[AI_Agents]]
---

# Memo — Documentation Index

> **The local AI that remembers everything — and acts before you ask.**

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, E2E-encrypted cloud sync, multi-model orchestration, and an agent tool system. Designed for offline desktop use with optional API fallback.

---

## Table of Contents

- [1. What is Memo?](#1-what-is-memo)
- [2. Tech Stack](#2-tech-stack)
- [3. Architecture Overview](#3-architecture-overview)
- [4. Module Map](#4-module-map)
- [5. Core Features](#5-core-features)
- [6. LLM Routing](#6-llm-routing)
- [7. Data Layout](#7-data-layout)
- [8. Configuration](#8-configuration)
- [9. Development](#9-development)
- [10. Security Model](#10-security-model)
- [11. Known Issues & Technical Debt](#11-known-issues--technical-debt)
- [12. Related Documents](#12-related-documents)

---

## 1. What is Memo?

Memo is not just another chat UI. It is a full AI companion that runs entirely on your machine:

| Capability | Description |
|-----------|-------------|
| **Memory** | Every chat is embedded into a local vector database (SQLite + vec0). Remembers conversations for weeks. |
| **Learning** | Background observer tracks *when* you work (not *what*), learns rhythms, and anticipates your needs. |
| **Action** | 8 built-in agent tools: read/write files, run commands, search web, send WhatsApp messages. All sandboxed, permission-gated. |
| **Multi-model** | Orchestra mode decomposes tasks across multiple LLMs (Claude for reasoning, Gemini for speed, local for code). |
| **Offline** | Works 100% offline with bundled llama.cpp. External providers are optional fallback. |
| **Private** | No telemetry, no analytics, no cloud dependency. API keys encrypted with AES-256-GCM. |

---

## 2. Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | Go | 1.26 |
| Frontend | Flutter | 3.10+ |
| State Management | Riverpod | 2.4 |
| HTTP Client | Dio (SSE streaming) | 5.4 |
| Markdown Rendering | flutter_markdown | 0.6 |
| Vector Database | SQLite + sqlite-vec | — |
| SQLite Driver | mattn/go-sqlite3 | — |
| Local Inference | llama.cpp (bundled) | — |
| WhatsApp Bridge | whatsmeow | — |
| Voice Input | whisper.cpp (bundled) | — |
| License | AGPL-3.0 | — |

---

## 3. Architecture Overview

**Two-process decoupled design:** Go backend (headless REST API, port `:8090`) + Flutter desktop UI. Communication is plain HTTP/JSON + SSE streaming — no TLS (local only).

```
┌──────────────────────────────────┐    ┌───────────────────────┐
│  Flutter Desktop (Linux/Windows)  │    │  Flutter Mobile        │
│  Chat · Agent · Orchestra         │    │  Chat · Notifications  │
│  Settings · Model Store           │    │  Remote connect        │
└──────────────┬───────────────────┘    └───────────┬───────────┘
               │  REST + SSE (:8090)                │  LAN / ngrok
               └──────────────┬─────────────────────┘
┌──────────────────────────────┴─────────────────────────────────┐
│              Go Backend — 25 packages, ~90 endpoints             │
│  ┌─────────┐ ┌──────┐ ┌──────┐ ┌─────────┐ ┌──────┐ ┌───────┐ │
│  │ Memory  │ │Sess. │ │Llama │ │WhatsApp │ │Agent │ │Provide│ │
│  │ vec0    │ │JSON  │ │GPU   │ │whatsmeow│ │Pipe  │ │Router │ │
│  └─────────┘ └──────┘ └──────┘ └─────────┘ └──────┘ └───────┘ │
│  Orchestra · ModelStore · CloudSync · Calendar · Mood           │
│  ngrok · Tailscale · Whisper · Skills · Intent · Observer       │
└─────────────────────────────────────────────────────────────────┘
```

**Bridge Pattern:** `AppBridge` interface in `internal/webserver/bridge.go` decouples HTTP handlers from the `App` orchestrator. `FullBridge` extends it for Flutter-specific endpoints.

---

## 4. Module Map

| Directory | Responsibility | Key Files |
|-----------|---------------|-----------|
| `internal/app/` | Central orchestrator (25 files) | `app.go`, `llm.go`, `chat.go`, `learning.go`, `whatsapp.go` |
| `internal/webserver/` | REST API (~45 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go`, `installer.go`, `gpu.go` |
| `internal/memory/` | Vector store (SQLite + sqlite-vec) | `store.go`, `embedder.go` |
| `internal/database/` | SQLite connection management | `sqlite.go`, `vec_register.go` |
| `internal/provider/` | External LLM providers (8 types) | `provider.go`, `router.go`, `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go`, `llamacpp.go` |
| `internal/orchestra/` | Multi-model orchestration | `conductor.go`, `roles.go`, `types.go` |
| `internal/agent/` | Agent / tool execution sandbox | `executor.go`, `pipeline.go`, `sandbox.go`, `permissions.go`, `tools.go` |
| `internal/cloudsync/` | Google Drive E2E encrypted backup | `drive.go`, `crypto.go`, `sync_manager.go` |
| `internal/sessions/` | Chat session persistence | `sessions.go` |
| `internal/modelstore/` | HuggingFace model search/download | `modelstore.go` |
| `internal/identity/` | System prompt & persona | `identity.go`, `styles.go` |
| `internal/config/` | Configuration management | `config.go` |
| `internal/api/` | llama.cpp / OpenAI-compatible API client | `client.go`, `streaming.go`, `types.go` |
| `internal/calendar/` | Calendar event store + reminder loop | `store.go`, `reminder.go`, `event.go`, `bridge.go` |
| `internal/intent/` | Intent extraction pipeline | `extractor.go`, `filter.go`, `result.go`, `decider_factory.go` |
| `internal/proactive/` | Proactive suggestion engine | `engine.go`, `decision.go`, `matcher.go`, `pending.go`, `prompt.go`, `feedback.go` |
| `internal/observer/` | Usage pattern analyzer | `analyzer.go`, `recorder.go`, `store.go`, `pattern.go` |
| `internal/whatsapp/` | WhatsApp bridge (whatsmeow) | `client.go`, `store.go` |
| `internal/whisper/` | Speech-to-text (whisper.cpp) | `whisper.go`, platform-specific files |
| `internal/skill/` | Skill system (plugin-like) | `manager.go`, `loader.go`, `types.go` |
| `internal/logx/` | Structured logging (slog wrapper) | `logx.go` |
| `internal/truncate/` | Token-aware context truncation | `tokens.go` |
| `internal/mood/` | Mood engine & self-interest protocol | `scorer.go` |

---

## 5. Core Features

### 5.1 Chat System
- Streaming token-by-token responses with Markdown, code highlighting, tables
- File/image drop support (PDF, code, images)
- Slash-command palette (`/`)
- On-device voice input via whisper.cpp (TR/EN auto-detect)
- Web search integration (no API key required)
- Incognito mode (zero trace)

### 5.2 RAG Memory
- 768-dimension vector embeddings stored in SQLite with `sqlite-vec` ANN indexing
- Automatic embedding on every chat exchange
- Context injection: relevant past conversations pulled into prompt
- Importance decay and consolidation rules
- Export/import support

### 5.3 Agent Engine
- 8 built-in tools: read_file, write_file, edit_file, delete_file, list_directory, run_command, web_search, whatsapp_send
- Permission system: Safe/Medium/Dangerous per tool, session/user policies
- Execution sandbox: path validation, symlink protection, command blacklist (23 patterns)
- 60s timeout per tool, max 20 iterations, cancel support
- Audit trail (last 1000 entries)

### 5.4 Orchestra Mode
- Chief model decomposes tasks into expert roles (Planner, Frontend, Backend, Bug Fixer, Reviewer, Security, DevOps, Generalist)
- Parallel (independent) or sequential (dependency-based) execution
- Fallback chain: when a task's primary provider fails, other enabled providers are tried in priority order
- Progress streaming to frontend in real-time

### 5.5 WhatsApp Integration
- QR code pairing (same protocol as WhatsApp Web, no Business API fees)
- Read, search, reply to messages
- Agent can send messages, resolve contacts by name
- Messages feed into memory, observer, and calendar pipelines

### 5.6 Smart Calendar
- Two-stage intent extraction: regex time patterns → LLM structured extraction
- Automatic event creation from chat messages
- Desktop and mobile reminders
- Ambiguity events for vague time references

### 5.7 Proactive Learning Engine
- Background observer tracks activity patterns (topic + timestamps only, never message content)
- Circular (directional) statistics for rhythm detection
- Suggestions, notifications, or auto-agent at configurable confidence levels
- Opt-in, transparent, patterns that fade are forgotten

### 5.8 Cloud Sync
- E2E-encrypted Google Drive backup
- AES-256-GCM + PBKDF2 (600K iterations)
- WAL checkpoint before archive (data integrity)
- Encrypt *before* upload — Google can't read it

### 5.9 Model Store
- HuggingFace model discovery
- Hardware-fit badges (GPU/CPU/too large)
- One-click download with auto GPU offloading
- Quantization quality labels in plain language

---

## 6. LLM Routing

Priority order defined in `internal/app/llm.go` `callLLMStream()`:

1. **Orchestra mode** — multi-model workflow (if `orchestraConductor.Config().Enabled`)
2. **External provider** — `provider.Router` with fallback chain (if `activeProvider` is set)
3. **Local llama.cpp** — `api.Client` pointed at local `llama-server`

### Provider Fallback
- Router sorts providers by `Priority` (descending)
- On failure: auto-fallback to next provider
- After 3 consecutive failures: auto-disable
- Health check goroutine: periodic test + re-enable on recovery
- Orchestra tasks also have fallback: `tryFallbackProviders` tries other enabled providers

---

## 7. Data Layout

| Path | Purpose |
|------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API, learning, calendar) |
| `data/memory/` | SQLite + vec0 vector store |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `data/whatsapp/` | WhatsApp SQLite message store + whatsmeow session |
| `data/calendar/` | Calendar events SQLite DB |
| `data/permissions.json` | Agent tool permission policies |
| `data/machine.key` | Machine-specific encryption key (0600) |
| `.env` | Optional environment overrides (OAuth creds, API keys) |
| `binaries/` | Platform-specific binaries (llama-server, vec0 extension) |

---

## 8. Configuration

All configuration lives in `config/config.yaml`. Key sections:

```yaml
api:
  base_url: ""           # llama.cpp server URL
  embedding_model: ""    # embedding model name
  timeout_seconds: 300   # request timeout

memory:
  persist_dir: "data/memory"
  embedding_dimension: 768
  auto_embed: true

identity:
  user_name: ""
  assistant_name: "Memo"
  style: "default"
  system_role: ""

proactive:
  enabled: false
  level: "off"           # off | low | medium | high

calendar:
  enabled: false
  reminder_lead_minutes: 15
  disable_time_guess: false

sync:
  enabled: false
  passphrase: ""         # empty = machine-derived key
  interval_messages: 50

learning:
  enabled: false
```

---

## 9. Development

### Quick Start

```bash
# Terminal 1 — Backend
CGO_ENABLED=1 go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

### Build

```bash
go build -o memo .                                    # backend binary
cd frontend && flutter build linux --release         # frontend binary
./build_releases.sh                                  # dist packages (tar.gz, AppImage, deb)
```

### Testing

```bash
go test ./... -race -count=1                        # all backend tests
cd frontend && flutter test                         # all frontend tests
cd frontend && flutter analyze                      # lint
```

### CI/CD

GitHub Actions runs on every push/PR:
- Go: `go vet`, `go test -race`, `go build`
- Flutter: `flutter analyze`, `flutter test`

---

## 10. Security Model

| Area | Implementation |
|------|---------------|
| API Keys | AES-256-GCM encrypted, key from `data/machine.key` (crypto/rand) |
| Rate Limiting | Token-bucket per-IP (100 req/s), no X-Forwarded-For trust |
| File Upload | MIME detected from content (http.DetectContentType), not client header |
| Import Safety | filepath.Rel validation prevents path traversal |
| Request Limits | 50MB body limit via `limitBodyMiddleware` |
| Config Files | Written with `0600` permissions |
| Cloud Sync | E2E encrypted before upload, PBKDF2 600K iterations |
| Agent Sandbox | Path validation, symlink protection, 23-command blacklist |
| WhatsApp | Mutex-protected init, serialized message writes |

---

## 11. Known Issues & Technical Debt

### Remaining Issues
| Priority | Issue | Status |
|----------|-------|--------|
| HIGH | `model_store_screen.dart` (2469 lines) — needs split | Open |
| HIGH | Mobile API client missing most endpoints | Open |
| HIGH | Whisper GPU variant missing | Open |
| MED | `connectionStatusProvider` polling | Acceptable (autoDispose) |
| LOW | `skill.DangerLevel` / `agent.DangerLevel` separate types | Cosmetic |

### Fixed (2026-06-28)
- Memory store write lock (data corruption fix)
- Cloud backup WAL checkpoint (incomplete backup fix)
- WhatsApp mutex (double connection fix)
- Sessions mutex (init race fix)
- Orchestra conductor mutex (data race fix)
- MIME spoofing (content-based detection)
- Rate limit bypass (removed X-Forwarded-For trust)
- Path traversal (filepath.Rel validation)
- Goroutine lifecycle (WhatsApp uses lifecycleCtx)
- Logging migration (all packages → logx.Printf)
- Orchestra fallback chain (tryFallbackProviders)
- Provider priority UI (dialog field added)
- Flutter const constructors (116 auto-fixes)

---

## 12. Related Documents

| Document | Location | Description |
|----------|----------|-------------|
| README | `README.md` | Project overview, features, screenshots |
| README (TR) | `READmeTR.md` | Türkçe proje özeti |
| Architecture | `docs/architecture.md` | Architecture deep dive |
| API Reference | `docs/API_REFERENCE.md` | REST API endpoint documentation |
| Technical Deep Dive | `docs/TECHNICAL_DEEP_DIVE.md` | Engineering decisions |
| Features | `docs/FEATURES.md` | Feature catalog |
| Roadmap | `docs/ROADMAP.md` | Development roadmap |
| Troubleshooting | `docs/TROUBLESHOOTING.md` | Common issues & fixes |
| Contributing | `docs/CONTRIBUTING.md` | Contribution guidelines |
| Known Issues | `docs/KNOWN_ISSUES.md` | Known bugs & limitations |
| Release Notes | `docs/RELEASE_NOTES.md` | Version changelog |
| CGO Flags | `docs/CGO_FLAGS.md` | Build configuration |
| Learning System | `docs/learning-system/README.md` | Observer & proactive engine |
| Obsidian Docs | `obsidian-doc/Memo/` | Full documentation set (20+ files) |

---

*Last updated: 2026-06-28 · Version: v3.1.0-beta*
