---
tags: project, ai, go, flutter, memory, rag, llm, agent
status: Active
version: 4.5.0
tech_stack: [Go 1.26, Flutter 3.10, SQLite, vec0, llama.cpp, whatsmeow]
category: [[AI_Agents]]
---

# Memo — Documentation Index

> **The local AI that remembers everything — and acts before you ask.**

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, E2E-encrypted cloud sync, multi-model orchestration, an autonomous agent/task-loop system, native voice (Live Mode), and a desktop mascot that shows what it's doing. Designed for offline desktop use with optional API fallback.

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
| **Memory** | Every chat is embedded into a local vector database (SQLite + vec0 + FTS5, hybrid search). Remembers conversations indefinitely, with durable facts pinned outside normal retrieval ranking. |
| **Action** | Agent tool system: 27+ built-in tools (file I/O, `run_command`, web search/fetch, calendar, routines, WhatsApp/Telegram, provider self-config) plus any skill that declares its own `command:` tool. Sandboxed, permission-gated. |
| **Code Mode** | Three switchable gears for agentic coding — **Plan** (investigates and writes a plan, touches nothing), **Auto** (confirms edits), **Build** (runs edits/commands without waiting) — cycled with Ctrl+Tab. |
| **Self-Driving** | A `Task.md` checklist becomes an unattended, multi-step run: planner/executor mode, plan approval, up to 3 parallel sub-agents (coder + analyzer/reviewer/test-runner), escalating retry, live in-chat activity. |
| **Automation** | Routines schedule a prompt or full agent run in plain language, on desktop or mobile, firing in your own device's timezone. |
| **Multi-model** | Orchestra mode decomposes tasks across multiple LLMs (Claude for reasoning, Gemini for speed, local for code). |
| **Voice** | Live Mode v2: native audio-to-audio conversation (Google Live / OpenAI Realtime), barge-in, delegate/standalone modes, plus a local Piper/whisper.cpp fallback path. |
| **Presence** | A small animated desktop mascot (two selectable skins) shows Memo's live activity — thinking, writing, running a tool — with a plain-language status bubble, independent of the chat window. |
| **Learning** | Background observer tracks *when* you work (not *what*), learns rhythms, and proactively nudges you about patterns — on by default, fully controllable per sub-feature. |
| **Offline** | Works 100% offline with bundled llama.cpp. External providers, including Claude Code/Codex CLI and a Google-account "gemini-sub" (Beta) as chat providers, are optional. |
| **Developer-friendly** | Sidebar → Developer exposes both an Anthropic-compatible and an OpenAI-compatible local API gateway, so tools like Claude Code can run against your own local model or API keys. |
| **Private** | No telemetry, no analytics, no cloud dependency. API keys and the `.memo` backup are encrypted; remote access requires a token, and every gateway endpoint now enforces it. |

---

## 2. Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | Go | 1.26 |
| Frontend | Flutter | 3.10+ |
| State Management | Riverpod | 2.4 |
| HTTP Client | Dio (SSE streaming) | 5.4 |
| Markdown Rendering | flutter_markdown | 0.6 |
| Vector Database | SQLite + sqlite-vec + FTS5 | — |
| SQLite Driver | mattn/go-sqlite3 | — |
| Local Inference | llama.cpp (bundled) | — |
| WhatsApp Bridge | whatsmeow | — |
| Voice Input | whisper.cpp (bundled) | — |
| Local TTS | Piper (bundled) | — |
| License | AGPL-3.0 | — |

---

## 3. Architecture Overview

**Two-process decoupled design:** Go backend (headless REST API, port `:8090`) + Flutter desktop UI, plus an optional third window (the mascot) sharing the same process/state. Communication is plain HTTP/JSON + SSE streaming — no TLS on localhost.

