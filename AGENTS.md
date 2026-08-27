# AGENTS.md — Memo

Memo is a local-first, privacy-focused LLM chat application with RAG memory, external provider support, and E2E-encrypted cloud sync. Designed for offline desktop use with optional API fallback.

---

## Agent Working Rules (READ FIRST, EVERY SESSION)

1. **Start of session:** read this file, then `handoff.md` (top entry = last session's state and pending work).
2. **End of session:** prepend a new handoff entry to `handoff.md` (what was done, commit status, verification results, what's next). This is how context survives between sessions and models.
3. **Never claim "done" without running verification** (below) and pasting the actual results.
4. Plan files (`plan.md`, `PLAN_*.md`) contain step-by-step implementation plans — follow them in order, tick checkboxes as you complete items, don't improvise a different architecture mid-plan.
5. Work in small units: max 1–2 plan items per session, each with tests, verified green before moving on.
6. **Commit automatically, without asking for confirmation, once a fix/feature is verified green.** Don't ask "commit edeyim mi?" — just commit. Conventional Commits format (`fix(frontend): ...`, `feat(backend): ...`), a detailed English body explaining the *why* (root cause, what changed, what it fixes), and **never** any AI attribution / Co-Authored-By / "Generated with Claude" line, under any circumstance. **Commit frequently, not just once per finished task** — break a multi-step request into checkpoints and commit after each one once it's verified, especially right before a risky/bug-prone step (a refactor touching a hot or shared code path, a change you're not fully certain about). Finer-grained history means a bad step can be bisected/reverted without losing unrelated good work from the same session. Don't fragment so much that a single atomic change gets split into a broken intermediate commit — this is about natural checkpoints, not every file save.
7. **Code exploration:** this repo is indexed in `codebase-memory-mcp` as project `home-bugra-Documents-memo` (10k+ nodes, Go + Dart). Before grepping around for "who calls X" / "what does X call" / impact-of-change questions, use `trace_path`/`search_graph`/`get_architecture` — cheaper and more precise than reading files blind. Re-run `index_repository` if the graph looks stale against a recent diff. A live 3D view is available at `http://localhost:9749` (start with `codebase-memory-mcp --ui=true --port=9749`, stdin must stay open — see `~/.local/bin/codebase-memory-mcp.no-ui.bak2` for the pre-UI binary if the swap ever needs reverting).
8. **Zero hardcoded user-facing strings in Flutter — no exceptions, no "I'll wire it later."** Every string a user can actually see — labels, buttons, dialogs, snackbars, tooltips, menu items, on-screen error/status messages — MUST go through `L10n.t('key')`, with BOTH a Turkish and an English entry added to `frontend/lib/core/l10n.dart` in the same change, not a stub and not one language only. A key existing in `l10n.dart` is not proof the call site is actually wired to it — see the L10n Gotcha below for the exact, previously-shipped bug class this causes (keys sitting unused for a long time while widgets still called `Text('hardcoded literal')` directly, and separately, keys holding plain English text inside the Turkish map). Before claiming any Flutter change done, grep the exact files you touched for quoted literals inside `Text(`/`Tooltip(`/`SnackBar(`/`AlertDialog(` and similar — an empty result is part of "verified," not optional polish. Raw non-UI strings (log lines, exceptions that never reach a widget) are exempt; this rule is about anything a user actually reads on screen. This is a hard rule because it has been violated repeatedly despite being asked for directly — treat it with the same weight as the commit-message rule above, not as a style preference.
9. No fabrication — ever. When in doubt, say so and ask.
Never invent, guess, or assert anything you cannot ground in evidence — code, facts, file paths, function/API names, versions, error messages, or anything a user might act on.
- Don't understand the request → say "I don't understand X, can you clarify?" and stop. Never silently proceed on a guessed interpretation.
- Can't find the answer (after graph tools / grep / docs) → say "I couldn't find X" — never manufacture a plausible-looking answer.
- Low confidence / about to guess → state "I'm not certain, but…" and ask for confirmation before acting; never present a guess as fact.
- Proof obligation: every concrete claim (file path, function name, line number, API, version, behavior) you make MUST be traceable to a source you actually read or a command you actually ran. If you cannot point to the file/line or the output that proves it, you are fabricating — stop and ask. Inventing snippets, errors, paths, or "facts" is precisely the failure this rule forbids.

### Verification Commands (mandatory before any "done" claim)

```bash
# Backend (CGO is required — sqlite; -tags "sqlite_fts5" is required too, see
# "Memory / Vector Store" below — without it FTS5 silently never activates)
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test -tags "sqlite_fts5" ./... -race

# Frontend (Flutter SDK is NOT in PATH on this machine)
export PATH="$PATH:/home/bugra/Documents/flutter/bin"
cd frontend && flutter analyze lib/ && flutter test

# L10n check (Agent Working Rules #8) — any new hardcoded UI string literal
# in the Flutter files this change touched? Heuristic, not exhaustive (a
# literal built via string concatenation/interpolation can still slip past
# it), but an empty result on files you actually edited is required before
# claiming a Flutter change done; anything it does flag needs a real look.
git diff --name-only -- '*.dart' | xargs -r grep -nE "(Text|Tooltip|SnackBar|AlertDialog)\(\s*['\"][A-Za-zÇĞİÖŞÜçğıöşü]"
```

Acceptable pre-existing noise: a few `use_build_context_synchronously` **info**-level findings in `flutter analyze`. Anything else new must be fixed.

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
| `internal/replcli/` | Terminal REPL client — talks to the REST API the same way Flutter does | `client.go`, `repl.go`, `sse.go`, `agent_event.go`, `clients_client.go`, `commands.go`, `editor.go`, `keys.go`, `models_client.go`, `remote_client.go`, `sessions_client.go`, `tasklist_client.go`, `color.go` (welcome panel), `l10n.go` (CLI's own TR/EN string table, separate from Flutter's `l10n.dart` — synced to the same language via backend `Identity.UILanguage`), `filematch.go` (`@` file-mention) |
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
| `internal/telegram/` | Telegram bridge (Bot API polling) — mirrors `internal/whatsapp/`'s shape | `client.go`, `store.go` |
| `internal/whisper/` | Speech-to-text (whisper.cpp) | `whisper.go`, platform-specific files |
| `internal/skill/` | Skill system (plugin-like) | `manager.go`, `loader.go`, `types.go`, `executor.go` |
| `internal/ngrok/` | ngrok tunnel integration | `installer.go`, `manager.go` |
| `internal/tunnel/` | Tailscale embedded tunnel (tsnet) | `tailscale.go` |
| `internal/truncate/` | Token-aware context truncation | `tokens.go` |
| `internal/models/` | Shared data types | `memory.go` |
| `internal/lora/` | LoRA adapter building (embryonic) | `build/` (cmake artifacts) |
| `internal/stats/` | Persistent LLM usage-event store (tokens, speed, model) for the Settings stats tab | `store.go` |

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
./scripts/build_releases.sh                          # dist packages (tar.gz, AppImage, deb) — already tagged
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

### Checkpoint / pre-release tags (lightweight, NOT the same as a release)

Separate from the full release process above — this is for snapshotting a
point in history (e.g. to hand a build to friends/testers) without going
through version bumps, changelog files, or the download.bugradev.com/
version.json publish targets. Added 2026-07-21 (`b50f481`, tag `v3.2.1`).

**How it works:** `build-linux.yml`/`build-windows.yml`/`build-macos.yml`
each trigger on `push: tags: ["v*"]` (in addition to their existing
main/PR/`workflow_dispatch` triggers) and, only when the ref is actually a
tag (`if: startsWith(github.ref, 'refs/tags/')`), run a final step that
publishes that platform's built zip to a GitHub **pre-release** via
`softprops/action-gh-release`, found-or-created by the tag name — all
three platforms' independently-running jobs converge on the same release
instead of each creating their own. Ordinary pushes to `main` and PR builds
are completely unaffected (that step never runs for them).

**To cut one:**
```bash
git tag -a v<X.Y.Z> -m "short description of this checkpoint"
git push origin v<X.Y.Z>
```
That push is what triggers the three builds — nothing else is needed. The
GitHub release itself is created empty (no notes) by whichever platform's
job finishes first; set notes afterward once at least one job has
completed its publish step:
```bash
gh release edit v<X.Y.Z> --notes "..."
```

**The tag name is independent of the app's embedded version** (the
`version` file / `/api/version`) — a checkpoint tag does not need to match
whatever `version` currently says, and nothing enforces that it does. This
is deliberately looser than the numbered-release process, which is exactly
why it must never be treated as a substitute for one.

**Hard rule: never push a tag matching `v*` without the user explicitly
asking for it in that specific moment.** Pushing one is a real, visible,
somewhat irreversible action — it immediately triggers CI on three
platforms and publishes a public GitHub pre-release with downloadable
binaries. This must never happen proactively, as a side effect of some
other task, or because a past request seemed like a standing instruction —
confirm every single time before tagging or pushing a tag.

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

**Client-side state vs. server identity (2026-08-13)**
- **A client's saved auth state can outlive the backend it was recorded against, and the browser will never tell you.** localStorage is keyed by *origin*, so wiping a self-hosted Memo and reinstalling it hands the same `http://<lan-ip>:8090` to a completely new backend while every stored key survives. Reported live from a Raspberry Pi: `memo_auth_setup_done` alone suppressed the setup screen permanently (`authGateProvider`'s `needs_setup && declined -> ok` branch read it as a deliberate choice) while every API call 401'd — and **neither Ctrl+Shift+R nor Ctrl+F5 helps, because a hard reload only bypasses the HTTP cache**; the only fix was clearing site data by hand in DevTools. Two layers guard this now and both must be kept working: (1) `/api/setup/status` returns `install_id` (`internal/app/install_id.go`, random per install, dies with the data dir), and the gate wipes server-coupled state on a mismatch; (2) the gate re-probes and drops the declined flag when a non-loopback source gets a 401, which is the only layer that works against backends too old to report an id. **If you add a SharedPreferences key that means anything about a specific backend (a credential, a session identity, a first-run milestone), add it to `serverCoupledPrefsKeys` in `frontend/lib/core/local_session_state.dart` in the same change** — that list is the whole mechanism, and a key missing from it silently reopens a narrower version of this bug. Device preferences (`memo_locale`, `memo_theme_mode`, `memo_streaming`, `memo_beta_features`) and `memo_api_base_url` must stay out of it: clearing the base URL strands the client. `install_id` is deliberately excluded from `ExportData` — a restored backup must read as a new install.
- First sight of an `install_id` records it **without** resetting. Every existing client upgrades into that state holding a valid sign-in; resetting on first sight would sign out every working user to catch the few who are broken.

**Auth-gate startup race — audit by SHAPE, not by class name (BUG-ONB4/5/6/11)**
- `AppShell`'s `IndexedStack` builds **every** screen at app start, before the auth gate opens. Any one-shot fetch reachable from that build hits a gated backend, 401s, and — unless something retries it — stays broken for the whole session while the rest of the app works fine. This has now been fixed **four separate times**, each time because the previous audit searched for the wrong *shape*: BUG-ONB4 covered polling providers, BUG-ONB6 audited `AsyncNotifier.build()` + `apiClientProvider` (17 providers), and both left survivors — `gpuInfoProvider` (BUG-ONB5) and then `gatewayModelsProvider` + `RoutinesScreen`/`CalendarScreen` (BUG-ONB11). **Before declaring this class of bug swept, check all four shapes, not just the one that matches the last fix:** (1) `AsyncNotifier`/`StateNotifier` `build()`/constructor fetches, (2) plain `FutureProvider`s (no retry loop — a 401 is permanent), (3) `StatefulWidget` `initState`/`addPostFrameCallback` fetches that set widget state, (4) polling loops. Shape (3) is invisible to any provider-oriented graph query.
- The fix is uniform: `if (authGateBlocked(ref.read(authGateProvider).valueOrNull))` → mount a safe default (never an error), and recover on the gate transition — providers via `app_shell.dart`'s single centralized `ref.listen` on `authGateProvider`, screens via their own `ref.listen` in `build()` (see `routines_screen.dart`). **Prefer the centralized listener over per-call-site invalidation:** `gpuInfoProvider` is still invalidated from `auth_gate_overlay.dart`'s five login paths, and that list only knows about logins — it misses every other way the gate can open.
- **A local pref that mirrors backend config is not a device preference.** `memo_beta_features` mirrors `cfg.Beta` and is meant to be consulted only until the server answers — but `_showSwarmNav` fell through to it whenever the answer wasn't `beta:true`, so a stale local `true` beat a perfectly good `beta:false` forever. When a provider swallows its failure into a plausible-looking map (`remoteAccessProvider` returns `{'enabled': false}` on error), callers cannot tell "server said no" from "server hasn't answered" by null-checking — test for the **key** the real response always carries. Such mirrors belong in `serverCoupledPrefsKeys`, not in the preserved set.
- Reproducing this needs a genuinely gated backend reached from a **non-loopback** address. A local desktop run never shows it: `remoteAuthOK` trusts loopback, so the gate is never up in the first place.
- **A gate check in `build()` is not enough by itself — the `catch` block matters too (BUG-ONB12).** `orchestraConfigProvider` had the `authGateBlocked()` guard from BUG-ONB6, but its `catch` still pushed any *other* fetch failure into `errorMessageProvider`. Harmless for a provider only a screen the user opened reads — but this one is also watched **ambiently** by always-mounted widgets (`engine_strip.dart`, `chat_input.dart`'s Orchestra icon), so a transient failure while Orchestra was off surfaced an unrelated toast with no connection to anything the user was doing. `activeProviderTypeProvider`/`remoteAccessProvider` already learned this: an ambiently-watched provider's `build()` must degrade **silently** on any failure (`debugPrint` only), not just on a blocked gate — a dialog/tab the user explicitly opened to look at that data can still show the error via its own `.when(error: ...)` branch. Before adding `errorMessageProvider.notifier).state = ...` inside any provider's `build()`/constructor, check whether it's read from an always-visible widget, not just a screen the user navigated to.

**Flutter**
- Localized labels vary in width far more than they look — Turkish generally runs longer than English. A `Row` of buttons that fits in one language can overflow in another: the auth gate footer threw `RenderFlex overflowed by 54 pixels` under Turkish the moment a second action button was added, while passing under English. Use `Wrap` for rows of localized actions, and remember that a widget test only exercises whatever locale happens to be the default.
- `IndexedStack` keeps every screen mounted forever: any polling loop must stop itself via `VisibilityDetector` / `AppLifecycleListener` / `ref.onDispose`, or it leaks and polls in background.
- **A file already having `import '../../../core/l10n.dart'` and calling `L10n.t(...)` in some places doesn't mean it's fully localized** — `settings/tabs/backup_restore_tab.dart` and `learning_tab.dart` had complete, correct TR+EN keys sitting unused in `l10n.dart` for a long time while the widgets still called `Text('hardcoded Turkish literal')` directly (a half-finished migration). Before assuming a string is covered, grep the actual widget for the literal, not just check whether the key exists — see the Agent Working Rules #8 grep, it exists specifically to catch this. Also: an existing key can itself be wrong — several (`add_provider`, `no_providers`, etc.) had plain English text sitting in the **Turkish** map, so even a correctly-wired `L10n.t()` call silently showed English under the Turkish locale. Fixed 2026-07-14 across all 11 `settings/tabs/*.dart` files (`fe872eb`) and, in a later pass, `chat_message_list.dart`/`chat_input.dart`/`skill_config_dialog.dart`/`permission_dialog.dart`/`permission_history.dart` (BUG-M3, `79bda62`/`fac700f`/`f53c2ec`). ~~Re-audited 2026-07-20 with the Rule #8 grep: `orchestra_config_dialog.dart` and `provider_config_dialog.dart` still have real hardcoded Turkish literals right now (`provider_config_dialog.dart`: "İptal", "Kaydet", "Model bulunamadı" among others) despite `f53c2ec`'s commit message claiming "provider and skill config dialogs" were both covered — skill's dialog is actually clean, provider's is not; don't trust a commit message's file list over grepping the file itself. `agent_screen.dart` has one leftover ("Agent" label, `:316`, likely low priority but unaudited either way). These three are still open, not yet fixed.~~ → stale, re-verified 2026-07-29: all three are clean now (fixed by intervening commits `af33c59`/`377d5be`/`36c8a38`/`1530a4d`, none of which called this bullet out explicitly — exactly the "don't trust the commit message" lesson this bullet already taught, now cutting the other way). Repo-wide Rule #8 grep across `frontend/lib/**/*.dart` for `Text(`/`Tooltip(`/`SnackBar(`/`AlertDialog(` with a quoted literal not wrapped in `L10n.t(...)` found zero real violations — the only matches were `'WhatsApp'` (brand name, routines_screen.dart), a dropdown's own data value (`chat['display_name']`), and a `fontFamily: 'JetBrains Mono'` (not user-facing text). Cross-checked common hardcoded UI words directly (İptal/Kaydet/Tamam/Sil/Ekle/Kapat/Devam/Vazgeç/Onayla/Ayarlar/"Model bulunamadı") — every hit lives only inside `l10n.dart`'s own maps.
- Backend JSON must be checked with `is` before casting — `as List`/`as Map` on unexpected payloads has crashed the UI in 5+ places before.
- New user-facing strings go through `frontend/lib/core/l10n.dart`.
- **The Flutter UI now defaults to English** (2026-08-13) — an unset `memo_locale` falls through to `MemoLocale.en`; only an explicit `'tr'` selects Turkish, so anyone who already chose a language keeps it. Rationale: first contact is now a browser pointed at a self-hosted box, not a Turkish desktop user. **Rule #8 is unchanged — every new string still needs BOTH a TR and an EN entry.** Widget tests must not hardcode a literal in either language; read the same `L10n.t('key')` the widget does (see `settings_dialog_test.dart` / `permission_dialog_test.dart`), or the next default change breaks them. **Known open seam:** the backend still emits some user-facing strings in Turkish regardless of this setting (see Code Style below) — they don't go through `L10n`, so an English UI still shows Turkish system messages. Closing that means routing them through `Identity.UILanguage`; not done, deliberately scoped out.
- Unexplained plugin build failure? Check `~/.pub-cache` for 0-byte/partial package downloads **before** adding `dependency_overrides` (see 2026-07-04 note above).
- **2026-08-11: jni 1.0.1+ is uncompilable with clang ≥ 16** (`dartjni.h`'s `attach_thread()` dropped the `(void**)` cast; `-Wincompatible-pointer-types` is a hard error in C since clang 16 / GCC 14). It entered the lockfile **collaterally** — commit `ad6f9aa` (an unrelated CI fix) ran `flutter pub get` and silently bumped jni 1.0.0 → 1.0.3, breaking every desktop build (Linux/macOS/Windows; jni is compiled because it declares `linux`/`windows` `ffiPlugin` and is forced into the graph by `path_provider_android 2.3.1`, the current latest). Fixed by pinning `jni: 1.0.0` in `dependency_overrides` (comment in `pubspec.yaml` explains why). No upstream fix exists yet (1.0.1 retracted, 1.0.2/1.0.3 share the regression). **Lesson: when an unrelated commit touches only `pubspec.lock`, diff it for collateral dependency bumps and spot-check the jni version — this class of failure re-enters the repo via plain `pub get`.**

**Types & misc**
- `skill.DangerLevel` and `agent.DangerLevel` are separate named types — they do not cross-assign.
- Turkish + English mixed user-facing text was intentional (target users were Turkish). As of 2026-08-13 the Flutter UI defaults to English — see the l10n bullet under "Flutter" above; the backend's own Turkish strings are now a known seam, not a design goal.
- sqlite-vec extension (`vec0.so`/`vec0.dll`) is bundled under `binaries/` — never add a runtime download for it.

---

## Known Open Work (pointers)

| Item | Where |
|------|-------|

Open bugs and technical debt are tracked in **`BUG_REPORT.md`** — don't duplicate the list here. As of 2026-07-12 (Session 24) it has 0 open items. ~~Onboarding / launchpad UX~~ is done and archived in `docs/plans/plan.md`.

---

## Code Style

- Go backend uses `http.ServeMux` — no external router dependency (gorilla/mux removed).
- Turkish error messages mixed with English across the Go backend. This was intentional when the target users were Turkish; since the Flutter UI defaults to English (2026-08-13) it is a **known open seam** instead — an English UI still surfaces Turkish backend strings (`"⚠️ Yerel model yüklenmemiş..."`, `"⏹️ Cevap durduruldu."`, `"hafıza kaydedildi"`, …). They bypass `L10n` entirely; fixing it means routing them through `Identity.UILanguage`. Not scheduled — don't add new Turkish-only backend strings in the meantime.
- CGO required: `CGO_ENABLED=1 go build/test/run`. `-tags "sqlite_fts5"` required too — see `docs/CGO_FLAGS.md`.
- sqlite-vec extension binary (`vec0.so`/`.vec0.dll`) is bundled under `binaries/` — no runtime download.

---

## Version

**v3.1.2** (open beta, 2026-07-06) (Go 1.26, Flutter 3.10+, flutter_riverpod 2.4, dio 5.4, flutter_markdown 0.6, mattn/go-sqlite3, sqlite-vec)
