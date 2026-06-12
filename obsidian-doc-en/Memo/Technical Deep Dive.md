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
data/memory/memo.db
├── vec0 table          ← Vector ANN index (sqlite-vec)
├── documents table     ← Content and metadata
└── metadata table      ← Collection info
```

| Table | Columns | Purpose |
|-------|---------|---------|
| `documents` | `id`, `content`, `created_at`, `metadata_json` | Memory records |
| `vec0` | `id`, `embedding` | Vector ANN index |
| `metadata` | `key`, `value` | Collection metadata |

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

### Provider Types
| Type | Providers | Implementation Pattern |
|------|-----------|----------------------|
| OpenAI-compatible | OpenAI, Grok, Groq, OpenRouter, Ollama | Common `openAIProvider` with different base URLs |
| Gemini | Google Gemini | Custom `generateContent`/`streamGenerateContent` |
| Claude | Anthropic Claude | Custom Messages API with `x-api-key` auth |

### Encrypted Config
- API keys stored in `data/providers.json`
- Encrypted with AES-256-GCM
- Encryption key derived from `/etc/machine-id`
- Config is non-portable across machines by design

## 7. Agent Engine (`internal/agent/`)

### Tool Registry
Thread-safe registry holding 8 built-in tools:

| Tool | Danger Level | Description |
|------|-------------|-------------|
| `read_file` | Safe | Read file contents |
| `write_file` | Medium | Write/create files |
| `delete_file` | Dangerous | Delete files |
| `list_directory` | Safe | List directory contents |
| `run_command` | Dangerous | Execute shell commands |
| `search_files` | Safe | Search files by pattern |
| `get_file_info` | Safe | Get file metadata |
| `read_env` | Safe | Read environment variables |

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
5. Repeat until final response or 20 iterations

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
