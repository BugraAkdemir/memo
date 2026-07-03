# Memo Roadmap — Strategic Vision

A privacy-first, local-first AI assistant that evolves with its user. All features respect the core principle: **your data never leaves your device unless you explicitly choose to share it.**

---

## ✅ v3.1.1 — Open Beta (Current, 2026-07-04)

First public open beta of the v3.1 line, gathering feedback ahead of a stable release targeted for Windows and Linux in 2-3 weeks. Carries forward everything below plus the v3.2.0 items already verified shipped (see note in that section).

---

## ✅ v3.1.0 — "Memory" (Released)

**Theme:** Persistent memory, local embedding, cross-modal architecture, WhatsApp foundation, mobile companion, remote access.

### Key Features
- SQLite + sqlite-vec vector store with ANN index (chromem-go removed)
- Cross-mode: external API chat + local embedding model run independently
- WhatsApp integration: QR pairing, bidirectional messaging, contact resolution, whitelist file transfer, agent tools
- Backup/Restore (.memo): full export/import with wipe capability
- Mobile companion app: thin Flutter client for Android/iOS (LAN/ngrok connection, basic chat, provider/model control)
- Remote access: built-in ngrok tunnel support
- Windows support: 8 compatibility fixes, Inno Setup installer
- Setup wizard rewrite: 6 persona presets (Normal, Fun, Formal, Technical, Creative, Buddy)
- Memory bug fixes: file sizes display, L10n for Turkish strings, proper error handling

---

## 🚧 v3.2.0 — "Scheduled Intelligence"

**Theme:** Proactive automation through calendar, reminders, cron scheduling, and a fully realized agent interface.

> **2026-07-04 status check (verified against source):** Calendar (`internal/calendar/`: store, reminders, natural-language events) and the full Agent UI (mode toggle, permission dialogs, tool-call cards, agent chat screen) **are already shipped**, along with the Flutter file-edit diff preview, mobile push notifications (`notification_service.dart`), and chat prompt templates listed further below. **Not yet built:** the standalone cron engine (`internal/scheduler/`, `/api/scheduler/tasks`) for scheduling arbitrary recurring tasks beyond calendar reminders — the calendar itself works, but "Memo, her gün saat 10'da günaydın yaz"-style recurring task scheduling does not exist yet.

### 📅 Calendar & Reminders

**Purpose:** Memo gains its own calendar. Users can create, edit, delete events via natural language. Memo can autonomously schedule reminders and recurring tasks.

#### Natural Language Event Parsing
- Date/time extraction from any chat context (including WhatsApp): *"Memo, yarın saat 10'da dişçi randevum var"* → event creation
- AI-powered parsing with regex fallback for reliability
- Support for recurring patterns: *"her hafta pazartesi 9'da toplantı"*

#### Calendar Store (`internal/calendar/`)
- SQLite-based event storage (`data/calendar/events.db`)
- Full CRUD: create, read, update, delete events
- Rich event model: title, description, start/end time, all-day flag, category (meeting/task/birthday/reminder), recurring rules, source (user/memo)
- Recurring event expansion (daily, weekly, monthly, yearly)

#### Cron Engine (`internal/scheduler/`)
- Schedule recurring tasks via natural conversation: *"Memo, her gün saat 10'da günaydın yaz"*
- Schedule WhatsApp messages, system commands, or custom agent actions
- Full cron expression support with friendly presets
- Task persistence across application restarts
- Execution logs and failure notifications
- Desktop OS notifications (via `libnotify`/Windows toast)

#### Calendar API
| Endpoint | Method | Description |
|----------|--------|-------------|
| `/api/calendar/events` | GET | List events (with date range filter) |
| `/api/calendar/events` | POST | Create event |
| `/api/calendar/events/:id` | PUT | Update event |
| `/api/calendar/events/:id` | DELETE | Delete event |
| `/api/calendar/natural` | POST | Natural language → event |
| `/api/calendar/today` | GET | Today's events |
| `/api/scheduler/tasks` | GET/POST/DELETE | Cron task management |

#### Flutter Calendar UI
- Monthly overview widget with event indicators
- Daily/weekly list view
- Event creation/editing form
- Memo-created events shown in distinct color
- Inline event creation from chat context menu

---

### 🤖 Agent UI Completion

**Purpose:** The agent engine is fully functional on the backend — now it needs a complete frontend UI for users to interact with it naturally.

#### Agent Mode Toggle
- Persistent toggle in chat screen header (on/off)
- Visual indicator when agent mode is active
- Keyboard shortcut for quick toggle
- Default state: off (opt-in)

