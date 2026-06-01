<div align="center">
  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=for-the-badge&logo=go" alt="Go 1.25"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
  <img src="https://img.shields.io/badge/License-AGPL_v3-blue?style=for-the-badge" alt="License AGPL v3"/>
  <img src="https://img.shields.io/badge/Status-v2.0.0--beta-blue?style=for-the-badge" alt="v2.0.0-beta"/>
  <br/>
  <img src="https://img.shields.io/badge/llama.cpp-Integrated-orange?style=flat-square&logo=llama" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-Enabled-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/Google_Drive_Cloud_Backup-Integrated-blue?style=flat-square&logo=googledrive" alt="Google Drive"/>
  <img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Cross Platform"/>
</div>

<h1 align="center">
  🧠 Memo — The AI Memory Shell
</h1>

<p align="center">
  <b>Your Local. Your Private. Your Second Brain.</b><br/>
  <i>A high-performance, privacy-first Memory Shell for Local LLMs</i>
</p>

<p align="center">
  <a href="#-features">Features</a> •
  <a href="#-quick-start">Quick Start</a> •
  <a href="#-architecture">Architecture</a> •
  <a href="#-screenshots">Screenshots</a> •
  <a href="#-roadmap">Roadmap</a> •
  <a href="docs/tr/README.md">Türkçe</a>
</p>

---

> **⚠️ Current Status:** v2.0.0-beta — Active development. All core features implemented, hardening pass underway for v3.0.0. [See known issues →](docs/KNOWN_ISSUES.md)

---

## ✨ Features

<table>
  <tr>
    <td width="50%">
      <h3>🧠 <b>Local RAG Engine</b></h3>
      Every interaction is semantically indexed. Memo retrieves your past context before every response — it remembers <i>how</i> you think.
    </td>
    <td width="50%">
      <h3>🔒 <b>100% Private</b></h3>
      Zero data leaves your machine. No cloud, no telemetry, no training on your conversations. Your second brain stays yours.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>⚡ <b>Local llama.cpp</b></h3>
      Integrated llama-server management. Auto-download, auto-start, GPU acceleration with VRAM detection. No Docker, no containers.
    </td>
    <td width="50%">
      <h3>💬 <b>Streaming Chat</b></h3>
      Real-time SSE token streaming with thinking/reasoning extraction. Supports multimodal models (text + images).
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🤗 <b>HuggingFace Integration</b></h3>
      Search, download, and manage GGUF models directly from the HuggingFace Hub. One-click model switching.
    </td>
    <td width="50%">
      <h3>☁️ <b>Encrypted Cloud Sync</b></h3>
      Optional Google Drive backup with AES-256-GCM end-to-end encryption. Your passphrase, your key.
    </td>
  </tr>
  <tr>
    <td width="50%">
      <h3>🎨 <b>Greige Design</b></h3>
      Premium Material 3 interface with a warm, minimal greige palette. Dark mode included. Easy on the eyes.
    </td>
    <td width="50%">
      <h3>🛠️ <b>Model Agnostic</b></h3>
      Works with any OpenAI-compatible local server. llama.cpp, Ollama, LM Studio — bring your own model.
    </td>
  </tr>
</table>

---

## 🚀 Quick Start

