<div align="center">

  <img src="docs/assets/logo.png" alt="Memo Logo" width="120"/>

  <h1>Memo</h1>
  <p><b>The AI assistant that learns your habits and acts before you ask.</b></p>
  <p>Local-first · Privacy-first · Zero cloud dependency · Full offline capable</p>

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
  <img src="https://img.shields.io/badge/Version-v3.1.0_beta-blue?style=for-the-badge" alt="Version"/>

  <br/><br/>

  <img src="https://img.shields.io/badge/Go-1.26-00ADD8?style=flat-square&logo=go" alt="Go"/>
  <img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=flat-square&logo=flutter" alt="Flutter"/>
  <img src="https://img.shields.io/badge/llama.cpp-bundled-orange?style=flat-square" alt="llama.cpp"/>
  <img src="https://img.shields.io/badge/RAG-SQLite_vec0-green?style=flat-square" alt="RAG"/>
  <img src="https://img.shields.io/badge/WhatsApp-integrated-25D366?style=flat-square&logo=whatsapp" alt="WhatsApp"/>
  <img src="https://img.shields.io/badge/Platform-Linux_|_Windows-lightgrey?style=flat-square" alt="Platform"/>
  <img src="https://img.shields.io/badge/CI-passing-success?style=flat-square&logo=githubactions" alt="CI"/>

</div>

---

## What is Memo?

Most AI assistants are chat boxes — you type, they answer, they forget.

**Memo is different.** It watches how you work, learns your patterns, and starts helping *before you ask*. It knows you code every evening at 21:00. It knows Monday mornings mean planning. When the time comes, it shows up — suggests, reminds your phone, or starts the agent on its own.

Everything runs on your machine. No cloud dependency. No telemetry. Your conversations, your memory, your habits — none of it leaves your computer. Use it fully offline with local models, or plug in API keys for cloud providers — the choice is yours.

---

## Features at a Glance

| Category | What it does |
|----------|-------------|
| **Chat** | Multi-turn AI chat with streaming, markdown rendering, image/file attachments, incognito mode. Fully bilingual TR/EN. |
| **RAG Memory** | SQLite + sqlite-vec vector store remembers your conversations and retrieves relevant context automatically. |
| **Agent Engine** | 8 built-in tools (file ops, shell commands, web search, WhatsApp). 6-policy permission system. 60s per-tool timeout. |
| **Orchestra** | Multi-model workflow: a Chief model decomposes tasks, 8 specialist roles execute in parallel, results are synthesized. |
| **WhatsApp** | Full WhatsApp Web via QR pairing (whatsmeow). Read, reply, search, summarize — no API fees, fully local. |
| **Calendar** | Intent extraction from conversations → auto-event creation → reminder notifications. Manual events also supported. |
| **Proactive** | 7-day silent observation → pattern detection → gentle suggestions → high-confidence auto-agent execution. |
| **Skill System** | Drop a `SKILL.md` into `data/skills/` to add personas, domain experts, or workflows. No code required. |
| **Model Store** | HuggingFace browser with hardware-fit badges (RAM/VRAM). One-click download with smart quantization selection. |
| **Voice (Whisper)** | On-device speech-to-text via whisper.cpp. Auto-detect TR/EN/mixed. No internet required. |
| **Cloud Sync** | Optional E2E-encrypted Google Drive backup (AES-256-GCM, PBKDF2 600K iterations). Multi-device passphrase. |
| **Remote Access** | Built-in ngrok + Tailscale (tsnet) tunnels. LAN access with auto-CORS. Mobile companion over tunnel. |
| **Onboarding UX** | Setup wizard → launchpad feature cards → spotlight icon tour → explanatory empty states. Everything labeled. |
| **GPU Auto-Detect** | NVIDIA (CUDA), AMD (ROCm/Vulkan), Metal. Automatic VRAM detection. Recommended engine mode. |
| **8 Providers** | OpenAI, Anthropic, Gemini, Grok, Groq, OpenRouter, Ollama, llama.cpp — with smart fallback chain. |
| **Dual Language** | Full TR/EN i18n with 300+ L10n keys. Calendar, Settings, Chat, Onboarding all bilingual. |
| **Production Ready** | Rate limiting (100 req/s/IP), 50MB body limit, 0600 file permissions, `crypto/rand` key derivation, CI/CD pipeline. |

