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

## What is Memo — and Why It's Different

Every AI assistant today is fundamentally the same product: a text box connected to a model. You type words. It returns words. The conversation ends. Nothing persists. Nothing learns. Nothing acts.

Memo breaks this pattern in three ways.

**First, it remembers.** Under the hood, every conversation feeds into a local vector database — a SQLite instance augmented with sqlite-vec for ANN (approximate nearest neighbor) search. When you talk to Memo, it doesn't just read your latest message. It pulls semantically relevant snippets from weeks or months of past conversations and injects them into the system prompt. You mention a project you discussed three weeks ago, and Memo already knows the context. You ask a follow-up, and it remembers what you decided last time. This is Retrieval-Augmented Generation (RAG) running entirely on your machine — no cloud vector database, no Pinecone, no embeddings API. Just SQLite and your GPU.

**Second, it learns your patterns.** Memo has an observer subsystem that runs in the background, watching *when* you do things — not what you say, but when you're active. After about a week, a circular-statistics analyzer detects rhythms: you code every evening at 21:00, you plan on Monday mornings, you take breaks around 15:00. These observations form patterns with confidence scores. When confidence crosses a threshold, the proactive engine consults an LLM (through Orchestra, Memo's multi-model coordinator) and decides what to do: suggest something in chat, push a notification to your phone, or — at high confidence — start the agent autonomously. Patterns that fade are forgotten. Patterns you explicitly dismiss are deleted immediately. The entire system is opt-in and transparent. Nothing is sent anywhere; the observer only stores topic labels and word counts, never raw message text.

**Third, it can act.** Memo isn't just a conversationalist. The agent engine gives it real tools — reading and writing files on your system, executing shell commands, searching the web, interacting with WhatsApp. These tools run inside a sandbox with path validation, symlink protection, command blacklisting, and a 6-policy permission system. Before the agent touches anything, you approve or deny the action — once, for this session, or permanently. The agent pipeline enforces a 60-second timeout per tool call and a 20-iteration maximum, so it can't loop forever or hang. And because the permission model is persistent, you only need to decide once per tool per context.

The result is an assistant that doesn't just talk. It watches, remembers, learns, and does — all on your hardware, with zero telemetry.

---

## How It Feels to Use

You download Memo. It's a single installer — no Docker, no Python environment, no PATH configuration. llama.cpp is bundled inside the binary. You launch it.

The first thing you see is a setup wizard — pick your language, pick a theme, pick a personality. Six presets (Casual, Formal, Technical, Creative, Fun, Buddy) or write your own system prompt. Thirty seconds and you're done.

Then a launchpad appears — five cards that explain what each section does. Chat. Agent. Orchestra. WhatsApp. Calendar. Each card has a real explanation, not a marketing slogan. You tap one and go. A 4-step spotlight tour walks you through the navigation icons one by one, with a "Skip" button always visible. After that, you never see it again unless you ask.

You open the Model Store. It shows curated models with hardware-fit badges — computed from your actual RAM and VRAM, not from a static compatibility list. You see "Fits your device — fast on GPU" or "Runs (CPU)" or "Too large — may not fit in memory." You pick one, click download, and when it finishes, click Start. The engine strip at the bottom shows it's running.

You type "merhaba." Memo responds in Turkish because that's what you wrote. You switch to English mid-conversation, and it switches too. You attach a code file — it reads and reviews it. You upload an image — it describes what it sees (if your model supports vision). You toggle the web search button in the top bar, and suddenly every response is enriched with fresh results from DuckDuckGo — no API key, no configuration.

You've been using Memo for two weeks. You open it on a Monday morning. Before you type anything, it says: "Pazartesi planlaması yapalım mı? Geçen hafta şu projeye başlamıştın." It learned that Monday mornings are your planning time. It remembered the project. It suggested, it didn't assume. You can accept, dismiss, or tell it to never suggest this again.

That's Memo. It's not a chatbot. It's an assistant that earns its name.

