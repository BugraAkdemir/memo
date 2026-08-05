# Core Features

This is a Map of Content (MOC) for Memo's core feature documentation. Each linked page covers one major subsystem in depth.

---

## 🧠 Memory & Intelligence

| Page | Description |
|------|-------------|
| [[RAG and Semantic Memory]] | Vector-based retrieval-augmented generation — how Memo remembers |
| [[Memory Store (SQLite + vec0)]] | Database schema, ANN indexing, persistence architecture |
| [[Vector Search Logic]] | Cosine similarity, parallel workers, Top-K search |
| [[Incognito Mode]] | Ephemeral sessions that leave no trace |

## 🏭 Model Management

| Page | Description |
|------|-------------|
| [[Model Management (The Factory)]] | HuggingFace search, download, local inference lifecycle |
| [[Llama.cpp Integration]] | Subprocess management, health checks, GPU offloading |

## 🌐 External Connectivity

| Page | Description |
|------|-------------|
| [[External Providers]] | OpenAI, Claude, Gemini, Grok, Groq, OpenRouter, Ollama |
| [[WhatsApp Integration]] | QR pairing, bidirectional messaging, file transfer |
| [[Backup & Restore]] | `.memo` zip-based export/import with encryption |
| [[Cloud Sync]] | Google Drive E2E encrypted backup |
| [[Remote Access (ngrok)]] | Secure tunnel for anywhere access |

## 🧰 Advanced Features

| Page | Description |
|------|-------------|
| [[Agent Mode]] | AI tool calling with permission system and sandbox — 19 built-in tools + executable skill tools |
| [[Orchestra Mode]] | Multi-model orchestration with expert roles |
| [[Multimodal Capabilities (Vision and Voice)]] | Image uploads, STT transcription, and (Beta) hands-free Live Mode voice conversation |
| [[Developer API Gateway]] | Anthropic-compatible endpoint — point Claude Code at Memo |
| [[Memo Swarm]] | Beta — pool several PCs for one oversized local model |

## ⏰ Automation & Proactivity

| Page | Description |
|------|-------------|
| [[Proactive Learning and Calendar]] | Routines (scheduled automations), ambient nudges, Self-Insight, intent extraction, calendar |

## 🗂️ Version Features

| Page | Description |
|------|-------------|
| [[v3.1.1 Features]] | WhatsApp, mobile, backup, agent, orchestra, providers — historical snapshot, frozen at v3.1.0/v3.1.1, two majors behind current |
| [[Features Catalog]] | Complete, current feature-by-feature listing |
| Release Notes | `versinNote/v3.3.3.md` (released), `versinNote/v3.3.4.md` (in development) |
