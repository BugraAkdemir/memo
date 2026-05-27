# Technical Deep Dive

A granular look into the engineering decisions behind Memo.

## 1. The Bridge Pattern (`app.go`)
The `App` struct acts as the central hub. It implements the `AppBridge` interface, which defines all the actions the web server can trigger. This decoupling allows us to theoretically swap the web server for a CLI or a GUI without touching the core logic.

## 2. GOB Persistence vs. SQL
Why no SQLite?
- **Serialization Speed:** GOB is native to Go and requires zero translation layers.
- **Binary Integrity:** Storing embeddings (large float arrays) in SQL blobs is often slower and more complex than direct binary files.
- **Zero-Config:** No migration scripts or DB engines to manage.

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
1. Collect all `.gob` and `.json` files.
2. Compress into a single stream.
3. Encrypt using **AES-256-GCM** with the user's passphrase.
4. Upload to a hidden app-data folder on Google Drive.
5. This ensures even if Google is compromised, the "memories" are useless without the passphrase.
