# AGENTS.md — Memo

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync. Designed for offline desktop use with optional API fallback.

---

## Tech Stack

| Layer | Technology | Version |
|-------|-----------|---------|
| Backend | Go | 1.26 |
| Frontend | Flutter | 3.10+ |
| State | Riverpod | 2.4 |
| HTTP | Dio | 5.4 |
| Markdown | flutter_markdown | 0.6 |
| Vector DB | SQLite + sqlite-vec | — |
| SQLite driver | mattn/go-sqlite3 | — |

---

## Architecture

**Two-process decoupled:** Go backend (headless REST API, port `:8090`) + Flutter desktop UI. Communication is plain HTTP/JSON + SSE streaming — no TLS.

**Bridge pattern:** `AppBridge` interface in `internal/webserver/bridge.go` decouples HTTP handlers from the `App` orchestrator. `FullBridge` extends it for Flutter-specific endpoints.

### Module Map

| Directory | Responsibility | Key Files |
|-----------|---------------|-----------|
| `internal/app/` | Central orchestrator (25 files) | `app.go`, `llm.go`, `chat.go`, `learning.go`, `whatsapp.go` |
| `internal/replcli/` | Terminal REPL client — talks to the REST API the same way Flutter does | `client.go`, `repl.go`, `sse.go`, `agent_event.go`, `clients_client.go`, `commands.go`, `editor.go`, `keys.go`, `models_client.go`, `remote_client.go`, `sessions_client.go`, `tasklist_client.go` |
| `internal/webserver/` | REST API (~45 endpoints) | `server.go`, `handlers_flutter.go`, `bridge.go` |
| `internal/llama/` | llama.cpp subprocess lifecycle | `llama.go`, `installer.go`, `gpu.go` |
| `internal/memory/` | Vector store (SQLite + sqlite-vec) | `store.go`, `embedder.go` |
| `internal/database/` | SQLite connection management | `sqlite.go`, `vec_register.go` |
| `internal/provider/` | External LLM providers (9 types) | `provider.go`, `router.go`, `openai.go`, `gemini.go`, `claude.go`, `grok.go`, `groq.go`, `openrouter.go`, `ollama.go`, `llamacpp.go`, `opencode_zen.go`, `opencode_go.go` |
| `internal/orchestra/` | Multi-model orchestration | `conductor.go`, `roles.go`, `types.go` |
| `internal/agent/` | Agent / tool execution sandbox | `executor.go`, `pipeline.go`, `sandbox.go`, `permissions.go`, `tools.go`, `tools/` |
| `internal/cloudsync/` | Google Drive E2E encrypted backup | `drive.go`, `crypto.go`, `sync_manager.go` |
| `internal/sessions/` | Chat session persistence | `sessions.go` |
| `internal/modelstore/` | HuggingFace model search/download | `modelstore.go` |
| `internal/identity/` | System prompt & persona | `identity.go`, `styles.go` |
| `internal/config/` | Configuration management | `config.go` |
| `internal/api/` | llama.cpp / OpenAI-compatible API client | `client.go`, `streaming.go`, `types.go` |
| `internal/calendar/` | Calendar event store + reminder loop | `store.go`, `reminder.go`, `event.go`, `bridge.go` |
| `internal/intent/` | Intent extraction pipeline | `extractor.go`, `filter.go`, `result.go`, `decider_factory.go` |
| `internal/proactive/` | Proactive suggestion engine | `engine.go`, `decision.go`, `matcher.go`, `pending.go`, `prompt.go`, `feedback.go` |
| `internal/observer/` | Usage pattern analyzer | `analyzer.go`, `recorder.go`, `store.go`, `pattern.go` |
| `internal/whatsapp/` | WhatsApp bridge (whatsmeow) | `client.go`, `store.go` |
| `internal/whisper/` | Speech-to-text (whisper.cpp) | `whisper.go`, platform-specific files |
| `internal/skill/` | Skill system (plugin-like) | `manager.go`, `loader.go`, `types.go`, `executor.go` |
| `internal/ngrok/` | ngrok tunnel integration | `installer.go`, `manager.go` |
| `internal/tunnel/` | Tailscale embedded tunnel (tsnet) | `tailscale.go` |
| `internal/truncate/` | Token-aware context truncation | `tokens.go` |
| `internal/models/` | Shared data types | `memory.go` |
| `internal/lora/` | LoRA adapter building (embryonic) | `build/` (cmake artifacts) |

### Flutter Entrypoint

`frontend/lib/main.dart` → `AppShell` (NavRail + ChatScreen / ModelStoreScreen / WhatsAppScreen). State management via Riverpod `AsyncNotifierProvider`. API client is a singleton via `Provider<MemoApiClient>`.

---

## LLM Routing (Priority Order)

Defined in `internal/app/llm.go` `callLLMStream()`:

1. **Orchestra mode** — multi-model workflow (if `orchestraConductor.Config().Enabled`)
2. **External provider** — `provider.Router` with fallback chain (if `activeProvider` is set)
3. **Local llama.cpp** — `api.Client` pointed at local `llama-server`

---

## Config & Data Layout

| Path | Purpose |
|------|---------|
| `config/config.yaml` | All settings (llama, sync, identity, memory, API, learning, calendar) |
| `data/memory/` | SQLite + vec0 vector store |
| `data/sessions/` | JSON chat history |
| `data/models/` | Downloaded GGUF files |
| `data/providers.json` | External provider config + encrypted API keys |
| `data/orchestra.json` | Orchestra mode config |
| `data/whatsapp/` | WhatsApp SQLite message store + whatsmeow session |
| `data/calendar/` | Calendar events SQLite DB |
| `data/permissions.json` | Agent tool permission policies |
| `.env` | Optional environment overrides (OAuth creds, API keys) |
| `binaries/` | Platform-specific binaries (llama-server, vec0 extension) |

---

## Development

### Quick Start

```bash
# Interactive terminal chat (starts the backend if needed, opens a REPL)
go run -tags "sqlite_fts5" .

# Headless backend only, to pair with the Flutter frontend
go run -tags "sqlite_fts5" . --headless --port 8090
cd frontend && flutter run -d linux
```

`-tags "sqlite_fts5"` is required, not optional — see "Memory / Vector Store"
below and `docs/CGO_FLAGS.md`. Without it, `mattn/go-sqlite3` silently skips
FTS5 support and memory retrieval permanently degrades to vector-only search,
with no visible error.

`memo` auto-detects whether stdin is an interactive terminal: interactive →
opens the terminal REPL (agent mode on by default, see `internal/replcli/`);
non-interactive or `--headless` → today's headless server, unchanged. If a
backend is already listening on the port, a second `memo` invocation attaches
to it instead of trying to bind again.

### Build

```bash
go build -tags "sqlite_fts5" -o memo .                # backend binary
cd frontend && flutter build linux --release         # frontend binary
./build_releases.sh                                  # dist packages (tar.gz, AppImage, deb) — already tagged
```

### Testing

```bash
go test ./...                          # all backend tests
cd frontend && flutter test            # all frontend tests
cd frontend && flutter analyze         # lint
```

Ad-hoc API smoke tests: `dart run test_api_all.dart` (requires running backend).

CI: GitHub Actions runs Go vet/test/build + Flutter analyze/test on every push/PR.

### Release

Never release from memory — use the **memo-release skill**
(`.claude/skills/memo-release/SKILL.md`). It carries the full checklist:
seven version locations, EN+TR release notes, per-platform build commands,
the versioned→generic artifact rename for `download.bugradev.com`, and the
`version.json` update beacon that must be bumped LAST.

---

## Known Pitfalls & Technical Debt

