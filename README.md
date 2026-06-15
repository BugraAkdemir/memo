<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="120"/>

  <h1>Memo</h1>
  <p><b>The AI assistant that learns your habits and acts before you ask.</b></p>
  <p>Local-first · Privacy-first · Zero cloud dependency</p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download-memo.bugradev.com-B08D57?style=for-the-badge" alt="Download"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57" alt="Stars"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/License-AGPL_v3-blue?style=for-the-badge" alt="License"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Status-v3.1_beta-blue?style=for-the-badge" alt="Status"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.25-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-bundled-orange?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_vec0-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-integrated-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-lightgrey?style=flat-square" alt="Platform"/>

</div>

---

<!-- SCREENSHOT: Ana ekran görüntüsü buraya gelecek -->
<!-- ![Memo Demo](docs/assets/demo.gif) -->

---

## What is Memo?

Most AI assistants are chat boxes — you type, they answer, they forget.

**Memo is different.** It watches how you work, learns your patterns, and starts helping *before you ask*. It knows you code every evening at 21:00. It knows Monday mornings mean planning. When the time comes, it shows up — suggests, reminds, or just starts the agent on its own.

Everything runs on your machine. Your conversations, your memory, your habits — none of it leaves your computer.

---

## See it in action

<!-- SCREENSHOT: Proaktif öneri ekranı -->
<!-- ![Proactive Suggestion](docs/assets/proactive.gif) -->

<!-- SCREENSHOT: RAG hafıza arama -->
<!-- ![Memory Search](docs/assets/memory.gif) -->

<!-- SCREENSHOT: WhatsApp entegrasyonu -->
<!-- ![WhatsApp](docs/assets/whatsapp.gif) -->

<!-- SCREENSHOT: Orchestra modu -->
<!-- ![Orchestra](docs/assets/orchestra.gif) -->

> 📸 *Screenshots and demo GIFs coming with stable release.*

---

## Features

### 🧠 Proactive Learning Engine
Memo observes your usage patterns silently. After a few days it starts recognising them — when you code, when you write, when you plan. The built-in proactive engine matches your current time against learned patterns and asks an Orchestra Chief LLM to decide what to do: suggest, notify your phone, or start the agent automatically. It forgets patterns that fade, and immediately drops any pattern you say you no longer follow. **Fully local, opt-in, transparent.**

```
Day 1-7   →  Silent observation only
Day 8-14  →  Patterns form, gentle suggestions begin
Day 30+   →  High-confidence patterns can trigger the agent automatically
```

### 💬 WhatsApp Integration
Full WhatsApp Web pairing via QR code. Send and receive messages, search your chat history, and let the agent read and respond — all stored locally via whatsmeow. No WhatsApp API fees, no data leaving your device.

### 🤖 Agent Engine
Tool-calling pipeline with 8 built-in tools: file read/write, shell commands, web search, memory query, and more. A 6-policy permission system (allow/deny — once / this session / forever) gives you full control over what the agent can touch.

### 🎵 Orchestra Mode
Multi-model collaboration. A Chief model breaks a task into roles, expert models execute in parallel, the Chief synthesises the result. 8 built-in roles. Supports mixing local and external models in the same pipeline.

### 🧩 Skill System
Drop a `SKILL.md` file into `data/skills/` and Memo gains a new capability — a custom persona, a domain expert, a specialised workflow. No code required.

### ⚡ Local llama.cpp
Bundled `llama-server` with full lifecycle management: auto-start, GPU offloading with automatic VRAM detection (NVIDIA / AMD / Metal). No Docker, no containers, no PATH configuration.

### 🏪 Model Store
Curated model recommendations with a **hardware-fit badge** computed from your actual RAM and VRAM. One click downloads and auto-picks the right quantization. No cryptic `Q4_K_M` guessing.

### 🔌 Provider Agnostic
OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama — or any OpenAI-compatible local server. Mix providers: use a powerful external API for chat while a tiny local model handles embeddings.

### 📱 Mobile Companion
A Flutter mobile app (Android/iOS) connects over LAN or via a built-in ngrok tunnel. Proactive suggestions arrive as mobile notifications. Full streaming chat on your phone.

### 📦 Backup & Sync
Full `.memo` ZIP export — sessions, memory, config, WhatsApp data, providers. Optional encrypted Google Drive sync (AES-256-GCM). Double-confirm data wipe.

### 🔒 Privacy by Construction
- Zero telemetry
- No training on your conversations
- No cloud dependency (optional sync only if you enable it)
- Incognito mode: messages not stored, not observed, not embedded
- Observation layer stores only topic labels and word counts — never message text

---

## Quick Start

**No terminal. No build steps. One click.**

| Platform | Download | How |
|----------|----------|-----|
| **Windows** | `Memo-Setup.exe` | Run installer → done |
| **Linux** | `.AppImage` | `chmod +x` → launch |
| **Linux** | `.deb` | `sudo dpkg -i` → done |

llama.cpp is bundled. First launch copies everything it needs to `~/.memo`. Open the app, go to **Model Store**, pick a model, start chatting.

<div align="center">
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download_Memo-memo.bugradev.com-B08D57?style=for-the-badge" alt="Download"/>
  </a>
</div>

<details>
<summary><b>Build from source</b></summary>

