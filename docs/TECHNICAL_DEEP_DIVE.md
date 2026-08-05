# Technical Deep Dive

A granular look into the engineering decisions behind Memo.

## 1. The Bridge Pattern (`app.go`)
The `App` struct acts as the central hub. It implements the `AppBridge` interface, which defines all the actions the web server can trigger. This decoupling allows us to theoretically swap the web server for a CLI or a GUI without touching the core logic.

## 2. SQLite + vec0 + FTS5 Persistence
Why SQLite?
- **Unified Storage:** Vector embeddings, keyword index, and metadata all live in the same database — no separate `.gob` files to manage.
- **ANN Indexing:** The `sqlite-vec` extension provides a `vec0` virtual table for approximate nearest neighbor search, replacing O(N) brute-force scans with O(log N) lookups.
- **Keyword Indexing:** SQLite's built-in `FTS5` extension provides a second, independent search path (bm25-ranked exact/keyword matches), merged with vector results via Reciprocal Rank Fusion. Both `vec0` and `fts5` require `-tags "sqlite_fts5"` at build time — see [CGO_FLAGS.md](CGO_FLAGS.md) for why this matters (missing it doesn't error, it just silently degrades to vector-only search, which shipped unnoticed for a long time).
- **ACID Compliance:** Built-in transaction support ensures atomic writes without per-file crash risk.
- **Zero-Config:** SQLite requires no external server — the database is a single file in `data/memory/`.

## 3. Llama Process Lifecycle
Memo doesn't just "call" Llama; it manages its lifecycle.
- **Health Checks:** Periodic pings to ensure the model is responsive.
- **Port Management:** If the default port is taken, Memo increments until it finds a free one, then updates the API client dynamically.
- **Cleanup:** On application exit, Memo sends a `SIGTERM` to all child processes to prevent "zombie" servers.

## 4. Hybrid Search and Pinned Facts
When a user queries their memory:
1. The query is embedded. If it's a compound, multi-topic question, it's also split into segments on conjunctions (`splitCompoundQuery`) — each segment is embedded and searched separately, so one topic doesn't get diluted into an averaged vector with the others.
2. Vector search runs (`vec0`'s ANN index, or a Go-side sequential cosine-similarity scan as a fallback when `vec0` isn't available — not a parallel worker pool).
3. FTS5 keyword search runs alongside it, words joined with `OR` (not `AND` — a natural-language question would otherwise never match any single row) and ranked by bm25.
4. The two result sets are merged via Reciprocal Rank Fusion (RRF), then re-weighted by each memory's `importance` field and filtered by `top_k`/`min_similarity`.
5. Separately, every `source='explicit'` memory ("pinned fact" — set via `/remember` or automatic background detection) is added unconditionally, bypassing all of the above.

## 5. E2E Sync Strategy
1. Collect the SQLite database and all `.json` files.
2. Compress into a single stream.
3. Encrypt using **AES-256-GCM** with the user's passphrase.
4. Upload to a hidden app-data folder on Google Drive.
5. This ensures even if Google is compromised, the "memories" are useless without the passphrase.

## 6. External Provider System (`internal/provider/`)
Memo supports external LLM APIs alongside local models:

- **Provider Interface:** All providers implement a common `Provider` interface with `ChatCompletion`, `ChatCompletionStream`, and `ListModels`.
- **Router with Fallback:** The `Router` maintains an ordered list of providers. On failure (rate limit, timeout, auth error), it automatically falls to the next. After 3 consecutive failures, a provider is auto-disabled. A health check goroutine periodically tests disabled providers and re-enables them on recovery.
- **Encrypted Config:** API keys are stored in `data/providers.json` encrypted with AES-256-GCM. The encryption key is derived from the machine's `/etc/machine-id`, making the config non-portable across machines by design.
- **Provider Types:** Three implementation patterns exist:
  - **OpenAI-compatible** (OpenAI, Grok, Groq, OpenRouter, Ollama) — share a common `openAIProvider` with different base URLs
  - **Gemini** — custom implementation using Google's `generateContent`/`streamGenerateContent` API
  - **Claude** — custom implementation using Anthropic's Messages API with `x-api-key` auth

## 7. Agent Engine (`internal/agent/`)
Memo's agent system enables LLMs to interact with the user's computer:

- **Tool Registry:** Thread-safe registry holding 8 built-in tools, each with JSON Schema parameter definitions and a `DangerLevel` (Safe/Medium/Dangerous).
- **Permission Manager:** When the LLM requests a tool call, the `PermissionManager` evaluates the action against stored policies. Safe tools are auto-allowed. Others prompt the user via an event channel.
- **Execution Sandbox:** The `Sandbox` validates all paths (symlink resolution, traversal prevention), enforces rate limits (30 calls/minute), and maintains a blacklist of 23 dangerous command patterns.
- **Pipeline:** The core loop: (1) Send user message + tool definitions to LLM → (2) If tool calls requested, check permissions → (3) Execute tool → (4) Feed result back to LLM → (5) Repeat until final response or 20 iterations. Events are streamed via channels.
- **Audit Trail:** All tool executions are logged with timestamps, arguments, results, and permission decisions (last 1000 entries in memory).

