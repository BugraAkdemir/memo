---
name: memo-rag-memory
description: Use when modifying, debugging, or extending Memo's RAG semantic memory subsystem and you need a fast, disciplined workflow without sacrificing code quality
---

# Memo RAG Memory System

## Overview

Memo's memory is a local-first, SQLite-backed RAG store. This skill is a fast, disciplined workflow for changing it safely without cutting corners on tests, docs, or concurrency.

## When to Use

- Before editing `internal/memory/store.go`, `internal/app/memory.go`, `internal/app/helpers.go`, or `internal/models/memory.go`
- Before adding memory endpoints, filters, lifecycle rules, or retrieval stages
- When debugging recall issues, hallucinated memories, or stale results
- When a memory change touches Flutter, HTTP API, or user-facing docs

## What Are You Doing? — Decision Tree

| If you are... | Start at | Key file |
|---|---|---|
| Fixing a retrieval bug | Troubleshooting Matrix | `internal/memory/store.go` |
| Adding a filter or search stage | Implementation Playbook > Store Change | `internal/memory/store.go` |
| Adding an HTTP endpoint | Implementation Playbook > App/Bridge Change | `internal/webserver/` |
| Adding a UI control | Implementation Playbook > UI Change | `frontend/lib/` |
| Tuning scoring or thresholds | Code Quality Guardrails | `internal/memory/store.go` |
| Refactoring large sections | Red Flags | — |

## Pre-Flight Reading Map

Read these in order before writing code:

| Order | File | What to look for |
|---|---|---|
| 1 | `internal/memory/store.go` | Schema, retrieval pipeline, lifecycle goroutine |
| 2 | `internal/app/memory.go` | Orchestration, `storeMu`, error handling |
| 3 | `internal/app/helpers.go` | `buildMemoryQuery` enrichment logic |
| 4 | `internal/webserver/bridge.go` | `FullBridge` interface contract |
| 5 | `handoff.md` | Last session's commits and known limitations |
| 6 | `AGENTS.md` | Project conventions and known pitfalls |

## Implementation Playbook

### Store Change (`internal/memory/store.go`)

1. Add schema migration in `initSchema` if you added a column or index.
2. Update all three stores inside one transaction:
   - `memories` (canonical row)
   - `vec_memories` (guard with `if s.useVec`)
   - `memories_fts` (guard with `if s.useFTS`)
3. Add or update a unit test in `internal/memory/`.
4. Run `go test ./internal/memory/... -race`.

### App / Bridge / Handler Change

1. Add the method to `FullBridge` in `bridge.go`.
2. Implement it on `App` in `internal/app/memory.go`.
3. Add the HTTP handler in `handlers_flutter.go`.
4. Register the route in `server.go`.
5. Add the Dart API client method in `frontend/lib/core/api_client.dart`.
6. Add provider/screen logic in Flutter.

### UI Change

1. Call the API through a provider or repository.
2. Add the widget or control.
3. Add user-facing help text or command hint when behavior is not obvious.
4. Run `dart analyze lib/` and Flutter tests if they exist.

## Code Quality Guardrails

| Rule | Why | Pattern |
|---|---|---|
| Mirror writes across stores | Missing vec/FTS updates cause stale search results | One `s.db.Write` tx inserts/deletes from all three |
| Guard `useVec` and `useFTS` | sqlite extensions may be unavailable | `if s.useVec { ... }` around every vec operation |
| Hold `storeMu` in `App` | `a.store` can be re-initialized concurrently | `RLock()` for reads, `Lock()` for writes/re-init |
| Never block UI on embedding | Embedding calls can take seconds | Route saves through `memorySaveCh` |
| Keep migrations idempotent | `NewStore` can be called multiple times | `CREATE IF NOT EXISTS`, column-existence checks |
| Validate embedding dimensions | vec0 schema mismatch corrupts search | Return error if `len(embedding) != s.dim` |
| Preserve fallback paths | App must work without vec0/FTS5 | `goSearch` when `useVec == false` |
| Use contexts with timeouts | Prevents hung goroutines | `context.WithTimeout(..., 5*time.Second)` |

## Testing Protocol

Run these before every commit:

```bash
# Required
go test ./internal/memory/... -race -count=1 -timeout 30s
go build ./...

# If Flutter touched
cd frontend && dart analyze lib/
```

Add tests when you change:

- Filters → `FilteredSearch`, `sqlFilteredFallback`
- Lifecycle → `MarkStaleForDeletion`, `PurgePendingDeletions`
- Scoring → RRF, importance boost, topK truncation
- Concurrency → store re-init during save/retrieve
- Schema → migration idempotency

## Commit & Docs Protocol

### Commit Message Template

```
<type>(memory): <short summary under 72 chars>

- What changed
- Why
- Test evidence (e.g., `go test ./internal/memory/... -race`)
```

Use Conventional Commits types: `feat`, `fix`, `docs`, `refactor`, `test`.

### Docs Checklist

Update these when user-facing behavior changes:

- [ ] `handoff.md` — session summary, commits, next steps
- [ ] `versinNote/v3.1.0.md` + `versinNote/tr/v3.1.0.md` — feature/fix note
- [ ] `obsidian-doc/Memo/RAG ve Semantik Hafıza.md` — retrieval logic details
- [ ] `obsidian-doc-en/Memo/RAG and Semantic Memory.md` — English mirror
- [ ] `yapılacaklar.md` / `docs/ROADMAP.md` — task completion or new follow-ups

Do **not** change `AGENTS.md` unless you altered project-wide conventions.

## MVP & Stable Discipline

Every memory change is either **fully stable** or **not committed at all**.

A memory commit is stable only when:

- [ ] `go test ./internal/memory/... -race` passes
- [ ] `go build ./...` passes
- [ ] `flutter analyze lib/` passes (if Flutter touched)
- [ ] Vec/FTS writes mirrored in the same transaction
- [ ] `storeMu` used on all `App` paths
- [ ] Docs updated for user-facing changes
- [ ] No `TODO`, `FIXME`, or temporary hacks

### Rules

1. **Finish before you switch.** Next memory task starts only when the current one is stable.
2. **No WIP commits.** Unstable work is stashed or reverted, never committed.
3. **No "temporary" retrieval stages.** Do not leave half-implemented scoring or filters.
4. **If blocked, stop and ask.** Do not invent a fragile workaround to keep moving.

### Red Flags — Not Stable Yet

- "Şimdilik böyle kalsın"
- "Bu edge case'i sonra hallederiz"
- "Testleri sonra yazarım"
- "Docs sonraya kalsın"
- "Store mutex gerekmez"

**Stable first. Next step second.**

## Architecture Reference

### Storage Model

| Table | Purpose | Fallback |
|---|---|---|
| `memories` | Canonical rows with BLOB embedding and metadata | Always present |
| `vec_memories` | sqlite-vec virtual table (cosine, `FLOAT[dim]`) | `goSearch` |
| `memories_fts` | FTS5 over `content`, `user_msg`, `assist_msg` | Disabled if unavailable |
| `_metadata` | Embedding dimension and migration flags | — |

### Retrieval Pipeline

1. Enrich query with up to 3 prior user turns (`buildMemoryQuery`)
2. Embed the enriched query
3. Vector search: `candidateK = min(max(topK*5, 20), 100)`
4. Multi-query expansion if query is > 7 words
5. FTS5 token search
6. RRF fusion with `k = 60`
7. Noise filter: `minRRFScore >= 0.008`
8. Importance boost: `similarity *= 0.8 + importance * 0.1`
9. Re-sort and truncate to `topK`
10. Increment `retrieve_count` asynchronously

### Critical Constants

| Constant | Value | Purpose |
|---|---|---|
| `minRRFScore` | 0.008 | RRF noise floor |
| RRF `k` | 60 | Fusion damping |
| Importance boost | `0.8 + imp * 0.1` | ×0.9 to ×1.3 re-scoring |
| `chunkMaxWords` | 300 | User-message chunk size |
| `chunkOverlapWords` | 50 | Chunk overlap |
| Decay initial delay | 5 min | First lifecycle pass |
| Decay period | 24 h | Later passes |
| Stale mark | 180 days | Soft-delete mark |
| Hard purge | 187 days | Permanent deletion |

## Troubleshooting Matrix

| Symptom | Likely Cause | Fix |
|---|---|---|
| "Memo forgot X" | `minRRFScore` too high or query enrichment missing | Check `buildMemoryQuery`; lower threshold carefully |
| Irrelevant memories in prompt | FTS noise or boost applied wrong | Verify RRF filter and boost-then-sort order |
| Stale results after delete | vec/FTS rows not mirrored | Delete from all three stores in one tx |
| Memory analytics tab crashes | Missing mounted check in async callback | Add `if (!mounted) return;` in Flutter |
| UI freeze after send | Embedding on request hot path | Route through `memorySaveCh` |
| vec0 error on startup | Embedding dimension mismatch | `ensureVecMetadata` drops and recreates table |
| High CPU on retrieval | Full table scans or Go fallback on large DB | Add index; verify vec0 loaded |
| Import loses some rows | Duplicate UUIDs | `INSERT OR IGNORE` already handles this; verify JSON schema |

## Quick Reference

| Task | Entry Point |
|---|---|
| Save conversation turn | `App.saveMemoryAsync` → `Store.SaveInteraction` |
| Retrieve context | `App.retrieveMemory` → `Store.RetrieveContext` |
| Explicit remember | `App.SaveExplicitMemory` → `Store.SaveExplicit` |
| Explicit forget | `App.DeleteExplicitMemory` → `Store.DeleteByContent` |
| Filtered search | `App.FilteredMemorySearch` → `Store.FilteredSearch` |
| Analytics | `App.GetMemoryStats` → `Store.Stats` |
| Export / import | `Store.Export` / `Store.Import` |
| Health check | `App.CheckEmbeddingHealth` |

## Red Flags — Stop and Re-read This Skill

- "I'll tweak this constant without a test"
- "The fallback path is rare, I'll skip it"
- "I don't need `storeMu` here"
- "Docs can wait until after merge"
- "I'll refactor the whole file while I'm here"
- "This UI change doesn't need backend"

**All of these mean: slow down, test first, mirror writes, update docs.**