---

## Every Feature, Explained

### 💬 Chat

The chat is where everything comes together. Multi-turn conversations with streaming token-by-token display. Full markdown rendering — code blocks with syntax highlighting, tables, lists, bold, italic. You can attach images (the model sees them if it supports vision) and files (text files, PDFs, code). An incognito toggle at the top switches to a mode where nothing is saved to the session file, nothing is indexed for RAG memory, and nothing is observed by the learning engine. The web search toggle enriches every message with live DuckDuckGo results injected into the system context before the model replies. The WhatsApp mode toggle lets you chat through your WhatsApp account using the same interface.

The chat top bar shows the current mode with descriptive badges — "Gizli Mod" when incognito is on, "Agent" when an agent chat is active, "WhatsApp" when WhatsApp mode is engaged. Hover over any badge for a tooltip explaining what it does.

The composer is more than a text field. Type `/` for a slash-command palette with Phosphor icons. Press and hold the microphone button for voice input via whisper.cpp. Click the attachment button to send files. The composer remembers your draft if you switch chats.

Messages support editing and deleting. You can change what you said or what the model said, and the conversation context updates accordingly. Export any chat as a markdown file with one click.

Every message you send feeds into four systems simultaneously: the RAG memory indexer, the proactive observer, the intent extractor (for calendar events), and — if enabled — the stochastic mood engine.

### 🧠 RAG Memory

Most AI tools have the memory of a goldfish. Every conversation starts from zero. Memo's memory system changes that.

When you and Memo exchange messages, the conversation pair (your message + the response) is embedded into a 768-dimensional vector using a local embedding model (Nomic Embed v1.5 is the default, but any embedding model works). The vector is stored in a SQLite database with the sqlite-vec extension providing ANN (approximate nearest neighbor) indexing. This means future queries can find semantically similar past conversations in milliseconds, even with thousands of stored interactions.

When you ask a new question, Memo retrieves the top-K most similar past exchanges based on cosine similarity, formats them into a structured memory block, and injects them into the system prompt before the model responds. The model sees something like:

```
Below are relevant memories from your past conversations with Buğra:

[Memory 1 | Relevance: 85%]
User: What's the database schema for the users table?
Assistant: The users table has id, email, name, created_at...

[Memory 2 | Relevance: 72%]
User: How do I add a new column?
Assistant: Use ALTER TABLE users ADD COLUMN...
```

The model now has context. It doesn't need you to repeat yourself. It already knows the schema you discussed last week.

Memory can be managed from Settings — adjust top-K (how many memories to retrieve) and minimum similarity (the relevance threshold). You can view all stored memory files, inspect their content, and delete individual entries or clear everything. Memory can be enabled or disabled at any time.

### 🤖 Agent Engine

The agent is where Memo goes from "talking" to "doing."

You create an agent chat by selecting a project folder on your computer. The agent can now see your files, read them, write to them, execute commands inside that folder, and search the web. It has 8 built-in tools:

- `read_file` — reads any file in the project
- `write_file` — creates or overwrites a file (with backup)
- `edit_file` — makes targeted text replacements in a file (with backup)
- `delete_file` — removes a file or directory
- `list_directory` — shows the contents of a folder
- `run_command` — executes a shell command inside the project directory (with a blacklist blocking `rm -rf /`, `sudo`, `mkfs`, `shutdown`, fork bombs, and similar dangerous commands)
- `web_search` — searches DuckDuckGo for live information
- `search_files` — finds files matching a pattern

Each tool invocation triggers a permission dialog. You see exactly what the agent wants to do — the tool name, the arguments, and a warning if the operation is dangerous. You can allow or deny it once, for the rest of the session, or permanently. Denied patterns are remembered so the agent learns what you don't want it to touch. The permission system has 6 policies, giving you precise control without constant interruptions.

