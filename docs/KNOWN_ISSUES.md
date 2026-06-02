# Known Issues & Technical Risks (Exhaustive Audit)

This document tracks all identified bugs, architectural limitations, and edge cases in the Memo project, updated after a deep codebase audit.

**Priority legend:**
- 🔴 **Critical** — crash, data loss, security vulnerability, or complete feature breakage
- 🟠 **High** — major bug, significant performance issue, or reliability concern
- 🟡 **Medium** — UX degradation, minor bug, or non-critical reliability issue
- 🔵 **Low** — cosmetic, minor improvement, or edge case
- ⚪ **Info** — design note, risk, or observation

All 55 identified bugs have been resolved. This file now only tracks design observations.

---

## ⚪ Info / Observations

### I1. GOB Encoding vs. Forward Compatibility
- **File:** `internal/memory/store.go:302-306`
- **Note:** `chromem.Document` is serialized with Go's `gob` encoding. Gob is sensitive to struct field changes: adding, removing, or renaming fields in a future version will make all existing memory files unreadable. Consider a self-describing format (JSON, CBOR, or protobuf).

### I2. Single-File-Per-Interaction Design
- **File:** `internal/memory/store.go`
- **Note:** Every memory interaction is a separate `.gob` file. This design simplifies deletion (`os.Remove`) but creates pathological behavior for:
  - Startup: O(N) file reads
  - Cloud sync: O(N) API calls per sync
  - File descriptor usage
  - Disk seek times on HDDs

### I3. Filepath.Walk Error Swallowing
- **File:** `internal/memory/store.go:182-196`, `internal/modelstore/modelstore.go:329-331`
- **Note:** Several `filepath.Walk` callbacks return `nil` for all errors. Permission-denied directories and I/O errors are invisible to the user.

### I4. Embedding Client Stale Reference After Reinit
- **File:** `app.go:148-149`, `app.go:124-125`
- **Note:** When `a.client` is swapped (new LLM endpoint), the embedding function in `a.store` still references the old client. The store continues to use the previous endpoint for embeddings until `reinitMemoryStore` is called.

### I5. Model Auto-Classification via Filename
- **File:** `internal/modelstore/modelstore.go:58-64`
- **Note:** `isEmbeddingModel` checks if the filename or repo ID contains embedding-related keywords (bge, e5, etc.). This heuristic misclassifies models that have these strings in their name but are actually chat models.

### I6. `unsanitizePath` Can Inject `/` from `__` in Repo IDs
- **File:** `internal/modelstore/modelstore.go:345`
- **Note:** `unsanitizePath` replaces `__` with `/`. If a HuggingFace repo ID naturally contains `__`, this creates unexpected directory structures. Path traversal is prevented by `filepath.Join` normalization, but directory layout may surprise users.

### I7. Llama Server Stderr Mixed with App Logs
- **File:** `internal/llama/llama.go:118-119`
- **Note:** Subprocess stdout/stderr are set to `os.Stdout`/`os.Stderr`. Llama.cpp's diagnostic output (prompt processing stats, timing, warnings) appears directly in the app's output stream with no prefix or filtering.

### I8. Race Between `UpdateSyncSettings` and `ensureSyncManager`
- **File:** `app.go:1505-1510`, `app.go:1627-1652`
- **Note:** `ensureSyncManager()` reads `a.syncManager` without a lock. `UpdateSyncSettings` sets `syncManager = nil` and creates a new instance without synchronization. Concurrent calls can result in stale or double initialization.

---

> **Last updated:** 2026-06-02  
> **Audit scope:** Full codebase — Go backend (app.go, all internal/ packages) and Flutter frontend  
> **Total observations:** 8
