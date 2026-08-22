# Memo v3.9.0 (in development)

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Zero cloud dependency · Full offline capable

> The last **released** version is **v3.5.5** (open beta, August 17, 2026). **v3.9.0 is in active development** (not yet released, no tag cut) — the through-line is making things actually correct instead of merely present: web search now decides per-message via real tool-calling instead of injecting the raw message every time, reasoning-effort control moved from static per-vendor tables to live per-model capability discovery, and a security review of a real self-hosted deployment found and closed a genuine auth bypass (a reverse-tunnel forwarding external traffic to loopback, previously trusted unconditionally) plus three previously-open High-severity issues. Also new: **WhatsApp and Telegram self-chat assistants** (message yourself, Memo replies — with routines creatable straight from the conversation), **granular per-account permissions** for self-hosted multi-account setups, a **Kilo Code** provider with a free/paid model browser, a system tray icon, Dream's own schedule, a Stats token-spend breakdown, and a redesigned mobile navigation. Changelog: `versinNote/v3.9.0.md` (running draft, not final).
>
> v3.5.5 itself completed the self-hosting story: a full 4-mode auth system with per-device tokens, multi-account (admin/user) support, a real Flutter-web UI replacing the old hand-rolled one, Docker/CasaOS images, dedicated server-only installers, and a full mobile-responsive pass — all managed end-to-end over SSH via `memo config/remote/service`. Changelog: `versinNote/v3.5.5.md`.

---

## v3.9.0 Highlights (in development)

- **[[WhatsApp Integration|WhatsApp]] and [[Telegram Integration|Telegram]] self-chat assistants** — message yourself (or your bot) and Memo replies as a full assistant: chat, memory, agent tools, all without opening the app
- **Routines via chat** — create, list, or cancel a routine just by asking, from a WhatsApp/Telegram self-chat or normal chat, no need to open the Routines tab
- **Granular per-account permissions (Faz 5.1.1)** — 7 independent checkboxes per self-hosted account (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines), enforced server-side, not just hidden in the UI
- **[[External Providers|Kilo Code]] provider** — live model list, free models sorted to the top with a green checkmark, same treatment extended to OpenCode Zen
- **Web search redesigned** — real per-message tool-calling decides whether to search, instead of injecting the raw message on every turn
- **Reasoning-effort control rebuilt** — live per-model capability discovery (Claude/Gemini/Ollama/OpenRouter) instead of static, occasionally-wrong tables
- **[[Developer API Gateway]] redesigned** — LM-Studio-style nav, a new OpenAI-compatible endpoint (`/v1/chat/completions`), one-click Claude Code CLI connect, configurable system prompt
- **System tray icon**, **Dream's own configurable schedule**, **Stats token-spend-by-category breakdown**, a visible **"N memories used" badge** on replies
- **Mobile navigation redesign** — hamburger drawer replaces the desktop NavRail below 600px
- **Security fixes** — a Cloudflare Tunnel auth-bypass (loopback trust didn't account for a local proxy forwarding external traffic), the setup wizard re-appearing per-origin, plus three previously-open High-severity issues (ngrok download integrity, a WhatsApp search wildcard bug, the agent audit log not actually persisting)
- **Fixed: self-chat's agent tools were never actually reachable** — the most consequential bug of this stretch; `SendMessageStreamTo`'s agent-mode gate never applied to self-chat's own background sessions, so every routine tool and `web_search` silently didn't exist from WhatsApp/Telegram's point of view until this was found and fixed
- The CLI closed its last self-hosting gap (`memo remote list-accounts/add-account/delete-account`) and gained `-chat <id> -list`/`-memory usage` for inspecting an existing chat

Full list: `versinNote/v3.9.0.md` (running draft, not final)

## v3.5.5 Highlights (last released)

- **A real 4-mode auth system** — no-credential, token-only, password, or token+password, with argon2id hashing, brute-force lockout, and per-device tokens (hashed at rest, revocable individually)
- **Multi-account support (Faz 5.1)** — admin/user roles for self-hosted Memo, managed from Settings → Accounts
- **A real web UI** — the built-in headless/CasaOS/browser page is now a genuine Flutter web build, not a hand-rolled HTML/JS client
- **Docker/CasaOS image**, **dedicated server-only installers** (`get-memo-server.sh`), managed entirely over SSH via `memo config/remote/service`
- **Full mobile-responsive pass** across every screen
- Fixed: agent mode silently ignored by non-streaming chat requests, Task Loop doing nothing on start, Orchestra Mode's utility calls (titles, routine parsing) going through the wrong pipeline

Full list: `versinNote/v3.5.5.md`

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
- [[Telegram Integration]] — Bot setup, owner lock, self-chat assistant
- [[Orchestra Mode]] — Multi-model workflow
- [[RAG and Semantic Memory]] — Vector store and retrieval
- [[Proactive Learning and Calendar]] — Observer + intent extraction
- [[External Providers]] — 14 provider types + fallback chain
- [[Developer API Gateway]] — Point Claude Code (or anything Anthropic-compatible) at Memo
- [[Memo Swarm]] — Multi-PC large models (Beta)
- [[Remote Access & Self-Hosting]] — Run just the server on a Pi/home server, four auth modes, multi-account permissions, SSH-only management
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — 180+ REST endpoints
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v3.5.5 released (Open Beta) · v3.9.0 in development · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