The agent pipeline runs up to 20 iterations: the model decides a tool to call → you approve → the tool executes → the result feeds back into the model → it decides the next step or returns a final answer. Each tool has a 60-second timeout. If the model loops or gets stuck, the iteration limit stops it. If you cancel mid-stream, all running tools stop immediately through context cancellation.

The sandbox validates every file path. It resolves symlinks before checking boundaries — so a symlink inside your project pointing to `/etc/` won't be accepted. Windows case-insensitive paths are handled correctly. Protected system paths (`/etc/`, `/proc/`, `/sys/`, `/dev/`, and platform-specific paths) are blocked for write and delete operations.

In the chat UI, agent actions appear as a clean, animated status line — not raw JSON blobs. You see "Dosya işleniyor..." while the tool runs, then "Dosya tamam ✅" when it finishes. Completed actions become compact badges like `[Dosya okudu 120ms]`. The agent feel is similar to Claude Code or Cursor — minimal, informative, never technical noise.

### 🎵 Orchestra — Multi-Model Workflow

A single model is good. Multiple models working as a team is better.

Orchestra is Memo's multi-model coordination system. When you send a complex request, a Chief model (configurable — can be any provider and model) analyzes the task and decomposes it into sub-tasks. Each sub-task is assigned to a specialist role:

- **Planner** — breaks requirements into structured implementation plans
- **Frontend** — handles UI, styling, user-facing code
- **Backend** — handles server logic, APIs, database queries
- **Bug Fixer** — finds and fixes bugs in provided code
- **Reviewer** — reviews code for issues, style, and correctness
- **Security Auditor** — checks for vulnerabilities and security issues
- **DevOps** — handles CI/CD, Docker, infrastructure configuration
- **Generalist** — handles anything that doesn't fit the other roles

You can assign different providers and models to each role. Use Claude for reasoning-heavy tasks (Planner, Reviewer), Gemini for fast search and analysis, Groq for speed, and your local llama.cpp model for code generation. Mix and match however you want. The Orchestra configuration screen makes this easy — assign one model to the Chief and all enabled roles with a single click, or fine-tune each role individually.

Roles execute in parallel when they're independent. You see progress in real time as each specialist completes its work. The Chief model synthesizes all outputs into a single, coherent response. Parallel execution means a task that would take 3 minutes sequentially finishes in seconds.

Orchestra can also work with the Agent. When both are enabled, the Chief plans first, then the Agent executes the plan step by step — reading files, writing code, running commands — and the Chief reviews the results and synthesizes the final answer. It's the best of both systems: high-level strategy from Orchestra, low-level execution from the Agent.

### 📱 WhatsApp Integration

Memo connects to your WhatsApp account through the multi-device Web API — the same protocol that WhatsApp Web uses. No WhatsApp Business API fees. No phone number registration. Just scan a QR code and you're connected.

Once paired, your chat list appears in Memo's WhatsApp tab. You can read messages, search your entire chat history, and send replies. The chat UI matches Memo's own theme — bronze accents, panel colors, message bubbles in the same style as the main chat. Profile photos are fetched and cached, with a tap-to-enlarge dialog that includes a download button for full-resolution avatars.

The WhatsApp integration goes deeper than a chat client. The agent can access WhatsApp through its tools — `whatsapp_send` to send messages, `whatsapp_search` to find messages, `whatsapp_latest` to list recent conversations. You can ask Memo "Berra'ya mesaj at" and it resolves the contact name automatically without you needing to know their JID.

WhatsApp messages feed into the same systems as regular chat: the RAG memory indexer, the proactive observer, the intent extractor, and the mood engine. A friend says "kanka cuma akşamı sinema" and Memo detects the intent, creates a calendar event, and later reminds you.

Reconnection is automatic. If you've paired before and restart the app, WhatsApp reconnects without you clicking anything — you land straight on the chat list. The backend handles disconnects gracefully with exponential backoff, a reconnect attempt limit, and a 5-second logout timeout so a flaky network never freezes the app.

### 📅 Calendar

