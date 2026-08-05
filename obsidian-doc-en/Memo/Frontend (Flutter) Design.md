# 📱 Frontend (Flutter) Design

Memo's user interface is developed using Flutter to provide a modern, fluid, and professional experience.

## Design Language: "Greige" Minimalism
- **Color Palette:** Eye-friendly pastel beige and gray tones (Greige).
- **Typography:** Modern fonts focused on readability.
- **Layout:** Focused workspaces with a side navigation rail (NavRail).

## Technical Stack
- **Framework:** Flutter (Linux, Windows, macOS). macOS builds require the App Sandbox entitlements `network.client` (Dio calls to the local backend), `device.audio-input` (mic access for `record`), and `files.user-selected.read-write` (`file_picker`), plus `NSMicrophoneUsageDescription` in `Info.plist` — missing these caused a real user-reported "connection error" on macOS, fixed in commit `420e6a5`. See [[Build and Packaging]] and [[Troubleshooting]].
- **State Management:** Riverpod 2.x (AsyncNotifierProvider patterns).
- **Communication:** Communication with the Go Backend via Dio (HTTP/SSE client).

## Main Screens
1. **Chat (ChatScreen):** Rich text support (Markdown), streaming messages, multimodal (image/file) input, `/orchestra` slash command, agent-mode toggle + web-search toggle in the top bar, `@` file-mention autocomplete (v3.3.4), a quick model/provider-switcher pill (v3.3.4), and (Beta) a Live Mode voice icon next to the input box.
2. **Model Store (ModelStore):** Model search and download management via Hugging Face; first-run hardware-matched recommendations, parallel downloads.
3. **Sidebar screens** (own top-level screens, not inside Settings): **Agent** (permission dialog, tool call cards), **Routines**, **Developer** (API Gateway), **Swarm** (Beta), **WhatsApp**, **Calendar**, **Tasks**.
4. **Settings (SettingsDialog):** reorganized in v3.3.4 into a **searchable, grouped rail** on the left (replacing a flat row of ~20 tabs) — a search box up top jumps straight to a setting instead of scanning every tab. Groups include: General, Identity, Memory (+ Memory Import), Model Parameters, GPU Config, API Providers, CLI Connections, Orchestra, Agent Permissions, Learning, Mood, Incognito, Cloud Sync, Backup & Restore, Remote Access, Beta Features, Stats, Report Bug, Skills, About.

## Dialogs
- **ProviderConfigDialog:** Add/edit external providers (13 types, incl. OpenCode Zen/Go and the CLI providers) with API key, model selection, test connection.
- **OrchestraConfigDialog:** Configure chief model, assign models to expert roles, edit system prompts.
- **Agent permission dialog:** renders the backend's `EventPermissionRequest` events — allow once/session/forever, deny once/forever.

## State Providers
- `ChatProvider` — Message state, stream handling
- `ModelsProvider` — Local model list, download progress
- `SettingsProvider` — App settings, llama config
- `ProviderListNotifier` — External provider configs (CRUD + test)
- `OrchestraConfigNotifier` — Orchestra mode config
- `ActiveProviderNotifier` — Currently active provider
- `voice_mode_provider` (v3.3.4) — Live Mode state (listening/thinking/speaking, barge-in)

### Linked Notes:
- [[Architecture]]
- [[Multimodal Capabilities (Vision and Voice)]]
- [[Agent Mode]]
- [[Advanced Settings]]