```
┌────────────────────────────────────────────────────────┐    ┌──────────────┐
│  Flutter client (Linux/Win/Mac · Android/iOS · web)     │    │  Mascot      │
│  Chat · Agent · Code Mode · Calendar · Routines         │    │  window      │
│  Orchestra · Settings · Models                          │    │  (same proc) │
└──────────────────────────┬─────────────────────────────┘    └──────┬───────┘
          REST + SSE (:8090 local, or LAN / ngrok / Tailscale)       │
                           └─────────────────────────────────────────┘
┌──────────────────────────────┴─────────────────────────────────────────────────┐
│                 Go Backend — 40+ packages, 180+ endpoints                        │
│  Memory(vec0+FTS5) · Sessions · Llama · WhatsApp/Telegram · Agent · TaskLoop     │
│  Provider Router · Orchestra · ModelStore · CloudSync · Calendar · Routine      │
│  LiveMode(v2) · TTS/STT · Swarm · Stats · ngrok · Tailscale · Skills · Observer  │
└───────────────────────────────────────────────────────────────────────────────────┘
```

**Bridge Pattern:** `AppBridge` interface in `internal/webserver/bridge.go` decouples HTTP handlers from the `App` orchestrator. `FullBridge` extends it for Flutter-specific endpoints.

---

## 4. Module Map

See [`PROJECT_MAP.md`](PROJECT_MAP.md) for a file-by-file breakdown. Package-level summary:

| Directory | Responsibility |
|-----------|---------------|
| `internal/app/` | Central orchestrator (40+ files) — chat/LLM routing, agent bridge, Live Mode, Telegram/WhatsApp, task-loop bridge, Code Mode sub-mode state |
| `internal/webserver/` | REST API (180+ endpoints) — `server.go`, `handlers_flutter.go`, `handlers_tasks.go`, `bridge.go` |
| `internal/agent/` + `internal/agent/tools/` | Agent/tool execution sandbox — 27+ built-in tools, permissions, danger levels |
| `internal/taskloop/` | Self-Driving task loop: `Task.md` schema, planner/executor engine, sub-agent orchestration, retry/escalation |
| `internal/provider/` | External LLM providers (15 `ProviderType` values, incl. Custom Anthropic-compatible, OpenCode Zen/Go, Kilo Code) |
| `internal/agentcli/` | Claude Code / Codex CLI as chat providers (Beta) |
| `internal/geminisub/` | "gemini-sub" — sign in with a personal Google account, reach Gemini via Code Assist on your own quota (Beta) |
| `internal/anthropicapi/`, `internal/openaiapi/` | Developer API Gateway — local Anthropic- and OpenAI-compatible endpoints, both key-enforced |
| `internal/livemode/` | Live Mode v2 engine: Google Live / OpenAI Realtime session management, reconnect, transcript, delegate mode |
| `internal/tts/`, `internal/stt/` | Local Piper TTS + whisper.cpp STT, plus optional external TTS/STT (OpenAI, ElevenLabs, custom) |
| `internal/memory/`, `internal/database/` | Vector + FTS5 store (SQLite + sqlite-vec), serialized single-writer connection |
| `internal/orchestra/` | Multi-model orchestration (chief + expert roles) |
| `internal/whatsapp/`, `internal/telegram/` | Messaging bridges (whatsmeow / Bot API), each with its own isolated SQLite store |
| `internal/routine/` | Scheduled automations ("Routines") — desktop + mobile |
| `internal/calendar/`, `internal/intent/`, `internal/proactive/`, `internal/observer/`, `internal/mood/` | Smart calendar, intent extraction, proactive nudges, usage-pattern learning, mood engine |
| `internal/cloudsync/` | Google Drive E2E encrypted backup |
| `internal/modelstore/`, `internal/gguf/` | HuggingFace model search/download, GGUF metadata parsing |
| `internal/swarm/` | Memo Swarm (Beta) — pools several machines' compute via llama.cpp `rpc-server` |
| `internal/remoteauth/` | Token/password auth, per-device tokens, brute-force lockout, JWT session tokens |
| `internal/ngrok/`, `internal/tunnel/` | ngrok tunnel management, embedded Tailscale (tsnet) |
| `internal/browserengine/`, `internal/websearch/` | Optional headless-browser rendering for JS-heavy pages, DuckDuckGo scraping |
| `internal/skill/`, `internal/skills/` | Plugin-like skill system; `go:embed`ded built-ins |
| `internal/stats/` | Usage Stats (Settings → Stats) |
| `internal/config/`, `internal/identity/`, `internal/sessions/` | Config management, persona/system-prompt, chat session persistence |
| Frontend: `frontend/lib/mascot_main.dart` | Separate entry point for the desktop mascot window (same process, `desktop_multi_window`) |

