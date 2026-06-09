<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go" alt="Go 1.25"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/License-AGPL_v3-blue?style=for-the-badge" alt="License AGPL v3"/>
  <img src="https://img.shields.io/badge/Status-v3.1.0--beta-blue?style=for-the-badge" alt="v3.1.0-beta"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Integrated-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Enabled-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-Integrated-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Backup-.memo_ZIP-blue?style=flat-square" alt="Backup"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Cross Platform"/>
</div>

<h1 align="center">
  🧠 Memo — The AI Memory Shell
</h1>

<p align="center">
  <b>Your Local. Your Private. Your Second Brain.</b><br/>
  <i>A privacy-first, local-first AI assistant with persistent memory, WhatsApp integration, and smart automation</i>
</p>

<p align="center">
  <a href="#-features">Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="#-roadmap">Roadmap</a> •
  <a href="#-documentation">Documentation</a> •
  <a href="READmeTR.md">Türkçe</a>
</p>

---

> **Current Version:** v3.1.0-beta — RAG memory, WhatsApp integration, local embedding, backup/restore. [See roadmap →](docs/ROADMAP.md)

---

## ✨ Features

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Local RAG Engine</b></h3>
      SQLite + sqlite-vec ANN vector index. Every interaction is semantically indexed. O(log n) retrieval at any scale. No cloud, no third-party embeddings.
    </td>
    <td width="50%">
      <h3>💬 <b>WhatsApp Integration</b></h3>
      Full WhatsApp Web pairing via QR. Send/receive messages, contact name resolution, whitelist-based file transfer, dedicated agent tools. Your chats stay local.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🔒 <b>100% Private</b></h3>
      Zero data leaves your machine. No telemetry, no training on your conversations. Your second brain stays yours. Optional encrypted cloud backup if you choose.
    </td>
    <td width="50%">
      <h3>⚡ <b>Local llama.cpp</b></h3>
      Integrated llama-server management. Auto-download, auto-start, GPU acceleration with VRAM detection. No Docker, no containers, no cloud dependency.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🔄 <b>Cross-Mode Architecture</b></h3>
      Use external API providers (OpenAI, Claude, Gemini, etc.) for chat while a tiny local model handles embeddings. Best of both worlds — power + privacy.
    </td>
    <td width="50%">
      <h3>📦 <b>Backup & Restore</b></h3>
      Full .memo zip export/import — sessions, config, memory, WhatsApp data, providers. Wipe all data with double confirmation. Your data, fully portable.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🤗 <b>HuggingFace Integration</b></h3>
      Search, download, and manage GGUF models directly from the HuggingFace Hub. One-click model switching. Built-in model store with search and filters.
    </td>
    <td width="50%">
      <h3>🎨 <b>Greige Design</b></h3>
      Premium Material 3 interface with a warm, minimal greige palette. Dark mode included. Clean, focused, easy on the eyes during long sessions.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🛠️ <b>Model Agnostic</b></h3>
      Works with any OpenAI-compatible local server. llama.cpp, Ollama, LM Studio — bring your own model. External providers: OpenAI, Anthropic, Google, Grok, Groq, OpenRouter, Ollama.
    </td>
    <td width="50%">
      <h3>🖥️ <b>Cross-Platform</b></h3>
      Linux, Windows, macOS support. Native desktop apps via Flutter. Windows installer with Inno Setup. Your assistant, anywhere you run it.
    </td>
  </tr>
</table>

---

## 🚀 Quick Start