**Prerequisites:** Go 1.25+ · Flutter 3.10+

```bash
# Backend
git clone https://github.com/BugraAkdemir/memo.git
cd memo
go run . --port 8090

# Frontend (separate terminal)
cd frontend
flutter run -d linux
```

Release packages:
```bash
./build_releases.sh     # Linux  → AppImage / deb / tar.gz
.\build_releases.bat    # Windows → Inno Setup installer / zip
```
</details>

---

## Architecture

```
┌─────────────────────────────┐    ┌──────────────────────────┐
│   Flutter Desktop (Linux/   │    │   Flutter Mobile         │
│   Windows)                  │    │   (Android / iOS)        │
│                             │    │                          │
│  Chat · Agent · Orchestra   │    │  Chat · Notifications    │
│  Settings · Model Store     │    │  Remote connect          │
└──────────────┬──────────────┘    └───────────┬──────────────┘
               │  REST + SSE (:8090)            │  LAN / ngrok
               └──────────────┬─────────────────┘
┌─────────────────────────────┴──────────────────────────────────┐
│                        Go Backend                               │
│                                                                 │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │  Web Server │  │  App Engine  │  │  Proactive Engine      │ │
│  │  ~35 routes │  │  (internal/  │  │  Observer → Analyzer   │ │
│  │  SSE stream │  │   app/)      │  │  → Chief → Act         │ │
│  └─────────────┘  └──────┬───────┘  └────────────────────────┘ │
│                           │                                     │
│  ┌────────┐ ┌──────────┐ ┌┴─────────┐ ┌──────────┐ ┌────────┐ │
│  │ Memory │ │ Sessions │ │ Llama +  │ │WhatsApp  │ │ Agent  │ │
│  │ SQLite │ │          │ │ Embedding│ │whatsmeow │ │ Engine │ │
│  │ vec0   │ │          │ │ GPU/RAM  │ │          │ │8 tools │ │
│  └────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                                 │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐ ┌──────────┐          │
│  │Orchestra │ │  Model   │ │  Cloud   │ │  ngrok   │          │
│  │ 8 roles  │ │  Store   │ │  Sync    │ │  Tunnel  │          │
│  └──────────┘ └──────────┘ └──────────┘ └──────────┘          │
└────────────────────────────────────────────────────────────────┘
```

**Docs:** [Architecture](docs/architecture.md) · [API Reference](docs/API_REFERENCE.md) · [Design System](frontend/DESIGN.md)

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.25, `net/http`, SSE streaming |
| **Desktop Frontend** | Flutter 3.10, Riverpod 2.x, Dio |
| **Mobile Frontend** | Flutter 3.10, Riverpod 2.x, Dio |
| **LLM Runtime** | llama.cpp (bundled), OpenAI-compatible API |
| **External Providers** | OpenAI · Anthropic · Gemini · Grok · Groq · OpenRouter · Ollama |
| **Vector Store** | SQLite + sqlite-vec (vec0 ANN index) |
| **WhatsApp** | whatsmeow (multi-device Web API) |
| **Learning Engine** | Custom observer + circular-stats analyzer + Orchestra Chief |
| **Hardware Detection** | nvidia-smi · rocm-smi · /proc · GlobalMemoryStatusEx |
| **Cloud Sync** | Google Drive OAuth2 + AES-256-GCM |
| **Build** | Go toolchain · Flutter build · Inno Setup · AppImage |
| **License** | GNU AGPL v3 |

---

## Roadmap

| Version | Theme | Status |
|---------|-------|--------|
| **v3.1** | RAG memory · WhatsApp · Backup · Mobile · Remote access · Proactive engine | ✅ Beta |
| **v3.2** | Stable release · UI polish · Proactive UI · Mobile notifications | 🚧 In progress |
| **v3.3** | Voice assistant · Mobile v2 · Calendar integration | 📅 Planned |
| **v3.4** | Plugin system · Web search · Browser extension | 📅 Planned |
| **v3.5** | Knowledge graph · Self-improving memory | 🔮 Future |

[Full roadmap →](docs/ROADMAP.md)

---

## Documentation

| | |
|-|-|
| [🏛️ Architecture](docs/architecture.md) | Technical deep dive |
| [📡 API Reference](docs/API_REFERENCE.md) | All REST endpoints |
| [🎨 Design System](frontend/DESIGN.md) | "Pewter Study" UI tokens and components |
| [🛣️ Roadmap](docs/ROADMAP.md) | Release plan |
| [📱 Mobile](mobile/README.md) | Mobile companion docs |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | Common issues |
| [📝 Contributing](docs/CONTRIBUTING.md) | How to contribute |

---

## Contributing

Memo is AGPL-3.0 licensed and open to contributions.

- Browse [Known Issues](docs/KNOWN_ISSUES.md) for good first tasks
- Check the [Roadmap](docs/ROADMAP.md) for planned features
- Open a [Discussion](https://github.com/BugraAkdemir/memo/discussions) for ideas

---

<div align="center">
  <br/>
  <p><b>Your mind. Your data. Your machine.</b></p>
  <p>Built by <a href="https://github.com/BugraAkdemir">Buğra Akdemir</a></p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Bug Report</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Discussion</a> ·
  <a href="READmeTR.md">Türkçe</a>
</div>
