# Memo v3.3.4 (in development)

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Zero cloud dependency · Full offline capable

> The last **released** version is **v3.3.3** (open beta, July 23, 2026). **v3.3.4 is in active development** (not yet released) — a reliability and polish pass: the backend no longer crashes from a background error, a 4-5x local-generation slowdown from having memory on is fixed, a new (beta) **Live Mode** for hands-free voice conversation, **Tailscale remote access graduates out of Beta**, **Claude Code CLI / Codex CLI as chat providers** (beta), Settings reorganized into a searchable rail, and a batch of real bug fixes (agent mode on small-context local models, "Delete All Data" on Windows, speech-to-text from an installed CLI, web search firing on every message). Changelog: `versinNote/v3.3.4.md`.
>
> v3.3.3 itself was a stability/trust release: terminal CLI reliability fixes, the CLI and desktop app running independently of each other, memory recall fixes, Memo finally giving a real, grounded answer about who made it, **Routines** (scheduled automations) and **Proactive Learning/ambient nudges**, the [[Features Catalog|Usage Stats]] tab, a fully complete `.memo` backup (including `machine.key`), and the [[Developer API Gateway]]. Changelog: `versinNote/v3.3.3.md`.

---

## v3.3.4 Highlights (in development)

- **Backend-wide panic recovery** — a crash in any background task (memory, routines, WhatsApp, cloud sync, STT, notifications, tunnels, ...) is now logged and contained instead of taking the whole app down
- **4-5x local-generation slowdown from memory being on — fixed** — the embedding server no longer fights the chat model for VRAM (defaults to CPU-only), and the memory-block token budget is capped at 4096
- **[[Multimodal Capabilities (Vision and Voice)|Live Mode]] (Beta)** — hands-free voice conversation via a small icon next to the chat input; local Piper TTS by default, offline voice picker, one-directional barge-in, bundled VAD model; no echo cancellation yet
- **Claude Code CLI / Codex CLI as chat providers (Beta)** — see [[External Providers]] — per-chat, runs as a real background agent job, own slash commands
- **Remote Access (Tailscale) graduates out of Beta** — one-click login, Funnel on by default, auto-reconnect
- **Settings reorganized into a searchable rail** — no longer 20 flat tabs
- **`@` file-mention** in chat, a quick model-switcher pill in the chat top bar
- Fixed: agent mode failing on short messages with small-context local models (default local context 4096 → 8192), "Delete All Data" failing on Windows, STT not finding bundled files from an installed CLI, web search firing on every message, Windows installer now bundles the VC++ Redistributable
- **macOS App Sandbox fix** (`420e6a5`) — missing `network.client`/`device.audio-input`/`files.user-selected.read-write` entitlements were causing a real "connection error" on macOS; see [[Troubleshooting]] and [[Build and Packaging]]

Full list: `versinNote/v3.3.4.md` (running draft, not final)

## v3.3.3 Highlights (last released)

- **[[Proactive Learning and Calendar|Routines]]** — schedule something for Memo to do on its own, in plain language, on desktop and mobile
- **Proactive Learning & ambient nudges** — Memo notices patterns and gently brings them up, with a real suggestion banner (Yes / Not now / Stop asking)
- **Self-Insight (`/insight`)** — ask Memo to describe patterns in your mood/memory history
- **CLI reliability fixes** — model download no longer hangs, multi-line paste works correctly, the terminal no longer stays broken, plus a full visual redesign
- **CLI/desktop independence** — closing the CLI no longer takes down the desktop app's backend
- **Memory recall fixes** — keyword search is genuinely active now, multi-topic questions no longer return incomplete answers, `/remember` actually saves again
- **Self-identity** — Memo now gives a real answer when asked who made it and why
- **Minimal Mode**, **two new providers (OpenCode Zen/Go)**, **memory import from another AI**, **skill tools that actually run**
- **[[Features Catalog|Usage Stats]]** — Settings → Stats: token/speed/model breakdown chart
- **Complete `.memo` backup** — calendar, routines, permissions, skills, and (most critically) `machine.key` are now included
- **[[Developer API Gateway]]** — point Claude Code (`ANTHROPIC_BASE_URL`) or any OpenAI-compatible tool at Memo, with full agentic tool-calling support
- **[[Memo Swarm]] (Beta)** — pool several PCs to run a model that will not fit on one machine (sidebar → Swarm; Settings → Beta Features)

Full list: `versinNote/v3.3.3.md`

---

## Quick Links

- [[Architecture]] — Package map and module responsibilities
- [[System Overview]] — How all subsystems fit together
- [[Known Issues]] — Current state of known problems
- Release notes: `versinNote/v3.3.3.md`
- [[v3.1.1 Features]] — v3.1.1's full feature catalog (historical record)
- [[Features Catalog]] — Current feature list (including usage stats and the developer gateway)
- [[Agent Mode]] — Agent pipeline, tools, permissions
- [[WhatsApp Integration]] — Setup and features
- [[Orchestra Mode]] — Multi-model workflow
- [[RAG and Semantic Memory]] — Vector store and retrieval
- [[Proactive Learning and Calendar]] — Observer + intent extraction
- [[External Providers]] — 8 provider types + fallback chain
- [[Developer API Gateway]] — Point Claude Code (or anything Anthropic-compatible) at Memo
- [[Memo Swarm]] — Multi-PC large models (Beta)
- [[Remote Access & Self-Hosting]] — Run just the server on a Pi/home server, four auth modes, SSH-only management
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — 160+ REST endpoints
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v3.3.3 released (Open Beta) · v3.3.4 in development · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