## 8. Orchestra Mode (`internal/orchestra/`)
Multi-model orchestration enables multiple LLMs to collaborate:

- **Conductor Pattern:** A `Conductor` manages the three-phase workflow:
  1. **Plan** — Chief model receives the user request with a system prompt describing available expert roles. It outputs a JSON plan with tasks, role assignments, and dependency graph.
  2. **Execute** — Tasks run in parallel (independent) or sequential (based on `depends_on`). Each task creates a fresh provider instance for its assigned model. Retry logic handles transient failures with exponential backoff.
  3. **Synthesize** — Chief model receives all task results and produces a coherent final response.
- **Provider Bypass:** Unlike normal chat mode, orchestra creates providers directly via factory rather than going through `provider.Router`. This means no fallback chain at the provider level — each role is pinned to its configured model.
- **Progress Streaming:** Each phase emits typed progress updates (`ProgressPlan`, `ProgressTaskStart`, `ProgressTaskChunk`, `ProgressSynthChunk`, etc.) that the frontend renders in real-time.

## 9. Backend-Wide Panic Recovery (`internal/logx`)
Before this was addressed, Go gave no automatic protection for panics in background goroutines the way `net/http` gives a single request — an unexpected error in memory saving, a routine tick, a WhatsApp handler, a proactive-suggestion check, or a streaming reply could crash the *entire* process, not just that one task. Only a few corners of the codebase guarded against this.

`logx.Recover(label)` (deferred) and `logx.GoRecover(label, fn)` (a `go fn()` replacement) now wrap essentially every background goroutine across the backend — memory, chat streaming, WhatsApp, cloud sync, local model management, speech-to-text, routines, proactive suggestions, notifications, remote-access tunnels. A panic is logged and contained instead of taking Memo down with it. This is purely defensive — it changes nothing about behavior when everything works normally.

## 10. Claude Code / Codex CLI as Chat Providers (`internal/agentcli/`, beta)
Rather than an HTTP call the way every `internal/provider` type works, `internal/agentcli` shells out to a locally installed `claude`/`codex` CLI binary as a real, stateful, side-effecting background process — deliberately not folded into `internal/provider` itself, since these have their own session/auth model rather than being stateless chat-completion APIs.

- Per-chat, not app-wide: each chat remembers its own CLI provider/working directory/model independently.
- No fixed timeout (unlike the 5-minute budget every other reply has); tracked independently of whichever chat is currently visible.
- Uses each CLI's own no-prompt permission mode (`--dangerously-skip-permissions` for Claude Code, `--dangerously-bypass-approvals-and-sandbox` for Codex) since there's no terminal for either to prompt in.
- Deliberately sends no memory/identity context — the CLI manages its own multi-turn session and project-context mechanisms.
- The CLI's own `/` slash commands (`.claude/commands`, `.codex/prompts`, skills, built-ins) are discovered and surfaced in Memo's own `/` popup, each labeled by origin.

## 11. Developer API Gateway (`internal/anthropicapi/`)
Implements the server side of Anthropic's Messages API wire format (`POST /v1/messages`) — the mirror image of `internal/provider/claude.go`, which implements the *client* side. This lets any tool that only knows how to speak to Anthropic — most notably Claude Code itself, via `ANTHROPIC_BASE_URL` — point at Memo instead, with Memo translating the request to the local model or a configured provider/API key. Model selection uses a `type/model-id` format. Full tool calling works for openai/custom/local/groq/openrouter/grok/opencode-zen/opencode-go providers; gemini/claude/ollama's own provider implementations don't support tools, so a tools-bearing request to one of those returns a clear error instead of silently dropping the tools.

## 12. Routines, Live Mode, and Memo Swarm
- **Routines (`internal/routine/`):** a natural-language description is parsed into a schedule + prompt/agent config, then fired by a background loop in the device's own captured timezone offset (resynced on every reconnect via `POST /api/routines/sync-offset`).
- **Live Mode (`internal/tts/`, beta):** on-device speech transcription (reused from elsewhere) feeds a normal chat message; replies are synthesized with a local **Piper** binary by default (`internal/tts/tts.go`), with an optional external OpenAI TTS provider and automatic fallback to Piper on failure or missing config. A small curated voice catalog (`voice_store.go`) downloads `.onnx`/`.onnx.json` Piper voices from Hugging Face on demand.
- **Memo Swarm (`internal/swarm/`, beta):** pools several machines' compute for one GGUF model too large for any single one of them, via llama.cpp's RPC backend (`rpc-server`). One machine is the Host (holds the model, creates a room/code); others Join and lend compute without downloading the model themselves. Not packaged on macOS yet.