### Prerequisites
- **Go 1.25+** — [download](https://go.dev/dl/)
- **Flutter 3.10+** — [install](https://docs.flutter.dev/get-started/install)
- **llama.cpp** — bundled! Pre-built binaries for your platform. No manual install needed.

### Development
```bash
# Terminal 1 — Backend
git clone https://github.com/BugraAkdemir/memo.git && cd memo
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

### Build a Release
```bash
# Linux
./build_releases.sh
# Output: build_output/dist/Memo-linux-x64-v3.1.0.tar.gz / .AppImage / .deb

# Windows
.\build_releases.bat
# Output: Memo-Setup-x64-v3.1.0.exe (Inno Setup) or Memo-win-x64-v3.1.0.zip
```

---

## 🏛️ Architecture

```
┌──────────────────────────────────────────────────────┐
│                 Flutter Desktop Client                │
│  ┌──────────┐  ┌──────────┐  ┌────────┐  ┌────────┐│
│  │ Chat UI  │  │Settings  │  │Model   │  │WhatsApp││
│  │ + Agent  │  │+ Backup  │  │Store   │  │Screen  ││
│  └────┬─────┘  └────┬─────┘  └───┬────┘  └───┬────┘│
│       └──────────────┼────────────┼───────────┘     │
│                ┌─────┴────────────┴──────┐           │
│                │   Riverpod Providers     │           │
│                │  + SSE Stream Handler   │           │
│                └─────┬───────────────────┘           │
│                ┌─────┴───────────────────┐           │
│                │    MemoApiClient (Dio)   │           │
│                └─────┬───────────────────┘           │
└──────────────────────┼───────────────────────────────┘
                       │ REST + SSE (localhost:8090)
┌──────────────────────┼───────────────────────────────┐
│               Go Backend Server                       │
│  ┌────────────────────┴────────────────────┐          │
│  │          Web Server (server.go)          │          │
│  │   ~35 endpoints (handlers_flutter.go)    │          │
│  └────────────────────┬────────────────────┘          │
│  ┌────────────────────┴────────────────────┐          │
│  │          App Engine (app.go)             │          │
│  └──┬──────────┬──────────┬──────────┬──────┘          │
│  ┌──┴──┐  ┌────┴────┐  ┌─┴─────────┐  ┌─┴─────────┐   │
│  │Mem  │  │Sessions │  │Llama Mgr  │  │WhatsApp   │   │
│  │Store│  │Manager  │  │(subproc)  │  │Client     │   │
│  │vec0 │  │JSON     │  │llama.cpp  │  │whatsmeow  │   │
│  │SQLite│  │         │  │+ Emb Mgr │  │msg store  │   │
│  └─────┘  └─────────┘  └───────────┘  └────────────┘   │
│  ┌──────────┐  ┌──────────────┐  ┌──────────┐          │
│  │Provider  │  │Model Store   │  │Orchestra │          │
│  │Router    │  │HF API+local  │  │Conductor │          │
│  │(6 types) │  │              │  │(8 roles) │          │
│  └──────────┘  └──────────────┘  └──────────┘          │
└────────────────────────────────────────────────────────┘
```

**Deep dive:** [docs/architecture.md](docs/architecture.md)

---

## 🛣️ Roadmap

| Version | Theme | Status |
|---------|-------|--------|
| **v3.1.0** | Memory — RAG, WhatsApp, Backup, Local Embedding | ✅ Released |
| **v3.2.0** | Scheduled Intelligence — Calendar, Cron, Voice, Smart Home | 🚧 In Development |
| **v3.3.0** | Mobile Companion — Thin mobile client, remote access | 🚧 Planned |
| **v3.4.0** | Personal Model — Fine-tune 1.2B model on your conversations | 🔮 Future |
| **v3.5.0** | Ecosystem — Plugins, Knowledge Graph, Multi-User | 🔮 Future |

[Full roadmap →](docs/ROADMAP.md)

---

## 📚 Documentation

| Document | Description |
|----------|-------------|
| [🛣️ Roadmap](docs/ROADMAP.md) | Full strategic vision and release plan |
| [🏛️ Architecture](docs/architecture.md) | Technical deep dive into components |
| [📡 API Reference](docs/API_REFERENCE.md) | All REST endpoints |
| [📖 Known Issues](docs/KNOWN_ISSUES.md) | Complete audit with priorities |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | Common problems and solutions |
| [📝 Contributing](docs/CONTRIBUTING.md) | How to contribute |

---

## 🧪 Tech Stack

<div align="center">

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, http.ServeMux |
| **Frontend** | Flutter 3.10+, Riverpod 2.x, Dio, flutter_markdown |
| **LLM Runtime** | llama.cpp (bundled), OpenAI-compatible API |
| **External Providers** | OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama |
| **Vector Store** | SQLite + sqlite-vec (vec0 ANN index) |
| **WhatsApp** | whatsmeow (multi-device Web API) |
| **GPU** | nvidia-smi, rocm-smi, sysfs, Metal |
| **Build** | Go toolchain, Flutter build, shell scripts, Inno Setup |
| **License** | GNU AGPL v3 |

</div>

---

## 🤝 Contributing

Contributions welcome! See:
- [Known Issues](docs/KNOWN_ISSUES.md) — pick an item to work on
- [Roadmap](docs/ROADMAP.md) — see upcoming features
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
