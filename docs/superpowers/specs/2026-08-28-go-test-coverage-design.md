# Go Test Coverage — Live Mode Baseline + Gap Closing

**Date:** 2026-08-28
**Branch:** `feature/live-mode-v2`
**Scope:** Test-only work. **No source code changes** (production `.go` files, excluding `*_test.go`, stay untouched). User is concurrently editing `internal/app/livemode_delegate.go`, `internal/app/livemode_session.go`, and `scripts/build_releases.sh` — those are out of scope and **must not** be touched in this session's commits either.

---

## 1. Problem

The Go backend ships ~70k LOC across `internal/**` with ~47k LOC of test code (~67% line-ratio, but this is not a coverage metric). The repo has **never** produced a coverage profile — `coverage/` does not exist, no `coverage.out` is checked in, `go test -cover` is not in any CI step. So the actual statement coverage of even heavily-tested packages is **unknown**, and the project has no objective way to find untested code.

Live Mode v2 is the most architecturally dense, recently-touched code in the repo (Faz 6–12 over the last 6 sessions, 4 commits this session already). It is also the area most likely to harbor regression-prone gaps that we won't notice until a real-session test exposes them. Closing those gaps *before* the next real test session is the goal of this work.

## 2. Goal (single session)

- Establish a Go coverage baseline for `internal/livemode/...` (all three packages: `livemode`, `livemode/google`, `livemode/openai_realtime`).
- Add the tooling and conventions needed to make coverage runs cheap and reproducible (Makefile target, documentation note in AGENTS.md if appropriate).
- Close the **highest-value** coverage gaps in those three packages, prioritized by:
  1. Error paths (hata yolları — hâlihazırda mutlu yol test edilmiş kodun hata kolları)
  2. Concurrency / context-cancellation paths (kaynak sızıntısı potansiyeli)
  3. Edge cases in serialization / encryption (`Save`, `Load`, `encrypt`/`decrypt` hata kolları)

Out of scope for this session: other `internal/**` packages with 0% coverage (e.g. `internal/api`, `internal/cloudsync`, `internal/intent/*`, `internal/proactive/*`, the 9 untested provider clients). The session limit of "1–2 plan items per session" in AGENTS.md rule 5 makes a project-wide coverage push its own multi-session effort, not this one.

## 3. Baseline Measurement (already run, evidence)

```
$ CGO_ENABLED=1 go test -tags "sqlite_fts5" -coverprofile=/tmp/cov_livemode.out \
      -covermode=atomic ./internal/livemode/...
ok  memo/internal/livemode                 0.011s  coverage: 73.2% of statements
ok  memo/internal/livemode/google          0.014s  coverage: 82.8% of statements
ok  memo/internal/livemode/openai_realtime 0.615s  coverage: 79.6% of statements
total: 78.6%
```

Per-function uncovered branches (`go tool cover -func`):

| Package | Function | Line | Coverage | Gap |
|---|---|---|---|---|
| `livemode` | `ConfigManager.Save` | 96 | **0.0%** | Public API; never called directly in tests (only via `Set`→`saveLocked`) |
| `livemode` | `ConfigManager.Load` | 55 | **35.0%** | Only "file missing" path tested; parse error, decrypt error, decode error all untested |
| `livemode` | `ConfigManager.saveLocked` | 102 | 64.7% | `os.MkdirAll` failure, `json.MarshalIndent` failure, `fileutil.AtomicWrite` failure untested |
| `livemode` | `ConfigManager.encrypt` | 186 | 76.9% | `aes.NewCipher`/`cipher.NewGCM`/`io.ReadFull` error branches untested |
| `livemode` | `ConfigManager.decrypt` | 206 | 68.4% | `hex.DecodeString` failure, "ciphertext too short", `aes.NewCipher`/`NewGCM`/`Open` failures untested |
| `livemode` | `EchoSession.SendAudio` | 32 | 80.0% | channel-closed / context-cancelled branch untested |
| `livemode/google` | `Client.Start` | 277 | 80.8% | `websocket.Dial` failure, `writeJSON` failure during setup untested |
| `livemode/google` | `writeJSON` | 359 | 75.0% | context-cancelled write, conn-closed write untested |
| `livemode/google` | `readLoop` | 374 | 73.2% | WS read error handling, context cancellation mid-loop untested |
| `livemode/google` | `emitTranscript` | 482 | 85.7% | one branch (event channel send) untested |
| `livemode/google` | `runToolCall` | 503 | 77.8% | `handleToolCall` returning error branch; `writeJSON` failure after handler untested |
| `livemode/google` | `ListLiveModels` | 51 | 90.6% | `httptest` 4xx/5xx + decode failure untested |
| `livemode/openai_realtime` | `Client.Start` | 224 | 78.3% | Same shape as Google's Start |
| `livemode/openai_realtime` | `writeJSON` | 314 | 75.0% | Same |
| `livemode/openai_realtime` | `readLoop` | 326 | 74.1% | Same |
| `livemode/openai_realtime` | `runToolCall` | 398 | **63.6%** | handler error path + two separate `writeJSON` failure paths untested |
| `livemode/openai_realtime` | `ListRealtimeModels` | 38 | 84.2% | Same shape as Google |

