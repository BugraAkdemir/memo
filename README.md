<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="160"/>

  <h1>Memo</h1>

  <h3>The AI assistant that learns your habits<br>and acts before you ask.</h3>

  <p>
    <sub>Local-first · Privacy-first · Zero cloud dependency · Full offline capable</sub>
  </p>

  <br/>

  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download-memo.bugradev.com-B08D57?style=for-the-badge&logoColor=white" alt="Download"/>
  </a>
  &nbsp;
  <a href="https://github.com/BugraAkdemir/memo/stargazers">
    <img src="https://img.shields.io/github/stars/BugraAkdemir/memo?style=for-the-badge&color=B08D57&logo=github&logoColor=white" alt="Stars"/>
  </a>
  &nbsp;
  <img src="https://img.shields.io/badge/License-AGPL_v3-0a0a0a?style=for-the-badge" alt="License"/>
  &nbsp;
  <img src="https://img.shields.io/badge/Version-v3.1.0_beta-B08D57?style=for-the-badge" alt="Version"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-bundled-F08705?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_+_vec0-0F9D58?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-integrated-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-6e6e6e?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-passing-0F9D58?style=flat-square&logo=githubactions" alt="CI"/>

</div>

---

## Why Memo Is Different

Most AI assistants are just a text box connected to a model. You type. It replies. The conversation ends. Nothing persists, nothing learns, nothing acts.

Memo breaks that pattern.

<table>
<tr>
<td align="center" width="33%">
  <h3>🧠 Remembers</h3>
  <p>Every conversation is embedded into a local vector database. Weeks later, Memo still recalls what you discussed — no cloud needed.</p>
  <sub><a href="#-rag-memory">Learn more →</a></sub>
</td>
<td align="center" width="33%">
  <h3>📊 Learns</h3>
  <p>Background observer detects your activity rhythms. After a week, Memo knows when you code, plan, or take breaks — and starts anticipating.</p>
  <sub><a href="#-proactive-learning-engine">Learn more →</a></sub>
</td>
<td align="center" width="33%">
  <h3>⚡ Acts</h3>
  <p>Agent engine with 8 real tools: read/write files, run commands, search the web, message on WhatsApp. All sandboxed, all permission-gated.</p>
  <sub><a href="#-agent-engine">Learn more →</a></sub>
</td>
</tr>
</table>

> **It's not a chatbot. It's an assistant that watches, remembers, learns, and does — all on your hardware, with zero telemetry.**

---

## Features

### 💬 Chat

<table>
<tr>
<td width="65%">

Streaming token-by-token responses. Full Markdown — code blocks with syntax highlighting, tables, images. Attach files (text, PDF, code) and images (vision-capable models can see them). Incognito mode leaves zero trace. Web search toggle enriches every reply with live DuckDuckGo results. WhatsApp mode lets you chat through your WhatsApp account in the same interface.

Type `/` for a slash-command palette. Press-and-hold the mic for on-device voice input via whisper.cpp. Edit or delete any message — context updates accordingly. Export chats as Markdown with one click.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/chatscreen.png" alt="Chat Screen" width="100%"/>
</td>
</tr>
</table>

---

### 🧠 RAG Memory

<table>
<tr>
<td width="65%">

Every user+assistant exchange is embedded into a 768-dimensional vector using a local embedding model, then stored in SQLite with sqlite-vec ANN indexing. When you ask something new, Memo retrieves the top-K most semantically similar past conversations and injects them into the system prompt — so the model already has context.

No Pinecone. No embeddings API. No cloud vector database. Just SQLite and your GPU.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/setting-sc.png" alt="Memory Settings" width="100%"/>
</td>
</tr>
</table>

---

### 🤖 Agent Engine

<table>
<tr>
<td width="60%">

The agent goes from <em>talking</em> to <em>doing</em>. Pick a project folder and the agent can read, write, edit, and delete files; list directories; run shell commands; search the web. 8 built-in tools running inside a sandbox with path validation, symlink protection, and a command blacklist.

