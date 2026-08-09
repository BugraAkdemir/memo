---
name: memo-project
description: Use when working on the Memo codebase and you need a fast, high-level map of the project architecture, conventions, and safe change workflow
---

# Memo Project Master Skill

## Overview

Memo is a local-first, privacy-focused LLM chat app: Go headless backend + Flutter desktop/mobile UI, SQLite-backed memory, optional E2E cloud sync. This skill gets you oriented fast so you don't burn tokens rediscovering the layout.

## When to Use

- First contact with the codebase
- Planning a change that touches multiple modules
- You need to know which file to open first
- Debugging backend ↔ frontend integration issues
- You want to avoid Memo-specific mistakes

## 60-Second Briefing

| Layer | Tech | Entry Point |
|---|---|---|
| Backend | Go 1.26 | `main.go`, `internal/app/app.go` |
| Frontend | Flutter 3.10+ | `frontend/lib/main.dart` |
| State | Riverpod | `frontend/lib/providers/` |
| HTTP | Dio | `frontend/lib/core/api_client.dart` |
| Memory | SQLite + sqlite-vec | `internal/memory/store.go` |
| Config | YAML | `config/config.yaml` |
| Data | SQLite / JSON | `data/` |

Run backend: `go run . --port 8090`
Run frontend: `cd frontend && flutter run -d linux`

## Module Map