#### Permission Dialog
- Real-time popup when agent needs to execute a tool
- Clear display of: tool name, proposed action, danger level (Safe/Medium/Dangerous)
- Action buttons: Allow Once, Allow This Session, Allow Forever, Deny Once, Deny Forever
- Show full command/arguments before execution
- Timer-based auto-deny fallback (30 seconds)

#### Tool Call Display
- Real-time streaming cards showing tool execution progress
- Each tool call rendered as a collapsible card with:
  - Tool icon + name
  - Status indicator (pending/running/success/failed/denied)
  - Arguments preview (expandable)
  - Execution output (stdout/stderr, expandable)
  - Duration badge
- Smooth animations for card transitions

#### Agent Chat Mode
- Dedicated visual mode (separate from regular chat)
- System prompt visible in agent mode header
- Context: selected project directory shown
- Token/session usage stats

#### API Client Integration
- `api_client.dart` — all agent endpoints fully typed
- `agent_enabled`, `agent_permissions`, `agent/respond` endpoints
- SSE event stream for real-time tool execution
- Error handling and reconnection logic

#### Setup Wizard Integration
- Optional "Enable Agent Mode" step in setup wizard
- Default permission policy selection
- Project directory configuration

---

### 📱 Mobile Notifications

**Purpose:** Calendar reminders and scheduled tasks deliver push notifications to the mobile companion app.

#### Push Notification Relay
- Desktop → mobile notification relay via WebSocket or polling
- Calendar event reminders delivered to phone
- Scheduled task completion/failure notifications
- Configurable notification categories (reminders, tasks, system)
- Silent/priority mode per category

#### Mobile UI Updates
- Notification history view in mobile settings
- Tap-to-snooze or tap-to-dismiss
- Badge count on app icon

---

## 🚧 v3.3.0 — "Mobile & Voice" *(Planned)*

**Theme:** Full-featured mobile experience with hands-free voice interaction. All AI processing stays on desktop.

### Mobile Application v2

**Purpose:** The mobile companion evolves from a thin viewport into a fully capable remote client.

#### Feature Parity
- **RAG Memory Browsing** — view, search, and delete memory files from mobile
- **WhatsApp UI** — QR pairing, send/receive messages, contact list, chat view
- **Calendar View** — monthly overview, event creation, reminder management
- **Agent Control** — permission responses, tool execution monitoring, mode toggle
- **Settings Access** — full settings sync from desktop (providers, models, orchestra, agent)

#### Mobile UX
- **Biometric Authentication** — Face ID / fingerprint for app access
- **Offline Message Queue** — compose messages offline, auto-send when reconnected
- **Push Notification Relay** — all event types delivered to mobile (reminders, tasks, messages)
- **Connection Status** — persistent indicator with auto-reconnect and graceful degradation

#### Remote Access
- **Tailscale-native connectivity** — zero-config for Tailscale users
- **TLS + password authentication** — for non-Tailscale remote access
- **End-to-end encrypted channel** — all communication between mobile and desktop
- **Auto-reconnect** with exponential backoff

### 🎤 Voice Assistant

**Purpose:** Hands-free interaction with Memo — "Hey Memo" wake word, speech-to-text, text-to-speech.

#### Wake Word Detection
- Configurable wake word ("Hey Memo") with adjustable sensitivity
- Local wake word engine — no cloud dependency
- Low-power background listening with microphone cooldown
- Visual indicator when listening is active

#### Speech-to-Text (STT)
- **whisper.cpp** integration for fully local speech recognition
- Cloud STT fallback option (OpenAI Whisper API)
- Turkish language optimized (whisper small/turbo)
- Real-time streaming transcription in chat
- Voice message recording for mobile (send audio → transcribed on desktop)

#### Text-to-Speech (TTS)
- Local TTS engine for voice responses
- Configurable voice and speech rate
- Auto-read option for incoming messages
- Hands-free mode: query → speak → listen → respond cycle

#### Voice Pipeline
- End-to-end: "Hey Memo, Buğra'ya mesaj gönder akşam yemeğe geliyorum de"
- Wake → listen → transcribe → LLM process → execute → TTS respond
- Full conversational memory in voice mode
- Google Assistant / Siri-style hands-free interaction model

---

## 🚧 v3.4.0 — "Plugin & Web" *(Planned)*

**Theme:** Plugin architecture for community tools + live web search for local models.

### 🔌 Plugin System

**Purpose:** Allow anyone to write custom tools without modifying Memo's core code.

#### Architecture
- **Plugin interface** defined in Go:
  ```go
  type PluginTool interface {
      Name() string
      Description() string
      Execute(ctx PluginContext, args map[string]any) (string, error)
      DangerLevel() agent.DangerLevel
  }
  ```
