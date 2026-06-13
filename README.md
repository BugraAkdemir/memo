<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go" alt="Go 1.25"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/License-AGPL_v3-blue?style=for-the-badge" alt="License AGPL v3"/>
  <img src="https://img.shields.io/badge/Status-v3.1.0--beta-blue?style=for-the-badge" alt="v3.1.0-beta"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Integrated-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Enabled-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-Integrated-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Agent-8_Tools-B08D57?style=flat-square" alt="Agent"/>
  <img src="https://img.shields.io/badge/Backup-.memo_ZIP-blue?style=flat-square" alt="Backup"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Cross Platform"/>
</div>

<h1 align="center">
  🧠 Memo — The AI Memory Shell
</h1>

<p align="center">
  <b>Your Local. Your Private. Your Second Brain.</b><br/>
  <i>A privacy-first, local-first AI assistant with persistent RAG memory, external providers, an agent engine, WhatsApp, and a premium desktop experience.</i>
</p>

<p align="center">
  <a href="#-why-memo">Why Memo</a> •
  <a href="#-features">Features</a> •
  <a href="#-design">Design</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="#-roadmap">Roadmap</a> •
  <a href="READmeTR.md">Türkçe</a>
</p>

---

> **Current version:** `v3.1.0-beta` — RAG memory, external providers, agent & orchestra engines, WhatsApp, encrypted cloud sync, mobile companion, and a from-scratch **"Pewter Study"** UI redesign. [See roadmap →](docs/ROADMAP.md)

---

## 🎯 Why Memo

Most AI assistants send your conversations to someone else's servers. **Memo doesn't.** It runs the model on your machine, stores every memory in a local vector database, and never phones home. You own the model, the data, and the disk it lives on.

But local-first shouldn't mean hard to use. Memo pairs a real RAG memory engine and a tool-calling agent with an interface a first-time user can navigate — **download a model in one click, see whether it fits your hardware before you commit, and always know what's running.**

- **🔒 Private by construction** — zero telemetry, no training on your chats, no cloud dependency. Optional encrypted backup only if *you* turn it on.
- **🧠 Real memory** — every interaction is embedded and indexed; relevant context is retrieved automatically on each turn.
- **🤝 Best of both worlds** — run chat through a powerful external API while a tiny local model handles embeddings, or stay 100% offline.
- **🖥️ Native, not a web wrapper** — a Flutter desktop app on Linux, Windows, and macOS, plus a mobile companion.

---

## ✨ Features

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Local RAG Engine</b></h3>
      SQLite + sqlite-vec (vec0 ANN index). Every interaction is semantically indexed for O(log n) retrieval at any scale. No cloud, no third-party embeddings.
    </td>
    <td width="50%">
      <h3>🤖 <b>Agent Engine</b></h3>
      Tool-calling pipeline with 8 built-in tools, a 6-policy permission system (allow/deny once/session/forever), an execution sandbox, rate limiting, and an audit log.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🎵 <b>Orchestra Mode</b></h3>
      Multi-model collaboration: a chief model plans and decomposes a task, expert roles execute in parallel, and the chief synthesizes the result. 8 built-in roles.
    </td>
    <td width="50%">
      <h3>🔄 <b>Cross-Mode Architecture</b></h3>
      Use external API providers for chat while a small local model handles embeddings — power and privacy, independently configurable.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>⚡ <b>Local llama.cpp</b></h3>
      Bundled <code>llama-server</code> lifecycle management: auto-download, auto-start, GPU offloading with VRAM detection (NVIDIA / AMD / Metal). No Docker, no containers.
    </td>
    <td width="50%">
      <h3>🏭 <b>Guided Model Store</b></h3>
      Curated recommendations, one-click download, and a <b>hardware-fit badge</b> driven by your RAM/VRAM. Quantization is auto-picked — no cryptic <code>Q4_K_M</code> guessing.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>💬 <b>WhatsApp Integration</b></h3>
      Full WhatsApp Web pairing via QR. Send/receive messages, contact name resolution, whitelist-based file transfer, dedicated agent tools. Your chats stay local.
    </td>
    <td width="50%">
      <h3>📦 <b>Backup & Restore</b></h3>
      Full <code>.memo</code> zip export/import — sessions, config, memory, WhatsApp data, providers — plus encrypted Google Drive sync (AES-256-GCM) and a double-confirm wipe.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🛠️ <b>Model & Provider Agnostic</b></h3>
      Any OpenAI-compatible server (llama.cpp, Ollama, LM Studio). External providers: OpenAI, Anthropic, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama — with a fallback router.
    </td>
    <td width="50%">
      <h3>📱 <b>Mobile & Remote</b></h3>
      A thin Flutter mobile companion (Android/iOS) over LAN or a built-in ngrok tunnel, with token auth and streaming chat.
    </td>
  </tr>
