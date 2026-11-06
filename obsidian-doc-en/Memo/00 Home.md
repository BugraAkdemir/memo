# Memo

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Fully offline with a local model

> **Latest tagged release: v4.7.0.** This vault also describes `main`, which is **v5.0** in the making: Memo can now handle your [[Digital Twin and Mail|mail]] (off by default), send a [[Daily Briefing]] to the app, Telegram, WhatsApp or mail, and walk you through a rebuilt [[Setup Wizard]]; official builds send an anonymous daily summary unless you turn it off ([[Telemetry and Privacy]]).

---

## What shipped this cycle (v4.0.0 → v5.0)

- **v4.0.0** — real time-awareness in the system prompt ("how long since the last message"), WhatsApp third-party conversation takeover.
- **v4.3.0** — **[[Multimodal Capabilities (Vision and Voice)|Live Mode v2]]**: native audio-to-audio voice (Google Live / OpenAI Realtime), delegate/standalone modes, barge-in, ElevenLabs + custom engines; **[[Telegram Integration|Telegram]]** added as a second messaging bridge alongside WhatsApp.
- **v4.4.0** — the **Self-Driving Task Loop**: a `Task.md` checklist becomes an unattended, multi-step run — planner/executor mode with plan approval, up to 3 parallel sub-agents (coder + analyzer/reviewer/test-runner), escalating retry, live in-chat activity; real tool-calling for the Claude and Gemini providers (previously entirely missing); a new Anthropic-compatible Custom provider type; an OpenAI-compatible Developer Gateway sibling; **gemini-sub** (Beta) — sign in with a personal Google account, reach Gemini on your own quota.
- **v4.6.0** — Memo can **open apps** and **drive an isolated browser** shown live in a panel next to the chat (you can use it by hand too); long replies are ended by silence rather than a fixed timeout; **Subscriptions** (Antigravity / Claude / Codex through the bundled CLIProxyAPI sidecar); one Flutter app for Android and iOS; a rebuilt Memory tab; skills active per chat. Full list: `versinNote/v4.6.0.md`.
- **v4.5.0** — the **desktop mascot**; **Code Mode's Plan/Auto/Build sub-modes**; both Developer Gateway endpoints key-enforced for non-loopback callers; imported skills no longer auto-activate.
- **v4.7.0 (latest tagged)** — **[[In-App Updates]]**: signed updates from inside Memo with automatic rollback, plus separate engine updates; ChatGPT and claude.ai through a real browser window (Beta); a leaner Code Mode and cheaper repeated turns.
- **v5.0 (on `main`, notes drafted)** — **[[Digital Twin and Mail]]** (Gmail/Outlook/IMAP, a 3-layer Gatekeeper, an injection shield, replies in your style, approval with an undo window), the **[[Daily Briefing]]**, the **[[Setup Wizard]]**, **[[Project Folders]]** with their own memory, **[[Chat Sub-agents]]**, one chat list with a live streaming agent, skills by name, faster local models, and the anonymous daily summary ([[Telemetry and Privacy]]).

---

## Quick Links

- [[Architecture]] — Package map and module responsibilities
- [[System Overview]] — How all subsystems fit together
- [[Known Issues]] — Current state of known problems
- [[Features Catalog]] — Full current feature list
- [[Agent Mode]] — Agent pipeline, tools, permissions, Code Mode's Plan/Auto/Build sub-modes
- [[Self-Driving Task Loop]] — Unattended multi-step `Task.md` execution
- [[Digital Twin and Mail]] — Mail, the Gatekeeper, the injection shield, replies in your style, approval and undo
- [[Daily Briefing]] — The morning card: sections, custom items, channels, model
- [[Setup Wizard]] — The guided first-run flow
- [[Chat Sub-agents]] — Background sub-agents in a chat
- [[In-App Updates]] — Signed updates with rollback, engine updates
- [[Telemetry and Privacy]] — The anonymous daily summary and its off switch
- [[Project Folders]] — `.memo/` in a project: working rules, notes, its own memory, project skills
- [[Desktop Mascot]] — The always-on-top activity companion
- [[WhatsApp Integration]] — Setup and features
- [[Telegram Integration]] — Bot setup, owner lock, self-chat assistant
- [[Orchestra Mode]] — Multi-model workflow
- [[RAG and Semantic Memory]] — Vector store and retrieval
- [[Proactive Learning and Calendar]] — Observer + intent extraction
- [[External Providers]] — 18 provider types + fallback chain
- [[Developer API Gateway]] — Point Claude Code, or anything OpenAI/Anthropic-compatible, at Memo
- [[Multimodal Capabilities (Vision and Voice)]] — Live Mode v2, vision, local STT
- [[Memo Swarm]] — Multi-PC large models (Beta)
- [[Remote Access & Self-Hosting]] — Run just the server on a Pi/home server, four auth modes, multi-account permissions, SSH-only management
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — 260+ REST routes
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v4.7.0 latest tagged, v5.0 in progress · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