Memo's calendar isn't a separate tool you fill out manually. It's an automatic capture system that watches your conversations and extracts time-bound plans and commitments.

The intent extraction pipeline works in two stages. First, a fast keyword filter scans every message — it looks for patterns like "yarın", "salı", "haftaya", "saat", numerical times, and dates in both Turkish and English. Only messages that match are sent to the second stage: an LLM that parses the natural language into structured event data — title, date, time, source (chat or WhatsApp), and contact name if applicable. Routine chatter never triggers an LLM call, so there's no performance impact.

When an intent is detected, an event is created in the built-in calendar SQLite database. The UI shows a monthly grid view with event dots on days that have events. You can tap a day to see its events, add events manually, or delete them. The calendar auto-refreshes every 20 seconds, so events added from chat or WhatsApp appear without a restart.

A reminder loop checks for upcoming events every minute. If an event is within the configured lead time (10 minutes to 2 hours, configurable in Settings), a notification fires on your desktop and mobile though the app event system. The `ClaimPendingReminders` method uses an atomic SQL transaction to prevent the same reminder from firing twice.

If Memo isn't sure about an event — for example, you said "belki yarın buluşalım" — it creates a cancellation ambiguity event. The UI shows it with a warning, and you can confirm or delete it with one click. You can also disable the time-guess feature entirely in Settings if you only want explicitly timed events.

### 🧠 Proactive Learning Engine

Memo is the only AI assistant that comes to you. You don't have to remember to ask it for things.

The observer subsystem tracks your activity patterns — *when* you use Memo, not what you say. Every message interaction is recorded with a timestamp and activity type. A circular-statistics analyzer processes this data periodically, looking for statistically significant patterns: "Monday mornings between 9-10 AM, planning activity" or "Daily between 21:00-23:00, coding activity."

The mathematics behind this is circular (directional) statistics applied to clock time — treating time as a circle where 23:59 and 00:01 are adjacent, not 24 hours apart. This gives more accurate pattern detection than linear time analysis. Patterns have confidence scores calculated from three factors: consistency (how regular the pattern is), frequency (how often it occurs), and recency (how recently it was observed).

When a pattern's confidence crosses a threshold, the proactive engine consults a Chief LLM (through Orchestra) with the current time, the matched pattern, and context about what's happening. The Chief decides: suggest something helpful, notify the user's phone, or at high confidence, start the agent automatically to do something.

The system has strict safety rules. It never acts without explicit opt-in. You can view all learned patterns in Settings → Learning and forget any pattern with one click. Patterns that fade from disuse are automatically forgotten. The observation layer stores only topic labels and word counts — never the raw text of your conversations.

### 🏪 Model Store

Finding and downloading the right AI model is usually a maze of HuggingFace repos, cryptic filenames, and guesswork about whether your hardware can run it. Memo's Model Store fixes all of that.

The Discover tab shows curated, officially-published models with a clean two-panel layout: model list on the left, rich detail view on the right. Each model shows the real company logo (Google, Microsoft, Qwen/Alibaba, nomic, and others) fetched automatically from HuggingFace. Models are filterable by size (1-8B, 8-14B, 14B+) and capability (Tools, Vision, Code).

The hardware-fit badge is the killer feature. When Memo starts, it detects your GPU (NVIDIA via nvidia-smi, AMD via rocm-smi and sysfs, or CPU-only) and measures available VRAM and RAM. Every download option for every model is evaluated against your actual hardware and displayed with a clear badge: "Fits your device — fast on GPU" or "Runs (CPU)" or "Too large — may not fit." No more downloading a 14B model only to find out your 8GB card can't handle it.

Download options use plain-language quality labels instead of raw quantization codes: "Balanced quality" instead of `Q4_K_M`, "High quality" instead of `Q5_K_M`, "Smallest size" instead of `Q2_K`. The detail panel shows the model's full README from HuggingFace, capability tags (Tool Use, Vision, Code), parameter count, architecture, and "More from this author" suggestions.

