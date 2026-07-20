# Features Catalog

Complete feature-by-feature listing of Memo. Full detail: `docs/FEATURES.md`.

---

## 🧠 Intelligence & Memory

| Feature | Status | Description |
|---------|--------|-------------|
| Persistent RAG | ✅ | Automatic vectorization of every interaction |
| Contextual Recall | ✅ | Top-K similarity search before each response |
| Infinite Context | ✅ | Long-term memory independent of model window limits |
| Cross-Mode | ✅ | External provider chat + local embedding simultaneously |
| Incognito Mode | ✅ | Ephemeral sessions, zero persistence |

## 🏭 Model Management (The Factory)

| Feature | Status | Description |
|---------|--------|-------------|
| Local llama-server | ✅ | High-performance GGUF inference |
| Dedicated Embedding Server | ✅ | Separate server, no chat performance impact |
| HuggingFace Search | ✅ | In-app model browser |
| Background Download Manager | ✅ | Real-time progress, start/stop/update |
| System Diagnostics | ✅ | NVIDIA/AMD VRAM detection, compatibility badges |

## 🔌 External Providers

| Provider | Status | Auth |
|----------|--------|------|
| OpenAI | ✅ | API key |
| Google Gemini | ✅ | API key |
| xAI Grok | ✅ | API key |
| Anthropic Claude | ✅ | API key |
| OpenRouter | ✅ | API key |
| Groq | ✅ | API key |
| Ollama | ✅ | URL |

Router features: fallback chain, auto-disable after 3 failures, health check goroutine.

## 🧑‍💻 Developer Tools (v3.3.3)

| Feature | Status | Description |
|---------|--------|-------------|
| Usage Stats | ✅ | Settings → Stats: token/speed/model breakdown, 30-day chart (fl_chart) |
| Developer API Gateway | ✅ | Its own screen in the sidebar (not inside Settings): point Claude Code (`ANTHROPIC_BASE_URL`) or any OpenAI-compatible tool at Memo's local/external model, includes a live request/response log — see [[Developer API Gateway]] |

## 🐝 Memo Swarm (Beta)

| Feature | Status | Description |
|---------|--------|-------------|
| Host / Join room | ✅ Beta | Sidebar → Swarm; multi-PC via room code |
| rpc-server (Join) | ✅ Beta | Joiners do not need the model file |
| llama-server `--rpc` (Host) | ✅ Beta | Model on host; layers split across machines (llama.cpp RPC) |
| Beta master switch | ✅ | Settings → **Beta Features** (moved out of Remote Access) |
| macOS | ❌ | UI hidden; RPC binary not packaged |

Plain-language guide: [[Memo Swarm]].

## 🧰 Agent Mode (Tool Calling)

| Feature | Status |
|---------|--------|
| 8 built-in tools | ✅ |
| 3-tier danger level | ✅ |
| 6 permission policies | ✅ |
| Execution sandbox | ✅ |
| Rate limiting (30 calls/min) | ✅ |
| Command blacklist (23 patterns) | ✅ |
| Audit trail (1000 entries) | ✅ |
| Agent frontend UI | ❌ (v3.2.0) |

## 🎵 Orchestra Mode (Multi-Model)

| Feature | Status |
|---------|--------|
| 8 expert roles | ✅ |
| Plan → Execute → Synthesize | ✅ |
| Parallel task execution | ✅ |
| Sequential with `depends_on` | ✅ |
| SSE progress streaming | ✅ |
| Exponential backoff retry | ✅ |

## 💬 WhatsApp Integration

| Feature | Status |
|---------|--------|
| QR pairing | ✅ |
| Bidirectional messaging | ✅ |
| Contact name resolution | ✅ |
| Whitelist file transfer | ✅ |
| 4 agent tools | ✅ |
| Local SQLite storage | ✅ |

## 🔐 Remote Access & Backup

| Feature | Status |
|---------|--------|
| ngrok tunnel | ✅ |
| Token auth (`X-Memo-Token`) | ✅ |
| `.memo` export/import | ✅ |
| Full wipe | ✅ |
| Google Drive E2E sync | ✅ |
| AES-256-GCM encryption | ✅ |

## 🎨 UI & UX

| Feature | Status |
|---------|--------|
| Streaming SSE responses | ✅ |
| Markdown rendering | ✅ |
| Image attach (vision) | ✅ |
| File context attach | ✅ |
| Edit/delete/export messages | ✅ |
| Incognito toggle | ✅ |
| Setup wizard (6 personas) | ✅ |
| Multi-language (TR/EN) | ✅ (924 keys) |
| Greige theme, Material 3 | ✅ |
| Mobile companion app | ✅ (basic) |
| Dark mode | ✅ |

## 🎵 Voice & Multimodal

| Feature | Status |
|---------|--------|
| Local STT | ✅ |
| Image upload (multimodal GGUF) | ✅ |
| Document indexing | ✅ |