| Module | Responsibility | Key Files |
|---|---|---|
| `internal/app/` | Central orchestrator | `app.go`, `llm.go`, `chat.go`, `memory.go` |
| `internal/webserver/` | REST API (~90 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/memory/` | RAG vector store | `store.go` |
| `internal/provider/` | External LLM providers + router | `router.go`, `openai.go`, `gemini.go`, ... |
| `internal/orchestra/` | Multi-model workflows | `conductor.go`, `roles.go` |
| `internal/agent/` | Tool-calling sandbox | `executor.go`, `pipeline.go`, `tools.go` |
| `internal/cloudsync/` | Google Drive E2E backup | `drive.go`, `crypto.go` |
| `internal/sessions/` | Chat persistence | `sessions.go` |
| `internal/whatsapp/` | WhatsApp bridge | `client.go`, `store.go` |
| `internal/identity/` | System prompt / persona | `identity.go`, `styles.go` |
| `internal/config/` | Configuration | `config.go` |
| `frontend/lib/` | UI | `main.dart`, `providers/`, `widgets/` |
| `frontend/lib/core/` | API client | `api_client.dart` |
| `frontend/lib/widgets/settings/tabs/` | Settings pages | `memory_tab.dart`, `provider_tab.dart`, ... |

## Architecture Rules

### Backend

- `http.ServeMux` — no external router
- `AppBridge` / `FullBridge` decouples HTTP handlers from `App`
- Never store `context.Context` in struct fields (except lifecycle goroutines)
- Concurrency: `sync.RWMutex` is the default tool; document why if you deviate
- SQLite requires CGO: `CGO_ENABLED=1 go build/test/run`
- Plain HTTP/JSON + SSE streaming between backend and UI

### Frontend

- Riverpod `AsyncNotifierProvider` for state
- API client singleton via `Provider<MemoApiClient>`
- Settings split into focused tabs under `settings/tabs/`
- Always add mounted checks before `setState` in async callbacks

### Cross-Cutting

- Config lives in `config/config.yaml`
- Data lives under `data/` (memory, sessions, providers, calendar, whatsapp)
- Error messages may be Turkish or English — follow the existing file's language

## Change Decision Tree

| If you are changing... | Start here | Then check | Also see |
|---|---|---|---|
| LLM routing / providers | `internal/app/llm.go` | `internal/provider/router.go` | provider docs |
| Memory / RAG | `internal/memory/store.go` | `internal/app/memory.go` | `memo` skill |
| HTTP endpoint | `internal/webserver/server.go` | `bridge.go`, `handlers_flutter.go` | Flutter API client |
| UI screen / widget | `frontend/lib/` | `frontend/lib/core/api_client.dart` | provider/state |
| Config / settings | `internal/config/config.go` | `frontend/lib/widgets/settings/` | YAML schema |
| Agent tools | `internal/agent/tools.go` | `permissions.go`, `pipeline.go` | danger levels |
| WhatsApp | `internal/whatsapp/client.go` | Flutter WhatsApp screen | handlers |
| Calendar | `internal/calendar/store.go` | `reminder.go`, `bridge.go` | intent extractor |
| Sync / backup | `internal/cloudsync/` | `crypto.go`, handlers | OAuth flows |
| System prompt | `internal/identity/identity.go` | `styles.go` | identity docs |

## Development Speedrun

```bash
# Terminal 1 — Backend
go run . --port 8090

# Terminal 2 — Frontend
cd frontend && flutter run -d linux
```

Build:

```bash
go build -o memo .
cd frontend && flutter build linux --release
./scripts/build_releases.sh
```

## Testing Protocol

Run these before every commit:

```bash
# Backend
go test ./... -race -count=1 -timeout 60s
go build ./...

# Frontend
cd frontend && flutter analyze
cd frontend && flutter test
```

Add tests when you touch:

- Store logic → `internal/memory/*_test.go`
- Provider routing → `internal/provider/*_test.go`
- Agent tools → `internal/agent/*_test.go`
- Concurrency paths → use `-race`

## Adding an Endpoint (Fast Path)

1. Add method to `FullBridge` in `internal/webserver/bridge.go`
2. Implement on `App` in the relevant `internal/app/<area>.go`
3. Add handler in `internal/webserver/handlers_flutter.go`
4. Register route in `internal/webserver/server.go`
5. Add Dart method in `frontend/lib/core/api_client.dart`
6. Call it from provider/screen/widget
7. Run `go build ./...` and `dart analyze lib/`

## Commit & Docs Protocol

### Commit Messages

Use Conventional Commits:

```
feat(memory): add filtered search
fix(provider): groq timeout handling
docs(obsidian): update RAG note
refactor(webserver): split handler file
```

Keep summary under 72 chars. Body bullet points: what, why, test evidence.

### Documentation Updates

Update these when behavior changes:

- `handoff.md` — session summary, commits, next steps
- `versinNote/v3.1.0.md` + `versinNote/tr/v3.1.0.md` — release notes
- `obsidian-doc/Memo/` — user-facing explanation
- `obsidian-doc-en/Memo/` — English mirror
- `yapılacaklar.md` / `docs/ROADMAP.md` — task completion
- `AGENTS.md` — only if you changed project-wide conventions

## MVP & Stable Discipline

Every change is either **fully stable** or **not committed at all**. There is no middle ground.

### Definition of Stable

A commit is stable only when:

- [ ] `go test ./... -race` passes
- [ ] `go build ./...` passes
- [ ] `flutter analyze lib/` passes (if Flutter touched)
- [ ] Docs updated for user-facing changes
- [ ] No known regressions in existing flows
- [ ] Fallback paths still work
- [ ] Concurrency-safe (no new races)
- [ ] No `TODO`, `FIXME`, or temporary hacks left in code

### Rules

1. **Finish before you switch.** The next step starts only after the current step is stable.
2. **No WIP commits.** If it is not stable, do not commit it. Revert or stash instead.
3. **No "temporary" features.** Do not merge behind a feature flag to "finish later".
4. **One concern per commit.** A stable commit does one thing well.
5. **If you hit a blocker, stop and ask.** Do not invent a half-solution to keep moving.

### Red Flags — This Work Is Not Stable Yet

- "Şimdilik böyle kalsın"
- "Sonra optimize ederiz"
- "Yarım commit atayım"
- "Bu edge case'i sonra hallederiz"
- "Testleri sonra yazarım"
- "Docs sonraya kalsın"

**All of these mean: do not proceed. Stash, revert, or ask for help. Stable first, next step second.**

## Common Pitfalls

| Mistake | Why it hurts | Fix |
|---|---|---|
| Adding endpoint without bridge | Compile error / inconsistent API | Add to `FullBridge` first |
| Forgetting Flutter API client | UI cannot call backend | Add to `api_client.dart` |
| CGO disabled | SQLite build fails | `CGO_ENABLED=1` |
| Long-lived context in struct | Leaks / misuse | Pass `ctx` as parameter |
| Skipping `storeMu` | Races during memory re-init | Lock around `a.store` |
| Not mirroring vec/FTS writes | Stale search results | Update all three tables |
| Mounted check missing | Flutter crash | `if (!mounted) return;` |
| Mixing unrelated refactor with feature | Harder review, riskier rollback | One concern per commit |

## Troubleshooting Matrix

| Symptom | Likely Cause | Fix |
|---|---|---|
| Backend won't start | Port 8090 in use / CGO disabled | Check port, set `CGO_ENABLED=1` |
| Flutter can't connect to backend | Wrong host/port | Verify `MemoApiClient` base URL |
| SQLite `database is locked` | Concurrent writes without `db.Write` queue | Use `database.DB.Write` |
| Memory not retrieved | Embedding backend down / store nil | Check `CheckEmbeddingHealth`, `storeMu` |
| Provider fallback not working | Router misconfig / health goroutine | Check `provider.Router` enabled list |
| Agent tool blocked | Permission policy / danger level | Check `data/permissions.json` |
| WhatsApp QR not shown | Polling stopped / not paired | Check client state, restart pairing |
| Cloud sync fails | OAuth token / crypto key mismatch | Re-auth, verify `data/machine.key` |

## Quick Reference

| Task | Entry |
|---|---|
| Run backend | `go run . --port 8090` |
| Run frontend | `cd frontend && flutter run -d linux` |
| Run tests | `go test ./... -race` |
| Build release | `./scripts/build_releases.sh` |
| API docs | `obsidian-doc/Memo/API Dokümantasyonu.md` |
| Config | `config/config.yaml` |
| Data | `data/` |
| Module-specific memory guide | `memo` skill |

## Red Flags — Stop and Re-read This Skill

- "I'll change the API without updating the Flutter client"
- "I don't need a test for this small fix"
- "CGO is optional"
- "I'll write the docs later"
- "This doesn't affect other modules"
- "I'll refactor the whole file while I'm here"

**All of these mean: slow down, read the module map, write the test, update the bridge/client/docs.**
