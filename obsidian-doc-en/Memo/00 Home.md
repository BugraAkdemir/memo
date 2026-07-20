# Memo v3.3.3

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Zero cloud dependency · Full offline capable

> v3.3.3 is an **open beta** release (July 10, 2026). A stability and trust-focused release: terminal CLI reliability fixes, the CLI and desktop app running independently of each other, memory recall fixes, Memo finally giving a real, grounded answer about who made it, the [[Features Catalog|Usage Stats]] tab, a fully complete `.memo` backup (including `machine.key`), and the [[Developer API Gateway]] (point Claude Code at Memo with full agentic tool-calling support). Changelog: `versinNote/v3.3.3.md`.

---

## What's New in v3.1.0

This is a major release — the biggest update to Memo since the project started. Key additions:

### Core Features
- **RAG Memory** — SQLite + sqlite-vec vector store remembers conversations
- **WhatsApp Integration** — Full WhatsApp Web via QR pairing, no API fees
- **Agent Mode** — Tool-calling pipeline with 8 tools and permission system
- **Orchestra** — Multi-model workflow with 8 specialist roles
- **Proactive Learning** — Pattern detection + auto-suggestion engine
- **Calendar** — Intent extraction from conversations → auto-events → reminders
- **Model Store** — Hardware-fit badges, curated models, one-click download
- **Skill System** — Drop `SKILL.md` files to add capabilities
- **Mood Engine** — Stochastic emotional state that shapes responses
- **Web Search** — DuckDuckGo integration, zero config

### Platform
- **Mobile Companion** — Flutter app for Android/iOS
- **Remote Access** — ngrok + Tailscale tunnels
- **Cloud Sync** — E2E encrypted Google Drive backup
- **Windows Support** — Full feature parity
- **Whisper STT** — On-device speech-to-text

### Polish (v3.1.0)
- **Onboarding UX** — Setup wizard, launchpad, spotlight tour, empty states
- **150+ L10n keys** — Full TR/EN bilingual support
- **Production hardening** — Rate limiting, 50MB body limit, 0600 permissions
- **Security** — `crypto/rand` key derivation, encrypted API keys
- **CI/CD** — GitHub Actions: auto-test on every push
- **Structured logging** — `logx` slog wrapper
- **settings_dialog split** — 5013 → 15 files

## v3.3.3 Highlights

- **CLI reliability fixes** — model download no longer hangs, multi-line paste works correctly, the terminal no longer stays broken
- **CLI/desktop independence** — closing the CLI no longer takes down the desktop app's backend
- **Memory recall fixes** — keyword search is genuinely active now, multi-topic questions no longer return incomplete answers
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
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — All ~90 REST endpoints
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v3.3.3 (Open Beta) · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