### Prerequisites
- **Go 1.25+** — [download](https://go.dev/dl/)
- **Flutter 3.10+** — [install](https://docs.flutter.dev/get-started/install)
- **llama.cpp** — bundled! The app ships with pre-built llama-server binaries for your platform. No manual install needed.

### Quick Start (Linux)
```bash
# Clone and run backend
git clone https://github.com/bugra/memo.git && cd memo
go run . --port 8090

# In another terminal, run frontend
cd frontend && flutter run -d linux
```

The first time you start a model, the app uses the bundled llama-server from `binaries/`. GGUF models are downloaded from HuggingFace directly through the UI.

### Build a Portable Release
```bash
./build_releases.sh
# Output: build_output/dist/
#   Memo-linux-x64-v3.5.0.tar.gz
#   Memo-linux-x64-v3.5.0.AppImage
#   Memo-linux-x64-v3.5.0.deb
```

---

## 🏛️ Architecture

```
┌─────────────────────────────────────────────────┐
│            Flutter Desktop Client                │
│  ┌─────────┐  ┌──────────┐  ┌───────────────┐  │
│  │ Chat UI │  │ Settings │  │ Model Store   │  │
│  └────┬────┘  └────┬─────┘  └──────┬────────┘  │
│       └────────────┼───────────────┘            │
│              ┌─────┴──────┐                     │
│              │  Riverpod  │                     │
│              │  Providers │                     │
│              └─────┬──────┘                     │
│              ┌─────┴──────┐                     │
│              │ ApiClient  │                     │
│              │ (Dio/SSE)  │                     │
│              └─────┬──────┘                     │
└────────────────────┼────────────────────────────┘
                     │ REST + SSE (localhost:8090)
┌────────────────────┼────────────────────────────┐
│          Go Backend Server                       │
│  ┌─────────────────┴─────────────────┐          │
│  │          Web Server               │          │
│  │  (server.go + handlers_flutter)   │          │
│  └─────────────────┬─────────────────┘          │
│  ┌─────────────────┴─────────────────┐          │
│  │          app.go (Engine)          │          │
│  └──┬──────────┬──────────┬──────────┘          │
│  ┌──┴──┐  ┌────┴────┐  ┌─┴─────────┐           │
│  │Mem  │  │Sessions │  │Llama Mgr  │           │
│  │Store│  │Manager  │  │(subproc)  │           │
│  │.gob │  │JSON     │  │llama.cpp  │           │
│  └─────┘  └─────────┘  └───────────┘           │
│  ┌──────────┐  ┌──────────────┐                 │
│  │Cloud Sync│  │Model Store  │                  │
│  │Drive+AE  │  │HF API+local │                  │
│  └──────────┘  └──────────────┘                 │
└─────────────────────────────────────────────────┘
```

**Deep dive:** [architecture.md](./architecture.md)

---

## 🖼️ Screenshots

<div align="center">
  <p><i>(Screenshots coming soon — v3.0.0 release)</i></p>
  <table>
    <tr>
      <td align="center"><b>💬 Chat</b></td>
      <td align="center"><b>⚙️ Settings</b></td>
      <td align="center"><b>🤗 Model Store</b></td>
    </tr>
    <tr>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Chat+Screen" alt="Chat Screen"/></td>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Settings" alt="Settings"/></td>
      <td><img src="https://via.placeholder.com/300x200/e8ddd3/333?text=Model+Store" alt="Model Store"/></td>
    </tr>
  </table>
</div>

---

## 📚 Documentation

| Document | Description |
|---|---|
| [📖 Known Issues](docs/KNOWN_ISSUES.md) | Complete audit — 55 known issues with priorities |
| [🛣️ Roadmap](docs/ROADMAP.md) | v3.0 → v4.0 → v5.0 release plan |
| [🏛️ Architecture](architecture.md) | Full technical deep dive |
| [📡 API Reference](docs/API_REFERENCE.md) | All REST endpoints |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | Common problems and solutions |
| [🤝 Contributing](docs/CONTRIBUTING.md) | How to contribute |

---

## 🛣️ Roadmap

| Version | Focus | Timeline |
|---|---|---|
| **v2.0.0** 🎯 | Current — All features live | Now |
| **v3.0.0** 🔒 | Security, stability, performance fix pass | Next |
| **v4.0.0** 🎨 | SQLite migration, UI overhaul, missing tabs | Future |
| **v5.0.0** 🚀 | Plugins, mobile, knowledge graph | Future |

[Full roadmap →](docs/ROADMAP.md)

---

## 🧪 Tech Stack

<div align="center">

| Layer | Technology |
|---|---|
| **Backend** | Go 1.25, chromem-go, gorilla/mux |
| **Frontend** | Flutter 3.10+, Riverpod 2.x, Dio |
| **LLM** | llama.cpp, OpenAI-compatible API |
| **Vector** | chromem-go (in-memory, .gob persist) |
| **Sync** | Google Drive API, AES-256-GCM |
| **GPU** | nvidia-smi, rocm-smi, sysfs, Metal |
| **Build** | Go toolchain, Flutter build, shell scripts |

</div>

---

## 🤝 Contributing

Contributions welcome! See:
- [Known Issues](docs/KNOWN_ISSUES.md) — pick a 🔴 or 🟠 item
- [Task List](task.md) — frontend-specific tasks
- [Contributing Guide](docs/CONTRIBUTING.md)

---

<div align="center">
  <h3>🧠 <i>Your Mind. Your Data. Your Computer.</i></h3>
  <p>Built with ❤️ by <b>Buğra</b></p>
  <p>
    <a href="https://github.com/bugra/memo/issues">Report Bug</a> •
    <a href="https://github.com/bugra/memo/discussions">Discussion</a> •
    <a href="docs/tr/README.md">Türkçe</a>
  </p>
</div>