Every tool call triggers a permission dialog — allow or deny once, for the session, or permanently. Up to 20 iterations per pipeline, 60s timeout per tool. Cancel anytime.

<details>
<summary>🎬 <b>Watch demo</b></summary>
<br/>
<video src="docs/assets/videos/agent.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/agent-sc.png" alt="Agent Screen" width="100%"/>
</td>
</tr>
</table>

---

### 🎵 Orchestra — Multi-Model

<table>
<tr>
<td width="65%">

A Chief model decomposes complex tasks, delegates to 8 specialist roles (Planner, Frontend, Backend, Bug Fixer, Reviewer, Security Auditor, DevOps, Generalist), then synthesizes results. Assign different providers and models per role — Claude for reasoning, Gemini for speed, local llama.cpp for code.

Independent roles run in parallel. Orchestra + Agent = the Chief plans, the Agent executes step by step.

</td>
<td align="center" width="35%">

| Role | Best For |
|------|----------|
| Planner | Structured plans |
| Frontend | UI, styling |
| Backend | APIs, databases |
| Reviewer | Code quality |
| Security | Vulnerability check |
| DevOps | CI/CD, Docker |
| Bug Fixer | Debugging |
| Generalist | Everything else |

</td>
</tr>
</table>

---

### 📱 WhatsApp Integration

<table>
<tr>
<td width="60%">

Connect via QR code — same protocol as WhatsApp Web. No Business API fees. Once paired: read, search, and reply to messages right inside Memo. Profile photos are cached, and the UI matches Memo's bronze theme.

The agent can send WhatsApp messages, search conversations, and resolve contact names — say "message Berra" without knowing her JID. WhatsApp messages feed into RAG memory, proactive observer, and calendar intent extractor.

<details>
<summary>🎬 <b>Watch demo</b></summary>
<br/>
<video src="docs/assets/videos/whats-my-name.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/whatsaap-qr-sc.png" alt="WhatsApp QR" width="100%"/>
</td>
</tr>
</table>

---

### 📅 Smart Calendar

<table>
<tr>
<td width="60%">

Not a calendar you fill out manually — an automatic capture system. Two-stage intent pipeline: fast keyword filter scans every message for time patterns ("tomorrow", "next Tuesday", "at 3pm"), then only matching messages hit an LLM for structured event extraction.

Reminders fire on desktop and mobile. Unsure about an event ("maybe tomorrow?") → ambiguity event with one-click confirm/delete. Atomic SQL transactions prevent duplicate reminders.

<details>
<summary>🎬 <b>Watch demo</b></summary>
<br/>
<video src="docs/assets/videos/calendar.mp4" controls width="100%"></video>
</details>

</td>
<td align="center" width="40%">
  <img src="docs/assets/screen/clander-dc.png" alt="Calendar" width="100%"/>
</td>
</tr>
</table>

---

### 🧠 Proactive Learning Engine

Memo is the only AI assistant that comes to <em>you</em>. The observer tracks <strong>when</strong> you're active — not what you say. Circular (directional) statistics detects rhythms: "Monday mornings, 9-10am, planning" or "Daily, 9pm-11pm, coding."

When confidence crosses the threshold, the proactive engine consults an LLM and decides: suggest something helpful, push a notification, or (at high confidence) auto-start the agent. Opt-in, transparent, and patterns that fade are forgotten. Only topic labels and word counts are stored — never raw message text.

<details>
<summary>🎬 <b>Watch demo</b></summary>
<br/>
<video src="docs/assets/videos/moods-and-ozcıkar.mp4" controls width="100%"></video>
</details>

---

### 🏪 Model Store

<table>
<tr>
<td width="65%">

Curated models with <strong>hardware-fit badges</strong> — computed from your actual RAM and VRAM, not a static list. "Fits your device — fast on GPU" or "Runs (CPU)" or "Too large."

Plain-language quality labels replace raw quantization codes: "Balanced quality" (Q4_K_M), "High quality" (Q5_K_M). Real company logos fetched from HuggingFace. Filter by size (1-8B, 8-14B, 14B+) and capability (Tools, Vision, Code). Download with real-time progress, cancel anytime. One-click Start auto-configures GPU offloading.

