# 📱 Frontend (Flutter) Design

Memo's user interface is developed using Flutter to provide a modern, fluid, and professional experience.

## Design Language: "Greige" Minimalism
- **Color Palette:** Eye-friendly pastel beige and gray tones (Greige).
- **Typography:** Modern fonts focused on readability.
- **Layout:** Focused workspaces with a side navigation rail (NavRail).

## Technical Stack
- **Framework:** Flutter (Linux, Windows, macOS).
- **State Management:** Riverpod 2.x (AsyncNotifierProvider patterns).
- **Communication:** Communication with the Go Backend via Dio (HTTP/SSE client).

## Main Screens
1. **Chat (ChatScreen):** Rich text support (Markdown), streaming messages, multimodal (image/file) input, `/orchestra` slash command.
2. **Model Store (ModelStore):** Model search and download management via Hugging Face.
3. **Settings (SettingsDialog):** 8-tab settings dialog including:
   - General, Identity, Memory, Model Parameters
   - **API Providers** — Add/edit/configure external LLM providers
   - **Orchestra** — Multi-model orchestration configuration
   - Cloud Sync, Remote Access, About

## New Dialogs (v3.0.0)
- **ProviderConfigDialog:** Add/edit external providers with API key, model selection, test connection.
- **OrchestraConfigDialog:** Configure chief model, assign models to expert roles, edit system prompts.

## State Providers
- `ChatProvider` — Message state, stream handling
- `ModelsProvider` — Local model list, download progress
- `SettingsProvider` — App settings, llama config
- `ProviderListNotifier` — External provider configs (CRUD + test)
- `OrchestraConfigNotifier` — Orchestra mode config
- `ActiveProviderNotifier` — Currently active provider

## Planned (Agent Frontend UI)
- Agent mode toggle in chat screen
- Permission dialog for tool execution requests
- Tool call cards showing execution results
- Permission history panel

### Linked Notes:
- [[Architecture]]
- [[Multimodal Capabilities (Vision and Voice)]]