When you download, progress is streamed in real-time. Downloads are cancelable. When complete, imported models appear in My Models. The Start button configures and launches llama-server with GPU offloading automatically based on the detected engine mode — you don't need to know what `-ngl` means.

### 🔌 8 Providers, One Interface

Memo works with local models, cloud APIs, or both simultaneously. Eight provider types are supported:

OpenAI, Anthropic Claude, Google Gemini, xAI Grok, Groq, OpenRouter, Ollama, and any local llama.cpp server. Each provider is configured with an API key (encrypted with AES-256-GCM using a machine-generated random key) and a model selection. Multiple providers can be enabled at once with a priority-ordered fallback chain.

Here's what makes the provider system production-grade: if a provider fails (rate limit, network error, API downtime), Memo automatically falls through to the next enabled provider without interrupting your conversation. After three consecutive failures, the problematic provider is auto-disabled to prevent cascading timeouts. A background health check runs every 5 minutes and re-enables providers that have recovered. You never need to manually toggle providers on and off because of transient issues.

Provider switching is live. Type `/model` mid-conversation, pick a different provider, and continue without losing context. The engine strip at the bottom of the chat window shows which provider is currently active, with the real company logo and a green connection dot.

Per-model context windows are configurable. Gemini defaults to 1M tokens, Claude to 200K, others to 128K — but you can set any provider to any window to match your specific model variant. The context budget (how much chat history is packed into each request) follows the model's actual window, not the provider type.

### 🎤 Voice Input

Built-in speech-to-text via whisper.cpp. Press and hold the microphone button in the composer, speak, release, and your words are transcribed and sent. All processing happens on your device — the audio never leaves your computer. Auto-detects Turkish, English, and mixed-language input. The whisper model downloads automatically on first use.

### ☁️ Cloud Sync

For users who want backup across devices, Memo supports encrypted Google Drive sync. Your data (sessions, memory, settings, provider configs, WhatsApp data, mood database, learning data) is archived, encrypted with AES-256-GCM using a key derived from your passphrase via PBKDF2 with 600,000 iterations, and uploaded to Google Drive. Google cannot read your data because the encryption happens before upload.

If you don't set a passphrase, Memo uses a machine-specific key generated by `crypto/rand` and stored in `data/machine.key` with 0600 permissions. The sync runs automatically every N messages (configurable, default 50), or you can trigger push/pull/full sync manually.

Export everything as a single `.memo` file for offline backup. Import it on any machine to restore your entire Memo state — conversations, memory, settings, models list.

### 🔒 Privacy by Design

Every design decision in Memo starts from the question: "does this need to leave the user's machine?" The answer is almost always no.

