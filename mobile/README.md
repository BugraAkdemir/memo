# Memo Mobile — Remote AI Companion

<img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
<img src="https://img.shields.io/badge/Platform-Android_%7C_iOS_%7C_Linux_%7C_Windows_%7C_macOS%7C_Web-lightgrey?style=flat-square" alt="Cross Platform"/>

**Memo Mobile** is a thin Flutter client that connects to the [Memo Go backend](https://github.com/BugraAkdemir/memo) over LAN or remote tunnel. All AI/ML processing stays on the desktop — the mobile app serves as a secure remote viewport.

---

## ✨ Features

| Feature | Status | Description |
|---------|--------|-------------|
| ConnectScreen | ✅ Done | Connect via LAN IP or remote URL with optional token auth |
| ChatScreen | ✅ Done | Streaming SSE, Markdown rendering, session management |
| Settings | ✅ Done | Backend URL/token config, provider view/toggle, model start/stop |
| Model Switching | ✅ Done | `/model` command for runtime model switching |
| Remote Access | ✅ Done | ngrok and Tailscale (beta) tunnel support via backend |

### Planned (v3.3.0+)
- RAG memory browsing
- WhatsApp UI
- Calendar view + reminders
- Agent control (permission responses, mode toggle)
- Push notifications
- Voice input (STT)
- Biometric authentication
- Offline message queue

---

## 📱 Architecture

```
┌─────────────────────────────────────┐
│          Memo Mobile App            │
│  ┌────────────┐  ┌───────────────┐  │
│  │ ConnectScreen│  │  ChatScreen   │  │
│  └──────┬─────┘  └───────┬───────┘  │
│         └────────┬───────┘           │
│          ┌───────┴───────┐           │
│          │  Connection   │           │
│          │   Provider    │           │
│          │  (Riverpod)   │           │
│          └───────┬───────┘           │
│          ┌───────┴───────┐           │
│          │ MemoApiClient │           │
│          │  (Dio + SSE)  │           │
│          └───────┬───────┘           │
└──────────────────┼───────────────────┘
                   │ REST + SSE
                   │ (LAN / ngrok / Tailscale)
┌──────────────────┼───────────────────┐
│         Memo Go Backend Server       │
│  All AI processing happens here      │
└─────────────────────────────────────┘
```

**Key design decision:** The mobile app is deliberately "dumb" — it only renders UI and streams tokens. All AI, memory, WhatsApp, and automation run on the desktop backend. This keeps the mobile app lightweight and secure.

---

## 🚀 Quick Start

### Prerequisites
- Memo Go backend running (see [root README](../README.md))
- Flutter 3.10+ SDK

### Run
```bash
cd mobile

# Android
flutter run -d android

# iOS
flutter run -d ios

# Linux (for testing)
flutter run -d linux

# Web (for testing)
flutter run -d chrome
```

### Build
```bash
cd mobile

# Android APK
flutter build apk --release

# Android App Bundle
flutter build appbundle --release

# iOS
flutter build ios --release
```

---

## 🔐 Authentication

Memo Mobile supports three connection modes:

| Mode | How it works | Best for |
|------|-------------|----------|
| **Local (LAN)** | Direct IP:port connection | Home/office same network |
| **Tailscale** (beta) | Fixed `*.ts.net` URL from the backend's embedded tsnet tunnel, entered once | Anywhere, no re-pairing after the first setup — the backend self-heals a dropped tunnel, and the app auto-reconnects with the saved URL on every launch |
| **Remote (ngrok)** | Tunnel URL from backend's ngrok | Access from anywhere without Tailscale set up |

The Tailscale tab only appears once beta features are enabled on the connected desktop backend (Settings → Beta Features) and this phone has connected at least once to learn that.

Optional `X-Memo-Token` header for authentication against the backend.

---

## 📦 What's Not on Mobile

These features run on the **desktop backend only** and are accessible from mobile via the API:
- RAG memory indexing and search
- WhatsApp message processing
- Orchestra multi-model orchestration
- Agent tool execution
- Model download and management
- Backup/restore (.memo)

Mobile will render results (e.g., memory search results, WhatsApp messages) in a future release.

---

## 🛣️ Roadmap

| Version | Theme | Mobile Features |
|---------|-------|-----------------|
| **v3.1.0** | Memory | ✅ Initial mobile scaffold, basic chat, settings |
| **v3.2.0** | Scheduled Intelligence | 📱 Push notifications for calendar reminders |
| **v3.3.0** | Mobile & Voice | 🎯 Full feature parity, biometric auth, offline queue |
| **v3.4.0** | Plugin & Web | 🔮 Mobile plugin management |
| **v3.5.0** | Smarter Memo | 🔮 Knowledge graph on mobile |

---

## 📁 Project Structure

```
mobile/lib/
├── core/
│   ├── api_client.dart      # HTTP + SSE client
│   └── theme.dart           # Mobile theming
├── models/
│   └── chat_message.dart    # Message model
├── providers/
│   ├── chat_provider.dart   # Chat state + stream
│   └── connection_provider.dart  # Backend connection
├── screens/
│   ├── chat_screen.dart     # Main chat UI
│   ├── connect_screen.dart  # Connection setup
│   └── settings_screen.dart # Settings tabs
├── widgets/
│   ├── chat_bubble.dart     # Message bubble
│   ├── message_input.dart   # Input field
│   └── session_drawer.dart  # Chat session list
└── main.dart                # App entry point
```