---

## Quick Start

**No terminal. No build steps. One click.**

| Platform | Download | How |
|----------|----------|-----|
| **Windows** | `Memo-Setup.exe` | Run installer → done |
| **Linux** | `.AppImage` | `chmod +x` → launch |
| **Linux** | `.deb` | `sudo dpkg -i` → done |

llama.cpp is bundled. First launch copies everything to `~/.memo`. Open the app, go to **Model Store**, pick a model, start chatting.

<div align="center">
  <a href="https://memo.bugradev.com">
    <img src="https://img.shields.io/badge/⬇_Download_Memo-memo.bugradev.com-B08D57?style=for-the-badge" alt="Download"/>
  </a>
</div>

<details>
<summary><b>Build from source</b></summary>

**Prerequisites:** Go 1.26+ · Flutter 3.10+ · SQLite dev libraries (for CGO)

```bash
# Backend
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090

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
│  Flutter Desktop (Linux/     │    │  Flutter Mobile           │
│  Windows)                    │    │  (Android / iOS)          │
│                              │    │                           │
│  Chat · Agent · Orchestra    │    │  Chat · Notifications     │
│  Settings · Model Store      │    │  Remote connect           │
└──────────────┬───────────────┘    └───────────┬───────────────┘
               │  REST + SSE (:8090)             │  LAN / ngrok
               └──────────────┬──────────────────┘
┌──────────────────────────────┴──────────────────────────────────┐
│                       Go Backend (25 packages)                    │
│                                                                  │
│  Web Server ── App Engine ── Proactive Engine                    │
│  (~55 routes)    (app/)        Observer → Analyzer → Chief → Act │
│       │              │                                           │
│  ┌────┴────┬─────────┼──────────┬──────────┬──────────┐         │
│  │ Memory  │ Provider│ Llama    │ Agent    │ WhatsApp │         │
│  │ vec0    │ Router  │ GPU      │ Pipeline │ whatsmeow│         │
│  └─────────┴─────────┴──────────┴──────────┴──────────┘         │
│                                                                  │
│  Orchestra  ·  Model Store  ·  Cloud Sync  ·  Calendar           │
│  ngrok      ·  Tailscale    ·  Whisper     ·  Skills             │
└──────────────────────────────────────────────────────────────────┘
```

**Two-process, plain HTTP.** Frontend communicates with backend over `localhost:8090` via REST + SSE streaming. No TLS (local only). No router framework — pure `net/http` ServeMux.

**Docs:** [Architecture](docs/architecture.md) · [API Reference](docs/API_REFERENCE.md) · [Design System](frontend/DESIGN.md)

---

## Tech Stack

| Layer | Technology |
|-------|-----------|
| **Backend** | Go 1.26, `net/http`, CGO for SQLite |
| **Desktop Frontend** | Flutter 3.10, Riverpod 2.4, Dio 5.4, flutter_markdown 0.6 |
| **Mobile Frontend** | Flutter 3.10, Riverpod, Dio |
| **LLM Runtime** | llama.cpp (bundled binary), OpenAI-compatible HTTP API |
| **Vector Store** | SQLite + sqlite-vec (vec0 ANN index, 768-dim) |
| **WhatsApp** | whatsmeow (Go, multi-device Web API) |
| **Speech-to-Text** | whisper.cpp (bundled binary) |
| **Cloud Sync** | Google Drive API v3, OAuth2, AES-256-GCM, PBKDF2 |
| **GPU Detection** | nvidia-smi, rocm-smi, sysfs, GlobalMemoryStatusEx |
| **Logging** | `internal/logx` — structured slog wrapper with levels |
| **CI/CD** | GitHub Actions: Go vet + test + build, Flutter analyze + test |
| **License** | GNU AGPL v3 |

