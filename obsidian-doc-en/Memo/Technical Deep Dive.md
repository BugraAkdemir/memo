# Technical Deep Dive

A granular look into the engineering decisions behind Memo.

---

## 1. The Bridge Pattern (`app.go`)

The `App` struct acts as the central hub. It implements the `AppBridge` interface (`internal/webserver/bridge.go`), which defines all the actions the web server can trigger. This decoupling allows swapping the web server for a CLI or GUI without touching core logic.

```
HTTP Handlers → AppBridge interface → App implementation
```

The `FullBridge` interface extends `AppBridge` with Flutter-specific endpoints.

## 2. SQLite + vec0 Persistence

Why SQLite?
- **Unified Storage**: Vector embeddings and metadata live in the same database — no separate `.gob` files to manage
- **ANN Indexing**: `sqlite-vec` provides a `vec0` virtual table for approximate nearest neighbor search, replacing O(N) brute-force scans with O(log N) lookups
- **ACID Compliance**: Built-in transaction support ensures atomic writes without per-file crash risk
- **Zero-Config**: SQLite requires no external server — the database is a single file in `data/memory/`

### Database Schema

```
data/memory/memory.db
├── memories table       ← Content, importance, source (conversation/explicit/merged), embedding blob
├── memories_fts table   ← FTS5 virtual table — keyword search
├── vec_memories table   ← vec0 virtual table (sqlite-vec) — vector ANN index
└── _metadata table      ← Migration flags, embedding dimension
```

| Table | Purpose |
|-------|---------|
| `memories` | Memory records — content, timestamp, importance, source, embedding blob |
| `memories_fts` | FTS5 keyword search (requires `-tags "sqlite_fts5"`, see [[CGO Flags]]) |
| `vec_memories` | Vector ANN index (`vec0`) |
| `_metadata` | Internal state |

Search is **hybrid**: vector similarity and FTS5 keyword search both run, merged via Reciprocal Rank Fusion — see [[RAG and Semantic Memory]] and [[Vector Search Logic]] for the full retrieval pipeline (compound-question splitting, pinned facts, importance re-weighting).

### Go Fallback Mode

When `vec0` extension is not available, Memo falls back to pure Go:
- Vectors stored as BLOBs in `memories.embedding` column
- Cosine similarity calculated in Go
- Same interface, slower performance

## 3. Llama Process Lifecycle

Memo manages the full lifecycle of `llama-server`:

1. **Start**: Finds a free port, spawns `llama-server` as subprocess with configured GGUF model
2. **Health Checks**: Periodic pings to ensure model responsiveness
3. **Port Management**: If default port is taken, increments until finding a free one; updates API client dynamically
4. **GPU Detection**: Automatic NVIDIA/AMD VRAM detection; configurable `n_gpu_layers`
5. **Cleanup**: On application exit, sends `SIGTERM` to all child processes via process group

### Embedded Server Lifecycle

```
Start → Port discovery → Spawn process → Health check loop → Stop (SIGTERM)
                                                              ↓
                                                         Cleanup
```

## 4. Multi-Worker Vector Search

When a user queries their memory:

1. Query text is sent to the embedding model → vector embedding
2. Search space is divided into chunks
3. Multiple Go routines (workers) calculate Cosine Similarity in parallel
4. Results are gathered, sorted by similarity score
5. Filtered by `top_k` (max results) and `min_similarity` threshold
6. Retrieved contexts are injected into the LLM prompt

## 5. E2E Sync Strategy (Google Drive)

1. Collect SQLite database + all `.json` files
2. Compress into a single stream
3. Encrypt using AES-256-GCM with user's passphrase (PBKDF2, 600K iterations)
4. Upload to hidden app-data folder on Google Drive
5. Even if Google is compromised, data is useless without the passphrase

### Encryption Details

- **Algorithm**: AES-256-GCM
- **Key Derivation**: PBKDF2 (600,000 iterations) + random 16-byte salt
- **Salt Position**: Prepended to ciphertext
- **Fallback**: Machine-specific UUID from `data/.machine-id` if no passphrase
- **Legacy**: Old SHA-256 derived keys still work for decryption

## 6. External Provider System (`internal/provider/`)

### Provider Interface
All providers implement a common interface:

```go
type Provider interface {
    ChatCompletion(ctx context.Context, req api.ChatCompletionRequest) (*api.ChatCompletionResponse, error)
    ChatCompletionStream(ctx context.Context, req api.ChatCompletionRequest) (<-chan api.StreamEvent, error)
    ListModels(ctx context.Context) (*api.ModelList, error)
}
```

### Router with Fallback
- Ordered list of providers
- On failure (rate limit, timeout, auth error) → auto-falls to next
- After 3 consecutive failures → auto-disabled
- Health check goroutine periodically tests disabled providers → re-enables on recovery

### Provider Types (13, up from 7)
| Type | Providers | Implementation Pattern |
|------|-----------|----------------------|
| OpenAI-compatible | OpenAI, Grok, Groq, OpenRouter, Ollama, Custom, OpenCode Zen, OpenCode Go | Common `openAIProvider` with different base URLs |
| Gemini | Google Gemini | Custom `generateContent`/`streamGenerateContent` |
| Claude | Anthropic Claude | Custom Messages API with `x-api-key` auth |
| CLI-based (Beta, v3.3.4) | Claude Code CLI, Codex CLI | Subprocess (`internal/agentcli/`), not an HTTP call — see [[External Providers]] |

