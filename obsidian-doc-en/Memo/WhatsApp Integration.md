# WhatsApp Integration

Memo integrates with WhatsApp Web's multi-device protocol via the [`whatsmeow`](https://github.com/tulir/whatsmeow) library, enabling bidirectional messaging, file transfer, and AI-powered automation.

---

## ✨ Features

| Feature | Status |
|---------|--------|
| QR code pairing | ✅ |
| Contact name resolution | ✅ |
| Send messages | ✅ |
| Receive messages | ✅ |
| Message history on first pair | ✅ |
| Whitelist-based file transfer | ✅ |
| Agent tool integration | ✅ (4 tools) |
| Auto-reconnect | ⚠️ (not tested) |
| History sync on reconnect | ❌ (only on first pair) |
| QR polling stop on success | ❌ (never stops) |

## 🔐 Pairing Process

1. Start the WhatsApp service from the backend (via Settings → WhatsApp or API)
2. A QR code is generated and can be polled via `GET /api/whatsapp/qr`
3. Scan the QR code with WhatsApp mobile (Settings → Linked Devices)
4. Once paired, session data is persisted in `data/whatsapp/`

## 🤖 Agent Tools

The following agent tools are available for WhatsApp automation:

| Tool | Description | Danger Level |
|------|-------------|-------------|
| `SendWhatsApp` | Send a message to a contact | Medium |
| `SearchWhatsApp` | Search message history | Safe |
| `LatestWhatsAppChats` | Get recent conversations | Safe |
| `GetWhatsAppMessages` | Get messages from a specific chat | Medium |

## 🗂️ Data Storage

WhatsApp data is stored in an isolated SQLite database at `data/whatsapp/`. This database is separate from the main Memo memory store and is included in `.memo` backup exports.

## ⚠️ Known Limitations

- History sync only fires on first pairing — subsequent reconnects won't get history
- QR code polling never stops (runs entire app lifetime)
- Session reconnection behavior not well-tested with whatsmeow
- Messages are stored locally — this does NOT provide E2E encryption on the desktop side