---

## How It Works

```
┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  You chat    │ ──→ │  RAG Memory  │ ──→ │  Model sees  │
│  with Memo   │     │  adds context│     │  full picture│
└──────────────┘     └──────────────┘     └──────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Observer    │ ──→ │  Proactive   │ ──→ │  Agent acts  │
│  watches     │     │  matches     │     │  or suggests │
└──────────────┘     └──────────────┘     └──────────────┘

┌──────────────┐     ┌──────────────┐     ┌──────────────┐
│  Calendar    │ ──→ │  Intent      │ ──→ │  Reminder    │
│  stores      │     │  extraction  │     │  fires       │
└──────────────┘     └──────────────┘     └──────────────┘
```

---

## First-Run Experience

1. **Setup Wizard** — pick language, theme, and assistant personality (6 presets or custom)
2. **Launchpad** — 5 feature cards explain Chat, Agent, Orchestra, WhatsApp, Calendar
3. **Spotlight Tour** — 4-step coachmark highlights nav icons one by one
4. **Explanatory Empty States** — every empty tab explains what it does before you use it

All re-triggerable from Settings → General. Never shows on subsequent launches.

---

## Security & Privacy

| Area | Implementation |
|------|---------------|
| **API keys** | AES-256-GCM encrypted with `crypto/rand`-generated machine key (`data/machine.key`, 0600) |
| **Config files** | Written with `0600` permissions (owner-only) |
| **Cloud sync** | PBKDF2 (600K iterations) → AES-256-GCM. Encrypted *before* upload. |
| **Rate limiting** | Token-bucket per IP: 100 req/s, 200 burst. Excess → 429. |
| **Body limits** | 50MB `MaxBytesReader` on all handlers |
| **Incognito mode** | No session storage, no memory writes, no observation |
| **Observation** | Only topic labels and word counts stored — never raw message text |
| **Telemetry** | None. Zero. |

---

## Roadmap

| Version | Theme | Status |
|---------|-------|--------|
| **v3.1** | RAG memory · WhatsApp · Backup · Mobile · Proactive · Onboarding · Production hardening | ✅ Beta |
| **v3.2** | Stable release · UI polish · Proactive UI · Mobile notifications | 🚧 In progress |
| **v3.3** | Voice assistant · Mobile v2 · Calendar v2 | 📅 Planned |
| **v3.4** | Plugin system · Web search v2 · Browser extension | 📅 Planned |

[Full roadmap →](docs/ROADMAP.md) · [Changelog →](versinNote/v3.1.0.md)

---

## Documentation

| | |
|-|-|
| [🏛️ Architecture](docs/architecture.md) | Package map, data flow, module responsibilities |
| [📡 API Reference](docs/API_REFERENCE.md) | All 55+ REST endpoints with request/response formats |
| [🎨 Design System](frontend/DESIGN.md) | "Pewter Study" theme tokens and component patterns |
| [🛣️ Roadmap](docs/ROADMAP.md) | Versioned release plan with feature targets |
| [📱 Mobile](mobile/README.md) | Flutter mobile companion setup and tunnel config |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | Common issues, GPU setup, port conflicts |
| [📝 Contributing](docs/CONTRIBUTING.md) | Setup, code style, PR process |
| [📋 Changelog](versinNote/v3.1.0.md) | Full v3.1.0 feature list and bug fixes |

---

## Contributing

Memo is AGPL-3.0 licensed and open to contributions.

- Check the [Roadmap](docs/ROADMAP.md) for planned features
- Browse [Known Issues](docs/KNOWN_ISSUES.md) for good first tasks
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
