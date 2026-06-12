# 🧠 Memo: Local AI Memory Shell

> **Version:** v3.1.0-beta | **Developer:** Buğra | **License:** MIT
> 
> *"Control your AI. Own your Memory."*

Welcome to the official technical documentation and knowledge base for **Memo** — a local-first, privacy-focused AI assistant with persistent RAG memory, external provider support, E2E-encrypted cloud sync, and a powerful agent/orchestra system. This vault is the complete **"Second Brain"** for the project.

---

## 📊 Project Stats

| Metric | Value |
|--------|-------|
| Go Backend | ~15,000 lines across 30+ packages |
| Flutter Frontend | ~8,000 lines, 20+ widgets, 924 L10n keys |
| Mobile App | Flutter thin client (Android/iOS) |
| REST API | ~35 endpoints (chat, memory, models, providers, agents, sync) |
| Database | SQLite + sqlite-vec (ANN vector search) |
| External Providers | 7 (OpenAI, Gemini, Claude, Grok, Groq, OpenRouter, Ollama) |
| Bug Fixes (v3.1.0) | 61 documented fixes (54 tracked → 46 fixed, 8 open) |

---

## 🏛️ Architecture

Discover how Memo works, how components communicate, and the cornerstones of the system.

| Page | Description |
|------|-------------|
| [[System Overview]] | High-level architecture: Go backend ↔ Flutter frontend via HTTP/JSON + SSE |
| [[Backend (Go) Architecture]] | App bridge pattern, module map, data flow |
| [[Frontend (Flutter) Design]] | Riverpod state management, widget tree, 11-tab settings |
| [[Data Layer and Persistence]] | SQLite + vec0, session JSON files, provider config encryption |
| [[Technical Deep Dive]] | Bridge pattern, llama lifecycle, E2E sync, agent engine, orchestra internals |

```
┌─────────────────────────────────────────────────────────────┐
│  Flutter Desktop / Mobile App                               │
│  ┌─────────┐ ┌──────────┐ ┌──────────┐ ┌────────────────┐  │
│  │ Chat UI │ │ Settings │ │ Model    │ │ WhatsApp UI    │  │
│  │ (SSE)   │ │ (11 tab) │ │ Store    │ │ (QR + Chat)    │  │
│  └────▲────┘ └────▲─────┘ └────▲─────┘ └───────▲────────┘  │
│       │           │            │                │           │
├───────┼───────────┼────────────┼────────────────┼───────────┤
│       └───────────┼────────────┼────────────────┘           │
│                   │   HTTP/JSON + SSE (port 8090)           │
│  ┌────────────────┼────────────┼────────────────────────┐   │
│  │  Go Backend    │            │                        │   │
│  │  ┌─────────────▼────────────▼──────────────────┐    │   │
│  │  │           App (AppBridge)                    │    │   │
│  │  │  ┌────────┐ ┌──────────┐ ┌───────────────┐  │    │   │
│  │  │  │ LLM    │ │ Memory   │ │ Sessions      │  │    │   │
│  │  │  │ Router │ │ (SQLite  │ │ (JSON files)  │  │    │   │
│  │  │  │        │ │ + vec0)  │ │               │  │    │   │
│  │  │  └───┬────┘ └──────────┘ └───────────────┘  │    │   │
│  │  │      │                                        │    │   │
│  │  │  ┌───┴────────┐ ┌──────────┐ ┌────────────┐  │    │   │
│  │  │  │ llama.cpp  │ │ Provider │ │ Agent      │  │    │   │
│  │  │  │ (local)    │ │ Router   │ │ Engine     │  │    │   │
│  │  │  │            │ │ (7 APIs) │ │ (8 tools)  │  │    │   │
│  │  │  └────────────┘ └──────────┘ └────────────┘  │    │   │
│  │  │  ┌────────────┐ ┌──────────┐ ┌────────────┐  │    │   │
│  │  │  │ Orchestra  │ │ WhatsApp │ │ Cloud Sync │  │    │   │
│  │  │  │ (8 roles)  │ │(whatsmeow)│ │ (Drive)   │  │    │   │
│  │  │  └────────────┘ └──────────┘ └────────────┘  │    │   │
│  │  └───────────────────────────────────────────────┘    │   │
│  └────────────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────────────┘
```

---

## 🧠 Core Features

Features that make Memo more than just a chat interface — a true **memory shell** with AI agency.

### 💾 Memory & Intelligence

| Page | Status | Description |
|------|--------|-------------|
| [[RAG and Semantic Memory]] | ✅ Complete | Persistent vector-based retrieval-augmented generation with Top-K similarity search |
| [[Incognito Mode]] | ✅ Complete | Ephemeral sessions — zero persistence, volatile context |
| [[Multimodal Capabilities (Vision and Voice)]] | ✅ Complete | Image upload (multimodal GGUF), local STT, document indexing |

### 🏭 Model Management

