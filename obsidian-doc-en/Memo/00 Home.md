# Memo v4.5.0

**The AI assistant that learns your habits and acts before you ask.**

Local-first · Privacy-first · Zero cloud dependency · Full offline capable

> **Current version: v4.5.0.** The two headline additions this release: a small animated **desktop mascot** that shows what Memo is actually doing in real time (its own always-on-top window, two selectable skins), and **Code Mode split into three gears** — Plan / Auto / Build, cycled with Ctrl+Tab, each with its own editable system prompt. Alongside those: both Developer Gateway endpoints (Anthropic- and OpenAI-compatible) now enforce their API key for non-loopback callers, imported skills no longer auto-activate themselves, Live Mode reports real failure reasons instead of a silent self-echo, and a round of reliability work (retry buttons on ~20 previously-dead error screens, friendlier error translation, a fixed HuggingFace avatar 404 spam). Changelog: `versinNote/v4.5.0.md`.

---

## What shipped this cycle (v4.0.0 → v4.5.0)

- **v4.0.0** — real time-awareness in the system prompt ("how long since the last message"), WhatsApp third-party conversation takeover.
- **v4.3.0** — **[[Multimodal Capabilities (Vision and Voice)|Live Mode v2]]**: native audio-to-audio voice (Google Live / OpenAI Realtime), delegate/standalone modes, barge-in, ElevenLabs + custom engines; **[[Telegram Integration|Telegram]]** added as a second messaging bridge alongside WhatsApp.
- **v4.4.0** — the **Self-Driving Task Loop**: a `Task.md` checklist becomes an unattended, multi-step run — planner/executor mode with plan approval, up to 3 parallel sub-agents (coder + analyzer/reviewer/test-runner), escalating retry, live in-chat activity; real tool-calling for the Claude and Gemini providers (previously entirely missing); a new Anthropic-compatible Custom provider type; an OpenAI-compatible Developer Gateway sibling; **gemini-sub** (Beta) — sign in with a personal Google account, reach Gemini on your own quota.
- **v4.5.0 (current)** — the **desktop mascot**; **Code Mode's Plan/Auto/Build sub-modes**; both Developer Gateway endpoints now key-enforced for non-loopback callers; imported skills no longer auto-activate; clearer Live Mode failure messages; a reliability pass across ~20 dead-end error screens; a fixed Model Store avatar 404 spam.

---

## Quick Links

- [[Architecture]] — Package map and module responsibilities
- [[System Overview]] — How all subsystems fit together
- [[Known Issues]] — Current state of known problems
- [[Features Catalog]] — Full current feature list
- [[Agent Mode]] — Agent pipeline, tools, permissions, Code Mode's Plan/Auto/Build sub-modes
- [[Self-Driving Task Loop]] — Unattended multi-step `Task.md` execution
- [[Desktop Mascot]] — The always-on-top activity companion
- [[WhatsApp Integration]] — Setup and features
- [[Telegram Integration]] — Bot setup, owner lock, self-chat assistant
- [[Orchestra Mode]] — Multi-model workflow
- [[RAG and Semantic Memory]] — Vector store and retrieval
- [[Proactive Learning and Calendar]] — Observer + intent extraction
- [[External Providers]] — 16 provider types + fallback chain
- [[Developer API Gateway]] — Point Claude Code, or anything OpenAI/Anthropic-compatible, at Memo
- [[Multimodal Capabilities (Vision and Voice)]] — Live Mode v2, vision, local STT
- [[Memo Swarm]] — Multi-PC large models (Beta)
- [[Remote Access & Self-Hosting]] — Run just the server on a Pi/home server, four auth modes, multi-account permissions, SSH-only management
- [[Cloud Sync]] — E2E encrypted Google Drive backup
- [[API Documentation]] — 180+ REST endpoints
- [[Developer Setup Guide]] — Build from source
- [[Contributing]] — How to contribute

---

**Version**: v4.5.0 · **License**: AGPL v3 · **Tech**: Go 1.26 + Flutter 3.10