## 4. Approach (selected, with rationale)

**A. Add a Makefile target for one-shot coverage.** A new `make coverage-livemode` (or `scripts/coverage.sh`, see §5) runs the three packages with `-coverprofile` and prints a per-function summary. Cost: ~20 LOC of shell, no Python. Alternative considered: a CI workflow that uploads to Codecov. Rejected because (a) no Codecov integration exists in the repo, (b) CI changes are out of scope for a single session, (c) the user is not asking for CI integration, just for me to *find* gaps and *close* them. The Makefile target gives the user a one-command reproducer for the baseline and lets us track it locally.

**B. Add new test functions, do not modify existing happy-path tests.** Existing tests in `client_test.go`, `models_test.go`, `config_test.go`, `echo_session_test.go` are dense and well-structured; rewriting them risks regressions in tests we already trust. New tests will be added as new `TestXxx` functions in the existing `_test.go` files (or a new file if a clean grouping is helpful). **The 3 WIP files the user is editing stay untouched** — even their `_test.go` pairs, because the user said *"haricinde diğer dosyalara dokunma"*.

**C. Prioritize: error paths first, then concurrency, then serialization edge cases.** The ranking comes from observed session history: Phase 23 alone (top of `handoff.md`) fixed 7 Live-Mode bugs in 1 day — most of them surfacing from real-session tests where an error path silently swallowed the failure. Better error-path coverage means future regressions in those branches fail loudly in CI instead of in production.

**D. Stay narrowly inside `internal/livemode/...`.** A user note explicitly says: *"ilk 1-2 paket"* — picking `livemode` + `google` + `openai_realtime` is the maximal three-package set the user authorized. Other 0%-coverage packages (the rest of `internal/app`, `internal/webserver`, `internal/agent/sandbox.go`, `internal/memory/embedder.go`, all 9 `internal/provider/*.go` clients, etc.) get **mentioned in §7 handoff** as future work but not touched in this session.

## 5. Concrete Plan (this session only)

### 5.1 Tooling: `scripts/coverage-livemode.sh`

A small bash script that runs the three packages with `-coverprofile` and emits a sorted "lowest-coverage functions" summary. Reasoning for a script (not Makefile): the repo already uses `scripts/run_memo.sh` and `scripts/build_releases.sh` for this kind of thing; consistent with project conventions (AGENTS.md references `./scripts/build_releases.sh` as a build entry point).

```sh
#!/usr/bin/env bash
# Run coverage for the Live Mode packages and print the worst-covered
# functions. See docs/superpowers/specs/2026-08-28-go-test-coverage-design.md
# for context.
set -euo pipefail
cd "$(dirname "$0")/.."
PROFILE=$(mktemp /tmp/cov_livemode.XXXXXX.out)
trap 'rm -f "$PROFILE"' EXIT
CGO_ENABLED=1 go test -tags "sqlite_fts5" \
    -coverprofile="$PROFILE" -covermode=atomic \
    ./internal/livemode/... 2>&1
echo
echo "=== Worst-covered functions (< 100%) ==="
go tool cover -func="$PROFILE" \
  | awk 'NR>1 && $NF != "100.0%" { printf "%-12s %s\n", $NF, $1 }' \
  | sort -n
```

