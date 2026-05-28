# Known Issues & Technical Risks (Exhaustive Audit)

This document tracks all identified bugs, architectural limitations, and edge cases in the Memo project.

## 🧵 Concurrency & Race Conditions
- **Session Manager Mutex Scope:** In `internal/sessions/sessions.go`, while `mu` is used, some operations like `newSession` called during `NewManager` are done without explicit locks (though safe during init). However, rapid switching and adding messages could lead to rare race conditions if not careful with the RWMutex.
- **Llama Process Monitoring:** The `waitDone` channel in `internal/llama/llama.go` could potentially be closed twice in very rare timing edge cases, although a `select` check has been added to mitigate this.
- **Memory Store Re-init:** In `app.go`, `reinitMemoryStore` replaces the `store` pointer. If a RAG search is happening simultaneously in another goroutine, it could cause a nil pointer dereference or data inconsistency.

## 🚀 Performance & I/O
- **Brute-Force Vector Search:** Memory search uses O(N) linear scanning of all embeddings in RAM. While fast for small collections, it will degrade linearly as memory grows (e.g., 10,000+ interactions).
- **Synchronous Persistence:** Every message triggers a synchronous write to both GOB (memory) and JSON (sessions). On slow filesystems or network drives, this will cause noticeable lag in response time.
- **Large Directory Walking:** `ListGobFiles` in `internal/memory/store.go` walks the entire directory. With thousands of files, the Settings -> Memory screen will load slowly.

## 🛡️ Error Handling & Reliability
- **Silent Failures:** Many backend operations in `app.go` (like `autoStartEmbeddingModel` or `saveMemoryAsync`) log errors to `server.log` but do not inform the UI. The user may not know why RAG isn't working.
- **SSE Connection Orphans:** If a client disconnects during an SSE stream, the backend goroutine might continue processing until it tries to write again.
- **WaitReady Hard Timeout:** The 180s timeout for model loading is hardcoded. Extremely large models (e.g., 70B+) on mid-range hardware might exceed this.
- **Port Collision Logic:** While `killByPort` exists, if a port is taken by a critical system process, Memo might fail to start without a clear user-facing explanation.

## 📡 Hardware & OS Compatibility
- **AMD Windows Limitations:** Automated VRAM detection for AMD on Windows is unreliable without `rocm-smi` in the system PATH.
- **Linux Sysfs Paths:** The `/sys/class/drm/card*/` path logic assumes standard Linux kernel naming. Non-standard drivers might break GPU detection.

## 📱 Frontend (Flutter)
- **State Desync:** Frontend does not poll for backend state changes (e.g., if another client changes the model).
- **Memory File Deletion:** Deleting a memory file from the UI doesn't immediately invalidate the LLM's current internal context if it was already "read" into the prompt.
