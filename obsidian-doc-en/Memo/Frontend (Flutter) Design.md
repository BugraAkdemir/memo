# 📱 Frontend (Flutter) Design

Memo's user interface is developed using Flutter to provide a modern, fluid, and professional experience.

## Design Language: "Greige" Minimalism
- **Color Palette:** Eye-friendly pastel beige and gray tones (Greige).
- **Typography:** Modern fonts focused on readability.
- **Layout:** Focused workspaces with a side navigation rail (NavRail).

## Technical Stack
- **Framework:** Flutter (Linux & Windows Native).
- **State Management:** Riverpod (Reactive and testable state management).
- **Communication:** Communication with the Go Backend via Dio (HTTP client).

## Main Screens
1. **Chat (ChatScreen):** Rich text support, streaming messages, and multimodal (image/file) input area.
2. **Model Store (ModelStore):** Model search and download management via Hugging Face.
3. **Settings (Settings):** System prompt, memory parameters, and sync controls.

### Reactive Components
- **Thinking State:** Visual feedback generated while the model is responding.
- **Performance HUD:** `tokens/sec` and `ms` data that appears when hovering over messages.

### Linked Notes:
- [[Architecture]]
- [[Multimodal Capabilities (Vision and Voice)]]