---

## 5. Core Features

See [`FEATURES.md`](FEATURES.md) for the full catalog. Headlines as of v4.5.0:

- **Desktop Mascot** — an always-on-top animated character (two skins) reflecting Memo's live activity across every channel (chat, WhatsApp, Telegram, task loop), with idle flourishes and a plain-language status bubble.
- **Code Mode: Plan / Auto / Build** — three cycled sub-modes for agentic coding, each with its own editable system prompt; Plan writes a saved plan and asks in-chat whether to proceed; auto-permission skips the question and chains straight into Build.
- **Self-Driving Task Loop** — unattended multi-step execution from a `Task.md` checklist, with planner/executor mode, sub-agent parallelism, and resilient retry/escalation.
- **Live Mode v2** — native audio-to-audio voice (Google Live / OpenAI Realtime), barge-in, mid-session memory refresh, clearer failure messages.
- **RAG Memory** — hybrid vector + FTS5 search (RRF-merged), pinned durable facts, compound-question splitting.
- **Agent Engine** — 27+ built-in tools, skill `command:` tools on the same pipeline, Safe/Medium/Dangerous permissioning.
- **Orchestra Mode** — chief + expert-role multi-model decomposition with fallback chains.
- **WhatsApp & Telegram** — self-chat assistant surfaces (chat, memory, agent tools, routines) on both, each isolated.
- **Developer API Gateway** — Anthropic- and OpenAI-compatible local endpoints, both now key-enforced for non-loopback callers.
- **Remote Access & Self-Hosting** — token/password/token+password auth, per-device tokens, multi-account with 7 granular permissions, native or Docker/CasaOS self-hosted server.
- **Memo Swarm (Beta)** — pool several PCs' compute for one oversized local model.

---

## 6. LLM Routing

Priority order defined in `internal/app/llm.go` `callLLMStream()`:

1. **Orchestra mode** — multi-model workflow (if `orchestraConductor.Config().Enabled`)
2. **External provider** — `provider.Router` with fallback chain (if `activeProvider` is set)
3. **Local llama.cpp** — `api.Client` pointed at local `llama-server`

Agent mode (including Code Mode's Auto/Build sub-modes and the Self-Driving task loop) overrides this via `callAgentStream`, which runs the same routing internally per tool-calling turn.

### Provider Fallback
- Router sorts providers by `Priority` (descending)
- On failure: auto-fallback to next provider
- After 3 consecutive failures: auto-disable
- Health check goroutine: periodic test + re-enable on recovery
- Orchestra tasks also have fallback: `tryFallbackProviders` tries other enabled providers
- The Self-Driving task loop can be pinned to one provider (`# sağlayıcı: sabit`, default) or opt into roaming (`# sağlayıcı: otomatik`), with its own escalating transient-fault retry independent of the router's

---

## 7. Data Layout

| Path | Purpose |
|------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API, learning, calendar) |
| `data/memory/` | SQLite + vec0 + FTS5 vector/keyword store |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `data/whatsapp/`, `data/telegram/` | Isolated SQLite message stores per messaging bridge |
| `data/calendar/` | Calendar events SQLite DB |
| `data/permissions.json` | Agent tool permission policies |
| `data/machine.key` | Machine-specific encryption key (0600) |
| `data/routines/*.json` | One file per Routine |
| `data/tasklists/` | Self-Driving `Task.md`/`Plan.md` state per task list |
| `data/plans/<project>/plan.md` | Code Mode Plan sub-mode output (sandboxed outside the project directory) |
| `data/usage.db` | Usage Stats — per-turn request/token records (SQLite) |
| `data/tts/voices/` | Downloaded local Piper voice files |
| `data/skills/` | Materialized built-in + imported skills |
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
# Interactive terminal chat (starts the backend if needed, opens a REPL)
go run -tags "sqlite_fts5" .