Usage: `bash scripts/coverage-livemode.sh`. Reproduces the §3 baseline.

### 5.2 Test additions — `internal/livemode/config_test.go`

Append (do not modify existing tests):

| New test | What it covers |
|---|---|
| `TestConfigManager_Save_PersistsEncryptedToFile` | `Save()` happy path: writes JSON, `APIKey` does not appear in plaintext, decrypt roundtrips. *This is the 0% line.* |
| `TestConfigManager_Load_FileExistsButMalformedJSON` | `Load` error branch: write garbage JSON, expect logged error + empty configs. |
| `TestConfigManager_Load_EntryWithUndecryptableAPIKey` | Write a config with an undecryptable `api_key_encrypted` (not hex, or truncated). Expect: the engine's `APIKey` is empty, others load fine. |
| `TestConfigManager_Encrypt_Decrypt_RoundTripForEmptyString` | Edge: `encrypt("")` → `decrypt("")` → `""`. |
| `TestConfigManager_Decrypt_RejectsCiphertextTooShort` | `decrypt` branch `if len(ciphertext) < nonceSize`. |
| `TestConfigManager_Decrypt_RejectsTamperedCiphertext` | Tamper one byte → `aesGCM.Open` returns error. |
| `TestConfigManager_Save_DirectCallDoesNotRequireSet` | A *direct* `Save()` with no prior `Set` writes an empty `{"engines": []}` file (currently 0% — only reached via `Set`→`saveLocked` in existing tests). |

### 5.3 Test additions — `internal/livemode/echo_session_test.go`

| New test | What it covers |
|---|---|
| `TestEchoSession_SendAudioAfterCloseReturnsError` | `SendAudio` 80% → 100%: the channel-closed branch (`select { case ... <- : case <-ctx.Done() }`). |
| `TestEchoSession_ContextCancelDuringStartDrainsEvents` | `Start` with a context cancelled *during* the receive: events channel closes cleanly. |

### 5.4 Test additions — `internal/livemode/google/client_test.go`

| New test | What it covers |
|---|---|
| `TestClient_Start_FailsWhenDialFails` | Point `WSBaseURL` at a `httptest` server that closes the conn immediately. Expect `Start` returns wrapped error. |
| `TestClient_Start_FailsWhenSetupWriteFails` | Server accepts WS, but closes the conn *before* the client writes `setup`. Expect `Start` returns the write error. |
| `TestClient_ToolCall_HandlerErrorPropagatesAsToolError` | `handleToolCall` returns `(result="", err=someErr)`. Expect the `toolResponse` event carries `"Error: someErr"`. |
| `TestClient_Close_Idempotent` | Calling `Close` twice does not panic. |
| `TestClient_ReadLoop_ContextCancelClosesConnCleanly` | Cancel ctx mid-stream → read loop returns, no goroutine leak. Use a `time.After` watchdog. |
| `TestListLiveModels_HTTP4xxReturnsError` | `httptest` returning 401 → `ListLiveModels` returns error with status code. |
| `TestListLiveModels_DecodeFailureReturnsError` | Server returns malformed JSON → `ListLiveModels` returns decode error. |

### 5.5 Test additions — `internal/livemode/openai_realtime/client_test.go`

Same shape as §5.4 (the two client implementations mirror each other):

| New test |
|---|
| `TestClient_Start_FailsWhenDialFails` |
| `TestClient_Start_FailsWhenSessionUpdateWriteFails` |
| `TestClient_ToolCall_HandlerErrorPropagatesAsToolError` |
| `TestClient_ToolCall_ResponseCreateWriteFailsEmitsErrorEvent` |
| `TestClient_Close_Idempotent` |
| `TestClient_ReadLoop_ContextCancelClosesConnCleanly` |
| `TestListRealtimeModels_HTTP4xxReturnsError` |
| `TestListRealtimeModels_DecodeFailureReturnsError` |

## 6. Risks & Limits

