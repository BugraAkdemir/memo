# Memo Mobile App

**Memo Mobile** is a thin Flutter client connecting to the Memo Go backend over LAN or remote tunnel. All AI/ML processing stays on the desktop — the mobile app is a secure remote viewport.

---

## ✨ Current Features (v3.1.0)

| Feature | Status |
|---------|--------|
| Connect via LAN IP or remote URL | ✅ |
| Token-based auth (`X-Memo-Token`) | ✅ |
| Streaming chat (SSE) | ✅ |
| Markdown rendering | ✅ |
| Session management (list, switch, new) | ✅ |
| Provider view/toggle | ✅ |
| Model list, start/stop | ✅ |
| `/model` command for switching | ✅ |

## 🏗️ Architecture

```
┌──────────────────────────────┐      ┌──────────────────────────┐
│       Mobile App             │      │    Desktop Backend       │
│  ┌────────┐ ┌──────────┐    │ LAN  │  ┌────────────────────┐  │
│  │Connect │ │  Chat    │    │◄────►│  │ Go Server (:8090)  │  │
│  │Screen  │ │  Screen  │    │ngrok │  │ + llama.cpp + RAG  │  │
│  └────────┘ └──────────┘    │      │  │ + WhatsApp + Mem   │  │
│  ┌──────────────────┐       │      │  └────────────────────┘  │
│  │  Settings / Prov. │       │      └──────────────────────────┘
│  │  / Models         │       │
│  └──────────────────┘       │
└──────────────────────────────┘
```

**Key principle:** Zero AI processing on mobile. All inference, memory, and automation run on the desktop.

---

## 🔐 Connection Modes

| Mode | Transport | Use Case |
|------|-----------|----------|
| **Local** | Direct IP:port | Home/office same network |
| **Remote** | ngrok tunnel | Access from anywhere |
| **Tailscale (v3.3)** | Tailscale | Zero-config VPN |

---

## 🛣️ Mobile Roadmap

| Version | Features |
|---------|----------|
| **v3.1.0** | ✅ Initial scaffold, basic chat, settings |
| **v3.2.0** | 🚧 Push notifications for reminders |
| **v3.3.0** | 🔮 Full parity (RAG, WhatsApp, calendar, agent), biometric auth, offline queue, STT |
| **v3.4.0** | 🔮 Plugin management |
| **v3.5.0** | 🔮 Knowledge graph |

## 📂 Project Structure

```
mobile/lib/
├── core/
│   ├── api_client.dart      # HTTP + SSE client
│   └── theme.dart
├── providers/
│   ├── chat_provider.dart
│   └── connection_provider.dart
├── screens/
│   ├── chat_screen.dart
│   ├── connect_screen.dart
│   └── settings_screen.dart
├── widgets/
│   ├── chat_bubble.dart
│   ├── message_input.dart
│   └── session_drawer.dart
└── main.dart
```
