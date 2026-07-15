# Memo — Comprehensive Feature Catalog

This document provides a detailed breakdown of every feature integrated into the **Memo AI Memory Shell**. From architectural persistence to sensory multimodality, here is how Memo empowers your local AI experience.

---

## 1. 🧠 Core Intelligence & Memory

### Persistent RAG (Retrieval-Augmented Generation)
Memo isn't just a chat; it's a "Second Brain."
- **Semantic Indexing**: Every interaction is automatically embedded and stored in a local vector database.
- **Hybrid Search**: Retrieval combines vector similarity with FTS5 keyword search (merged via Reciprocal Rank Fusion), so a short, exact fact isn't only found by "close enough" semantic distance.
- **Compound-Question Splitting**: A multi-topic question ("what's my name, birthday, and favorite color") is split on conjunctions so each topic gets its own search instead of being diluted into one blended embedding.
- **Contextual Recall**: Before every response, Memo performs this hybrid search to retrieve the most relevant past conversations (Top-K matching).
- **Pinned Facts (2026-07-15)**: Durable personal facts (name, birthday, pets, etc.) — whether saved via `/remember` or automatically detected from ordinary conversation — are injected into every prompt unconditionally, bypassing retrieval ranking entirely, so they're never crowded out by routine chat.
- **Infinite Context**: Long-term memory allows the AI to remember details from weeks or months ago, regardless of the current model's window.

### Model-Agnostic Engine
- **Internal Llama-Server**: Powered by `llama.cpp` for high-performance GGUF inference.
- **Dedicated Embedding Server**: A second internal server runs specifically for memory indexing, ensuring chat performance remains untouched.
- **Cross-Mode Architecture**: Use external API providers (OpenAI, Claude, Gemini) for chat while a tiny local model handles embeddings — both run independently.
- **External Provider Support**: Seamlessly connects to LM-Studio or any OpenAI-compatible local API (Port 1234/8081).

### Remote Access
- **ngrok Tunnel**: Built-in ngrok integration for accessing your Memo backend from anywhere. Auto-download, tunnel management, configurable domain and region.
- **Token Authentication**: Optional `X-Memo-Token` header for secure remote connections.

---

## 2. 🏛️ Architecture & Persistence

### SQLite + sqlite-vec Persistence
- **Unified Storage**: Vector embeddings and metadata live in the same SQLite database.
- **ANN Indexing**: `vec0` virtual table provides approximate nearest neighbor search with O(log N) query time.
- **ACID Compliance**: Built-in transaction support ensures atomic writes and data integrity.
- **Go Fallback**: If vec0 extension is unavailable, brute-force cosine similarity fallback.

### Privacy & Local Isolation
- **100% Offline**: No data ever leaves your computer. No telemetry, no logs, no cloud dependencies.
- **Encrypted Local Storage**: Your mind stays on your hardware.
- **AES-256-GCM Encryption**: API keys encrypted with machine-derived key.

### Backup & Restore (.memo)
- **Full Export**: `GET /api/export` — zip archive of sessions, config, providers, orchestra, memory, WhatsApp data.
- **Full Import**: `POST /api/import` — restore from .memo zip. Optional model inclusion.
- **Wipe All Data**: `POST /api/wipe` — double-confirmation dialog, config file persists.

### Mobile Companion App
- **Thin Client**: Android/iOS Flutter app connecting over LAN or remote tunnel.
- **Zero Processing**: All AI stays on desktop — mobile is a secure remote viewport.
- **Features**: Chat (SSE streaming), settings (provider/model control), session management.
- **Planned (v3.3.0)**: Full feature parity, biometric auth, offline queue, voice input.

---

## 3. 🏭 Model Management (The Factory)

### Integrated Hugging Face Search
- **Direct Repository Access**: Search for models on Hugging Face directly within the app.
- **Repo ID Support**: Paste any Hugging Face GGUF repo ID to fetch available files instantly.

### System Diagnostics
- **VRAM & GPU Check**: Auto-detection of available NVIDIA/AMD VRAM.
- **Compatibility Badge**: Flags models as "GPU Compatible" or warns about "Insufficient VRAM" before you download.

### Background Download Manager
- **Parallel Downloading**: High-speed GGUF fetching with real-time percentage and speed tracking.
- **Lifecycle Control**: One-click Start, Stop, and Update for all local models.

---

## 4. ⚡ Interaction & User Experience

### Live Mode (Streaming)
- **Token-by-Token Rendering**: Watch the AI "type" its responses in real-time.
- **Thinking State**: A pulsing "Memo is thinking..." status provides visual feedback before the first token arrives.
- **Cursor UI**: A blinking terminal-style cursor (`▊`) follows the stream.

### Incognito Mode
- **Zero-Persistence**: A secure toggle that disables all memory saving and history logging for sensitive sessions.
- **Volatile Context**: Context exists only within that specific session and is wiped upon closing.

### Performance HUD
- **Real-time Stats**: Hover over the timestamp to see generation speed (tok/s), total tokens, and precise duration metrics.

