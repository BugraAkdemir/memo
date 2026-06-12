# Memo Desktop Frontend

<img src="https://img.shields.io/badge/Flutter-3.10-02569B?style=for-the-badge&logo=flutter" alt="Flutter 3.10"/>
<img src="https://img.shields.io/badge/Platform-Linux_%7C_Windows_%7C_macOS-lightgrey?style=flat-square" alt="Cross Platform"/>

Flutter desktop frontend for the Memo AI assistant. Connects to the Go backend over HTTP/JSON + SSE.

---

## ✨ Features

| Feature | Status | Description |
|---------|--------|-------------|
| Chat UI | ✅ | Streaming SSE responses, Markdown, thinking section |
| Message Management | ✅ | Edit, delete, export to Markdown |
| Image & File Attach | ✅ | Drag-and-drop, multimodal vision support |
| Incognito Mode | ✅ | Ephemeral sessions |
| Settings (11 tabs) | ✅ | General, prompts, memory, providers, orchestra, agent, GPU, backup, remote, about |
| Model Store | ✅ | HuggingFace search, download, start/stop |
| WhatsApp UI | ✅ | QR pairing, chat mode, message send/receive |
| Agent Mode | ✅ | Backend ready, frontend UI planned v3.2.0 |
| Setup Wizard | ✅ | 6 persona presets, theme/language selection |
| Multi-language | ✅ | Turkish (default) + English, 924 L10n keys |
| Theme | ✅ | Greige palette, Material 3, dark mode |

## 🏗️ Project Structure

```
frontend/lib/
├── core/
│   ├── api_client.dart      # HTTP + SSE client (~600 lines)
│   ├── theme.dart           # Greige theme, light/dark
│   └── l10n.dart            # Turkish + English localization
├── providers/
│   ├── chat_provider.dart    # Message state, stream handling
│   ├── models_provider.dart  # Model list, download progress
│   ├── settings_provider.dart # App settings, llama config
│   └── ...                   # Provider, orchestra, agent providers
├── screens/
│   ├── chat_screen.dart      # Main chat view
│   └── model_store_screen.dart # HuggingFace browser
├── widgets/
│   ├── chat_message_list.dart # Message bubbles, streaming
│   ├── chat_input.dart       # Text input, file attach, commands
│   ├── settings_dialog.dart  # 11-tab settings (~2500 lines)
│   └── ...                   # 20+ widgets
└── main.dart                 # App entry, AppShell
```

## 🚀 Quick Start

```bash
# Prerequisites: Go backend must be running on :8090
cd frontend
flutter pub get
flutter run -d linux   # or -d windows, -d macos
```

## 🧪 Build

```bash
flutter build linux --release
flutter build windows --release
flutter build macos --release
```
