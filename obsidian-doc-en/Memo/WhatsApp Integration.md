# WhatsApp Integration

Memo connects to your WhatsApp account through the multi-device Web API (whatsmeow). Scan a QR code and you're connected — no WhatsApp Business API fees, no phone number registration.

## Features

- **QR pairing** — scan a code to link your WhatsApp
- **Read & reply** — view messages, send replies from Memo's UI
- **Search** — full-text search across all chats and messages
- **AI assistant** — ask the AI to draft responses or summarize threads
- **Agent tools** — `whatsapp_send`, `whatsapp_search`, `whatsapp_latest`, `whatsapp_messages` exposed to the agent
- **Dedicated WhatsApp mode** — separate chat interface just for WhatsApp
- **Profile photos** — fetched and cached, tap to enlarge and download
- **Auto-reconnect** — exponential backoff on disconnect, auto-reconnects on restart
- **Instant send** — optimistic UI, messages appear in bubble immediately
- **Contact name resolution** — "message Berra" resolves names automatically

## Reliability (v3.1.0 Polish)

| Feature | Status |
|---------|--------|
| QR polling | Adaptive: 2s during QR wait, 15s heartbeat when connected |
| History sync | `INSERT OR IGNORE` — safe on reconnects, no duplicates |
| Write serialization | `sync.Mutex` on `SaveMessage` + `SaveContact` |
| Auto-reconnect | Exponential backoff (5s, 10s, 30s, 60s) |
| Logout timeout | 5s cap — local session cleared regardless |
| Message ordering | Fixed: newest at bottom, oldest at top |

## Technical

- **Library**: whatsmeow (Go, multi-device Web API)
- **Store**: SQLite with WAL mode, `sync.Mutex` on writes
- **Integration**: Messages feed into RAG memory, proactive observer, intent extractor, and mood engine
- **Data**: Stored in `data/whatsapp/` — messages, contacts, profile picture cache
- **Fixed (v3.3.3):** incoming images, videos, or documents with a caption were silently dropped — only plain-text messages were being read from live messages; a caption on media is no longer ignored.
- **Security (v3.3.3):** a `golang.org/x/text` infinite-loop vulnerability, reachable through the profile-picture lookup, was patched (along with a related `golang.org/x/net` fix).