- **Risk: `Save` has one error branch (`encrypt` failure mid-loop) that is hard to test without production-code changes.** The `ConfigManager` constructor always builds a 32-byte master key (`make([]byte, 32); copy(key, masterKey)`), so `aes.NewCipher` and `cipher.NewGCM` cannot be made to fail in test code. The `io.ReadFull(rand.Reader, nonce)` failure branch is also unreachable in practice. **Resolution:** skip this branch in v1; mark as future work. The `Save` happy-path test (§5.2 first row) and the other `Load`/`encrypt`/`decrypt` tests still push `Save`/`saveLocked` coverage up significantly. We accept ~85% on `saveLocked` as a stop, not 100%.

- **Naming precision:** the Google client sends a `setup` message; the OpenAI client sends a `session.update`. The two corresponding tests in §5.4 / §5.5 are deliberately named after their respective messages (`TestClient_Start_FailsWhenSetupWriteFails` for Google, `TestClient_Start_FailsWhenSessionUpdateWriteFails` for OpenAI) — they are **not** the same test.

- **Risk: user is concurrently editing `internal/app/livemode_*.go`.** This spec's scope is strictly `internal/livemode/...` (the three subpackages), so no collision. The user's WIP files are noted in `handoff.md` but not part of this design.

- **Limit: not a project-wide coverage push.** `internal/api`, the 9 untested `internal/provider/*.go` clients, `internal/cloudsync`, `internal/intent/*`, `internal/proactive/*`, `internal/observer/recorder.go`, `internal/skill/types.go`, `internal/calendar/*` — all 0%-coverage, all *out of scope* this session. §7 handoff will enumerate them so the next session can pick up.

- **Limit: no coverage gate in CI.** This work adds the *ability* to measure coverage, not a CI check that fails builds below a threshold. Adding a CI step is its own change (workflow YAML + threshold definition) and out of scope.

- **Limit: not a "100% coverage" push on this session.** Closing the *biggest* gaps (the 0% and <70% entries) is the goal. Getting to 95%+ on the three packages is multi-session work and not what was asked for. Stop condition: every function in §3 table at ≥85%, with documented exceptions (the `Save`→`encrypt-failure` test, which we accept as "untested branch" and note in handoff).

## 7. Verification (per session end)

```
bash scripts/coverage-livemode.sh
```

Expected output (approximate; not a contract):

```
ok  memo/internal/livemode                 0.011s  coverage: 88-92% of statements
ok  memo/internal/livemode/google          0.014s  coverage: 90-94% of statements
ok  memo/internal/livemode/openai_realtime 0.615s  coverage: 88-92% of statements
total: ~90%
```

Plus the standard verification block from AGENTS.md rule 3:

```
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...                    # must stay green
CGO_ENABLED=1 go vet   -tags "sqlite_fts5" ./...                    # must stay green
CGO_ENABLED=1 go test  -tags "sqlite_fts5" ./internal/livemode/... -race  # all new tests pass
```

(`-race` only on the touched packages to keep iteration fast; full `-race ./...` is part of handoff verification but not required per-commit.)

## 8. Handoff / Future Work (next session, if any)

| Item | Notes |
|---|---|
| `internal/livemode/session.go`, `livemode/models.go` | Pure interface / type defs — gofmt-clean, no executable code. Coverage tool reports them as 100% on test execution. No tests needed. |
| `internal/api/{client,streaming,types}.go` | 0% coverage. Touches everything that makes an LLM call. High-value next target. |
| `internal/provider/{provider,grok,groq,kilo,llamacpp,ollama,opencode_go,opencode_zen,openrouter}.go` | 9 untested provider clients. Each is a single file, often <200 LOC. Quick coverage wins. |
| `internal/cloudsync/drive.go` | E2E encryption for Google Drive backups. Critical, completely untested. |
| `internal/intent/*` (5 files), `internal/proactive/*` (6 files) | Small, isolated, 0% — easy baseline-improvement session. |
| `internal/observer/recorder.go` | 0%. |
| `internal/agent/sandbox.go` | 0%. Tool execution security boundary — should be tested before any further tool-system work. |
| `internal/memory/embedder.go` | 0%. Embedding server client; testing requires `httptest`. |
| Flutter coverage | Separate session; `flutter test --coverage` is the entry point. No coverage data exists. |
| CI coverage gate | Add to one of the workflow files once a baseline is established. |