### Data Races
- ~~`a.client`/`providerRouter` reassignment during active streams — data race~~ → verified 2026-07-10: not actually a race, both reads (`internal/app/llm.go`) and writes (`internal/app/llama.go`, `providers.go`) are correctly `clientMu`/`providerMu`-locked. The narrower risk that survived the reclassification (a stream copies the client into a local var under lock at request start, then holds it for the stream's whole duration — if the model/provider is swapped mid-stream, that in-flight request keeps talking to the now-stopped/replaced client instead of the new one) → fixed 2026-07-12 (BUG-L4): `clientSwapped`/`providerSwapped` (`llm.go`) compare the captured copy against the live `a.client`/`a.providerRouter` at each of the four places `callLLMStream`/`callLLM` report a failure, substituting a clear "model/provider değişti" message for the raw transport error (typically "connection refused") when that's the actual cause. Doesn't cancel/retry the in-flight call itself — that's a larger change, left out.

### Streaming (SSE)
- ~~Every SSE chunk producer/forwarder in the chat pipeline (`internal/provider/{openai,gemini,claude}.go`'s `processSSE`, `internal/app/llm.go`'s `agentLoop`/`providerLoop`/`localLoop`, `internal/app/chat.go`'s stream wrappers, `internal/webserver/handlers_flutter.go`'s `handleSendStream`/`handleSendFileStream`/`handleWhatsAppChatStream`) raced `ctx.Done()` against sending/receiving the next chunk in a single `select { case <-ctx.Done(): return; case chunk, ok := <-ch: ... }`. Go picks uniformly at random between simultaneously-ready `select` cases — when `ctx` became `Done` at the exact moment the final `Done:true` chunk was also ready, the chunk was silently dropped roughly half the time (measured 46-51% across 2000-trial regression tests). The Flutter client never saw the `"done":true` SSE line, so `isSendingProvider` stayed `true` forever and the send button stayed stuck on the stop icon indefinitely~~ → fixed 2026-07-12: every layer now tries a non-blocking send/receive first and only falls back to a `ctx`-aware blocking `select` if nothing was immediately available (`trySend`/`recvChunk` in `internal/app/llm.go` and `internal/provider/provider.go`, `forwardStream` in `internal/app/chat.go`, `streamSSE` in `internal/webserver/handlers_flutter.go`) — an already-ready value is always preferred over a racing cancellation. **General rule for any future SSE/streaming code in this codebase: never write a bare `select` that races `ctx.Done()` against a channel op meant to deliver a final/terminal value — always try the non-blocking op first.**
- ~~The 2026-07-12 fix above covered the case where a value was ready on the channel at the same instant as `ctx.Done()` — but `recvChunk`'s genuine `ctxDone=true` branch (nothing at all ready, `ctx` actually expired: the 300s generation budget elapsed, or the client disconnected) in all three consumers of `recvChunk` in `internal/app/llm.go` (`callAgentStream`, the external-provider loop and the local-model loop inside `callLLMStream`) called `recordStreamError` (session-only, not streamed) and then `return` — the one return path in the whole file that never called `trySend` before closing `outCh`. A response that ran long (slow external provider, weak local hardware) or lost its connection was cut off with zero explanation reaching the live client — indistinguishable from the response just silently stopping mid-sentence~~ → fixed 2026-07-14: all three now `trySend(ctx, outCh, api.StreamChunk{Error: "⏹️ Cevap durduruldu.", Done: true})` before returning, matching every other error branch in the file. Regression test: `TestCallLLMStream_ExternalProvider_CtxDoneSendsTerminalChunk` (`internal/app/llm_test.go`) — an `httptest` SSE server that holds the connection open past cancellation, asserting `outCh` yields a terminal chunk instead of just closing; fails against the pre-fix code.
- ~~That fix didn't hold: the user reproduced the exact same stuck-stop-button symptom again after rebuilding with `flutter run` (fresh code, not a stale binary), on fast/short replies (not the 300s-timeout scenario the fix above targets) and across every provider — a strong signal the real defect was purely frontend, provider-agnostic. Root cause found in `frontend/lib/providers/chat_provider.dart`'s `MessagesNotifier.sendMessage()`: the `isSendingProvider` guard check (`if (ref.read(isSendingProvider)) return;`) and the actual claim (`ref.read(isSendingProvider.notifier).state = true;`) were not atomic — `await _handleMemoryCommand(message, api)` sat between them. Two `sendMessage()` calls fired back-to-back (double Enter press, OS key-repeat while Enter is held, a double click on the send button — `chat_input.dart`'s `_handleKeyEvent` wires Enter straight to `_send()`) could both read `isSendingProvider` as `false` and both proceed, dispatching two overlapping HTTP requests, clobbering the shared `_cancelToken` field, and appending two user-message bubbles for what was a single send — with `isSendingProvider`'s final value after both settle depending on unpredictable completion order between the two racing streams~~ → fixed 2026-07-14: the claim now happens synchronously immediately after the guard check, before `_handleMemoryCommand`'s `await` — closing the window entirely (a second call's guard check now always sees the first call's claim). If `_handleMemoryCommand` turns out to have handled the message (a `/remember`/`/forget` command), the claim is reset back to `false` before returning. Regression test: `messages_notifier_reentrancy_test.dart` — fires `sendMessage()` twice with no `await` in between and asserts only one HTTP request was dispatched; fails (2 requests) against the pre-fix code.
- ~~The re-entrancy fix above still didn't hold either — the user retested on a genuine `flutter run -d linux` session and hit the identical stuck button on the very first message. Backend `backend.log` from that same session showed every turn completing cleanly (single request, `chat:done`, `memory:saved`) and the CLI (`internal/replcli`, hitting the same `/api/send/stream` endpoint) never reproduces this at all — narrowing it to something purely in Flutter's own state layer. Temporary `[SEND-DEBUG]` tracing (`chat_provider.dart`, `chat_input.dart`, later removed) caught it directly in a captured log: `sendMessage()` entered with its own `_disposed` field already `true` on the *very first* send of the session, before any chat switch or explicit `invalidate(messagesProvider)` call. `MessagesNotifier.build()` -> `onDispose` -> `build()` again happened once, automatically, very early in real app startup (root trigger not pinned down, and not necessary to pin down for the fix below) — and Riverpod reused the *same* `MessagesNotifier` object instance across that cycle. `_disposed` was only ever initialized once, in the field declaration, never reset in `build()` — so once that first, harmless-looking dispose+rebuild happened, `_disposed` stayed permanently `true` for the rest of the session, and every future `sendMessage()`'s `finally` block's `if (!_disposed) { isSendingProvider = false; }` guard (added for BUG-H2) permanently no-opped: the send/stop button got stuck on "stop" forever, from that point on, on every single turn, regardless of provider. (A first attempted fix — just resetting `_disposed = false` at the top of `build()` — broke `messages_notifier_dispose_test.dart`'s existing BUG-H2 regression test: since the object is reused, that reset also "un-disposed" an *old*, still-running, abandoned stream from a previous generation, letting it clobber `isSendingProvider` for a legitimately-in-progress new chat's send — reintroducing BUG-H2.)~~ → fixed 2026-07-14: `_disposed` replaced with an `int _generation` counter, bumped once per `build()` call. `sendMessage()`/`sendFile()`/`refresh()` each capture `final myGeneration = _generation;` at their own entry and compare (`_generation == myGeneration` / `!=`) at every point that used to check `_disposed` — a call only touches shared state while its captured generation is still the live one, correctly self-invalidating once *any* later `build()` runs (whether on a reused instance or a genuinely fresh one), while a new call started after that rebuild captures the new generation and works normally. Both `messages_notifier_reentrancy_test.dart`'s and `messages_notifier_dispose_test.dart`'s (BUG-H2) existing regression tests pass simultaneously, plus a new one, `messages_notifier_stale_disposed_flag_test.dart`, that forces one `invalidate()`+rebuild cycle (with a live listener, matching how `ChatScreen`'s permanent `ref.watch(messagesProvider)` triggers real disposal in the app) and asserts a subsequent `sendMessage()` still resets `isSendingProvider` — fails against both the original code and the first (reset-in-build) attempt, passes against the generation-counter fix.

### Memory / Vector Store
- ~~Full rebuild on every startup (`LoadCache` is O(N), no incremental index)~~ → stale note: `LoadCache` doesn't exist in the current codebase (`internal/memory/store.go`'s `NewStore`/`initSchema` don't do a startup full-scan — this refers to an older architecture, before the hybrid vector+FTS rework). Removed as a claim; re-audit from scratch if startup performance on a large memory store is ever reported as an issue.
- ~~`chunkText` sized chunks by word count (`strings.Fields`), a poor proxy for token count~~ → fixed 2026-07-12: `internal/memory/chunker.go` now sizes chunks by `truncate.EstimateTokens` (the same char/3 heuristic used elsewhere for context budgeting), so a message short by word count but made of long tokens (URLs, code, agglutinative Turkish) can no longer slip through as a single unsplit, over-budget chunk. Note: `chunkText` is only used by `SaveInteraction` to chunk a single chat turn (user message + reply) before embedding — Memo has no document/file-upload → RAG ingestion pipeline, so heading-aware/semantic splitting (relevant for chunking uploaded documents) doesn't apply to this codebase's actual usage.
- ~~Embedding model must be started separately (config-driven auto-start on model load)~~ → refined 2026-07-12: `EmbeddingAutoStart` (default `false`, opt-in) only gates launching a *new* model process. Separately, `Startup()` now always checks for an embedding server *already* alive on the configured port (left running by an earlier backend process, or started manually) and reconnects the memory store to it automatically (`reconnectEmbeddingIfAlreadyRunning`, `internal/app/embedding.go`) — previously only `StartEmbeddingModel` (auto-start or the explicit `/embedding`/Models-tab action) ever wired `a.embeddingClient`, so a fresh backend process with `EmbeddingAutoStart=false` (the default, and true in every real `config.yaml` on this machine) silently embedded through the `Startup()`-time placeholder (`a.client`, the main chat client) — while `GetStatus()`'s `pingPort()` fallback reported "running" the whole time without ever reconnecting anything, misleading the CLI's welcome banner in particular (GUI usually escaped this because opening the Models tab to pick a chat model incidentally wires embedding too, if the user had started one before).
- ~~**FTS5 (keyword) search — the other half of the "hybrid vector+FTS" retrieval `RetrieveContext` (`internal/memory/store.go:693`) is built to do — has silently never been active in any shipped build.** User-reported symptom: told Memo their favorite color in one CLI session (`✓ hafıza kaydedildi` shown, so the *save* genuinely succeeded), then in a brand-new session asked "do you know my name, birthday, and favorite color" in one compound question — got the name back, the birth *year* back (from older memory), but flatly "no" on the favorite color, even though it had just been saved. Root cause, confirmed by rebuilding and diffing backend startup logs: `tryCreateFTSTable`'s `CREATE VIRTUAL TABLE ... USING fts5(...)` fails at runtime with `no such module: fts5` — `mattn/go-sqlite3` does not compile FTS5 support unless built with `-tags "sqlite_fts5"`, and as of 2026-07-15 (checked: `.github/workflows/{ci,build-linux,build-macos,build-windows}.yml`, `build_releases.sh`, `macrelease.sh`, `package_linux.sh`, `package_windows.sh`) not a single build path anywhere in this repo ever passed that tag — CI, every released platform binary, all of it. `store.go` catches the failure and sets `s.useFTS = false`, silently degrading to vector-only search with zero user-visible error (`store_test.go`'s own `TestHybridSearch_MatchTypeSet` comment even acknowledges "test ortamında fts5 yok" as an accepted condition, rather than a bug). With `useFTS` false, `RetrieveContext`'s entire FTS5 + Reciprocal-Rank-Fusion hybrid-merge branch (lines 743-758) — already written, already has a dedicated test — never executes; only raw cosine-similarity vector search runs. A single embedding vector for a multi-topic compound query ("name + birthday + color") blends all three topics together, and against a memory store that's accumulated many turns of unrelated chit-chat across dozens of sessions, a short, simple fact like "favorite color: red" can easily rank outside the top-K purely on semantic vector distance — exactly the kind of exact-but-short keyword match FTS5 exists to catch independent of embedding drift~~ → fixed 2026-07-15: `-tags "sqlite_fts5"` added to every Go build/vet/test/run invocation in the repo (all 4 CI workflows, all 5 build/package shell scripts, `AGENTS.md`, `README.md`/`READmeTR.md`, `docs/CGO_FLAGS.md` — which also gained a dedicated section explaining why this flag is non-optional). Verified directly, not just inferred: built a throwaway binary with the tag and ran it from a scratch data directory — startup log changed from `"MEMORY: fts5 not available (no such module: fts5)"` to `"MEMORY: FTS migration complete"`. **Not yet verified:** an actual before/after comparison of retrieval quality for the exact reported scenario (compound multi-fact query) — the fix turns on an existing, already-tested hybrid mechanism rather than adding new retrieval logic, but a live re-test by the user, after rebuilding with the tag, is still the real confirmation this resolves the reported symptom specifically.
- ~~Even with FTS5 actually compiled in (previous bullet), the reported compound-query symptom would **still** reproduce: `escapeFTSQuery` (`internal/memory/store.go`) wrapped every word of the query in a quoted phrase and joined them with a plain space — and FTS5 treats space-separated terms as an implicit **AND**. A natural multi-topic question ("adımı ve doğum günümü ve en sevdiğim rengi biliyor musun") built a MATCH expression requiring one memory row to contain literally every one of those words, filler included ("ve", "biliyor", "musun") — no real memory row is ever that specific, so `ftsSearch` returned zero rows for exactly this class of question, and `RetrieveContext`'s `if len(ftsMemories) > 0` guard silently skipped the FTS/RRF merge, falling back to vector-only for exactly the queries that most need keyword backup. Found while writing a recall-focused test suite, not by user report — verified directly with a throwaway sqlite3 fts5 table: the AND-joined query matched 0 rows, the same words OR-joined ranked the correct row first by bm25~~ → fixed 2026-07-15: words now joined with `" OR "` instead of `" "`, so each word is an independent candidate match ranked by bm25 (which already downweights common filler via IDF, so this isn't just "everything matches"). New test suite `internal/memory/store_recall_test.go` covers this directly: `TestRecall_CompoundQuery_ShortFactSurvivesNoise` reproduces the exact scenario (a short fact buried among noise that partially overlaps the query's other topics) and is confirmed to fail against the pre-fix AND-joined query, pass against the OR-joined fix. The suite also uses a new `bagOfWordsEmbedding` test helper whose cosine similarity genuinely tracks word overlap (unlike the trivial 3-axis `testEmbedding` used elsewhere), so it can actually reproduce embedding-dilution effects instead of just asserting against a toy fixture.

### Local Model Process (`internal/llama/`)
- ~~An orphaned `llama-server` (parent Memo process died abnormally — crash, `kill -9`, OOM) kept its port bound forever, since none of `sysproc_{linux,darwin,other}.go`'s `newSysProcAttr()` set `Pdeathsig` (incompatible with the `Setpgid` process-group kill needs). The next `Start()` on that port had no memory of the orphan: `cmd.Start()` succeeded at the OS level, but `llama-server` itself failed to bind and exited within ~1s, and the `Stop()` that followed only SIGTERMed *that* (already-dead) attempt's own PID — `killByPort`'s port-discovery fallback is only reached when `s.cmd == nil`, which wasn't the case here. Every retry in `StartEmbeddingModel`'s 3-attempt loop hit the identical failure, and the port stayed stuck until a full reboot killed the real occupant~~ → fixed 2026-07-12: `Start()` now calls `killByPort` on its target port unconditionally, before every spawn attempt (no-op if the port is already free) — fixes both the embedding and chat-model servers, since `Start()` is shared by both.

### Security
- ~~Provider API keys encrypted with hardcoded fallback key~~ → fixed: random key generated via `crypto/rand`, persisted to `data/machine.key` (0600).
- ~~Cloud sync encryption falls back to hardware ID when passphrase is empty~~ → documented behavior, machine.key now provides better fallback.
- ~~No request body size limits~~ → fixed: 50MB `limitBodyMiddleware` on all handlers.
- ~~X-Forwarded-For trusted in rate limiter~~ → fixed: removed header trust, rate limiter uses r.RemoteAddr directly.
- ~~File upload MIME spoofing~~ → fixed: MIME detected from file content via http.DetectContentType, not client header.
- ~~Import path traversal weak~~ → fixed: filepath.Rel validation ensures extracted files stay within data directories.
- ~~Remote access (LAN/ngrok) had zero authentication~~ → fixed 2026-07-11: `remoteAuthMiddleware` (`internal/webserver/server.go`) requires `X-Memo-Token`/`Authorization: Bearer <token>` on every request once the listener is bound to `0.0.0.0` (LAN mode or ngrok — both go through `SetRemoteAccess`/`SetNgrokMode`); local-only mode (127.0.0.1, the default) is unaffected. The `RemoteAccess.Token` shown in Settings was already generated but never checked anywhere before this.
- ~~BUG-L5: ngrok auto-start + app restart could leave the desktop client without the token until it's re-toggled~~ → fixed 2026-07-12: the token is stable across restarts (`internal/app/remote.go` only generates one if empty), but the desktop client only ever held it in memory, reset to nothing on every launch — if `NgrokAutoStart` is on, `Startup()` rebinds straight to `0.0.0.0` (token-gated) before the client ever calls `getRemoteAccess()`/`setRemoteAccess()` to learn it fresh, so its first request after a restart 401'd. `frontend/lib/core/api_client.dart`'s `MemoApiClient` now takes a `savedRemoteToken` (applied to the `X-Memo-Token` header immediately, before any request) and an `onRemoteTokenLearned` callback; `chat_provider.dart`'s `apiClientProvider` wires both to a `prefsProvider`-backed `SharedPreferences` key (`memo_remote_access_token`), so the last-known token survives an app restart. A stale/rotated token still 401s (no worse than before), self-correcting on the next `getRemoteAccess()` call. Unit-tested (`api_client_test.dart`); **not** verified against a real ngrok tunnel + phone in this environment.
- ~~Agent's `rm -rf` blacklist regex never matched its own target~~ → fixed 2026-07-11: `\brm\s+-rf\s+/\b` (and the `~`/`.` variants) relied on `\b`, but `/`, `~`, `.` are non-word characters, so no boundary ever formed at end-of-string — "rm -rf /", "rm -rf /*", "sudo rm -rf /" all sailed through unblocked while a harmless "rm -rf /home/user/foo" was flagged instead. `internal/agent/tools/command.go` now uses an explicit terminator class (end-of-string/whitespace/shell-operator) instead of `\b`.

### Provider / Agent / Orchestra
- **~~`provider.Priority` field exists but unused by router~~** → fixed: router sorts by priority, frontend dialog now exposes priority field.
- Orchestra bypasses `provider.Router` — creates providers directly, but now has fallback chain via `tryFallbackProviders`.
- ~~Agent pipeline has no timeout per tool call~~ → fixed: pipeline grants 120s per tool call. 2026-07-04: `tools/command.go`/`tools/search.go` were silently truncating that budget to 60s via their own hard-coded `DefaultToolTimeout`; now they honor the caller's deadline and only fall back to 60s when none is set.
- ~~**No test files for `orchestra/` package** (~800 lines untested)~~ → 48 tests passing with `-race`. `provider/`, `agent/`, and `orchestra/` all have tests.
- **Agent frontend UI (permission dialog, tool call cards, activity panel, agent screen) — fully implemented.**
- ~~Agent mode (tool-using requests) failed against every external provider hitting an OpenAI-compatible-style API — confirmed live with OpenCode Zen: `all providers failed: [opencode-zen] status 400: {"error":{"message":"Error from provider (Console): Upstream request failed",...}}` — while plain chat with the same provider (no tools) worked fine. Root cause: `provider.ToolDefinition` (`internal/provider/provider.go`) carried a `Danger string` field (`json:"danger,omitempty"`) copied from the agent's internal `DangerLevel` (`agent/tools.go`'s `ToOpenAITools`) — but this exact struct is the literal wire type `openai.go` marshals verbatim into the outbound `"tools"` array (`openAIChatRequest.Tools []ToolDefinition`). Every tool call request therefore included a non-standard `"danger"` key inside each tool object, alongside the real OpenAI tool-calling schema (`{"type", "function"}`). `Danger` was never read anywhere in the codebase (permission checks use the separate internal-only `agent.ToolDef.DangerLevel`) — pure leaked internal state with zero purpose on the wire, tolerated by some upstreams (ignore unknown fields) and strictly rejected by others (OpenCode Zen's gateway)~~ → fixed 2026-07-14: `Danger` field removed from `provider.ToolDefinition` entirely. Regression test `TestToOpenAITools_OnlyStandardFields` (`internal/agent/tools_test.go`) marshals every built-in tool's wire representation and asserts each object has only `"type"`/`"function"` keys — fails (18 tools, all carrying `"danger"`) against the pre-fix code.
- ~~Relatedly, a failed provider request surfaced its **entire raw JSON error body** verbatim through `Router`'s `"all providers failed: %w"` wrapping straight into the chat UI — e.g. the full `{"error":{"message":"...","type":"invalid_request_error","param":null,"code":"invalid_request_error"}}` blob above, rather than just the actual message. Unreadable to a non-technical user trying to tell what went wrong~~ → fixed 2026-07-14: new `provider.ExtractErrorMessage(body []byte) string` (`provider.go`) unwraps the `{"error": {"message": "..."}}` shape OpenAI-compatible/Claude/Gemini APIs all use, falling back to the raw body only if that shape isn't there or the message is empty. Wired into all three providers' `parseError` (`openai.go`, `claude.go`, `gemini.go`). Tests: `TestExtractErrorMessage_UnwrapsTheRealMessage`/`_FallsBackToRawBody`/`_FallsBackWhenMessageEmpty` (`internal/provider/error_message_test.go`).

### Skill Tool Execution (`internal/skill/`) (2026-07-12)
- ~~A skill manifest's `tools:` entries were entirely inert (TD-1, BUG_REPORT.md): `skill.Manager.toolRegistrar` was never wired up anywhere in prod code (`SetToolRegistrar` had zero callers), and even if it had been, `skill.SkillTool` had no field describing how to actually run a tool — declaring one in a SKILL.md's front matter was purely documentation, never reachable by the LLM~~ → built as a real feature, not a stub bridge: `SkillTool` gained a `Command` field (shell command, resolved relative to the skill's own install directory so a manifest can reference its own bundled scripts). `internal/skill/executor.go`'s `Manager.ExecuteTool` runs it through the same sandboxing `internal/agent/tools`' `run_command` uses (`tools.PrepareCommand`/`tools.FormatCommandOutput`, shared via that refactor; `tools.CheckDestructivePatterns` — the destructive-pattern half of the blacklist, deliberately *without* the `$`/backtick shell-substitution block that `run_command` also applies, since a skill's `command` is author-trusted static content, not something assembled from live input, and legitimately needs `$VAR` to read the env vars below). The LLM's call args are delivered on the child process's stdin as raw JSON — never interpolated into the command string — plus `MEMO_SKILL_ARGS`/`MEMO_SKILL_NAME`/`MEMO_SKILL_DIR`/`MEMO_PROJECT_DIR` env vars. `internal/app/skill_tools.go`'s `skillToolRegistrar` is now actually wired via `skillManager.SetToolRegistrar(...)` in `Startup()`, adapting activation/deactivation into real `agent.ToolDef`s registered on the same `agent.ToolRegistry` the pipeline executes against — so danger-level gating and the existing permission-prompt UI apply to skill tools with no additional frontend work. A tool entry with no `Command` is skipped at registration (logged, not an error) — a manifest can still list one for documentation only.

### API Versioning (`internal/webserver/server.go`) (2026-07-12)
- ~~No versioning strategy: every endpoint sits under a flat `/api/` prefix, no path for a future breaking change without breaking every existing client~~ → fixed (TD-2, BUG_REPORT.md): `StartHTTPWithAddr`'s local `route()` helper registers every `/api/...` pattern under both its original path and an `/api/v1/...` mirror, covering plain routes, Go 1.22+ `{wildcard}` patterns, and trailing-slash prefix routes alike. `/api/` (unversioned) keeps meaning "latest stable" and every existing client (Flutter desktop, mobile, Go CLI) is completely unaffected — zero routes moved or renamed. A future breaking change gets its own `/api/v2/...` registrations the same way, with `/api/` re-pointed at v2 and `/api/v1/` kept alive for as long as it's supported.

### Shutdown (`internal/shutdown/`) (2026-07-12)
- ~~Backend self-shutdown (`internal/app/clients.go`'s client-registry auto-shutdown, and `internal/webserver/handlers_flutter.go`'s `POST /api/shutdown`) both self-delivered `os.Interrupt` via `os.Process.Signal` — a silent no-op on Windows (Go's `os` package only implements `Process.Signal` for `os.Kill` there; anything else, including `Interrupt`, returns `EWINDOWS`, discarded by both callers). A Windows-hosted backend never actually stopped from either path~~ → fixed 2026-07-12 (BUG-H3): new `internal/shutdown` package exposes a single process-wide `Request()`/`Requested()` channel pair with no OS-signal dependency; both call sites now use it, and `main.go`'s signal-wait loop `select`s on it alongside its OS signal channel. Verified with `GOOS=windows go vet ./...` (clean) — real runtime verification on Windows still pending, no Windows machine in this environment.

### Memory Import From Other AI (`internal/app/memory_import.go`) (2026-07-13)

New feature (not a bug fix): Settings → Memory tab has an "Hafızayı İçe Aktar" (Import Memory) section — a copyable prompt the user pastes into another AI (ChatGPT, Gemini, etc.) asking it to describe everything it knows about the user, plus a text box to paste that AI's answer back in.

`App.ImportMemoryFromText(ctx, rawText)` sends the pasted text to the *active* provider/model (via `a.providerRouter`, same lightweight call shape as `mergeMemoriesLLM`) with a system prompt asking it to return `{"facts": [...], "style_summary": "..."}`. Each fact is saved individually via the existing `SaveExplicitMemory` (same path as the `/remember` chat command) — deliberately not chunked, since the structuring step is instructed to keep facts short/atomic, so this doesn't need the `chunkText`/`SaveInteraction` machinery. If `style_summary` is non-empty, it's persisted as `Identity.LearnedStyleNotes` (new field, `cfg.Identity.LearnedStyleNotes` in config.yaml) and injected additively in `BuildSystemPrompt` right after the fixed style instructions — additive rather than replacing `CustomRole`/`SystemRole`, so it still applies under a wizard persona or a hand-written system prompt, same reasoning as the existing origin block. `extractJSON` (balanced-brace, string-aware JSON extraction from LLM prose) is a private per-package copy, matching the existing convention in `internal/intent`/`internal/orchestra`/`internal/taskloop`/`internal/proactive` rather than a shared exported helper.

Route: `POST /api/memory/import-text` (`handleMemoryImportText`, `internal/webserver/handlers_flutter.go`), bridged via `FullBridge.ImportMemoryFromText(ctx, rawText) (factsSaved int, styleUpdated bool, err error)` — plain primitives, not an `app`-package struct, to preserve the bridge pattern's decoupling (`internal/webserver` never imports `internal/app` types directly).

Not verified against a real external AI's actual output shape (ChatGPT/Gemini free-form prose) in this environment — only unit-tested (JSON extraction, empty-input/no-provider error paths, identity injection). The structuring LLM call assumes the *active* local/provider model can follow a "return ONLY JSON" instruction reasonably; a weak local model may need a retry loop if this turns out to be unreliable in practice.

**Update (same day, real-world test):** the user actually ran the copy-prompt against Gemini and ChatGPT. Gemini's answer came back nearly empty (Gemini apparently doesn't scan full conversation history the way ChatGPT's memory does — a Gemini-side limitation, not a prompt problem); ChatGPT, given the identical prompt, produced a rich, well-structured profile across all 6 categories. The frontend's copy-prompt (`MemoryImportTab`, moved out of `memory_tab.dart` into its own file/settings page — see below) was rewritten from scratch, always-English now (no more TR/EN branch), modeled on Gemini's own real "Hafızayı Gemini'a aktarın" settings page design the user shared as a screenshot: third-person narration (no I/you pronouns), 6 labeled categories (Demographics, Interests & Preferences, Relationships, Dated Events/Projects/Plans, Communication Style & Personality — this one's not in Gemini's original, added since it's core to what this feature needs, Instructions), each entry backed by an "Evidence:"/"Basis:" sub-line, ending in "Source: <name>". `importMemorySystemPrompt` (backend structuring step) was updated to match: ignores category headers/Evidence-Basis sub-lines/the Source trailer, turns categories 1-4 into atomic facts, folds 5+6 into one `style_summary` (since standing behavioral instructions like "always explain why" should be always-on via the system prompt, not just probabilistically retrieved like a fact).

**Settings placement changed:** per user request, this is now its own top-level settings page (`MemoryImportTab`, `translate.svg` icon, inserted right after the Memory tab in `settings_dialog.dart`), not a section inside the Memory tab — plus a `warningOrange`-tinted tip banner telling the user to actually chat with the target AI a bit first (Gemini's thin result is exactly the failure mode this warns about). Deliberately does **not** include Gemini's other feature on the same reference page ("Sohbetleri içe aktarın" / import full conversation exports via .zip) — out of scope, user explicitly excluded it.

~~**Update (same day, real bug caught by the user):** `ImportMemoryFromText` called `a.providerRouter.ChatCompletion` directly, entirely bypassing `callLLM`'s actual routing chain (Orchestra → external provider → local model, `internal/app/llm.go`) that every other LLM call in this codebase goes through. Two real consequences: (1) a local-only setup (no external provider selected) silently never even tried the local model — the feature was broken for anyone not using an external provider; (2) with nothing connected at all, it made a live network call and hung until its own 90s timeout instead of failing immediately, with zero indication to the user of what was wrong~~ → fixed 2026-07-13: now builds `[]api.Message` (`api.NewTextMessage`) and calls `a.callLLM(ctx, msgs)` like every other simple structured-extraction call in this package (`updateMoodAsync`, `buildLearningDecider`) — gets the correct routing priority for free, plus callLLM's existing immediate, clear "⚠️ Yerel model yüklenmemiş. Lütfen bir model başlatın veya API sağlayıcı seçin." when nothing is connected (detected via the existing `isLLMErrorReply` prefix check, same convention `memory.go` already uses). No more bespoke timeout — defers to callLLM's own per-branch budgets (300s orchestra/provider, 120s local). Regression test: `TestImportMemoryFromTextNoModelFailsFastWithClearMessage`.

~~**Update (same day, second bug caught by the user): "no reaction" after clicking Import/Copy.**~~ → fixed: `MemoryImportTab` showed its result via `ScaffoldMessenger.of(context).showSnackBar(...)` — but the whole page lives inside `SettingsDialog`'s modal `Dialog`, which sits above the app's root `Scaffold` in the overlay stack, so the SnackBar rendered *behind* the still-open dialog and was invisible until Settings was closed. Looked exactly like the button did nothing. Both the submit and copy-prompt actions now set inline widget state (`_statusMessage`/`_statusIsError`) rendered as a banner inside the page itself, guaranteed visible regardless of the dialog. (This SnackBar-behind-modal gotcha likely affects other Settings tabs too that call `ScaffoldMessenger` directly — not audited/fixed elsewhere, out of scope for this session.)

### Report a Bug (`frontend/lib/widgets/settings/tabs/report_bug_tab.dart`) (2026-07-14)

New Settings tab (last tab, "Hata Bildir"/"Report Bug", wrench icon — no dedicated bug icon asset exists yet). Deliberately manual and explicit, not automatic telemetry: the user writes what happened in a text box, optionally checks "also include the last 10 errors" (unchecked by default), and hitting the submit button opens a prefilled GitHub "new issue" page (`https://github.com/BugraAkdemir/memo/issues/new?title=...&body=...`) in the browser via `url_launcher` — nothing is transmitted until the user reviews and submits it themselves on GitHub's own page, using their own account. No screenshot capture (screen content could contain private chat text). No new backend endpoint or bugradev-controlled collection point — this deliberately keeps "we collect zero data" true, since the report goes to GitHub, sent by the user, not to us.

"Last 10 errors" reuses the existing `internal/app`'s `eventRing`/`GetEvents()` (`GET /api/events`, already wired, previously unused by the Flutter client — new `MemoApiClient.getEvents()` added in `api_client.dart`) rather than adding new tracking: client-side filters the ring buffer's recent `AppEvent{Name, Data}` entries for `Name` containing "error" (memory:error, etc.), takes the last 10.

Design rationale came out of an explicit conversation with the user about telemetry vs. the app's privacy-first pitch — see this session's chat log if the reasoning needs revisiting. Key point: a version-check-style *pull* (client asks a static file "what's latest," reveals nothing about the user) is not the same trust category as a *push* (data about the user leaving the machine), so the existence of the former doesn't lower the bar for adding the latter — this report feature avoids that by never having Memo itself transmit anything; it only hands off to the user's browser + GitHub account.

Not yet added: a widget test (this codebase's other settings tabs mostly don't have one either, e.g. `memory_import_tab.dart` — verified manually against real usage instead); not verified against a real GitHub issue submission in this environment (no browser/display here). `flutter analyze`/`flutter test` (105/105) are clean.

### REPL CLI (`internal/replcli/`)
- ~~The welcome banner's "Hafıza:" line and the per-message "✓ hafıza kaydedildi" confirmation were the REPL's only memory-related feedback — if embedding actually failed (no embedding model file found, download failed, or `StartEmbeddingModel` itself failed, e.g. its port already occupied by something else), the backend already emitted a specific `memory:error` event for it (`autoStartEmbeddingModel`/`startupEmbeddingModel` in `internal/app/llama.go`+`embedding.go`, and `saveMemorySync` in `memory.go`) — but the REPL never listened for that event at all, so the user saw either a stale/misleading "active" banner or, per message, just silence where the save confirmation should have been~~ → fixed 2026-07-14: new `eventDataSince(events, before, hadBefore, name)` helper (`repl.go`), generalizing `memorySavedSince`'s "after a snapshot point" logic to return an arbitrary event's data. `printWelcome()` now checks for any existing `memory:error` when the banner shows memory as inactive, printing the real reason right under it; `reportMemorySaved` checks for a `memory:error` since the turn started as a fallback when no `memory:saved` shows up, instead of silently giving up after ~2.4s. `saveMemorySync`'s existing suppression of pure connectivity errors (`isEmbeddingBackendDown`, "expected" when a user runs an API/Orchestra-only setup with no embedding intended) is unchanged — this only makes the REPL actually surface `memory:error` events that were already being emitted and previously went nowhere.
- ~~`/model-download` ran an in-terminal Hugging Face search-and-download flow whose progress loop only read from a ticker, never the keyboard — a stalled download left the whole REPL stuck with no way to cancel (not even Esc/Ctrl+C, since raw mode turns those into plain keypresses, not signals)~~ → fixed 2026-07-09: `/model-download` no longer downloads anything itself; it prints a short message and opens the desktop GUI (`cmdGui`) instead. Heavy, long-running work (model search/download with real progress bars) belongs in the GUI, not the terminal client.
- ~~`/gui` looked for the bundled `memo_flutter` binary only next to the CLI's own executable, so it never found it on a real install~~ → fixed 2026-07-09: the installed CLI binary lives one level deeper (`~/.memo/bin/memo`) than the bundled GUI (`~/.memo/memo_flutter`) — `cmdGui` now searches the exe's own directory *and* its parent (`guiSearchDirs`, same pattern as `binarySearchBasesFrom` in `internal/llama`), and runs the GUI with its own directory as `cmd.Dir` (needed for Flutter's `lib/`/`flutter_assets/`, which sit next to the binary, not next to the CLI).
- ~~`/model`/`/embedding` used the plain JSON client's blanket 10s timeout, but the backend's own model-load budget is 120–180s (`WaitReady` in `internal/app/llama.go`/`embedding.go`) — any load slower than 10s reported a false "başlatılamadı" while the backend kept loading successfully in the background~~ → fixed 2026-07-09: `StartModel`/`StartEmbedding` now run on a dedicated no-fixed-timeout client (`Client.longOpHTTP`) with an explicit deadline matching the real backend budget (185s/125s), and `startAndReport` wraps the call in the same Esc/Ctrl+C-cancellable, spinner-shown pattern `sendMessage` uses for streaming replies — cancelling only stops the CLI from waiting (the backend handler doesn't watch the request context), so a cancelled load may still finish in the background; `/models` shows the outcome.
- ~~An external SIGTERM/SIGINT during the interactive REPL left the terminal in raw mode (`stty sane`/`reset` required) because `main.go`'s signal-select branch returned without waiting for `replcli.Run()`'s goroutine, so its deferred `term.Restore` never ran~~ → fixed 2026-07-09: `main.go` captures the pre-raw terminal state via `term.GetState` before starting the REPL goroutine and restores it directly in the signal branch. (Ctrl+C typed at the keyboard was never affected — raw mode disables ISIG, so it's decoded as a plain keypress by `keys.go`, not delivered as a signal at all.)
- ~~No bracketed-paste support: every embedded newline in a pasted multi-line block decoded as a real Enter press, splitting one paste into several separately-submitted messages (and running any "/"-prefixed line inside it as a command)~~ → fixed 2026-07-09: `Run()` enables bracketed paste (`ESC[?2004h`, disabled again on exit) and `keys.go`'s `readBracketedPaste` decodes the `ESC[200~ … ESC[201~`-wrapped block as one `keyPaste`, collapsing embedded line breaks to spaces (`collapsePasteNewlines`) so the whole paste lands as a single insertable chunk at the cursor, submitted only on an actual Enter press.
- ~~A `memo` interactive session that spawned its own backend ran it *in-process* (`a.Startup`/`a.StartWebServerHTTP` inside `main()`), so exiting the REPL unconditionally shut the backend down via `defer a.Shutdown(ctx)` — if the user had opened the Flutter GUI mid-session via `/gui`, it was still relying on that exact backend, and lost it the moment the terminal exited~~ → fixed 2026-07-09, see "Backend process model" below.
- ~~Every `memo` launch auto-resumed the most recently used agent chat for the current project path (`resumeOrStartChat`/`findRecentChat`), so the terminal always opened with stale context instead of a clean slate — and `/session` only listed chats whose project path matched the CLI's cwd, so chats created in the Flutter GUI (no project path) or by a CLI run in another directory were invisible/unreachable from the terminal, even though the GUI's own chat list already showed every chat regardless of origin~~ → fixed 2026-07-12: `Run()` now always calls `startFreshChat()`, never auto-resumes; `/session` (`allChats()`, was `projectChats()`) lists every chat with no project-path filter, matching what the GUI shows, so either client can resume a chat the other one started (list/picker entries show the project's base name as a hint when one is set, to tell CLI-agent and GUI-plain chats apart).
- ~~The GUI side of that same CLI/GUI split had the opposite problem: `startFreshChat()` (`repl.go`) always creates chats via `NewAgentChat(s.projectPath)` — unconditionally, regardless of whether agent mode is ever actually used in that conversation — and the backend's `IsAgentChat`/frontend's `ChatSession.isAgentChat` are both defined purely as "does this chat have a non-empty project path" (`internal/sessions/sessions.go`, `frontend/lib/models/chat.dart`). Since every CLI-created chat gets the CLI's cwd as its project path, **every single CLI chat** — including ordinary chit-chat with agent mode never invoked — was permanently tagged `isAgentChat=true`, and `chat_sidebar.dart`'s "Sohbetler" list explicitly filtered those out (`chats.where((c) => !c.isAgentChat)`), leaving them visible only under the separate "Ajan" tab. Confirmed against real data: 11 of 13 sessions on this machine had `project_path` set to the CLI's cwd and titles like "Kanka Muhabbeti"/"Hobi Sohbeti" — ordinary conversations, invisible in the GUI's main chat list~~ → fixed 2026-07-14: `chat_sidebar.dart`'s "Sohbetler" list no longer excludes agent-tagged chats — every chat shows there regardless of origin (CLI or GUI). The "Ajan" tab (`agent_screen.dart`) is unaffected and still shows its own filtered agent-only view; a chat can now appear in both. `ChatScreen` already renders `agent_event` badges from message history (see the Streaming gotcha below), so opening an ex-CLI chat there displays tool activity correctly.

### Backend process model — client reference counting (2026-07-09)

An interactive `memo` session that finds no backend running now spawns one as a **genuinely separate, detached OS process** (`spawnDetachedBackend` in `main.go`, `exec.Command(exe, "--headless", "--port", ..., "--auto-shutdown")`, `Setsid`/`CREATE_NEW_PROCESS_GROUP` via `main_unix.go`/`main_windows.go`) instead of running it in-process — the REPL is now always a pure HTTP client of the backend, whether it just spawned one or attached to an existing one (unifies what used to be two different code paths).

That backend tracks which clients are attached (`internal/app/clients.go`'s `clientRegistry`: `RegisterClient`/`HeartbeatClient`/`UnregisterClient`, exposed as `POST /api/clients/{register,heartbeat,unregister}`) and shuts itself down (self-`SIGINT`, same mechanism `/api/shutdown` uses) once the last one disconnects — **but only if `--auto-shutdown` was passed**, which only `spawnDetachedBackend` does. A plain `--headless` backend run as a standalone service (remote access, WhatsApp bridge) never arms this, regardless of which clients happen to register with it — this is the guard that keeps a persistent service safe.

Both the CLI (`internal/replcli/clients_client.go`, wired into `repl.go`'s `Run()`) and the Flutter GUI (`frontend/lib/providers/chat_provider.dart`'s `connectionStatusProvider`, which already polled every 30s — extended to register once then heartbeat each tick instead of adding a new timer) register on startup and heartbeat every ~25-30s. The CLI unregisters explicitly on a clean exit (`/exit`, Ctrl+D, double Ctrl+C — not reachable on external SIGTERM, same blocking-read limitation as the terminal-restore fix above); Flutter has no reliable window-close hook on desktop today (confirmed: no `window_manager`, no lifecycle observer wired up), so a closed GUI is only noticed once it stops heartbeating — the backend prunes a client silently after `clientStaleAfter` (90s, ~3 missed beats).

This is what makes "CLI open, then open the GUI via /gui, then close the CLI" and the reverse both safe: whichever client is left keeps the backend alive via its own heartbeat, and only when the registry drains to zero does the backend shut down — bounded, not indefinite background lingering.

Verified against the real compiled binary (not just unit tests) in this session: `--auto-shutdown` backend with two registered clients survives one leaving and self-shuts-down when the second leaves; a plain `--headless` backend (no `--auto-shutdown`) survives a client registering and unregistering exactly as before. Not yet verified: an actual `/gui`-launched Flutter window in this environment (no display), and the Windows detach path (`CREATE_NEW_PROCESS_GROUP`) — only compile-checked via `GOOS=windows go vet`.

### Identity / System Prompt (`internal/identity/`) (2026-07-10)

The default system prompt (`buildIdentityBlock`) was translated from Turkish to English — instructions generally get followed more reliably in English regardless of conversation language, and the block already tells the model to always reply in whatever language it's written to (this is independent of the setup wizard's own persona prompts in `frontend/lib/widgets/setup_wizard_view.dart`, which carry separate `prompt_tr`/`prompt_en` text per persona and override the default entirely via `CustomRole` when picked).

`buildOriginBlock` was added and is **always** appended in `BuildSystemPrompt`, independent of whether `CustomRole` (a wizard persona or a hand-written prompt) is set — it used to live inside `buildIdentityBlock`, which only runs when `CustomRole` is empty, so picking any wizard persona silently dropped "who made you" grounding entirely. Kept deliberately terse (~100 tokens, down from an initial ~260) since it only pays off on the rare message where someone actually asks — Memo's target audience runs small local models on tight context budgets, where padding a permanent tax for a rarely-used fact is the wrong trade.

`Identity.MinimalMode` (`cfg.Identity.MinimalMode`, toggle in Settings → General → "Minimal Mod") strips identity/origin/style injection from `BuildSystemPrompt` entirely, and gates the mood-directive/web-search injection in `internal/app/helpers.go`'s `buildMessages` too — memory context still goes in if `cfg.Memory.MemoryEnabled` is separately on, since MinimalMode only targets persona/mood/search injection, not memory. With both toggles off, zero extra tokens reach the model beyond the raw conversation. `MinimalMode` overrides `CustomRole` too, not just the default block — a wizard persona is also fully suppressed when it's on.

~~The 2026-07-12 `buildCapabilitiesBlock` fix (above, in Provider/Agent/Orchestra) only covers *toggleable* features (agent mode, web search) that are currently off. A user asked directly "can you set a reminder for me" in a plain chat and was told flatly there's no such system — at the exact moment `processMessageIntent` (`internal/app/chat.go`, called unconditionally on every message, no config gate) was silently scanning that very message and about to create exactly such a reminder in the background. The model had zero information this background behavior exists at all, toggleable or not~~ → fixed 2026-07-14: new `buildPassiveFeaturesBlock()` (`identity.go`) states the always-on calendar/reminder auto-detection explicitly — placed right after `buildOriginBlock` (same unconditional-except-MinimalMode gating, independent of `CustomRole`), since unlike agent mode/web search this has no toggle to report as "off," it just needs to be affirmed as existing. Tests: `TestBuildSystemPrompt_MentionsPassiveReminderFeature`, `TestBuildSystemPrompt_MinimalMode_OmitsPassiveFeaturesBlock` (`identity_test.go`).

### Flutter
- ~~`settings_dialog.dart` is 4391 lines~~ → split into 15 focused files under `settings/tabs/`.
- ~~`model_store_screen.dart` is 2612 lines (re-verified 2026-07-11) — should be split into components~~ → fixed 2026-07-12 (BUG-M1): split via `docs/plans/PLAN_modelstore_refactor.md`'s 5 phases into a 180-line shell + `screens/model_store/discover_item.dart`/`discover_tab.dart`/`model_detail_panel.dart`/`my_models_tab.dart`, mirroring the `settings/tabs/` pattern above. Pure mechanical move — only the 8 symbols referenced across the new file boundaries went from private to public.
- ~~`EngineStrip`'s divider before the memory warning only checked `chatRunning || isApiProvider`, missing the case where the offline hint rendered instead (fully offline: no chat model, no API provider, embedding not running) — the offline hint and the memory warning rendered stuck together with zero gap ("Start a model●No memory model")~~ → fixed 2026-07-10: two named booleans (`firstSlotHasContent`, `secondSlotHasContent`) capture what the strip's first/second slot actually rendered, used for all three divider checks (embedding indicator, memory warning, download indicator) instead of the incomplete `chatRunning || isApiProvider` shorthand. Regression test in `engine_strip_test.dart` asserts both the divider's presence and a minimum pixel gap between the two texts.
- ~~Widespread missing `const` constructors.~~ → fixed: 116 auto-fixes via `dart fix`.
- ~~connectionStatusProvider polling runs forever~~ → still autoDispose but polling loop is acceptable for status checks.
- ~~Chat SSE client timeout (60s) shorter than backend's own generation budget (300s), no server heartbeat~~ → fixed 2026-07-04: `chat_provider.dart` now uses 300s for both regular and agent chat streams (and file-send stream), matching `llm.go`'s `context.WithTimeout(ctx, 300*time.Second)`. Previously a slow first-token (long prompt, weak/CPU hardware) past 60s aborted an in-progress, otherwise-valid generation.

### Build / Dependencies
- 2026-07-04: `frontend/pubspec.yaml` had accumulated reactive `dependency_overrides` (`jni: 0.13.0`, `path_provider_android: 2.3.1`, `jni_flutter: 1.0.1`) added to work around what was actually a **corrupted pub-cache** (`jni-1.0.0` and `path_provider_windows-2.3.0` had 0-byte/partial downloads), not a real version conflict. Removed the overrides and re-fetched the packages; Linux build is clean again. Worth remembering if a similarly "unexplained" plugin build failure shows up again — check the pub-cache contents before reaching for a version pin.

### Flutter / Mobile
- ~~Mobile API client (`mobile/lib/core/api_client.dart`) missing most backend endpoints~~ → stale: verified 2026-07-10, it covers 111 of the backend's 118 registered routes. The 7 missing are either brand-new (today's client-registry/minimal-mode endpoints, which mobile has no use for — no CLI/detached-backend concept there) or CLI-install-management endpoints (`/api/cli/*`, `/api/uninstall`) that don't apply to a mobile app either. Not a real gap anymore.

### WhatsApp
- ~~QR code polling never stops~~ → adaptive: 2s during QR wait, 15s heartbeat when connected.
- ~~`handleHistorySync` only fires on first pairing~~ → uses `INSERT OR IGNORE`, safe on reconnects.
- ~~WhatsApp store no serialized writes~~ → fixed: `sync.Mutex` on `SaveMessage` and `SaveContact`.
- ~~WhatsApp client init not mutex-protected~~ → fixed: `waMu` mutex protects initialization and lifecycle.

### Other
- ~~Config file written with `0644`~~ → fixed: `config.Save()` uses `0600`. Agent permissions/backup also `0600`.
- ~~`app.go` stores `context.Context` in struct field~~ → `lifecycleCtx` is for goroutine lifecycle only, not request-scoped. All request methods accept `ctx` as parameter. Correct pattern.
- ~~`config/config.yaml` has hardcoded `active_provider: openai`~~ → fixed: empty string default.
- ~~Shutdown does not cancel lifecycle context~~ → fixed: lifecycleCancel added, all goroutines stopped on shutdown.
- ~~Cloud backup missing WAL checkpoint~~ → fixed: PRAGMA wal_checkpoint(TRUNCATE) before archive.
- ~~Memory store write operations used read lock~~ → fixed: SaveExplicitMemory/DeleteExplicitMemory use write lock.
- ~~Sessions init not mutex-protected~~ → fixed: sessionsMu.Lock() added.
- ~~orchestraConductor read without mutex~~ → fixed: providerMu.RLock() added.
- ~~proactiveDecide swallows LLM errors~~ → fixed: empty/error responses now return proper errors.
- ~~Nil client dereference when no model loaded~~ → fixed: nil guards added in callLLMStream and callLLM.
- ~~Silent error discards in memory/selfclone/whatsapp~~ → fixed: errors now logged instead of silently ignored.
- CI: GitHub Actions runs Go vet/test/build + Flutter analyze/test on every push/PR.
- **Rate limiting** — token-bucket per-IP (100 req/s) on all handlers via `rateLimitMiddleware`.
- **Structured logging** — `internal/logx` wraps `log/slog` with levels; all packages migrated to `logx.Printf`.

---

## Düzeltilen Buglar (2026-06-28)

Aşağıda bulunan ve düzeltilen hataların basitçe özeti:

### Veri Kaybı / Bozulma
- **Memory silme/kaydetme kilidi yanlıştı** — Aynı anda iki kişi memory ekler veya silerse veritabanı bozulabiliyordu. Write lock ile düzeltildi.
- **Cloud backup eksik kalabiliyordu** — SQLite WAL modda veriler ana dosyaya yazılmadan önce yedek alınıyordu. Artık önce checkpoint çalıştırılıyor, yedek tam oluyor.
- **Uygulama kapatılınca arka plan işlemleri durmuyordu** — Proactive öneriler, takvim hatırlatmaları, WhatsApp dinleme gibi işlemler kapanışta devam ediyordu. Artık lifecycle context iptal ediliyor, her şey duruyor.

### Eşzamanlılık (Aynı Anda Erişim)
- **WhatsApp çift bağlantı riski** — WhatsApp başlatılırken mutex kullanılmıyordu. İki çağrının aynı anda gelmesi durumunda iki ayrı bağlantı oluşabiliyordu, mesajlar kaybolabiliyordu.
- **Sessions başlatma kilitlenmemişti** — Oturum yöneticisi mutex olmadan atanıyordu, eşzamanlı erişim panic'e neden olabiliyordu.
- **Orchestra motoru kilitsiz okunuyordu** — Mod geçişlerinde data race oluşabiliyordu.

### Dosya Yükleme / Güvenlik
- **Görseller tanınamıyordu** — Dosya yüklerken görsel olup olmadığını anlamak için dosya yolunun sonuna bakılıyordu ama temp dosya isimleri rastgele olduğu için hiçbir zaman eşleşmiyordu. Artık dosya içeriğinden MIME tespit ediliyor.
- **Rate limit aşılabilir LAN'da** — X-Forwarded-For header'ı güvenilir olduğu varsayılıyordu. Saldırgan rastgele IP ile limiti aşabiliyordu. Artık sadece gerçek IP kullanılıyor.
- **Import ile dosya dışı yollara yazılabilir** — .memo dosyası import edilirken path traversal kontrolü zayıftı. filepath.Rel ile doğrulama eklendi.

### Ön Yüz (Flutter)
- **Takvim ekranı çökebiliyordu** — Backend'den hatalı tarih gelirse tüm takvim ekranı kırılıyordu. Try-catch ile güvenli hale getirildi.
- **WhatsApp konuşmada metin kaybı** — Streaming sırasında metin eklemeleri atomik değildi, bazı kelimeler kaybolabiliyordu. Accumulator ile düzeltildi.
- **Dosya gönderiminde eski durum kalıyordu** — Dosya gönderildikten sonra agent event'leri temizlenmiyordu, eski durum rozetleri görünüyordu.
- **Çalışma animasyonu yanlış zamanda görünüyordu** — Metin akarken de "Memo çalışıyor" noktaları görünüyordu. Sadece boşken gösteriliyor artık.
- **Proactive öneri hataları yutuluyordu** — LLM hatalı cevap verdiğinde bile öneri olarak kaydediliyordu. Artık hatalar raporlanıyor.
- **Takvim dialog hafıza sızıntısı** — TextController'lar dialog kapatılınca temizlenmiyordu.
- **Versiyon kontrolü yanlış uyarı** — Backend erişilemezse eski versiyon dönüyordu, yanlış "güncelleme var" bildirimi geliyordu.
- **WhatsApp QR ekranı compile hatası** — Fazladan parantez nedeniyle QR eşleştirme ekranı hiç açılmıyordu.
- **Güvensiz tip dönüşümleri** — Backend response'u beklenmedik formattaysa `as List`/`as Map` cast'leri crash oluşturuyordu. 5 farklı noktada `is` kontrolü eklendi.
- **Agent ekranı sessizce başarısız oluyordu** — `createAgentChat` hatası kullanıcıya gösterilmiyordu. Try-catch ve snackbar eklendi.
- **Skill dialog Windows uyumsuzluğu** — Unix path hint'ı Windows'ta gösteriliyordu. Platform algılama eklendi.

---

## Agent Working Rules (READ FIRST, EVERY SESSION)

1. **Start of session:** read this file, then `handoff.md` (top entry = last session's state and pending work).
2. **End of session:** prepend a new handoff entry to `handoff.md` (what was done, commit status, verification results, what's next). This is how context survives between sessions and models.
3. **Never claim "done" without running verification** (below) and pasting the actual results.
4. Plan files (`plan.md`, `PLAN_*.md`) contain step-by-step implementation plans — follow them in order, tick checkboxes as you complete items, don't improvise a different architecture mid-plan.
5. Work in small units: max 1–2 plan items per session, each with tests, verified green before moving on.
6. **Commit automatically, without asking for confirmation, once a fix/feature is verified green.** Don't ask "commit edeyim mi?" — just commit. Conventional Commits format (`fix(frontend): ...`, `feat(backend): ...`), a detailed English body explaining the *why* (root cause, what changed, what it fixes), and **never** any AI attribution / Co-Authored-By / "Generated with Claude" line, under any circumstance.
7. **Code exploration:** this repo is indexed in `codebase-memory-mcp` as project `home-bugra-Belgeler-memo` (9k+ nodes, Go + Dart). Before grepping around for "who calls X" / "what does X call" / impact-of-change questions, use `trace_path`/`search_graph`/`get_architecture` — cheaper and more precise than reading files blind. Re-run `index_repository` if the graph looks stale against a recent diff. A live 3D view is available at `http://localhost:9749` (start with `codebase-memory-mcp --ui=true --port=9749`, stdin must stay open — see `~/.local/bin/codebase-memory-mcp.no-ui.bak2` for the pre-UI binary if the swap ever needs reverting).

### Verification Commands (mandatory before any "done" claim)

```bash
# Backend (CGO is required — sqlite; -tags "sqlite_fts5" is required too, see
# "Memory / Vector Store" below — without it FTS5 silently never activates)
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race

# Frontend (Flutter SDK is NOT in PATH on this machine)
export PATH="$PATH:/home/bugra/Belgeler/flutter/bin"
cd frontend && flutter analyze lib/ && flutter test
```

Acceptable pre-existing noise: a few `use_build_context_synchronously` **info**-level findings in `flutter analyze`. Anything else new must be fixed.

---

## Gotchas (project-specific traps — violating these causes real, shipped bugs)

**Paths & data**
- All data paths go through `config.DataPath()`. Never hardcode `data/` — on Windows data lives in `%ProgramData%\Memo\data`.
- The installed CLI lives at `~/.memo/bin/memo` (one level deep): lookups for bundled files must also check the **parent** of the executable's directory. REPL debug log: `~/.memo/data/repl.log`.
- Windows path bugs in Flutter: always `package:path` (`p.join`, `p.basename`) — never `split('/')` or `'$a/$b'` string concat.

**Concurrency & architecture**
- SQLite writes go through `database.DB.Write()` (serialized write loop) — never call `ExecContext`/`Exec` directly on the DB; it corrupts the single-writer design.
- The app has **one global active chat** and a global agent-mode flag. `App.SendMessageStream` writes to whatever chat is currently active. Any automated caller (e.g. task loop) must `SwitchChat` under `taskloopRunMu` and restore state after. In Flutter, don't trust `activeChatIdProvider` blindly — pass explicit chat IDs.
- `a.client` and `providerRouter` are swapped at runtime (model/provider changes during active streams) — always take `clientMu`/`providerMu` before touching them.

**Streaming / SSE**
- Timeout contract: backend generation budget in `internal/app/llm.go` is **300s**; every frontend SSE consumer (`chat_provider.dart`, `chat_input.dart` WhatsApp stream, file-send stream) must use the **same 300s — as a per-call option on the streaming request**, NOT via Dio's global `receiveTimeout` (that stays at 120s for regular API calls; see commit `2abf8dd`). A 60s frontend timeout once aborted valid slow generations on CPU hardware.
- SSE `finishReason == 'agent_event'` chunks carry JSON tool events — render as badges, never as raw text; parse defensively (payload may be malformed).
- **Any "is this stream still in-progress" flag (`isSendingProvider` etc.) must be touched by *every* code path that can end that stream** — every `return`/error/timeout branch in `callAgentStream`/`callLLMStream`'s provider and local-model loops (`internal/app/llm.go`), not just the happy path. A branch that just logs/records an error and `return`s without sending a terminal SSE chunk leaves the frontend waiting forever with no explanation. Grep for every branch of a stream-producing function before declaring a streaming bug fixed — a fix in one branch doesn't mean the sibling branches got the same treatment (this is exactly how the 2026-07-14 "durdur" button saga went three rounds: the first fix covered a `select`-race, a second covered a `ctxDone` branch, and neither was the actual live bug — see below).

**Riverpod / async notifiers**
- Riverpod can rebuild the **same `Notifier`/`AsyncNotifier` class instance** (not a fresh one) when a provider is invalidated while something is still watching it — confirmed empirically in this codebase (`build()` → `onDispose` → `build()` again, same object), not just a theoretical edge case. **Never use a plain `bool` "am I disposed" flag that's only initialized once in a field declaration or reset unconditionally in `build()`** — both are wrong: never resetting it means it stays permanently `true` after the *first* such cycle (poisoning every future call for the rest of the session — this was the real, final root cause of the 2026-07-14 "durdur" button bug, three fix-attempts deep); resetting it unconditionally in `build()` "un-disposes" an *old*, still-running, abandoned call from the previous generation, letting it clobber shared state meant for the new one (this is what BUG-H2 already guards against — see `messages_notifier_dispose_test.dart`). **Use a monotonically-incrementing `int _generation` counter instead**: bump it once per `build()`; every async method captures `final myGeneration = _generation;` at its own entry and only touches shared state while `_generation == myGeneration` still holds. See `MessagesNotifier` in `frontend/lib/providers/chat_provider.dart` for the reference implementation, and `messages_notifier_stale_disposed_flag_test.dart` for the regression test that forces this exact instance-reuse cycle.

**Flutter**
- `IndexedStack` keeps every screen mounted forever: any polling loop must stop itself via `VisibilityDetector` / `AppLifecycleListener` / `ref.onDispose`, or it leaks and polls in background.
- **A file already having `import '../../../core/l10n.dart'` and calling `L10n.t(...)` in some places doesn't mean it's fully localized** — `settings/tabs/backup_restore_tab.dart` and `learning_tab.dart` had complete, correct TR+EN keys sitting unused in `l10n.dart` for a long time while the widgets still called `Text('hardcoded Turkish literal')` directly (a half-finished migration). Before assuming a string is covered, grep the actual widget for the literal, not just check whether the key exists. Also: an existing key can itself be wrong — several (`add_provider`, `no_providers`, etc.) had plain English text sitting in the **Turkish** map, so even a correctly-wired `L10n.t()` call silently showed English under the Turkish locale. Fixed 2026-07-14 across all 11 `settings/tabs/*.dart` files (`fe872eb`) — same class of bug likely still present in `agent_screen.dart`, `chat_input.dart`, `permission_dialog.dart`/`permission_history.dart`, and the `orchestra_config_dialog.dart`/`provider_config_dialog.dart`/`skill_config_dialog.dart` trio (found via a grep sweep, not yet audited/fixed — out of scope for that session by user request).
- Backend JSON must be checked with `is` before casting — `as List`/`as Map` on unexpected payloads has crashed the UI in 5+ places before.
- New user-facing strings go through `frontend/lib/core/l10n.dart`.
- Unexplained plugin build failure? Check `~/.pub-cache` for 0-byte/partial package downloads **before** adding `dependency_overrides` (see 2026-07-04 note above).

**Types & misc**
- `skill.DangerLevel` and `agent.DangerLevel` are separate named types — they do not cross-assign.
- Turkish + English mixed user-facing text is intentional (target users are Turkish).
- sqlite-vec extension (`vec0.so`/`vec0.dll`) is bundled under `binaries/` — never add a runtime download for it.

---

## Known Open Work (pointers)

| Item | Where |
|------|-------|

Open bugs and technical debt are tracked in **`BUG_REPORT.md`** — don't duplicate the list here. As of 2026-07-12 (Session 24) it has 0 open items. ~~Onboarding / launchpad UX~~ is done and archived in `docs/plans/plan.md`.

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the codebase (intentional for target users).
- CGO required: `CGO_ENABLED=1 go build/test/run`. `-tags "sqlite_fts5"` required too — see `docs/CGO_FLAGS.md`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.2** (open beta, 2026-07-06) (Go 1.26, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