</table>

---

## 🎨 Design

Memo's interface is **"Pewter Study"** — a deliberately **mid-tone** identity that sits between a bright light theme and a cave-dark one: warm graphite surfaces, a soft off-white ink for text, and a single **muted bronze** accent. No neon, no glare — a calm, premium workspace for a second brain.

| Principle | In practice |
|-----------|-------------|
| **One surface, three layers** | Depth comes from elevation, not color. |
| **Spend the accent once** | Bronze marks only primary actions, active state, and progress. |
| **Plain language** | "Balanced — recommended", never "Q4_K_M". |
| **Hardware-aware** | Every model answers "does this fit my device?" up front. |
| **Engine Strip** | A persistent footer shows which chat + memory models are running, with one-tap stop. |

Typography pairs **Schibsted Grotesk** (display) with **Inter** (body) and **JetBrains Mono** (code). Two themes ship: **Pewter** (default) and **Night** (deeper). Full system in [frontend/DESIGN.md](frontend/DESIGN.md).

### The model download, reimagined

Downloading a model used to mean searching HuggingFace, opening a repo, and picking from a list of files named `…Q4_K_M.gguf`, `…Q5_K_S.gguf`, `…fp16.gguf` — meaningless to most people. Memo replaces that with a guided flow:

1. **Discover** shows curated, known-good models with a one-line description and size.
2. A **hardware-fit badge** ("✓ Fits your device — fast on GPU" / "⚠ May be insufficient") is computed from your detected **RAM and VRAM**.
3. **One click** resolves the best-fit quantization automatically and downloads it.
4. Power users can still open **Advanced search** for any HuggingFace repo, with every quant translated into plain language.

---

## 🚀 Quick Start

No terminal, no cloning, no build steps. **Download and install with one click.**

<div align="center">
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download_Memo-memo.bugradev.com-B08D57?style=for-the-badge" alt="Download Memo"/>
  </a>
</div>

| Platform | Installer | How |
|----------|-----------|-----|
| **Windows** | `Memo-Setup.exe` | Run the installer, click through, done. |
| **Linux** | `.AppImage` / `.deb` | Make it executable (or install the `.deb`) and launch. |

