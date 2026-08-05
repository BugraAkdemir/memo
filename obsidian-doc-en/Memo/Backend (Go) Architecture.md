# 🖥️ Backend (Go) Architecture

Memo's backend is a "Headless" server written in Go for speed, reliability, and low resource consumption.

## Modular Structure (`internal/`)
The codebase is divided into modules, each with a specific responsibility:

### 1. `webserver`
- **Role:** Manages the REST API layer.
- **Technology:** Standard `http.ServeMux` (no external router).
- **Feature:** Secure local communication with Flutter via REST/SSE.

### 2. `llama`
- **Role:** Manages `llama-server` (C++) processes.
- **Feature:** Automatic installation (`llamaInstaller`), GPU/VRAM detection, and multi-process (chat + embedding) management.

### 3. `memory`
- **Role:** Semantic memory and vector database.
- **Feature:** Cosine similarity calculations and vec0 ANN index-based search using SQLite + sqlite-vec.

### 4. `cloudsync`
- **Role:** Google Drive integration.
- **Feature:** AES-256-GCM E2E encryption with PBKDF2 key derivation for data backup.

### 5. `identity`
- **Role:** Manages system prompts and user identities (personas).

### 6. `provider` (NEW in v3.0.0)
- **Role:** External LLM provider integration.
- **Feature:** OpenAI-compatible API client, custom Gemini/Claude implementations, multi-provider router with fallback chain, AES-256 encrypted API key storage.

### 7. `agent`
- **Role:** AI agent execution engine.
- **Feature:** Tool registry (19 built-in tools, up from the original 8), permission manager (6 policy types), execution sandbox (path validation, rate limiting, hardened command blacklist + symlink sandbox-escape fix), LLM ↔ tool pipeline. Skill-defined `command:` tools (v3.3.3) execute through this same pipeline.

### 8. `orchestra`
- **Role:** Multi-model orchestration.
- **Feature:** Chief model plans and delegates to expert roles, parallel/sequential task execution, result synthesis.

### 9. `agentcli` (NEW in v3.3.4, Beta)
- **Role:** Claude Code CLI / Codex CLI as chat providers — shells out to the locally installed `claude`/`codex` binary instead of calling an HTTP API.
- **Feature:** Registers itself into `internal/provider` via `RegisterConstructor` (avoids an import cycle), runs as a real per-chat background job tied to `App.lifecycleCtx` rather than the HTTP request, own slash-command resolution.

### 10. `anthropicapi` + Developer API Gateway (NEW in v3.3.3)
- **Role:** Anthropic Messages API-compatible server (`POST /v1/messages`) so tools like Claude Code can point straight at Memo.
- **Feature:** Full agentic tool-calling translation (Anthropic ⇄ OpenAI-shaped tool calls), optional token auth, optional memory integration — see [[Developer API Gateway]].

### 11. `routine` (NEW in v3.3.3)
- **Role:** Scheduled automations ("Routines").
- **Feature:** Plain-language → schedule parsing, per-device timezone with auto-resync, plain-prompt or full-agent execution path — see [[Proactive Learning and Calendar]].

### 12. `swarm` (NEW in v3.3.3, Beta)
- **Role:** Memo Swarm — pools several machines' compute for one oversized local model over llama.cpp's RPC feature. Not on macOS. See [[Memo Swarm]].

### 13. `tts` (NEW in v3.3.4, Beta)
- **Role:** Text-to-speech for Live Mode — local Piper by default, optional external provider. See [[Multimodal Capabilities (Vision and Voice)]].

### Reliability: panic recovery (v3.3.4)
Every background task across the backend (memory saving, chat streaming, WhatsApp, cloud sync, local model management, STT, routines, proactive suggestions, notifications, remote-access tunnels, and more) now runs under panic recovery — previously only three small corners of the codebase had this, so an unexpected error in almost any background goroutine could crash the entire process instead of just that one task.

## Bridge Pattern
The `app.go` file acts as the main "Brain" that brings all these modules together. The web server communicates with this engine via the `AppBridge`/`FullBridge` interface. `FullBridge` extends `AppBridge` with Flutter-specific handlers including provider management, agent control, and orchestra configuration.

### Linked Notes:
- [[Architecture]]
- [[API Documentation]]
- [[Llama.cpp Integration]]
