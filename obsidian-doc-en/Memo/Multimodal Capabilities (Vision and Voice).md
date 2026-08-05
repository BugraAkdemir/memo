# 👁️ Multimodal Capabilities (Vision and Voice)

Memo is not limited to text only; it can see images and hear sounds.

## Vision Analysis
If the GGUF model you are using supports multimodality (e.g., `Llava`, `Moondream`, `BakLLaVA`):
- **Drag-and-Drop:** You can drag and drop images into the chat area for analysis.
- **Local Processing:** Images are locally converted to Base64 format and securely transmitted to the LLM. No images are uploaded to the cloud.

## Voice Command and Transcription (STT)
Memo includes a local Speech-to-Text (STT) engine:
- **Offline Recording:** You can record your voice using the microphone icon within the application.
- **Private Transcription:** Audio files are converted to text locally (with a Whisper or Vosk-based engine).
- **Low Latency:** As soon as the process finishes, the text is automatically written into the input field.
- Fixed in v3.3.4: starting STT from an installed terminal CLI (as opposed to the desktop app) could fail with "whisper-server binary not found" — it was only looking next to the CLI's own executable, not the separate folder an installed CLI's bundled files actually live in.

## Live Mode — Hands-Free Voice Conversation (Beta, v3.3.4)

A small voice icon **next to the chat input box** (not a sidebar tab — turn it on via Settings → Beta Features first) lets you have a real spoken back-and-forth with Memo instead of typing: it listens, detects when you start/stop speaking, transcribes locally, sends it as a normal chat message, and speaks the reply back — hands-free, right inside any chat.

- **Local by default.** Speech is transcribed with the same on-device engine used elsewhere; replies are spoken with **Piper**, a local, offline TTS engine — nothing about a Live Mode conversation has to leave the machine.
- **Optional external TTS.** Settings → Beta Features can configure an external provider (OpenAI) instead, trading the local-only guarantee for a different voice; local Piper remains the fallback whenever nothing is configured or a call fails.
- **Offline voice picker.** Settings → Beta Features downloads a small, curated set of Piper voices (Turkish and English) and switches instantly, no restart needed.
- **One-directional barge-in.** Speaking again while Memo is thinking/replying stops it and lets it listen to the new message instead of talking over you.
- A short, locally-synthesized "thinking" sound plays during the generation gap so the pause doesn't feel frozen.
- The voice-activity detection (VAD) model ships **bundled with the app** rather than downloading from a CDN at runtime.
- Works on Linux, Windows, and macOS.
- **Known limitation:** no echo cancellation yet — using speakers instead of headphones can occasionally make Memo mistake its own voice for an interruption. Full duplex audio is planned for a later release.

Backend: `internal/tts/` (synthesis, provider abstraction, voice picker), plus VAD/barge-in logic in the Flutter layer (`frontend/lib/core/live_mode_controller.dart`, `duplex_audio_engine.dart`).

## File Contextualization
Not just media, but also code files (.go, .js, .py) or documents can be fed into the system. Memo reads the content of these files and uses them as instant context via the RAG mechanism.

### Linked Notes:
- [[Frontend (Flutter) Design]]
- [[RAG and Semantic Memory]]