### WhatsApp Integration
- **QR Pairing**: Full WhatsApp Web multi-device pairing via QR code displayed in-app.
- **Bidirectional Messaging**: Send and receive messages with contact-aware display.
- **Contact Resolution**: Phonebook sync, push names, fallback to phone number.
- **Whitelist File Transfer**: Trusted contacts can request files from whitelisted directories.
- **Agent Tools**: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`.
- **Dedicated Chat Mode**: Isolated executor and tool registry for WhatsApp-only interactions.
- **Local Storage**: All WhatsApp messages stored in an isolated SQLite database.

---

## 5. 🔌 External Provider Support

### Multi-Provider Architecture
Memo connects to external LLM APIs alongside local models:
- **Supported Providers:** OpenAI (GPT-4o, o1, o3), Google Gemini (2.0 Flash, 2.5 Pro), xAI Grok (2, 3), Anthropic Claude (3.5 Sonnet, 3 Opus), OpenRouter (unified access), Groq (fast inference), Ollama (local alternative)
- **Provider Interface:** Common `Provider` interface with `ChatCompletion`, `ChatCompletionStream`, `ListModels`
- **Fallback Chain:** Router tries providers in order; auto-disables after 3 consecutive failures; health-check re-enables on recovery

### Encrypted Key Management
- **AES-256-GCM Encryption:** API keys encrypted with machine-derived key (`/etc/machine-id`)
- **Key Storage:** `data/providers.json` with encrypted key values
- **Test Connection:** Built-in test button validates connectivity before saving

### Frontend Provider UI
- **API Providers Tab:** Settings tab for adding/editing providers
- **Configuration Dialog:** Provider type selector, API key input (masked), base URL, model dropdown
- **Active Provider Selection:** Choose which provider is active for chat

---

## 6. 🧠 Agent Mode (Tool Calling)

### Tool Execution Engine
Memo acts as an AI agent with full computer control:
- **8 Built-in Tools:** `read_file`, `write_file`, `delete_file`, `list_directory`, `run_command`, `search_files`, `get_file_info`, `read_env`
- **Tool Registry:** Thread-safe registry with JSON Schema parameter definitions
- **Danger Level System:** `safe` (auto-allowed), `medium` (prompt user), `dangerous` (prompt + delay)

### Permission System
- **6 Policy Types:** PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever
- **Session Persistence:** Permissions stored in `data/permissions.json`
- **Arg Hashing:** SHA-256 hashing for permission matching

### Security Sandbox
- **Path Traversal Protection:** Symlink resolution, `..` blocking, project root confinement
- **Command Blacklist:** 23 dangerous patterns blocked (`rm -rf /`, `sudo`, fork bombs, etc.)
- **Rate Limiting:** 30 tool calls/minute, 5s cooldown per command

### Agent Pipeline
- **LLM ↔ Tool Loop:** Sends user message + tool definitions to LLM, executes tool calls, feeds results back, loops until final response (max 20 iterations)
- **Event Streaming:** Tool execution events streamed to frontend via SSE
- **Audit Log:** Last 1000 tool executions logged with timestamps

> **Note:** Agent frontend UI (permission dialogs, tool call cards, mode toggle) is planned for v3.2.0. Agent currently works via backend API only.

---

## 7. 🎵 Orchestra Mode (Multi-Model Orchestration)

### Concept
Multiple AI models collaborate as a team:
1. **Chief Model** analyzes user request, breaks into subtasks
2. **Expert Roles** execute tasks in parallel (frontend, backend, bug_fixer, etc.)
3. **Chief Model** synthesizes results into a single coherent response

### Built-in Roles
| Role | Default Model | Purpose |
|------|-------------|---------|
| Planner | Claude | Software architecture, task decomposition |
| Frontend | Grok | UI development |
| Backend | GPT-4o | API/server logic |
| Bug Fixer | Gemini | Debugging, root cause analysis |
| Reviewer | Claude | Code quality review |
| Security | GPT-4o | Security auditing |
| DevOps | Grok | Infrastructure/deploy |
| General | GPT-4o | General-purpose fallback |

### Execution Model
- **Parallel Tasks:** Independent tasks run concurrently (goroutines + WaitGroup)
- **Sequential Tasks:** Dependency resolution via `depends_on` field
- **Retry:** Rate-limit aware retry with exponential backoff (up to 3 retries)
- **Streaming:** Progress updates streamed per phase (plan → execute → synthesize)

### Frontend Controls
- **Settings Tab:** Enable/disable, configure chief model, assign models to roles
- **Config Dialog:** Role editor with model selection, system prompt editing, custom role support
- **Slash Command:** `/orchestra on`, `/orchestra off`, `/orchestra config`, `/orchestra status`

---

## 8. 👁️ Multimodality & Senses

### Vision Support (Multimodal)
- **Image Integration**: Drag-and-drop or upload images for analysis (requires a multimodal-capable GGUF like Llava or Moondream).
- **Base64 Processing**: Local, secure image encoding.

### File Contextualization
- **Document Indexing**: Attach code files (.go, .js, .py) or documents (.md, .txt) to give the AI massive instant context for a specific task.

### Local STT (Speech-to-Text)
- **Offline Transcription**: Record voice messages directly in the app.
- **Bundled Engine**: Uses a localized environment (Vosk/Whisper equivalent) for zero-latency, private transcription.

---

## 🎨 Design Philosophy: "Greige" Minimalism
- **Focus-First UI**: Minimalist color palette to reduce cognitive load.
- **Responsive Layout**: Designed for both desktop-wide and mobile-narrow views.
- **Onboarding Wizard**: A guided setup for name, persona, and initial diagnostics.

---
**Built by Buğra.**
*Control your AI. Own your Memory.*