- Plugins compiled as `.so` files with `go build -buildmode=plugin`
- Auto-discovered from `plugins/` directory on startup
- Registered into the agent's tool registry automatically
- Permission system works out of the box — allow/deny once/session/forever

#### Go Plugin Mode (v1)
- Same Go version requirement as main binary
- Shared dependency constraint (minimal deps)
- Hot-reload on plugin file change (optional)
- Built-in example plugins: `echo.so`, `hello.so` for reference

#### gRPC Plugin Mode (v2 — future)
- Plugin runs as a separate process
- Communication over gRPC with mTLS
- Language-agnostic — write plugins in Python, Rust, or Node.js
- Resource limits per plugin process (CPU, memory, runtime)
- Plugin health check and auto-restart on crash

#### Plugin Registry
- `GET /api/plugins` — list installed plugins with version and author
- `POST /api/plugins/install` — download plugin from URL
- `DELETE /api/plugins/:name` — remove plugin
- Plugin manifest: `plugin.json` (name, version, author, description, danger_level)
- Community plugin index URL (configurable)

#### Marketplace (future)
- Community plugin directory in GitHub
- One-command install: `/plugin install github/user/memo-plugin-name`
- Signed plugin verification (optional GPG)

---

### 🌐 Web Search for Local Models

**Purpose:** Give local models the ability to search the internet in real-time via agent tool calling.

#### How It Works
- Local model gets a question requiring current information: *"Bugün dolar ne kadar?"*
- Model calls `web_search` tool (agent tool calling)
- Tool executes the search via a search API
- Results are returned to the model as context
- Model generates a natural response with sources

#### Architecture
- `internal/agent/tools/websearch.go` — new built-in tool
- Configurable search providers:
  - **Bing Search API** (primary, cheap, good for Turkish)
  - **SerpAPI** (Google results via API)
  - **DuckDuckGo** (free, no API key needed, less reliable)
  - **SearXNG** (self-hosted, privacy-first)
- Config: `search_provider`, `api_key`, `max_results`, `region`
- Rate limiting and caching to avoid redundant queries
- Result format: title, snippet, URL — model uses this to answer

#### User Experience
- Web search works inside **Agent Mode** — model decides when to search
- Manual trigger: `/search bugün dolar ne kadar`
- Results shown as tool call card in agent UI
- Source URLs displayed in responses for verification

#### Agent Integration
- `web_search` tool registered in the default tool registry
- Permission system applies (can be set to auto-allow)
- Supports follow-up searches for multi-turn research
- Web search results cleanly separated from memory context

---

## 🔮 v3.5.0 — "Smarter Memo" *(Future)*

**Theme:** Memo evolves from a passive assistant into a self-improving, context-aware intelligence that connects your knowledge, learns from feedback, and acts proactively.

### 🧠 Knowledge Graph

**Purpose:** Transform flat memory entries into a connected web of knowledge. Memo understands relationships between conversations, people, projects, and concepts.

#### Graph Engine (`internal/knowledge/`)
- **Entity extraction** — automatically extract people, places, projects, technologies, dates from conversations using local NER or LLM-based extraction
- **Relationship mapping** — discover semantic connections between entities: "Buğra → works_on → Memo", "Memo → uses → sqlite-vec"
- **Graph database** — lightweight SQLite-based graph store (adjacency list + node properties)
- **Incremental indexing** — new conversations update the graph in real-time, no full rebuild

#### User Interaction
- **Graph visualization** — Obsidian-style interactive graph in both desktop and mobile UI
  - Node sizes reflect entity importance (frequency + recency)
  - Edge thickness reflects relationship strength
  - Click a node → show related conversations
  - Filter by entity type, date range, relevance score
- **Natural language graph queries**: *"Memo, Buğra'nın üzerinde çalıştığı projeleri göster"* → graph highlights + list
- **Conversation context** — when discussing a topic, Memo surfaces related entities from the graph

#### Memory Enhancement
- Graph enriches RAG retrieval: when retrieving memories, include entities connected to the query topic
- *Example:* user asks about "vektör deposu" → graph surfaces related memories about "sqlite-vec", "ANN index", "embedding model"
- Automatic tag generation for conversations based on extracted entities

---

### 🤖 Self-Improving Intelligence

**Purpose:** Memo learns from user interactions and gets better over time without manual configuration.

#### Feedback Learning
- **Explicit feedback**: user corrects Memo → "hayır yanlış oldu" → Memo stores correction, avoids repeating the same mistake
- **Implicit feedback**: user edits a message → Memo analyzes the edit to understand what was wrong
- **Repeated pattern detection**: if user consistently dismisses certain suggestions, Memo stops making them
- **Correction store** (`data/feedback/`): SQLite-based, stores corrections with context for future reference