| Page | Status | Description |
|------|--------|-------------|
| [[Model Management (The Factory)]] | ✅ Complete | HuggingFace search/download, background download manager, system diagnostics |
| [[Llama.cpp Integration]] | ✅ Complete | Subprocess lifecycle, health checks, GPU offloading (NVIDIA/AMD), port management |

### 🌐 Connectivity

| Page | Status | Description |
|------|--------|-------------|
| [[External Providers]] | ✅ Complete | 7 providers with router, fallback chain, auto-disable, health checks |
| [[Cloud Sync]] | ✅ Complete | Google Drive E2E encrypted backup (AES-256-GCM + PBKDF2) |
| [[WhatsApp Integration]] | ✅ Complete | QR pairing, bidirectional messaging, 4 agent tools |
| [[Backup & Restore]] | ✅ Complete | `.memo` zip export/import, full wipe, zip bomb protection |
| [[Mobile App]] | ✅ Basic | Thin Flutter client (LAN/ngrok, token auth, streaming chat) |

### 🧰 Advanced AI

| Page | Status | Description |
|------|--------|-------------|
| [[Agent Mode]] | ✅ Backend | 8 tools, 6 permission policies, execution sandbox, rate limiting |
| [[Orchestra Mode]] | ✅ Backend | 8 expert roles, Plan→Execute→Synthesize workflow, SSE progress |
| [[Features Catalog]] | ✅ Complete | Complete feature-by-feature listing |

---

## 🆕 v3.1.0 Features

Major new capabilities added in the current release.

| Feature | Page | Impact |
|---------|------|--------|
| 📱 WhatsApp | [[WhatsApp Integration]] | Bidirectional messaging, file transfer, agent tools |
| 🔌 External Providers | [[External Providers]] | 7 LLM APIs with smart routing and fallback |
| 🤖 Agent Mode | [[Agent Mode]] | AI tool calling with permission system and sandbox |
| 🎵 Orchestra Mode | [[Orchestra Mode]] | Multi-model collaboration with expert roles |
| 📦 Backup & Restore | [[Backup & Restore]] | `.memo` format, Google Drive sync, full wipe |
| 📱 Mobile App | [[Mobile App]] | Android/iOS companion client |
| 🐛 Bug Fixes | [[Resolved Issues]] | 61 documented fixes across the entire codebase |

---

## 🔧 Technical Reference

In-depth technical information for developers and power users.

| Page | Content |
|------|---------|
| [[API Documentation]] | Complete REST API endpoint reference (~35 endpoints) |
| [[Llama.cpp Integration]] | Process lifecycle, health checks, port management, GPU detection |
| [[Vector Search Logic]] | Cosine similarity, parallel workers, Top-K, min_similarity |
| [[Advanced Settings]] | Model parameters, engine modes, configuration reference |
| [[CGO Flags]] | CGO build requirements, sqlite-vec extension compilation |
| [[Default System Prompt]] | Memo's identity directives and anti-hallucination rules |
| [[Known Issues]] | Exhaustive bug audit — 54 tracked, 46 fixed, 8 open |
| [[Resolved Issues]] | 61 documented fixes with code references |

---

## 🚀 Guides

Set up, develop, and use Memo.

| Guide | Audience | Pages |
|-------|----------|-------|
| [[Developer Setup Guide]] | Developers | Environment setup, quick start |
| [[Build and Packaging]] | Developers | Cross-platform build, release packaging |
| [[User Guide]] | End Users | Day-to-day usage |
| [[Troubleshooting]] | Everyone | Common problems and solutions |
| [[Contributing]] | Contributors | How to contribute, code standards, key areas |

---

## 🗺️ Quick Navigation

| Section | Pages |
|---------|-------|
| 🏛️ Architecture | [[System Overview]], [[Backend (Go) Architecture]], [[Frontend (Flutter) Design]], [[Data Layer and Persistence]], [[Technical Deep Dive]] |
| 🧠 Features | [[RAG and Semantic Memory]], [[Model Management (The Factory)]], [[Incognito Mode]], [[Cloud Sync]], [[Multimodal Capabilities (Vision and Voice)]], [[WhatsApp Integration]], [[Backup & Restore]], [[Mobile App]] |
| 🧰 Advanced | [[Agent Mode]], [[Orchestra Mode]], [[External Providers]], [[Features Catalog]] |
| 🔧 Reference | [[API Documentation]], [[Llama.cpp Integration]], [[Vector Search Logic]], [[Advanced Settings]], [[CGO Flags]], [[Default System Prompt]] |
| 📋 Operations | [[Known Issues]], [[Resolved Issues]], [[Troubleshooting]], [[Contributing]], [[Roadmap]] |
| 🆕 v3.1.0 | [[v3.1.0 Features]] |

---

> **Our Philosophy:** *Control your AI. Own your Memory.*
> **Developer:** Buğra
> **Current Version:** v3.1.0-beta