# Headless backend only, to pair with the Flutter frontend
go run -tags "sqlite_fts5" . --headless --port 8090
cd frontend && flutter run -d linux
```

`-tags "sqlite_fts5"` is required, not optional — see [CGO_FLAGS.md](CGO_FLAGS.md). Without it, FTS5 silently degrades to vector-only search with no visible error.

### Build

```bash
go build -tags "sqlite_fts5" -o memo .              # backend binary
cd frontend && flutter build linux --release       # frontend binary
./build_releases.sh                                # dist packages (tar.gz, AppImage, deb)
```

### Testing

```bash
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race   # all backend tests
cd frontend && flutter test                             # all frontend tests
cd frontend && flutter analyze                           # lint
```

A real end-to-end layer (`internal/e2e/`) drives an actual `app.App` behind a real HTTP server against a scripted fake provider — real SSE streaming, real agent permission round-trips, real Self-Driving task-loop state transitions.

### CI/CD

GitHub Actions runs on every push/PR:
- Go: `go vet`, `go test -race`, `go build` (all with `-tags "sqlite_fts5"`)
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
| Agent Sandbox | Path validation, symlink protection, hardened dangerous-command blacklist |
| Developer Gateway | Both the Anthropic-compatible (`/v1/messages`) and OpenAI-compatible (`/v1/models`, `/v1/chat/completions`) endpoints now enforce the API key for any non-loopback caller |
| Skills | An imported skill (auto-picked-up from another tool's skill folder) no longer auto-activates — it's discovered but stays off until explicitly enabled |
| WhatsApp / Telegram | Mutex-protected init, serialized message writes, owner-lock on Telegram (first sender wins, everyone else silently ignored) |
| Remote Access | Four selectable auth modes (none/token/password/token+password); per-device tokens (hashed at rest, shown once); argon2id password hashing; brute-force lockout; short-lived signed session tokens |
| `.memo` Backup | Full export includes calendar/habits/routines/tasks/permissions/skills and `machine.key` |

---

## 11. Known Issues & Technical Debt

Open bugs are tracked in the repo's actively-maintained `BUG_REPORT.md`. [`KNOWN_ISSUES.md`](KNOWN_ISSUES.md) and [`RESOLVED_ISSUES.md`](RESOLVED_ISSUES.md) are intentionally frozen historical snapshots from earlier in the v3.x line — kept for reference, not maintained further; do not treat their contents as current status.

As of this writing, open items worth tracking live in `docs/ROADMAP.md`'s "Near-term" section (Self-Driving task-loop plan-approval UX, live status reporting for a running task, escalation step-count display) and the local-model + Plan-mode + auto-permission chaining path, which is code-reviewed but not yet live-verified against a real local model.

---

## 12. Related Documents

| Document | Location | Description |
|----------|----------|--------------|
| README | `README.md` | Project overview, features, screenshots |
| README (TR) | `READmeTR.md` | Türkçe proje özeti |
| Architecture | `docs/architecture.md` | Architecture deep dive |
| Project Map | `docs/PROJECT_MAP.md` | File-by-file reference tree |
| API Reference | `docs/API_REFERENCE.md` | REST API endpoint documentation |
| Technical Deep Dive | `docs/TECHNICAL_DEEP_DIVE.md` | Engineering decisions |
| Features | `docs/FEATURES.md` | Feature catalog |
| Self-Hosting | `docs/SELF_HOSTED.md` | Running just the backend on a Pi/home server/VPS |
| Troubleshooting | `docs/TROUBLESHOOTING.md` | Common issues & fixes |
| Contributing | `docs/CONTRIBUTING.md` | Contribution guidelines |
| Release Notes | `versinNote/` | Per-version changelog (`v4.5.0.md` is current) |
| CGO Flags | `docs/CGO_FLAGS.md` | Build configuration |
| Turkish docs | `docs/tr/` | Turkish mirrors of the above |
| Obsidian wiki (EN) | `obsidian-doc-en/Memo/` | Full browsable documentation set |
| Obsidian wiki (TR) | `obsidian-doc/Memo/` | Türkçe belge seti |
| Bug Report | `BUG_REPORT.md` | Actively-maintained open-bug tracker |

---

*Last updated: 2026-09-15 · Version: v4.5.0*
