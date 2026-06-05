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
- **Feature:** Cosine similarity calculations, `.gob` serialization, and fast RAM-based indexing using `chromem-go`.

### 4. `cloudsync`
- **Role:** Google Drive integration.
- **Feature:** AES-256-GCM E2E encryption with PBKDF2 key derivation for data backup.

### 5. `identity`
- **Role:** Manages system prompts and user identities (personas).

### 6. `provider` (NEW in v3.0.0)
- **Role:** External LLM provider integration.
- **Feature:** OpenAI-compatible API client, custom Gemini/Claude implementations, multi-provider router with fallback chain, AES-256 encrypted API key storage.

### 7. `agent` (NEW in v3.0.0)
- **Role:** AI agent execution engine.
- **Feature:** Tool registry (8 built-in tools), permission manager (6 policy types), execution sandbox (path validation, rate limiting, command blacklist), LLM ↔ tool pipeline.

### 8. `orchestra` (NEW in v3.0.0)
- **Role:** Multi-model orchestration.
- **Feature:** Chief model plans and delegates to expert roles, parallel/sequential task execution, result synthesis.

## Bridge Pattern
The `app.go` file acts as the main "Brain" that brings all these modules together. The web server communicates with this engine via the `AppBridge`/`FullBridge` interface. `FullBridge` extends `AppBridge` with Flutter-specific handlers including provider management, agent control, and orchestra configuration.

### Linked Notes:
- [[Architecture]]
- [[API Documentation]]
- [[Llama.cpp Integration]]
