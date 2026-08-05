# 💾 Data Layer and Persistence

Memo uses a single embedded **SQLite** database for memory storage — there is no "one file per interaction" structure; everything lives in one file (`data/memory/memory.db`).

## SQLite Format: Persistence

### Advantages:
- **ACID Transactions:** Errors during a write can't corrupt the whole database.
- **Concurrent Access:** WAL (Write-Ahead Logging) mode allows reads and writes to happen at the same time.
- **Incremental Writes:** Saving a new message runs a single `INSERT` — the whole database is never rewritten.

## Actual Schema (`internal/memory/store.go`)

A single `memory.db` file contains several tables/virtual tables:

| Table | Purpose |
|-------|---------|
| `memories` | The real rows: content, timestamp, `importance`, `source` (`conversation`/`explicit`/`merged`), embedding blob |
| `memories_fts` | FTS5 virtual table — for keyword (full-text) search |
| `vec_memories` | `sqlite-vec`'s `vec0` virtual table — for vector ANN search |
| `_metadata` | Internal state: migration flags, embedding dimension, etc. |

If `vec0` or `fts5` isn't available at build time (see [[CGO Flags]]), that specific capability silently disables itself and falls back to a Go-side search path — which is why building the backend with the correct build flags is critical.

## Folder Structure (`data/`)
- `data/memory/memory.db`: All memory (single file).
- `data/sessions/`: Chat history (JSON format, separate from memory).
- `data/models/`: Downloaded GGUF model files.
- `data/sync_token.json`: Cloud sync authorization data (device-specific — excluded from `.memo` backups).
- `data/routines/`: Scheduled automations (Routines, v3.3.3).
- `data/calendar/`: Calendar events (`events.db`).
- `data/profile/`: Learned habits and pending proactive suggestions.
- `data/tasklists/`: Task lists.
- `data/stats/`: Usage stats (v3.3.3, Settings → Stats).
- `data/skills/`: Installed skills.
- `data/agent-backups/`: `.bak` files the `write_file`/`edit_file` tools create before overwriting.
- `data/permissions.json`: Agent tool permission policies.
- `data/machine.key`: Decrypts `providers.json`'s encrypted API keys — see [[Backup & Restore]] for why this matters.
- `data/tailscale/`: Machine-specific tsnet identity (device-specific — excluded from `.memo` backups).

## Memory (RAM) Management

There is **no** separate "preload all vectors into RAM" mechanism — every search queries SQLite directly, at the time it's needed:
- If `sqlite-vec` is available, the `vec0` virtual table keeps its own ANN index on disk; the query goes to disk.
- Otherwise, the Go-side fallback reads all embeddings once and computes cosine similarity in memory (fine for small/medium memory stores, not a persistent RAM cache).

### Linked Notes:
- [[RAG and Semantic Memory]]
- [[Vector Search Logic]]