llama.cpp is bundled and everything is set up on first run — just **[download from memo.bugradev.com](https://memo.bugradev.com)**, open the app, pick a model from **Discover**, and start chatting.

<details>
<summary><b>🛠️ Build from source (developers)</b></summary>

**Prerequisites:** Go 1.25+ · Flutter 3.10+ (llama.cpp is bundled).

```bash
# Terminal 1 — Backend
git clone https://github.com/BugraAkdemir/memo.git && cd memo
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

Build release packages:
```bash
./build_releases.sh    # Linux  → tar.gz / AppImage / deb
.\build_releases.bat   # Windows → Inno Setup installer or portable zip
```
</details>

---

## 🏛️ Architecture

```
┌──────────────────────────────────┐  ┌───────────────────────────────┐
│     Flutter Desktop Client       │  │    Flutter Mobile Client      │
│  ┌──────┐ ┌────────┐ ┌────────┐  │  │  ┌──────────┐ ┌──────────┐    │
│  │Chat  │ │Settings│ │ Model  │  │  │  │ Connect  │ │   Chat   │    │
│  │+Agent│ │+Backup │ │ Store  │  │  │  │  Screen  │ │  Screen  │    │
│  └──┬───┘ └───┬────┘ └───┬────┘  │  │  └────┬─────┘ └────┬─────┘    │
│  ┌──┴─────────┴──────────┴────┐  │  │  ┌────┴────────────┴────┐     │
│  │  Riverpod · SSE · Engine   │  │  │  │   Riverpod · Dio      │     │
│  │  Strip · MemoApiClient     │  │  │  │   MemoApiClient       │     │
│  └───────────┬────────────────┘  │  │  └───────────┬───────────┘     │
└──────────────┼────────────────────┘  └──────────────┼────────────────┘
               │ REST + SSE (:8090)                    │ LAN / ngrok / TLS
┌──────────────┼───────────────────────────────────────┼────────────────┐
│              └──────────────────┬─────────────────────┘                │
│                        Go Backend Server                               │
│  ┌──────────────────────────────┴──────────────────────────────┐      │
│  │   Web Server (server.go) · ~35 endpoints (handlers)          │      │
│  └──────────────────────────────┬──────────────────────────────┘      │
│  ┌──────────────────────────────┴──────────────────────────────┐      │
│  │                    App Engine (app.go)                        │      │
│  └──┬─────────┬──────────┬──────────┬──────────┬──────────┬─────┘      │
│  ┌──┴──┐ ┌────┴───┐ ┌────┴────┐ ┌───┴────┐ ┌───┴────┐ ┌───┴─────┐    │
│  │Mem  │ │Sessions│ │Llama +  │ │WhatsApp│ │Provider│ │ Agent   │    │
│  │vec0 │ │ JSON   │ │Emb Mgr  │ │whatsmeow│ │Router  │ │ Engine  │    │
│  │SQLite│ │       │ │+GPU/RAM │ │        │ │(7 APIs)│ │(8 tools)│    │
│  └─────┘ └────────┘ └─────────┘ └────────┘ └────────┘ └─────────┘    │
│  ┌──────────┐ ┌──────────────┐ ┌──────────┐ ┌──────────┐             │
│  │Orchestra │ │ Model Store  │ │Cloud Sync│ │ ngrok    │             │
│  │(8 roles) │ │ HF + local   │ │ (Drive)  │ │ Tunnel   │             │
│  └──────────┘ └──────────────┘ └──────────┘ └──────────┘             │
└──────────────────────────────────────────────────────────────────────┘
```

**Deep dive:** [docs/architecture.md](docs/architecture.md) · **API:** [docs/API_REFERENCE.md](docs/API_REFERENCE.md)

---

## 🛣️ Roadmap

| Version | Theme | Status |
|---------|-------|--------|
| **v3.1.0** | Memory — RAG, WhatsApp, Backup, Mobile, Remote Access | ✅ Released |
| **v3.2.0** | Scheduled Intelligence — Calendar, Agent UI, Mobile Notifications | 🚧 In development |
| **v3.3.0** | Mobile & Voice — Mobile v2 + Voice Assistant | 🚧 Planned |
| **v3.4.0** | Plugin & Web — Plugin System + Web Search | 🚧 Planned |
| **v3.5.0** | Smarter Memo — Knowledge Graph, Self-Improving Memory | 🔮 Future |

[Full roadmap →](docs/ROADMAP.md)

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [🎨 Design System](frontend/DESIGN.md) | The "Pewter Study" tokens, components, and screens |
| [🏛️ Architecture](docs/architecture.md) | Technical deep dive into every component |
| [📡 API Reference](docs/API_REFERENCE.md) | All REST endpoints |
| [🛣️ Roadmap](docs/ROADMAP.md) | Strategic vision and release plan |
| [📱 Mobile README](mobile/README.md) | Mobile companion app docs |
| [📖 Known Issues](docs/KNOWN_ISSUES.md) | Complete audit with priorities |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | Common problems and solutions |
| [📝 Contributing](docs/CONTRIBUTING.md) | How to contribute |

---

## 🧪 Tech Stack

<div align="center">

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, `http.ServeMux`, SSE streaming |
| **Frontend (Desktop)** | Flutter 3.10+, Riverpod 2.x, Dio, flutter_markdown, google_fonts |
| **Frontend (Mobile)** | Flutter 3.10+, Riverpod 2.x, Dio (Android · iOS · Web) |
| **LLM Runtime** | llama.cpp (bundled), OpenAI-compatible API |
| **External Providers** | OpenAI · Anthropic Claude · Google Gemini · xAI Grok · Groq · OpenRouter · Ollama |
| **Vector Store** | SQLite + sqlite-vec (vec0 ANN index) |
| **WhatsApp** | whatsmeow (multi-device Web API) |
| **Hardware Detection** | nvidia-smi · rocm-smi · Metal · system RAM (/proc, GlobalMemoryStatusEx, sysctl) |
| **Cloud Sync** | Google Drive OAuth2 + AES-256-GCM |
| **Build** | Go toolchain, Flutter build, shell scripts, Inno Setup |
| **License** | GNU AGPL v3 |

</div>

---

## 🤝 Contributing

Contributions welcome:
- [Known Issues](docs/KNOWN_ISSUES.md) — pick an item to work on
- [Roadmap](docs/ROADMAP.md) — see what's coming
- [Contributing Guide](docs/CONTRIBUTING.md)

---

<div align="center">
  <h3>🧠 <i>Your Mind. Your Data. Your Computer.</i></h3>
  <p>Built with ❤️ by <b>Buğra Akdemir</b></p>
  <p>
    <a href="https://github.com/BugraAkdemir/memo/issues">Report Bug</a> •
    <a href="https://github.com/BugraAkdemir/memo/discussions">Discussion</a> •
    <a href="READmeTR.md">Türkçe</a>
  </p>
</div>