### Encrypted Config
- API keys stored in `data/providers.json`
- Encrypted with AES-256-GCM
- Encryption key derived from `/etc/machine-id`
- Config is non-portable across machines by design

## 7. Agent Engine (`internal/agent/`)

### Tool Registry
Thread-safe registry holding 19 built-in tools (up from the original 8) — see [[Agent Mode]] for the full list with schemas:

| Tool | Danger Level | Description |
|------|-------------|-------------|
| `read_file` | Safe | Read file contents |
| `write_file` | Medium | Write/create files |
| `delete_file` | Dangerous | Delete files |
| `list_directory` | Safe | List directory contents |
| `run_command` | Dangerous | Execute shell commands |
| `search_files` | Safe | Search files by pattern |
| `get_file_info` | Safe | Get file metadata |
| `read_env` | Medium | Read environment variables |
| `edit_file` / `insert_line` / `delete_lines` | Medium | Line-level file editing with diff preview |
| `web_search` | Safe | DuckDuckGo, model decides when it's actually needed |
| `self_clone` | Dangerous | Copy the project to another local directory |
| `configure_provider` | Dangerous | Add/update a provider from chat |
| `get_calendar_events` | Safe | Read real calendar data |
| `whatsapp_send` / `whatsapp_search` / `whatsapp_latest` / `whatsapp_messages` | Medium/Safe | WhatsApp tools |

**Skill tools (v3.3.3):** a skill's `SKILL.md` `command:` field now executes for real through this exact pipeline (`internal/skill/executor.go`), previously purely declarative.

Each tool has a JSON Schema parameter definition.

### Permission Manager
When the LLM requests a tool call:
1. Evaluate action against stored policies
2. Safe tools → auto-allowed
3. Medium/Dangerous → prompt user via event channel
4. 6 policies: PromptAlways, AllowOnce, AllowSession, AllowForever, DenyOnce, DenyForever

### Execution Sandbox
- Path validation: symlink resolution, `..` traversal prevention, project root restriction
- Command blacklist: 23 dangerous patterns blocked
- Rate limiting: 30 calls/minute, 5-second cooldown
- Audit trail: last 1000 entries in memory

### Pipeline
Core loop:
1. Send user message + tool definitions to LLM
2. If tool calls requested → check permissions
3. Execute tool in sandbox
4. Feed result back to LLM
5. Repeat until final response or 40 iterations

As of v3.3.4, agent mode's tool schema (12+ tool definitions sent on every request) is correctly counted against the model's context budget — previously it wasn't, so even a one-word message could fail outright on a small-context local model with a confusing "request exceeds the available context size" error. Default local context was also raised 4096 → 8192.

## 8. Orchestra Mode (`internal/orchestra/`)

### Conductor Pattern
Three-phase workflow managed by `Conductor`:

1. **Plan** — Chief model analyzes request, outputs JSON plan with tasks, role assignments, dependency graph
2. **Execute** — Tasks run in parallel (independent) or sequential (based on `depends_on`). Each task creates a fresh provider instance. Retry with exponential backoff
3. **Synthesize** — Chief model receives all task results, produces coherent final response

### Built-in Expert Roles
| Role | Default Model | Purpose |
|------|--------------|---------|
| Planner | Claude | Architecture, task decomposition |
| Frontend | Grok | UI development |
| Backend | GPT-4o | API/server |
| Bug Fixer | Gemini | Debugging |
| Reviewer | Claude | Code quality |
| Security | GPT-4o | Security audit |
| DevOps | Grok | Infrastructure/deploy |
| General | GPT-4o | General purpose |

### Progress Streaming
Each phase emits typed progress updates:
- `ProgressPlan` — planning started/completed
- `ProgressTaskStart` — task execution begins
- `ProgressTaskChunk` — streaming token from worker
- `ProgressTaskResult` — task completed with result
- `ProgressSynthChunk` — synthesis streaming tokens
- `ProgressError` — error occurred

### Limitations
- Provider bypass: unlike normal chat, orchestra creates providers directly via factory (no Router fallback)
- Config validation: no validation at config time — runtime error if chief model is invalid

## 9. Backend-Wide Panic Recovery (v3.3.4)

Go gives automatic panic protection to a single HTTP request handler, but none at all to code running in a background goroutine — an unhandled panic there takes down the *entire* process, not just that task. Before v3.3.4, only three small corners of the codebase guarded against this.

Every background task across the backend is now wrapped in recovery: memory saving, chat streaming, WhatsApp message handling, cloud sync, local model management, speech-to-text, routines, proactive suggestions, notifications, and remote-access tunnels. An unexpected error in any one of them is logged and contained instead of crashing Memo. This is purely a stability floor — it doesn't change any feature's behavior when things are working normally.

## 10. Memory/Local-Generation Performance Fix (v3.3.4)

Reported directly: turning on memory/RAG for a local model could cut generation from ~10 tokens/sec to 2-3. Two compounding causes:

1. The dedicated embedding server auto-started with GPU auto-detect, sized as if it were the only model running, even while the chat model's own server was already resident and using most of the VRAM — the two oversubscribed together, pushing the chat model into partial CPU fallback. **Fixed:** the embedding server now defaults to CPU-only (small enough to stay fast there); `embedding_gpu_layers` opts it back onto the GPU if there's real headroom.
2. The memory block injected into every prompt (retrieved results + pinned `/remember` facts, which only grow over a session) had an old 16K-token budget that was never a real ceiling. **Fixed:** capped at 4096 tokens.
