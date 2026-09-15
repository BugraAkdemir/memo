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

## Live Mode v2 — Native Audio-to-Audio Voice Conversation (since v4.3.0)

A small voice icon **next to the chat input box** (not a sidebar tab) lets you have a real spoken back-and-forth with Memo. As of v4.3.0 this is genuinely **native audio-to-audio** — the engine (Google Live or OpenAI Realtime) hears and speaks audio directly, not a transcribe-then-synthesize round trip through separate STT/TTS steps.

> **Package:** `internal/livemode/` (`engine.go`, `session.go`, `reconnecting_session.go`, `echo_session.go`, `delegate_tool.go`, `transcript.go`, plus `google/` and `openai_realtime/` engine implementations), bridged into `internal/app/livemode*.go`
> **API endpoints:** `GET`/`PUT /api/livemode/engines`, `GET /api/livemode/engines/models`, `POST /api/livemode/session`, `GET`/`PUT /api/livemode/active`

- **Delegate or standalone modes.** Run Live Mode as its own conversation, or delegate it into an existing chat so the two share memory/context.
- **One-directional barge-in.** Speaking again while Memo is talking stops it and lets it listen to you instead of talking over you.
- **Mid-session memory refresh (v4.5.0).** Memory context is re-pulled during a long conversation, not just once at session start, so Memo can recall something mentioned partway through — the same way text chat already does.
- **Clearer failures (v4.5.0).** A session that can't start a real voice engine now says why — no engine selected, the engine's config is incomplete, or the background chat session failed to open — instead of silently falling back to hearing your own voice echoed back (`echo_session.go`, previously an unexplained default).
- **Reconnect handling.** `reconnecting_session.go` wraps a live session and reconnects automatically on a dropped connection, rather than silently going dead.
- **Local fallback path.** When no native engine is configured, Live Mode still works end-to-end via on-device STT + local **Piper** TTS (see the STT section above and [[External Providers]]) — an offline voice picker (Turkish/English), plus ElevenLabs and custom TTS engines, are also selectable.
- **The [[Desktop Mascot]] speaks along.** Its mouth animates in time with Live Mode's audio, with a "Speaking…" bubble — no "listening" pose exists, since neither engine currently reports a real listening signal.
- Works on Linux, Windows, and macOS.
- **Known limitation:** no echo cancellation yet — using speakers instead of headphones can occasionally make Memo mistake its own voice for an interruption.

## File Contextualization
Not just media, but also code files (.go, .js, .py) or documents can be fed into the system. Memo reads the content of these files and uses them as instant context via the RAG mechanism.

### Linked Notes:
- [[Frontend (Flutter) Design]]
- [[RAG and Semantic Memory]]
- [[Desktop Mascot]] — animates along with Live Mode speech
- [[External Providers]] — Google Live / OpenAI Realtime / ElevenLabs engine configuration
