# Memo v3.1.2

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Zero cloud dependency · Full offline capable

> v3.1.2 is the second **open beta** of the v3.1 line (July 6, 2026). Includes streaming fixes, cfgMu concurrency safety, atomic import staging, and WhatsApp session isolation. Changelog: `versinNote/v3.1.2.md`.

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

---

## Quick Links

- [[Architecture]] — Package map and module responsibilities
- [[System Overview]] — How all subsystems fit together
- [[Known Issues]] — Current state of known problems
- Release notes: `versinNote/v3.1.2.md`
- [[v3.1.1 Features]] — Full feature catalog
- [[Agent Mode]] — Agent pipeline, tools, permissions
- [[WhatsApp Integration]] — Setup and features
- [[Orchestra Mode]] — Multi-model workflow
- [[RAG and Semantic Memory]] — Vector store and retrieval
- [[Proactive Learning and Calendar]] — Observer + intent extraction
- [[External Providers]] — 8 provider types + fallback chain
- [[Developer API Gateway]] — Point Claude Code (or anything Anthropic-compatible) at Memo
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — All ~90 REST endpoints
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v3.1.2 (Open Beta) · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