</td>
<td align="center" width="35%">
  <img src="docs/assets/screen/models-discorver-sc.png" alt="Model Store Discover" width="100%"/>
  <br/>
  <img src="docs/assets/screen/models-my-sc.png" alt="My Models" width="100%"/>
</td>
</tr>
</table>

---

### 🔌 8 Providers, One Interface

OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama, and any local llama.cpp server.

**Production-grade provider system:** auto-fallback on failure, auto-disable after 3 consecutive errors, background health check every 5 minutes, live provider switching mid-conversation via `/model`. Per-model configurable context windows. API keys encrypted with AES-256-GCM using a machine-generated random key.

---

### 🎤 Voice · ☁️ Cloud Sync · 🔒 Privacy

- **Voice Input** — on-device speech-to-text via whisper.cpp. Press and hold to speak. Auto-detects TR/EN. Audio never leaves your machine.
- **Cloud Sync** — E2E encrypted Google Drive backup. AES-256-GCM + PBKDF2 (600K iterations). Encrypted before upload — Google can't read your data.
- **Privacy by Design** — no telemetry, no analytics, no crash reporting. Config files at 0600 permissions. Incognito mode. Observer stores only activity timestamps, never message content.

---

## Quick Start

**No terminal. No build steps. One click.**

| Platform | Download | How |
|----------|----------|-----|
| **Windows** | `Memo-Setup.exe` | Run installer |
| **Linux** | `.AppImage` | `chmod +x` → launch |
| **Linux** | `.deb` | `sudo dpkg -i` |

llama.cpp is bundled. Open the app, go to **Model Store**, pick a model, start chatting.

<div align="center">
  <br/>
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download_Memo-memo.bugradev.com-B08D57?style=for-the-badge" alt="Download"/>
  </a>
</div>

<details>
<summary><b>Build from source</b></summary>
<br/>

**Prerequisites:** Go 1.26+ · Flutter 3.10+ · SQLite dev libraries (for CGO)

```bash
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090          # Backend
cd frontend && flutter run -d linux          # Frontend
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
┌─────────────────────────────────┐    ┌──────────────────────────┐
│  Flutter Desktop (Linux/Windows) │    │  Flutter Mobile           │
│                                  │    │  (Android / iOS)          │
│  Chat · Agent · Orchestra        │    │  Chat · Notifications     │
│  Settings · Model Store          │    │  Remote connect           │
└──────────────┬───────────────────┘    └───────────┬───────────────┘
               │  REST + SSE (:8090)                 │  LAN / ngrok
               └──────────────┬──────────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│               Go Backend — 25 packages, ~90 endpoints            │
│                                                                  │
│  ┌─────────┐ ┌──────────┐ ┌────────────────┐                    │
│  │ HTTP    │ │ App      │ │ Proactive      │                    │
│  │ ServeMux│ │ Engine   │ │ Observer→Act   │                    │
│  └─────────┘ └────┬─────┘ └────────────────┘                    │
│                    │                                             │
│  ┌────────┐ ┌──────┐ ┌──────┐ ┌────────┐ ┌──────┐ ┌────────┐  │
│  │ Memory │ │Sess. │ │Llama │ │WhatsApp│ │Agent │ │Provider│  │
│  │ vec0   │ │JSON  │ │GPU   │ │whatsmeow│ │Pipe  │ │Router  │  │
│  └────────┘ └──────┘ └──────┘ └────────┘ └──────┘ └────────┘  │
│                                                                  │
│  Orchestra · ModelStore · CloudSync · Calendar · Mood            │
│  ngrok · Tailscale · Whisper · Skills · Intent · Observer        │
└──────────────────────────────────────────────────────────────────┘
```

Two processes over plain HTTP on `localhost:8090`. No TLS (local only). No external router — pure `net/http` ServeMux.

**Docs:** [Architecture](docs/architecture.md) · [API Reference](docs/API_REFERENCE.md) · [Design System](frontend/DESIGN.md)

---

## Tech Stack

