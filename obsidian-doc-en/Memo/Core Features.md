# Core Features

This is a Map of Content (MOC) for Memo's core feature documentation. Each linked page covers one major subsystem in depth.

---

## 🧠 Memory & Intelligence

| Page | Description |
|------|-------------|
| [[RAG and Semantic Memory]] | Hybrid vector + FTS5 retrieval-augmented generation — how Memo remembers |
| [[Data Layer and Persistence]] | Database schema, ANN indexing, persistence architecture |
| [[Vector Search Logic]] | Cosine similarity, hybrid RRF merging, Top-K search |
| [[Incognito Mode]] | Ephemeral sessions that leave no trace |

## 🏭 Model Management

| Page | Description |
|------|-------------|
| [[Model Management (The Factory)]] | HuggingFace search, download, local inference lifecycle |
| [[Llama.cpp Integration]] | Subprocess management, health checks, GPU offloading |

## 🌐 External Connectivity

| Page | Description |
|------|-------------|
| [[External Providers]] | 16 provider types — OpenAI, Claude, Gemini, Grok, Groq, OpenRouter, Ollama, Custom (OpenAI/Anthropic-compatible), OpenCode Zen/Go, Kilo Code, gemini-sub |
| [[WhatsApp Integration]] | QR pairing, bidirectional messaging, file transfer, self-chat assistant |
| [[Telegram Integration]] | Bot pairing, owner lock, self-chat assistant |
| [[Backup & Restore]] | `.memo` zip-based export/import with encryption |
| [[Cloud Sync]] | Google Drive E2E encrypted backup |
| [[Remote Access & Self-Hosting]] | Token/password auth, per-device tokens, ngrok/Tailscale, self-hosted server mode |

## 🧰 Advanced Features

| Page | Description |
|------|-------------|
| [[Agent Mode]] | AI tool calling with permission system and sandbox — 27 built-in tools, executable skill tools, and Code Mode's Plan/Auto/Build sub-modes |
| [[Self-Driving Task Loop]] | Unattended multi-step execution from a `Task.md` checklist, planner/executor mode, sub-agent orchestration |
| [[Desktop Mascot]] | An always-on-top window reflecting Memo's live activity |
| [[Orchestra Mode]] | Multi-model orchestration with expert roles |
| [[Multimodal Capabilities (Vision and Voice)]] | Image uploads, STT transcription, and Live Mode v2 native audio-to-audio voice |
| [[Developer API Gateway]] | Anthropic- and OpenAI-compatible endpoints — point Claude Code (or anything else) at Memo |
| [[Memo Swarm]] | Beta — pool several PCs for one oversized local model |

## ⏰ Automation & Proactivity

| Page | Description |
|------|-------------|
| [[Proactive Learning and Calendar]] | Routines (scheduled automations), ambient nudges, Self-Insight, intent extraction, calendar |

## 🗂️ Version Features

| Page | Description |
|------|-------------|
| [[Features Catalog]] | Complete, current feature-by-feature listing |
| Release Notes | `versinNote/v4.5.0.md` (current) — see `versinNote/` for the full per-version history |