#### System Prompt Auto-Refinement
- Memo analyzes which response styles get positive reactions (user continues conversation, doesn't correct)
- Periodically proposes system prompt tweaks: *"Son konuşmalara bakınca daha kısa cevap vermeni tercih ediyorsun. Stili güncellememi ister misin?"*
- A/B testing of prompt variants in low-stakes interactions
- Versioned prompt history with rollback capability

#### Usage Pattern Analysis
- Identifies user routines: *"Her pazartesi saat 9'da kod yazmaya başlıyorsun"*
- Proactively optimizes for user's workflow patterns
- Suggests automation based on observed behavior

---

### 🧹 Smart Memory

**Purpose:** Memory stops being a dumb log and becomes an intelligent, self-organizing knowledge base.

#### Automatic Memory Pruning
- Stale memory detection: entries older than X days with zero retrieval → auto-archive
- Duplicate detection: similar conversations merged into a single consolidated entry
- Low-value filtering: "merhaba", "teşekkürler" etc. skipped during memory save
- Configurable retention policies: by age, by relevance score, by storage quota

#### Memory Consolidation
- Weekly consolidation: similar memories merged into summary entries
- *Example:* 5 separate conversations about "proje mimarisi" → single consolidated entry with key decisions
- Original entries archived, consolidated entry promoted in retrieval
- User can view consolidated vs. raw memory

#### Semantic Organization
- Auto-categorization: memories grouped by topic, project, person
- Topic hierarchy: "Memo > Development > Backend > SQLite" style drill-down
- Full-text search over all memory metadata (topics, entities, categories)
- Memory browser in desktop and mobile UI with filters and search

---

### 💡 Proactive Suggestions

**Purpose:** Memo doesn't wait to be asked — it anticipates needs and offers help before the user asks.

#### Pattern Recognition
- Time-based patterns: *"Her cuma akşamı haftalık rapor yazıyorsun — başlayalım mı?"*
- Conversation-based patterns: *"Geçen sefer deploy sonrası hata almıştın, testleri koşayım mı?"*
- Event-based patterns: *"Takvimde 10 dakika sonra toplantın var, hazırlık notlarını özetleyeyim mi?"*

#### Delivery Channels
- In-chat suggestions (non-intrusive, single-dismiss)
- Desktop notification (configurable)
- Mobile push notification
- Configurable proactivity level: Off / Subtle / Normal / Assertive

#### Suggestion Types
- **Task reminders**: recurring tasks detected from conversation history
- **Information surfacing**: "Geçen ay benzer bir sorunu çözmüştün, çözümü hatırlatayım mı?"
- **Automation suggestions**: "Her push'ta test koşuyorsun, bunu CI'a eklemek ister misin?"
- **Learning suggestions**: "Son 3 konuşmanda aynı hatayı düzelttin, bunu kalıcı olarak öğreneyim mi?"

---

### 🧩 Multi-Step Reasoning

**Purpose:** Memo handles complex, multi-part questions by planning, executing sub-tasks, and synthesizing results — all within a single conversation turn.

#### Reasoning Pipeline
- **Plan phase**: LLM decomposes the question into sub-steps (*"önce hava durumunu bul, sonra etkinlik öner"*)
- **Execute phase**: each sub-step calls the appropriate tool (web search, memory, code execution)
- **Synthesize phase**: results from all sub-steps combined into a coherent answer
- **Verify phase**: model checks if the answer satisfies the original question

#### Integration
- Works inside Agent Mode (tool calling)
- Sub-step results shown as expandable cards in agent UI
- User can see the reasoning chain and verify each step
- Parallel execution when sub-steps are independent
- Timeout and partial result handling

#### Use Cases
- *"İstanbul'da bu hafta sonu ne yapabilirim?"* → check weather → search events → combine into weekend plan
- *"Memo'nun performansını nasıl optimize ederim?"* → check config → analyze memory size → suggest improvements
- *"Bu hatayı nasıl çözerim?"* → search memory → search web → combine solutions

| Principle | Description |
|-----------|-------------|
| **Local-first** | Every feature works offline. No cloud dependency. |
| **Privacy by design** | Data never leaves your device unless you explicitly enable sharing. |
| **User ownership** | You control your data, your model, your fine-tuning. |
| **Progressive complexity** | Features reveal themselves as you grow with the assistant. |
| **Open source** | AGPL v3 — inspect, modify, redistribute. |

---

> **Legend:** ✅ Released | 🚧 In Development | 🔮 Future  
> **Current version:** v3.1.0-beta  
> **Repository:** [github.com/BugraAkdemir/memo](https://github.com/BugraAkdemir/memo)