<table>
<tr>
<th>Layer</th><th>Technology</th><th>Notes</th>
</tr>
<tr>
<td>Backend</td>
<td><img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt=""/></td>
<td>CGO required for SQLite</td>
</tr>
<tr>
<td>HTTP</td>
<td><code>net/http</code> ServeMux</td>
<td>No external router dependency</td>
</tr>
<tr>
<td>Streaming</td>
<td>SSE (Server-Sent Events)</td>
<td>Token-by-token chat</td>
</tr>
<tr>
<td>Desktop Frontend</td>
<td><img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt=""/></td>
<td>Linux + Windows</td>
</tr>
<tr>
<td>State</td>
<td>Riverpod 2.4</td>
<td>AsyncNotifierProvider</td>
</tr>
<tr>
<td>HTTP Client</td>
<td>Dio 5.4</td>
<td>SSE parsing, interceptors</td>
</tr>
<tr>
<td>Markdown</td>
<td>flutter_markdown 0.6</td>
<td>Code blocks, tables, images</td>
</tr>
<tr>
<td>LLM Runtime</td>
<td>llama.cpp</td>
<td>Bundled binary, subprocess mgmt</td>
</tr>
<tr>
<td>Vector DB</td>
<td>SQLite + sqlite-vec</td>
<td>vec0 ANN, 768-dim embeddings</td>
</tr>
<tr>
<td>WhatsApp</td>
<td>whatsmeow</td>
<td>Multi-device Web API</td>
</tr>
<tr>
<td>Speech-to-Text</td>
<td>whisper.cpp</td>
<td>On-device, bundled binary</td>
</tr>
<tr>
<td>Cloud Sync</td>
<td>Google Drive API v3</td>
<td>AES-256-GCM + PBKDF2</td>
</tr>
<tr>
<td>GPU Detection</td>
<td>nvidia-smi, rocm-smi, sysfs</td>
<td>Auto VRAM measurement</td>
</tr>
<tr>
<td>Logging</td>
<td><code>internal/logx</code></td>
<td>slog wrapper</td>
</tr>
<tr>
<td>CI/CD</td>
<td><img src="https://img.shields.io/badge/GitHub_Actions-passing-0F9D58?style=flat-square&logo=githubactions" alt=""/></td>
<td>Go vet+test+build, Flutter analyze+test</td>
</tr>
<tr>
<td>License</td>
<td>GNU AGPL v3</td>
<td>Free software</td>
</tr>
</table>

---

## Documentation

| | |
|-|-|
| [Architecture](docs/architecture.md) | Package map, data flow, module boundaries |
| [API Reference](docs/API_REFERENCE.md) | All 90+ REST endpoints with request/response schemas |
| [Design System](frontend/DESIGN.md) | "Pewter Study" theme tokens, color palette, typography |
| [Roadmap](docs/ROADMAP.md) | Versioned release plan with feature targets |
| [Mobile](mobile/README.md) | Flutter mobile companion setup and tunnel configuration |
| [Troubleshooting](docs/TROUBLESHOOTING.md) | GPU setup, port conflicts, common errors |
| [Contributing](docs/CONTRIBUTING.md) | Dev setup, code style, PR process |
| [Changelog](versinNote/v3.1.0.md) | Full v3.1.0 feature list, bug fixes, and polish |

---

## Contributing

Memo is AGPL-3.0 licensed. Contributions welcome.

- Check the [Roadmap](docs/ROADMAP.md) for planned features
- Browse [Known Issues](docs/KNOWN_ISSUES.md) for good first tasks
- Open a [Discussion](https://github.com/BugraAkdemir/memo/discussions) for ideas

---

<br/>

<div align="center">
  <p><b>Your mind. Your data. Your machine.</b></p>
  <p>Built by <a href="https://github.com/BugraAkdemir">Buğra Akdemir</a></p>
  <br/>
  <a href="https://github.com/BugraAkdemir/memo/issues">Bug Report</a> ·
  <a href="https://github.com/BugraAkdemir/memo/discussions">Discussion</a> ·
  <a href="READmeTR.md">Türkçe</a>
</div>
