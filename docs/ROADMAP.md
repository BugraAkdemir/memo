# Memo Roadmap — Strategic Vision

A privacy-first, local-first AI assistant that evolves with its user. All features respect the core principle: **your data never leaves your device unless you explicitly choose to share it.**

---

## ✅ v3.1.0 — "Memory" (Current Release)

**Theme:** Persistent memory, local embedding, cross-modal architecture, and WhatsApp foundation.

### WhatsApp Integration
- Full WhatsApp Web pairing via QR code
- Local message store (isolated SQLite database)
- Contact name resolution (phonebook sync, push names, fallback to phone number)
- Bidirectional messaging with contact-aware display
- **Whitelist-based file transfer:** trusted contacts can request files from whitelisted directories; automatic authorization checking
- Agent toolset: `SendWhatsApp`, `SearchWhatsApp`, `LatestWhatsAppChats`, `GetWhatsAppMessages`
- Dedicated WhatsApp chat mode with isolated executor and tool registry

### RAG Memory
- SQLite + sqlite-vec vector store with ANN index
- Local embedding model (nomic-embed-text-v1.5, 768-dimension)
- Cross-modal architecture: external API chat + local embedding run independently
- Non-blocking goroutine-based initialization

### Backup & Recovery
- `.memo` zip format for full export/import (sessions, config, providers, orchestra, memory, WhatsApp data)
- Wipe all data with two-step confirmation dialog
- Config file persists across wipes for user convenience

### Platform & Stability
- Windows build support with Inno Setup installer
- `LoadExtension .so.so` double-extension fix
- `sqrtf` symbol resolution via patchelf
- Port safety and process group cleanup (`Setpgid` over `Pdeathsig`)
- License corrected to GNU AGPL v3

---

## 🚧 v3.2.0 — "Scheduled Intelligence"

**Theme:** Proactive automation through time-aware scheduling, calendar sync, voice control, and smart home integration.

### Calendar & Reminders
- Natural language date/time parsing from any chat context (including WhatsApp)
- Automatic calendar event creation from conversation: _"Memo, kanka yarın saat 10'da halısaha gel"_ → event + reminder
- Desktop and mobile push notifications at event time
- Recurring events, alarms, and snooze
- Bi-directional calendar sync (optional CalDAV)

### Scheduled Tasks (Cron Engine)
- Define recurring tasks through natural conversation: _"Memo, her gün saat 10'da günaydın yaz"_
- Schedule WhatsApp messages, system commands, or custom agent actions
- Visual cron editor in settings panel
- Task persistence across application restarts
- Execution logs and failure notifications

### Voice Control
- Wake word detection ("Hey Memo") with configurable sensitivity
- Speech-to-text via whisper.cpp (local) with cloud fallback option
- Full voice command execution: _"Memo, Buğra'ya mesaj gönder, akşam yemeğe geliyorum de"_
- Text-to-speech response output
- Hands-free Google Assistant-style interaction model

### Smart Home Integration
- Chat-based home automation: _"Memo, ışıkları kapat"_
- MQTT and Home Assistant protocol support
- Device whitelist with role-based access control
- Scene and routine definitions via conversation
- Energy monitoring and automation triggers

---

## 🚧 v3.3.0 — "Mobile Companion"

**Theme:** Thin mobile client — all AI processing stays on your computer.

### Mobile Application
- Flutter-based mobile app (Android & iOS)
- Secure tunnel to desktop Memo instance (LAN, Tailscale, or TLS-encrypted tunnel)
- Zero processing on mobile — serves as a remote viewport
- Full feature access: chat, settings, memory browsing, WhatsApp control, voice input
- Biometric authentication for app access

### Remote Access Infrastructure
- Tailscale-native connectivity or TLS + password authentication
- End-to-end encrypted communication channel
- Push notification relay from desktop to mobile
- Connection status indicator with auto-reconnect

---

## 🚧 v3.4.0 — "Personal Model"

**Theme:** The ultimate leap — fine-tune a custom model on your own conversations. Eliminates the vector store entirely.

### Personal Fine-Tuning Engine
- Automated pipeline converts all conversations into structured JSONL dataset (system/user/assistant format)
- Dataset cleaning: deduplication, quality filtering, privacy sanitization
- **Train a compact model (1.2B–3B parameters) on personal conversation data**
- Model replaces vector memory entirely — no embedding server, zero retrieval latency
- 1.2B model outperforms embedding + LLM pipeline in both speed and relevance
- Model internalizes user's communication style, preferences, contacts, and routines
- Periodic incremental fine-tuning ensures the model evolves with the user

### Dataset Pipeline
- Privacy-first architecture: dataset never leaves the local machine
- Quality scoring and automatic filtering of low-value conversations
- Optional anonymized export for community model contributions
- Incremental training support — no full retrain required
- Dataset versioning and rollback capability

### Meeting Intelligence
- Join Zoom/Google Meet calls via automated bot
- Real-time transcription with speaker diarization
- AI-powered meeting summarization and key point extraction
- Automatic action item detection → calendar entry creation
- Historical meeting search: _"Memo, bugünkü toplantıda ne konuştuk?"_

### Advanced Agent Capabilities
- Multi-step plan generation from complex requests
- Batch permission approval workflow
- Step-by-step execution with rollback on failure
- Line-based file editing with diff preview and undo
- Web scraping with SSRF protection
- Git integration (status, diff, commit, push)
- Session-based context management (cwd, env, history)
- Sandboxed script execution (bash, Python, Node.js)

---

## 🔮 v3.5.0 — "Ecosystem"

**Theme:** Plugin architecture, knowledge graph, multi-user support, and self-improvement.

### Plugin System
- Go plugin interface for custom tools and data sources
- Community plugin registry with code signing
- Sandboxed plugin execution with resource limits

### Knowledge Graph
- Obsidian-style graph visualization over memory entries
- Semantic relationship discovery between conversations
- Interactive graph exploration in the mobile and desktop UI

### Multi-User Architecture
- Isolated profiles with per-user models
- Shared context when explicitly configured
- Role-based access control for family/shared deployments

### Self-Improving Intelligence
- Automatic system prompt refinement from user feedback
- Usage pattern analysis for proactive suggestions
- Autonomous memory pruning and consolidation

### Import & Interoperability
- Data import wizards for Notion, Obsidian, Google Keep
- Export to standard formats (Markdown, JSON, PDF)
- Community model hub for anonymized personal models

---

## Guiding Principles

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