- No telemetry. No analytics. No crash reporting to external services. The only network requests Memo makes are the ones you explicitly configure: model API calls, WhatsApp pairing, web search queries, and optional cloud sync.
- Provider API keys are encrypted on disk with a machine-specific random key. They never appear in plaintext in config files, logs, or memory dumps.
- Config files and sensitive data files are written with 0600 permissions — only the file owner can read them.
- Incognito mode gives you conversations that leave zero trace: no session file, no memory index, no observation.
- The proactive observer stores only activity types and timestamps, not message content.
- WhatsApp messages are stored in a local SQLite database. Nothing goes through WhatsApp servers beyond what WhatsApp already sees.
- Rate limiting protects the backend from runaway clients (100 requests/second per IP, non-blocking token bucket).
- A 50MB body size limit prevents oversized uploads from exhausting memory.

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
git clone https://github.com/BugraAkdemir/memo.git
cd memo
CGO_ENABLED=1 go run . --port 8090          # Backend
cd frontend && flutter run -d linux          # Frontend (separate terminal)
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
│                    Go Backend — 25 packages, ~55 endpoints       │
│                                                                  │
│  ┌─────────────┐  ┌──────────────┐  ┌────────────────────────┐ │
│  │ Web Server   │  │ App Engine   │  │ Proactive Engine       │ │
│  │ ServeMux     │  │ orchestrator │  │ Observer→Analyzer→Act  │ │
│  │ SSE streaming│  │ (25 files)   │  │                        │ │
│  └─────────────┘  └──────┬───────┘  └────────────────────────┘ │
│                           │                                     │
│  ┌────────┐ ┌──────────┐ ┌┴─────────┐ ┌──────────┐ ┌────────┐ │
│  │ Memory │ │ Sessions │ │ Llama    │ │WhatsApp  │ │ Agent  │ │
│  │ vec0   │ │ JSON     │ │ GPU/RAM  │ │whatsmeow │ │ Pipe   │ │
│  └────────┘ └──────────┘ └──────────┘ └──────────┘ └────────┘ │
│                                                                  │
│  Orchestra · ModelStore · CloudSync · Calendar · Mood            │
│  ngrok · Tailscale · Whisper · Skills · Intent · Observer        │
└──────────────────────────────────────────────────────────────────┘
```

Two processes communicate over plain HTTP on `localhost:8090`. No TLS (local only). No external router framework — pure `net/http` ServeMux. The frontend is a single-page Flutter app with Riverpod state management, SSE streaming via Dio, and markdown rendering via flutter_markdown.

**Docs:** [Architecture](docs/architecture.md) · [API Reference](docs/API_REFERENCE.md) · [Design System](frontend/DESIGN.md)

---

## Tech Stack

| Layer | Technology | Notes |
|-------|-----------|-------|
| **Backend language** | Go 1.26 | CGO required for SQLite |
| **HTTP** | `net/http` ServeMux | No external router dependency |
| **Streaming** | SSE (Server-Sent Events) | Token-by-token chat streaming |
| **Desktop frontend** | Flutter 3.10 | Linux + Windows |
| **State management** | Riverpod 2.4 | AsyncNotifierProvider |
| **HTTP client** | Dio 5.4 | SSE parsing, interceptors |
| **Markdown** | flutter_markdown 0.6 | Code blocks, tables, images |
| **LLM runtime** | llama.cpp | Bundled binary, subprocess management |
| **Vector database** | SQLite + sqlite-vec | vec0 ANN index, 768-dim embeddings |
| **WhatsApp** | whatsmeow | Go library, multi-device Web API |
| **Speech-to-text** | whisper.cpp | Bundled binary, on-device |
| **Cloud sync** | Google Drive API v3 | OAuth2, AES-256-GCM, PBKDF2 |
| **GPU detection** | nvidia-smi, rocm-smi, sysfs | Auto VRAM measurement |
| **Logging** | `internal/logx` | slog wrapper with Info/Warn/Error/Debug |
| **CI/CD** | GitHub Actions | Go vet+test+build, Flutter analyze+test |
| **License** | GNU AGPL v3 | Free software |

---

## Documentation

| | |
|-|-|
| [🏛️ Architecture](docs/architecture.md) | Package map, data flow, module boundaries |
| [📡 API Reference](docs/API_REFERENCE.md) | All 55+ REST endpoints with request/response schemas |
| [🎨 Design System](frontend/DESIGN.md) | "Pewter Study" theme tokens, color palette, typography |
| [🛣️ Roadmap](docs/ROADMAP.md) | Versioned release plan with feature targets |
| [📱 Mobile](mobile/README.md) | Flutter mobile companion setup and tunnel configuration |
| [🔧 Troubleshooting](docs/TROUBLESHOOTING.md) | GPU setup, port conflicts, common errors |
| [📝 Contributing](docs/CONTRIBUTING.md) | Dev setup, code style, PR process |
| [📋 Changelog](versinNote/v3.1.0.md) | Full v3.1.0 feature list, bug fixes, and polish |

---

## Contributing

Memo is AGPL-3.0 licensed. Contributions welcome.

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
