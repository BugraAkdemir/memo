---
tags: project, ai, go, flutter, memory, rag, llm
status: Active
version: 3.3.3 (3.3.4 in development)
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
| **Learning** | Background observer tracks *when* you work (not *what*), learns rhythms, and proactively nudges you about patterns — on by default, fully controllable per sub-feature. |
| **Action** | Agent tool system: read/write files, run commands, search web, send WhatsApp messages, plus any skill that declares its own `command:` tool. All sandboxed, permission-gated. |
| **Automation** | Routines let you schedule a prompt or full agent run in plain language, on desktop or mobile, firing in your own device's timezone. |
| **Multi-model** | Orchestra mode decomposes tasks across multiple LLMs (Claude for reasoning, Gemini for speed, local for code). |
| **Voice** | Live Mode (beta) is a hands-free, spoken back-and-forth with Memo — local transcription + local Piper TTS by default, one-directional barge-in. |
| **Offline** | Works 100% offline with bundled llama.cpp. External providers, including Claude Code/Codex CLI (beta) as chat providers, are optional. |
| **Developer-friendly** | Sidebar → Developer exposes an Anthropic-compatible local API gateway, so tools like Claude Code can run against your own local model or API keys. |
| **Private** | No telemetry, no analytics, no cloud dependency. API keys and the `.memo` backup are encrypted; remote access requires a token. |

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
| `internal/skill/` | Skill system (plugin-like); skill `command:` tools now execute through the same agent pipeline/permission UI | `manager.go`, `loader.go`, `types.go` |
| `internal/logx/` | Structured logging (slog wrapper); also home to `Recover`/`GoRecover` — the backend-wide panic-recovery helper every background goroutine now uses | `logx.go` |
| `internal/truncate/` | Token-aware context truncation | `tokens.go` |
| `internal/mood/` | Mood engine & self-interest protocol | `scorer.go` |
| `internal/routine/` | Scheduled automations ("Routines") — desktop + mobile, own timezone offset | `loop.go`, `extractor.go`, `store.go`, `types.go` |
| `internal/agentcli/` | Claude Code / Codex CLI as chat providers (beta) | `claude_code.go`, `codex.go`, `commands.go`, `models.go` |
| `internal/anthropicapi/` | Developer API Gateway — local Anthropic-compatible endpoint (works with Claude Code) | `anthropicapi.go` |
| `internal/tts/` | Live Mode text-to-speech: local Piper by default, optional external OpenAI TTS | `tts.go`, `router.go`, `openai.go`, `filler.go`, `voice_store.go` |
| `internal/swarm/` | Memo Swarm (beta) — pools several PCs' compute for one oversized local model | `room.go`, `worker.go` |
| `internal/stats/` | Usage Stats (Settings → Stats) — per-turn token/request recording | `store.go` |
| `internal/taskloop/` | Worker/review-chief engine backing autonomous multi-step task lists | `engine.go`, `store.go` |
| `internal/shutdown/` | Cross-platform graceful-shutdown signaling (`main()` selects on it; fixes a Windows no-op in the old signal-based approach) | `shutdown.go` |
| `internal/gguf/` | GGUF file metadata parsing (real max context, tool-calling detection) | `gguf.go` |
| `internal/jsonutil/`, `internal/browseropen/` | Shared JSON helpers; cross-platform "open URL in browser" helper | `jsonutil.go`, `browseropen.go` |

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
- 19 built-in tools (`internal/agent/tools.go`): file I/O (read/write/edit/delete/insert-line/delete-lines/list/search/get-info), `run_command`, `web_search`, `self_clone`, `configure_provider`, calendar, WhatsApp (send/search/latest/messages), `read_env` — plus any skill that declares its own `command:` tool, wired into the same pipeline
- Permission system: Safe/Medium/Dangerous per tool, session/user policies
- Execution sandbox: path validation, symlink protection (incl. a fixed sandbox-escape gap), hardened dangerous-command blacklist
- 120s timeout per tool call (60s fallback if no deadline is set), max 20 iterations, cancel support
- Toggle now lives directly in Chat's top bar (no longer a separate Agent-only screen)
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
- HuggingFace model discovery, OR-combined multi-select filters (Tools/Vision/Code/Embedding/Size)
- Hardware-fit badges (GPU/CPU/too large) with hover tooltips explaining what they mean
- One-click download with auto GPU offloading; several downloads can run in parallel
- Quantization quality labels in plain language; real max-context read from the GGUF file itself (slider can't exceed it)

### 5.10 Routines (Scheduled Automations)
- Describe a task and a schedule in plain language; runs as a simple prompt or a full tool-using agent job
- Desktop and mobile (mobile delivers real pre-scheduled local notifications)
- Fires in the device's own timezone, resynced on every (re)connect

### 5.11 Self-Insight (`/insight`)
- On demand or on a weekly Routine, Memo reviews recent mood/memory history for a real pattern — explicitly told not to invent one if there isn't enough signal

### 5.12 Live Mode (beta)
- Hands-free spoken conversation via a small icon next to the chat input box (not a sidebar tab)
- Local on-device transcription; replies spoken with local Piper TTS by default, optional external OpenAI TTS
- One-directional barge-in, bundled VAD model; no echo cancellation yet (known limitation)

### 5.13 Claude Code / Codex CLI as Chat Providers (beta)
- Per-chat provider that shells out to a locally installed `claude`/`codex` CLI as a real background agent job, no fixed timeout
- The CLI's own slash commands resolve in Memo's `/` popup; no memory/identity context is injected

### 5.14 Developer API Gateway (Sidebar → Developer)
- Local Anthropic-compatible endpoint (`internal/anthropicapi/`) for tools like Claude Code (`ANTHROPIC_BASE_URL`)
- Model selection via `type/model-id`; full tool calling for openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go providers

### 5.15 Memo Swarm (beta)
- Pools several PCs' compute for one local model too large for a single machine's VRAM/RAM (Host + Join via llama.cpp `rpc-server`)
- Not available on macOS yet

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
| `data/routines/*.json` | One file per Routine (schedule, prompt/agent config) |
| `data/usage.db` | Usage Stats — per-turn request/token records (SQLite) |
| `data/tts/voices/` | Downloaded local Piper voice files (`.onnx` + `.onnx.json`) |
| `.env` | Optional environment overrides (OAuth creds, API keys) |
| `binaries/` | Platform-specific binaries (llama-server, vec0 extension, whisper-server) |

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
./scripts/build_releases.sh                          # dist packages (tar.gz, AppImage, deb)
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
| Agent Sandbox | Path validation, symlink protection, hardened dangerous-command blacklist (incl. a symlink-escape fix) |
| WhatsApp | Mutex-protected init, serialized message writes |
| Remote Access | Four selectable auth modes (none/token/password/token+password); per-device tokens (hashed at rest, shown once); argon2id password hashing; brute-force lockout on password login; short-lived signed session tokens |
| `.memo` Backup | Full export now includes calendar/habits/routines/tasks/permissions/skills and `machine.key` (previously incomplete) |

---

## 11. Known Issues & Technical Debt

**Current status: 0 open bugs at every severity** — see [`BUG_REPORT.md`](../BUG_REPORT.md) (repo root), the actively-maintained tracker. Older, larger audits from earlier in the v3.1.x line live in [`docs/KNOWN_ISSUES.md`](KNOWN_ISSUES.md) (frozen 2026-07-04 snapshot; several of its "open" items — e.g. mobile API client coverage, `a.client`/`providerRouter` reassignment — have since been re-verified as false positives or fixed) and [`docs/RESOLVED_ISSUES.md`](RESOLVED_ISSUES.md) (frozen v3.0.0 snapshot).

Two items previously listed here as open are resolved:
- `model_store_screen.dart` was split into a `settings/tabs/`-style module set (5-phase refactor).
- Mobile API client parity: 111 of the backend's 118 endpoints now exist on mobile; the remaining 7 are CLI-management/client-registry endpoints mobile has no use for.

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
| Self-Hosting | `docs/SELF_HOSTED.md` | Running just the backend on a Pi/home server/VPS: install paths, auth modes, CLI management |
| Troubleshooting | `docs/TROUBLESHOOTING.md` | Common issues & fixes |
| Contributing | `docs/CONTRIBUTING.md` | Contribution guidelines |
| Known Issues | `docs/KNOWN_ISSUES.md` | Known bugs & limitations |
| Release Notes | `docs/RELEASE_NOTES.md` | Version changelog |
| CGO Flags | `docs/CGO_FLAGS.md` | Build configuration |
| Learning System | `docs/learning-system/README.md` | Observer & proactive engine |
| Obsidian Docs | `obsidian-doc/Memo/` | Full documentation set (20+ files) |
| Bug Report | `BUG_REPORT.md` | Actively-maintained open-bug tracker (0 open as of this writing) |

---

*Last updated: 2026-08-05 · Version: v3.3.3 (open beta) · v3.3.4 in development*
