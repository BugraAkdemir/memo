# Technical Deep Dive

A granular look into the engineering decisions behind Memo.

## 1. The Bridge Pattern (`app.go`)
The `App` struct acts as the central hub. It implements the `AppBridge` interface, which defines all the actions the web server can trigger. This decoupling allows us to theoretically swap the web server for a CLI or a GUI without touching the core logic.

## 2. SQLite + vec0 Persistence
Why SQLite?
- **Unified Storage:** Both vector embeddings and metadata live in the same database — no separate `.gob` files to manage.
- **ANN Indexing:** The `sqlite-vec` extension provides a `vec0` virtual table for approximate nearest neighbor search, replacing O(N) brute-force scans with O(log N) lookups.
- **ACID Compliance:** Built-in transaction support ensures atomic writes without per-file crash risk.
- **Zero-Config:** SQLite requires no external server — the database is a single file in `data/memory/`.

## 3. Llama Process Lifecycle
Memo doesn't just "call" Llama; it manages its lifecycle.
- **Health Checks:** Periodic pings to ensure the model is responsive.
- **Port Management:** If the default port is taken, Memo increments until it finds a free one, then updates the API client dynamically.
- **Cleanup:** On application exit, Memo sends a `SIGTERM` to all child processes to prevent "zombie" servers.

## 4. Multi-Worker Vector Search
When a user queries their memory:
1. The query is embedded.
2. The search space is divided into chunks.
3. Multiple Go routines (workers) calculate Cosine Similarity in parallel.
4. Results are gathered, sorted, and filtered by the `top_k` and `min_similarity` thresholds.

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
