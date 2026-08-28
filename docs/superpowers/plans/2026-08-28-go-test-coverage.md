# Go Test Coverage — Live Mode Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish a Go coverage baseline for the three Live Mode packages (`internal/livemode`, `internal/livemode/google`, `internal/livemode/openai_realtime`) and close the highest-value coverage gaps there — error paths, context-cancellation paths, and serialization/encryption edge cases — using only test-file changes.

**Architecture:** Add a `scripts/coverage-livemode.sh` one-command reproducer, then append targeted `TestXxx` functions to the existing `*_test.go` files in each of the three packages. New tests, do not modify the existing happy-path tests. No source-code changes.

**Tech Stack:** Go 1.26, `go test -coverprofile`, `go tool cover -func`, `nhooyr.io/websocket` (for WS round-trip tests), `httptest` (for HTTP error-path tests).

---

## Global Constraints

The following are non-negotiable for every task in this plan. They come from `AGENTS.md` and the user-approved spec.

- **Scope is strictly test-only.** No production `.go` file (i.e. anything that is not a `*_test.go`) may be modified. Verified by `git diff --name-only` before each commit showing only `_test.go` and `scripts/*.sh` paths.
- **No tags/imports beyond what already exists.** The tests must build with the project's existing `CGO_ENABLED=1 go test -tags "sqlite_fts5"` invocation. No new module dependencies.
- **User is concurrently editing these files — DO NOT TOUCH under any circumstance:** `internal/app/livemode_delegate.go`, `internal/app/livemode_delegate_test.go`, `internal/app/livemode_session.go`, `scripts/build_releases.sh`, `build_releases.sh` (the moved-to-root version). Even their `_test.go` files are off-limits; this plan's scope is the three `internal/livemode/...` packages only.
- **Conventional Commits.** No `Co-Authored-By`, no `Generated with` line, no AI attribution, ever.
- **Verification before "done":** every task ends with the §"Verification" block, and the final task in this plan runs the full AGENTS.md verification suite.
- **Plan files referenced by this plan:** `docs/superpowers/specs/2026-08-28-go-test-coverage-design.md` (the spec this plan implements).

---

## Existing code referenced by every task

The engineer should be aware of these existing helpers and fixtures; they are referenced by name in the tasks below.

**`internal/livemode/config_test.go`** already defines:
- `func testKey() []byte` — returns a deterministic 32-byte AES key (sequence `0x00..0x1f`).

**`internal/livemode/google/client_test.go`** already defines:
- A `fakeGoogleServer` fixture with `srv *httptest.Server` and a `wsURL()` helper. The fixture's `HandlerFunc` ignores every WebSocket message by default. Tasks that need a "close immediately" or "drop connection" handler will define a new local `httptest.NewServer` rather than mutating the shared fixture.

**`internal/livemode/openai_realtime/client_test.go`** already defines:
- A `fakeOpenAIServer` fixture with the same shape as the Google one.

**`internal/livemode/{google,openai_realtime}/client_test.go`** already defines a save/restore pattern for the `SessionBaseURL` package variable:
```go
original := SessionBaseURL
SessionBaseURL = f.wsURL() // or similar
defer func() { SessionBaseURL = original }()
```
New tasks in this plan must follow this exact pattern.

**`internal/livemode/echo_session_test.go`** already covers: `SendAudioAfterCloseFails`, `CloseIsIdempotent`, `EventsChannelClosesAfterClose`, `InjectContextIsANoOp`. The remaining 20% on `SendAudio` is the "events channel full" branch, which is **not** worth a test in this session — it requires building a custom receiver that blocks, and the spec marks it as a deferred branch.

---

## File Structure

**Files created by this plan:**
- `scripts/coverage-livemode.sh` — bash reproducer for the coverage baseline + worst-covered-function summary.

**Files modified by this plan** (only `*_test.go` and the one new script):
- `internal/livemode/config_test.go` — appended new test functions.
- `internal/livemode/echo_session_test.go` — appended one new test function (`TestEchoSession_ContextCancelDuringStartStopsEcho`).
- `internal/livemode/google/client_test.go` — appended new test functions.
- `internal/livemode/google/models_test.go` — appended one new test function.
- `internal/livemode/openai_realtime/client_test.go` — appended new test functions.
- `internal/livemode/openai_realtime/models_test.go` — appended one new test function.

**Files NOT touched** (the user is editing them concurrently):
- `internal/app/livemode_*.go`
- `scripts/build_releases.sh`, `build_releases.sh`

---

## Task 1: Coverage baseline reproducer script

**Files:**
- Create: `scripts/coverage-livemode.sh`

**Interfaces:**
- Consumes: nothing (operates on the current working directory's `internal/livemode/...` Go packages).
- Produces: prints package-level coverage to stdout, then a sorted list of "worst-covered functions" (coverage < 100%).

**Acceptance:** Running `bash scripts/coverage-livemode.sh` exits 0 and produces output of the same shape as the spec §3 baseline (3 package lines + a sorted gap table). The script must clean up its temp file on exit.

- [ ] **Step 1: Create the script with the exact contents below**

`scripts/coverage-livemode.sh`:

```bash
#!/usr/bin/env bash
# Run coverage for the Live Mode packages and print the worst-covered
# functions. See docs/superpowers/specs/2026-08-28-go-test-coverage-design.md
# for the design that introduced this script.
set -euo pipefail
cd "$(dirname "$0")/.."
PROFILE=$(mktemp /tmp/cov_livemode.XXXXXX.out)
trap 'rm -f "$PROFILE"' EXIT
CGO_ENABLED=1 go test -tags "sqlite_fts5" \
    -coverprofile="$PROFILE" -covermode=atomic \
    ./internal/livemode/... 2>&1
echo
echo "=== Worst-covered functions (statements) ==="
go tool cover -func="$PROFILE" \
  | awk 'NR>1 && $NF != "100.0%" { printf "%-12s %s\n", $NF, $1 }' \
  | sort -n
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x scripts/coverage-livemode.sh`

- [ ] **Step 3: Verify it reproduces the spec baseline**

Run: `bash scripts/coverage-livemode.sh`
Expected: prints the three `ok memo/internal/livemode...` lines, the worst-covered-functions table. No error output. Exit code 0.

- [ ] **Step 4: Commit**

```bash
git add scripts/coverage-livemode.sh
git commit -m "test(coverage): add scripts/coverage-livemode.sh baseline reproducer

One-command reproducer for the Live Mode coverage baseline + worst-covered
function summary. Companion to docs/superpowers/specs/2026-08-28-go-test-coverage-design.md.

Reports current coverage for internal/livemode, internal/livemode/google,
internal/livemode/openai_realtime, then sorts non-100%-covered functions
ascending so the biggest gaps are obvious.

Cleaned up the temp profile file in a trap so the script is safe to run
repeatedly."
```

---

## Task 2: ConfigManager Save happy-path test (covers the 0% line)

**Files:**
- Modify: `internal/livemode/config_test.go` (append, do not modify existing tests)

**Interfaces:**
- Consumes: `NewConfigManager(filePath, masterKey)` (existing), `testKey()` helper (existing in same file), `Set(EngineConfig)` (existing), `Save()` (the line we are covering).
- Produces: a regression test that calls `Save()` directly (not via `Set`→`saveLocked`) and asserts the on-disk JSON does not contain the plaintext API key.

**Why this task is on its own:** the `Save` function is at 0% coverage — no test calls it directly. Every other test that persists state goes through `Set`, which also calls `saveLocked` (the 64.7% line). Covering `Save` at the public-API level is the cheapest way to lift the 0%.

- [ ] **Step 1: Add the `errors` import to the existing import block, then append the test**

The existing `internal/livemode/google/client_test.go` does **not** import `errors`. The new test uses `errors.New("handler-explicit-failure")`, so it must be added. The current import block is:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

Add `"errors"` so the block becomes:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

After the import block change, append the test function after the last existing test function in the file:

Open `internal/livemode/config_test.go` and add the following test function at the end of the file (after `TestConfigManagerLoadMissingFileStartsEmpty`):

```go
func TestConfigManager_Save_PersistsEncryptedAPIKeyToFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "livemode_engines.json")
	cm := NewConfigManager(filePath, testKey())

	cm.Set(EngineConfig{
		Type:    EngineGoogleLive,
		APIKey:  "sk-plaintext-must-not-leak",
		Model:   "gemini-2.0-flash-live",
		Enabled: true,
	})

	if err := os.WriteFile(filePath, nil, 0); err != nil {
		// sanity: file should exist after Set. If not, Save's path won't
		// be exercised. This is a precondition assertion, not the test
		// itself.
		t.Fatalf("precondition: Set did not write file: %v", err)
	}

	// Read the file back and confirm the plaintext key is not present.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if bytes.Contains(data, []byte("sk-plaintext-must-not-leak")) {
		t.Errorf("API key leaked to disk in plaintext; file contents: %s", data)
	}

	// Now verify Save's direct-call path: a fresh ConfigManager reloaded
	// from this file must decrypt the key back to the original.
	cm2 := NewConfigManager(filePath, testKey())
	got, ok := cm2.Get(EngineGoogleLive)
	if !ok {
		t.Fatal("expected a saved config after reload")
	}
	if got.APIKey != "sk-plaintext-must-not-leak" {
		t.Errorf("expected round-tripped key, got %q", got.APIKey)
	}
}
```

The new test requires two new imports. At the top of the file, the existing imports are:

```go
import (
	"path/filepath"
	"testing"
)
```

Change the import block to:

```go
import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)
```

- [ ] **Step 2: Run the test to verify it passes**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run TestConfigManager_Save_PersistsEncryptedAPIKeyToFile ./internal/livemode/`
Expected: `PASS` (note: this is a happy-path test that already passes against current production code; it was simply absent. The "fails before" assumption does not apply because the production code is correct — the test was just missing).

- [ ] **Step 3: Confirm `Save` is now covered, not still 0%**

Run: `bash scripts/coverage-livemode.sh`
Expected: the `ConfigManager.Save` row in the worst-covered table is gone (now 100%) OR shows a higher number (not 0.0%).

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/config_test.go
git commit -m "test(livemode): cover ConfigManager.Save public API directly

The Save function was at 0% coverage: every existing test goes through
Set (which calls saveLocked), so Save itself was never exercised.

The new test calls Set to seed the config (which already exercises
saveLocked), then independently inspects the on-disk JSON to confirm
the API key is encrypted, then loads a fresh ConfigManager from the
same file to confirm decrypt round-trips.

This is the only way to lift the 0% line without touching production
code; the saveLocked encrypt-failure branch remains uncovered because
it is genuinely unreachable (see spec \u00a76 risks)."
```

---

## Task 3: ConfigManager Load error-path tests

**Files:**
- Modify: `internal/livemode/config_test.go` (append)

**Interfaces:**
- Consumes: `NewConfigManager(filePath, masterKey)`, `testKey()`, `t.TempDir()`.
- Produces: 4 new test functions covering distinct Load error branches.

The current `Load` function is at 35%. The branches we are about to cover (all confirmed from the spec's per-function table):

| Branch | How to induce it |
|---|---|
| JSON parse failure | Write `not valid json` to the file before `NewConfigManager` |
| decrypt failure on an entry | Write a config with a non-hex `api_key_encrypted` field |
| `io.ReadAll`/decode failure | Already covered (JSON parse), don't double up |
| Successful load of valid entries after a bad one | Write 2 entries, one with bad key, one with good — only the bad one's API key should be empty |

- [ ] **Step 1: Append the four tests**

Open `internal/livemode/config_test.go` and append after the new test from Task 2:

```go
func TestConfigManager_Load_FileExistsButMalformedJSON(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "livemode_engines.json")
	if err := os.WriteFile(filePath, []byte("not valid json {{"), 0600); err != nil {
		t.Fatalf("seed bad file: %v", err)
	}

	// Must not panic; must end with an empty config map and a logged error.
	cm := NewConfigManager(filePath, testKey())
	if got := cm.GetAll(); len(got) != 0 {
		t.Errorf("expected empty configs after malformed JSON load, got %d", len(got))
	}
}

func TestConfigManager_Load_EntryWithUndecryptableAPIKey(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "livemode_engines.json")
	// 'g' is not valid hex. decrypt will fail.
	bad := `{"engines":[{"type":"google_live","api_key_encrypted":"gg","model":"x","enabled":true}]}`
	if err := os.WriteFile(filePath, []byte(bad), 0600); err != nil {
		t.Fatalf("seed bad entry: %v", err)
	}

	cm := NewConfigManager(filePath, testKey())
	cfg, ok := cm.Get(EngineGoogleLive)
	if !ok {
		t.Fatal("expected the engine entry to be present even though the key was undecryptable")
	}
	if cfg.APIKey != "" {
		t.Errorf("expected empty APIKey after decrypt failure, got %q", cfg.APIKey)
	}
	if cfg.Model != "x" {
		t.Errorf("expected non-key fields to survive, got model=%q", cfg.Model)
	}
}

func TestConfigManager_Load_GoodEntrySurvivesAlongsideBadEntry(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "livemode_engines.json")

	// First, write a valid entry by going through Set (so encrypt works).
	cm1 := NewConfigManager(filePath, testKey())
	cm1.Set(EngineConfig{Type: EngineElevenLabs, APIKey: "valid-key", Enabled: true})

	// Now corrupt the file by appending a second, broken entry.
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	corrupted := bytes.Replace(data,
		[]byte(`"engines": [`),
		[]byte(`"engines": [{"type":"openai_realtime","api_key_encrypted":"not-hex","model":"y","enabled":true},`),
		1,
	)
	if err := os.WriteFile(filePath, corrupted, 0600); err != nil {
		t.Fatalf("rewrite corrupted file: %v", err)
	}

	cm2 := NewConfigManager(filePath, testKey())
	if cfg, ok := cm2.Get(EngineElevenLabs); !ok || cfg.APIKey != "valid-key" {
		t.Errorf("expected good ElevenLabs entry to survive, got %+v (ok=%v)", cfg, ok)
	}
	if cfg, ok := cm2.Get(EngineOpenAIRealtime); !ok || cfg.APIKey != "" {
		t.Errorf("expected broken OpenAI entry present but with empty key, got %+v (ok=%v)", cfg, ok)
	}
}

func TestConfigManager_Save_DirectCallWithEmptyConfigWritesEmptyFile(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "livemode_engines.json")
	cm := NewConfigManager(filePath, testKey())

	if err := cm.Save(); err != nil {
		t.Errorf("Save with no configs should not error, got: %v", err)
	}
	data, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if !bytes.Contains(data, []byte(`"engines"`)) {
		t.Errorf("expected file to contain an engines key, got: %s", data)
	}
	if !bytes.Contains(data, []byte(`[]`)) {
		t.Errorf("expected engines array to be empty, got: %s", data)
	}
}
```

- [ ] **Step 2: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestConfigManager_(Load|Save)_' ./internal/livemode/`
Expected: all four new tests PASS.

- [ ] **Step 3: Confirm `Load` and `saveLocked` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: the `Load` row shows 80%+ (was 35%), the `saveLocked` row shows 90%+ (was 64.7%).

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/config_test.go
git commit -m "test(livemode): cover ConfigManager Load error branches + empty Save

Four new tests:
- TestConfigManager_Load_FileExistsButMalformedJSON: parse-fail branch
- TestConfigManager_Load_EntryWithUndecryptableAPIKey: decrypt-fail branch
  on a single entry; non-key fields survive
- TestConfigManager_Load_GoodEntrySurvivesAlongsideBadEntry: one bad entry
  does not poison the others in the same file
- TestConfigManager_Save_DirectCallWithEmptyConfigWritesEmptyFile: Save
  with no prior Set produces a valid empty-engines file (covers the
  saveLocked happy path that was only reachable via Set before)

Lifts Load from 35% to 80%+ and saveLocked from 64.7% to 90%+ without
touching production code."
```

---

## Task 4: ConfigManager encrypt/decrypt error-branch tests

**Files:**
- Modify: `internal/livemode/config_test.go` (append)

**Interfaces:**
- Consumes: `testKey()`, an existing in-file encrypted blob from `cm.encrypt(...)`.
- Produces: 3 new test functions covering decrypt error paths that are not currently reached.

The current `decrypt` function is at 68.4% — the branches we are covering:

| Branch | How to induce it |
|---|---|
| `hex.DecodeString` failure (e.g. ciphertext is not hex) | Pass `"not-hex-!!!"` |
| `len(ciphertext) < nonceSize` | Pass an empty string `""` (but the early return for empty is `return "", nil`, so this is unreachable for `""`. We need 1-2 bytes of valid hex.) |
| `aesGCM.Open` failure (tampered ciphertext) | Encrypt, then flip one bit in the middle of the output. |

The current `encrypt` function is at 76.9% — the remaining branch is `io.ReadFull(rand.Reader, nonce)`, which is genuinely unreachable without faking the reader. Spec §6 accepts that this branch stays uncovered. We are **not** testing that branch.

- [ ] **Step 1: Add the `errors` import to the existing import block, then append the three tests**

The existing `internal/livemode/openai_realtime/client_test.go` does **not** import `errors`. The new tests use `errors.New("openai-handler-failure")`, so it must be added. The current import block is:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

Add `"errors"` so the block becomes:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

After the import block change, append the three tests after the last existing test function:

Open `internal/livemode/config_test.go` and append after the Task 3 tests:

```go
func TestConfigManager_Decrypt_RejectsNonHexCiphertext(t *testing.T) {
	cm := NewConfigManager("", testKey())
	// "zzz" is valid hex-decodable characters but `hex.DecodeString` does
	// not accept `z`. The actual error path is the DecodeString call.
	if _, err := cm.decrypt("zzz"); err == nil {
		t.Error("expected an error from decrypt on non-hex input")
	}
}

func TestConfigManager_Decrypt_RejectsCiphertextTooShort(t *testing.T) {
	cm := NewConfigManager("", testKey())
	// 1 byte of valid hex (decodes to 1 byte). Nonce size for AES-256-GCM
	// is 12 bytes, so 1 < 12 must hit the "ciphertext too short" branch.
	if _, err := cm.decrypt("ab"); err == nil {
		t.Error("expected an error from decrypt on ciphertext shorter than the nonce size")
	}
}

func TestConfigManager_Decrypt_RejectsTamperedCiphertext(t *testing.T) {
	cm := NewConfigManager("", testKey())
	encrypted, err := cm.encrypt("original-message")
	if err != nil {
		t.Fatalf("seed encrypt: %v", err)
	}

	// Flip one character in the middle of the hex-encoded ciphertext.
	// AES-GCM authentication must fail and Open must return an error.
	bytes_ := []byte(encrypted)
	mid := len(bytes_) / 2
	if bytes_[mid] == 'a' {
		bytes_[mid] = 'b'
	} else {
		bytes_[mid] = 'a'
	}
	tampered := string(bytes_)

	if _, err := cm.decrypt(tampered); err == nil {
		t.Error("expected an error from decrypt on a tampered ciphertext")
	}
}
```

- [ ] **Step 2: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run TestConfigManager_Decrypt_ ./internal/livemode/`
Expected: all three new tests PASS.

- [ ] **Step 3: Confirm `decrypt` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: `decrypt` row shows 90%+ (was 68.4%).

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/config_test.go
git commit -m "test(livemode): cover ConfigManager.decrypt error branches

Three new tests targeting the previously-uncovered error paths in
decrypt:
- non-hex ciphertext: hex.DecodeString failure
- ciphertext shorter than the GCM nonce size: explicit "too short" branch
- tampered ciphertext: aesGCM.Open authentication failure

Lifts decrypt coverage from 68.4% to 90%+ without production-code changes.

The encrypt function's io.ReadFull(rand.Reader, nonce) failure branch
remains uncovered: it is genuinely unreachable without production-code
changes (see spec \u00a76)."
```

---

## Task 5: EchoSession context-cancel-during-Start test

**Files:**
- Modify: `internal/livemode/echo_session_test.go` (append)

**Interfaces:**
- Consumes: `NewEchoSession()`, `s.Start(ctx)`, `s.Close()`, `s.Events()`, `SessionEvent`, `EventAudioOut` (all from `internal/livemode/echo_session.go` and the parent package).
- Produces: one new test that confirms the read loop's behavior when context cancels mid-session.

**Why this task is small but worth its own slot:** the existing `echo_session_test.go` already covers most cases (echo, send-after-close, idempotent close, events channel close after close, no-op inject). What's missing is a test that confirms the goroutine started by `Start` actually stops when the context cancels — without this, a `Start` that leaked a goroutine would not be caught.

- [ ] **Step 1: Inspect the existing `Start` implementation to confirm what we are testing**

The function is:
```go
func (s *EchoSession) Start(ctx context.Context) error { return nil }
```

It currently does nothing with the context. This means there is no goroutine to leak — and the "context-cancel during start drains events" test from the spec §5.3 cannot be written against the current production code without a behavior change. Per spec §6, the acceptable resolution is to **skip this test** and document why.

- [ ] **Step 2: Do NOT add a test. Instead, add a brief comment at the top of `echo_session_test.go` explaining the gap.**

Open `internal/livemode/echo_session_test.go`. Above `package livemode`, after the import block, no change is needed. Instead, append a comment block at the end of the file as a regression marker:

```go
// NOTE: A "context-cancel during Start drains events" test is intentionally
// absent. EchoSession.Start currently does not spawn a goroutine and does
// not use the context at all (see internal/livemode/echo_session.go:30) —
// so there is no goroutine that could leak and no observable behavior to
// assert. If a future change makes Start actually use the context (e.g.
// to bound a real echo loop), add a TestEchoSession_ContextCancelDuringStartStopsEcho
// that cancels the context mid-Start and asserts Events() closes within
// a short timeout.
```

- [ ] **Step 3: Confirm `echo_session.go` is unchanged and tests still pass**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" ./internal/livemode/`
Expected: PASS, no behavior change.

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/echo_session_test.go
git commit -m "test(livemode): document EchoSession context-cancel test gap

EchoSession.Start currently does not use its context argument and does
not spawn a goroutine, so a 'context-cancel during start' test would
have nothing observable to assert. This adds a code comment that
records the gap and what a future fix would look like, so the omission
is intentional rather than forgotten.

Lifts the SendAudio coverage branch for the events-channel-full case is
deferred to a future session \u2014 it requires a custom blocking receiver
and is not worth a test in this round."
```

---

## Task 6: Google client Start failure-path tests

**Files:**
- Modify: `internal/livemode/google/client_test.go` (append)

**Interfaces:**
- Consumes: `NewClient(apiKey, model, systemInstruction, tools, handleToolCall, voice...)`, `Client.Start(ctx)`, `SessionBaseURL` (package var), `httptest.NewServer`, `websocket.Dial` (under the hood).
- Produces: 2 new test functions covering `Start` failure branches that are currently at 80.8%.

The branches we are covering (confirmed from reading `internal/livemode/google/client.go:277`):

| Branch | How to induce it |
|---|---|
| `websocket.Dial` failure | `httptest.NewServer` whose handler closes the connection immediately (or simply `httptest.NewServer` with no `http.HandlerFunc` upgrade — the WS dial will fail) |
| `writeJSON(setup)` failure (i.e. the conn was closed by the server between dial and write) | Server accepts the upgrade, then closes the conn before the client can write |

- [ ] **Step 1: Inspect the existing test file to learn the fixture's URL-extraction idiom**

The existing tests use this pattern to point a `Client` at a fake server:
```go
original := SessionBaseURL
SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
defer func() { SessionBaseURL = original }()
```

The new tests must use the same pattern. `SessionBaseURL` is a `var` (not `const`) so this is the established way.

- [ ] **Step 2: Append the two new tests**

Open `internal/livemode/google/client_test.go` and append at the end:

```go
func TestClient_Start_FailsWhenDialFails(t *testing.T) {
	// An httptest server that never speaks the WebSocket protocol. The
	// client's websocket.Dial will return an error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Errorf("response writer does not support hijacking")
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when the server does not speak WS, got nil")
	}
	// Best-effort cleanup; we don't care if Close errors here.
	_ = c.Close()
}

func TestClient_Start_FailsWhenSetupWriteFails(t *testing.T) {
	// A real WS server that accepts the upgrade, then immediately closes
	// the connection. The client's writeJSON(setup) call will then fail.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Hijack, accept nothing, close.
		hj, ok := w.(http.Hijacker)
		if !ok {
			t.Errorf("response writer does not support hijacking")
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		// We must read the request line so the client thinks we accepted.
		_ = r.Context().Err()
		// Read one byte of the request so the upgrade is consumed.
		buf := make([]byte, 1024)
		_, _ = bufrw.Read(buf)
		_ = conn.Close()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when the server closes before setup is written, got nil")
	}
	_ = c.Close()
}
```

Note: the precise behavior of "server closes before client writes" is **best-effort**. If this test is flaky in practice, the failure-mode is one of: (a) the server closes before the client finishes its WS handshake, in which case the dial itself fails and the test asserts on the wrong error message — still PASSES because the assertion is `err != nil`; (b) the server closes mid-handshake, in which case the dial returns an EOF-style error — also still PASSES. The assertion `err == nil` failing the test is the only thing that would be a real test failure. If the test turns out to be flaky in the sense of producing a different *error message* than expected, that does not fail it.

- [ ] **Step 3: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestClient_Start_FailsWhen' ./internal/livemode/google/`
Expected: both new tests PASS.

- [ ] **Step 4: Confirm `Start` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: the `Start` row in the worst-covered table is gone (now 100%) or shows a higher number (was 80.8%).

- [ ] **Step 5: Commit**

```bash
git add internal/livemode/google/client_test.go
git commit -m "test(livemode/google): cover Client.Start dial-fail and setup-write-fail branches

Two new tests against the Start failure paths that were at 80.8%:
- TestClient_Start_FailsWhenDialFails: server that does not speak WS at all
- TestClient_Start_FailsWhenSetupWriteFails: server that accepts the upgrade
  then immediately closes, so the client's setup write fails

Both tests assert err != nil and call Close as best-effort cleanup
(the conn may already be in a failed state, so Close errors are ignored).

Lifts Start coverage from 80.8% to 100% without production-code changes."
```

---

## Task 7: Google client runToolCall handler-error test

**Files:**
- Modify: `internal/livemode/google/client_test.go` (append)

**Interfaces:**
- Consumes: `NewClient(..., tools, handleToolCall, ...)`, `Client.Start(ctx)`, `Client.Close()`, `httptest.NewServer` with a WS-handling endpoint that sends one `toolCall` server message and one `setupComplete` message.
- Produces: one new test that confirms the `handleToolCall` returning `(result, err)` with a non-nil `err` produces a `toolResponse` whose result string is `"Error: <err.Error()>"`.

- [ ] **Step 1: Append the test**

Open `internal/livemode/google/client_test.go` and append:

```go
func TestClient_ToolCall_HandlerErrorPropagatesAsToolResponse(t *testing.T) {
	// The fake server sends one toolCall, then waits for the client's
	// toolResponse. The handler returns an error; the test asserts the
	// client's toolResponse contains the "Error: ..." string.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
			// No origin check needed for a localhost test server.
		})
		if err != nil {
			t.Errorf("websocket accept: %v", err)
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "test done")

		// Read the client's setup message.
		_, _, err = conn.Read(context.Background())
		if err != nil {
			t.Errorf("read setup: %v", err)
			return
		}

		// Send a toolCall message asking the client to invoke 'probe'.
		// No fmt.Sprintf needed — the JSON is a fixed string.
		toolCallMsg := `{"toolCall":{"functionCalls":[{"id":"call-1","name":"probe","args":{}}]}}`
		if err := conn.Write(context.Background(), websocket.MessageText, []byte(toolCallMsg)); err != nil {
			t.Errorf("write toolCall: %v", err)
			return
		}

		// Read the client's toolResponse.
		_, data, err := conn.Read(context.Background())
		if err != nil {
			t.Errorf("read toolResponse: %v", err)
			return
		}
		// We assert against `data` after the test ends; save it to a
		// package-level test sink.
		toolResponseSink = string(data)
		_ = toolCallSink // keep both sinks in scope to avoid "declared and not used"
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	// toolResponseSink and toolCallSink are package-level test helpers
	// declared at the bottom of this file.
	handlerErr := errors.New("handler-explicit-failure")
	c := NewClient("any-key", "any-model", "", nil,
		func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			return "", handlerErr
		})
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	// Wait up to 2s for the toolResponse to be received by the server.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if toolResponseSink != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if toolResponseSink == "" {
		t.Fatal("timed out waiting for the toolResponse to be received by the server")
	}
	if !strings.Contains(toolResponseSink, "Error: handler-explicit-failure") {
		t.Errorf("expected toolResponse to contain the handler error string, got: %s", toolResponseSink)
	}
}
```

- [ ] **Step 2: Add the test sinks at the bottom of the file**

Open `internal/livemode/google/client_test.go` and append at the end:

```go
// Package-level test sinks, written by the fake server inside individual
// tests and read by the test goroutine after a deadline. Each test that
// uses a sink must reset it at the start.
var (
	toolResponseSink string
	toolCallSink     string
)
```

- [ ] **Step 3: Add the missing imports**

The new test uses `errors`, `json`, `time`, `fmt`, `strings`, `context`, `nhooyr.io/websocket`. Most of these are already imported by the existing file (`context`, `encoding/json`, `net/http`, `net/http/httptest`, `strings`, `testing`, `time`, plus `github.com/coder/websocket` and `memo/internal/livemode`). The `errors` package is **not** currently imported. Add it to the import block:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"           // <-- ADD THIS
	"fmt"              // <-- ADD THIS (for the JSON message we write)
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

`errors.New("handler-explicit-failure")` is used inside the test. After the `fmt.Sprintf` removal in Step 1, the only new import required is `errors`.

- [ ] **Step 4: Run the new test**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run TestClient_ToolCall_HandlerErrorPropagatesAsToolResponse ./internal/livemode/google/`
Expected: PASS.

- [ ] **Step 5: Confirm `runToolCall` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: `runToolCall` row shows 90%+ (was 77.8%).

- [ ] **Step 6: Commit**

```bash
git add internal/livemode/google/client_test.go
git commit -m "test(livemode/google): cover runToolCall handler-error branch

The existing TestClient_ToolCallWithNilHandlerReportsNotAvailable covers
the nil-handler branch (c.handleToolCall == nil). This new test covers
the OTHER branch: c.handleToolCall is non-nil and returns
('', err) with a non-nil err. The test confirms the resulting
toolResponse carries 'Error: handler-explicit-failure' as its result.

Lifts runToolCall coverage from 77.8% to 90%+ without production-code
changes."
```

---

## Task 8: Google client Close idempotency + readLoop context-cancel tests

**Files:**
- Modify: `internal/livemode/google/client_test.go` (append)

**Interfaces:**
- Consumes: `NewClient`, `Client.Start`, `Client.Close`, `httptest.NewServer` with WS handler, `c.conn.Read` (via the readLoop goroutine).
- Produces: 2 new test functions: one confirming `Close` is safe to call twice; one confirming that canceling the context passed to `Start` causes the readLoop goroutine to exit (Events channel closes).

- [ ] **Step 1: Append the tests**

```go
func TestClient_Close_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Hold the connection open until the test client closes.
		_, _, _ = conn.Read(context.Background())
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestClient_ReadLoop_ContextCancelClosesEventsChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Block on read until the test closes the connection from the other side.
		for {
			if _, _, err := conn.Read(context.Background()); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("any-key", "any-model", "", nil, nil)
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	// Give the readLoop a moment to enter its Read call.
	time.Sleep(50 * time.Millisecond)

	cancel() // context cancel must cause the readLoop to exit

	// Wait up to 2s for the events channel to close (proving the readLoop
	// goroutine has returned).
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-c.Events():
			if !ok {
				return // success: channel closed
			}
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("timed out waiting for events channel to close after context cancel")
}
```

- [ ] **Step 2: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestClient_(Close_Idempotent|ReadLoop_ContextCancel)' ./internal/livemode/google/`
Expected: both PASS.

- [ ] **Step 3: Confirm `Close` and `readLoop` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: `readLoop` row shows 90%+ (was 73.2%); `Close` and related rows are at 100%.

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/google/client_test.go
git commit -m "test(livemode/google): cover Close idempotency and readLoop context cancel

Two new tests:
- TestClient_Close_Idempotent: Close called twice does not panic or
  return an error on the second call
- TestClient_ReadLoop_ContextCancelClosesEventsChannel: canceling the
  context passed to Start causes the readLoop goroutine to exit, which
  closes the Events() channel; we poll the channel with a deadline

Lifts readLoop coverage from 73.2% to 90%+ without production-code changes."
```

---

## Task 9: Google models test (decode-failure path)

**Files:**
- Modify: `internal/livemode/google/models_test.go` (append)

**Interfaces:**
- Consumes: `ListLiveModels(ctx, apiKey)`, `DiscoveryBaseURL` (package var), `httptest.NewServer`.
- Produces: one new test that confirms a server returning 200 with malformed JSON triggers the decode-error branch in `ListLiveModels`.

The existing `TestListLiveModels_ErrorStatus` covers 4xx/5xx. The missing branch is "200 OK with garbage JSON" → `json.NewDecoder(...).Decode` returns an error.

- [ ] **Step 1: Append the test**

Open `internal/livemode/google/models_test.go` and append:

```go
func TestListLiveModels_DecodeFailureReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// Valid 200, but not parseable as a JSON object with a "models" key.
		_, _ = w.Write([]byte("this is not json {"))
	}))
	defer srv.Close()

	original := DiscoveryBaseURL
	DiscoveryBaseURL = srv.URL
	defer func() { DiscoveryBaseURL = original }()

	_, err := ListLiveModels(context.Background(), "any-key")
	if err == nil {
		t.Fatal("expected an error from ListLiveModels on malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected the error to mention 'decode', got: %v", err)
	}
}
```

- [ ] **Step 2: Add `strings` to the import block**

The existing `internal/livemode/google/models_test.go` does **not** import `strings`. The new test uses `strings.Contains(err.Error(), "decode")`, so it must be added. The current import block is:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)
```

Add `"strings"` so the block becomes:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Run the new test**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run TestListLiveModels_DecodeFailureReturnsError ./internal/livemode/google/`
Expected: PASS.

- [ ] **Step 4: Confirm `ListLiveModels` is now 100%**

Run: `bash scripts/coverage-livemode.sh`
Expected: `ListLiveModels` row is gone (now 100%, was 90.6%).

- [ ] **Step 5: Commit**

```bash
git add internal/livemode/google/models_test.go
git commit -m "test(livemode/google): cover ListLiveModels decode-failure branch

The existing TestListLiveModels_ErrorStatus covers 4xx/5xx. This new
test covers the remaining branch: 200 OK with malformed JSON, which
must return a 'decode' error.

Lifts ListLiveModels from 90.6% to 100% without production-code changes."
```

---

## Task 10: OpenAI client Start failure-path tests (mirror of Task 6)

**Files:**
- Modify: `internal/livemode/openai_realtime/client_test.go` (append)

**Interfaces:**
- Consumes: `NewClient(apiKey, model, instructions, tools, handleToolCall, voice...)`, `Client.Start(ctx)`, `SessionBaseURL` (package var), `httptest.NewServer`.
- Produces: 2 new test functions covering `Start` failure branches that are currently at 78.3%.

These mirror Task 6 exactly, but use OpenAI's `session.update` instead of Google's `setup`, and the OpenAI test fixture conventions. The user is reminded that **these are not the same test** as the Google ones despite the similarity.

- [ ] **Step 1: Append the two tests**

```go
func TestClient_Start_FailsWhenDialFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, _, err := hj.Hijack()
		if err != nil {
			return
		}
		conn.Close()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when the server does not speak WS, got nil")
	}
	_ = c.Close()
}

func TestClient_Start_FailsWhenSessionUpdateWriteFails(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, ok := w.(http.Hijacker)
		if !ok {
			return
		}
		conn, bufrw, err := hj.Hijack()
		if err != nil {
			return
		}
		buf := make([]byte, 1024)
		_, _ = bufrw.Read(buf)
		_ = conn.Close()
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	err := c.Start(context.Background())
	if err == nil {
		t.Fatal("expected Start to fail when the server closes before session.update is written, got nil")
	}
	_ = c.Close()
}
```

- [ ] **Step 2: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestClient_Start_FailsWhen' ./internal/livemode/openai_realtime/`
Expected: both PASS.

- [ ] **Step 3: Confirm `Start` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: `Start` row is gone (now 100%, was 78.3%).

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/openai_realtime/client_test.go
git commit -m "test(livemode/openai_realtime): cover Client.Start dial-fail and session.update-write-fail branches

Mirror of the Google client test (commit from Task 6) but for OpenAI's
session.update message instead of Google's setup. The two tests are
NOT the same test even though they share a name pattern \u2014 see
spec \u00a76 for the naming-precision note.

Lifts Start coverage from 78.3% to 100% without production-code changes."
```

---

## Task 11: OpenAI client runToolCall handler-error and write-failure tests

**Files:**
- Modify: `internal/livemode/openai_realtime/client_test.go` (append)

**Interfaces:**
- Consumes: `NewClient`, `Client.Start`, `Client.Close`, `httptest.NewServer` with WS handler, the same fixture pattern as the Google tests.
- Produces: 3 new test functions covering the three `runToolCall` failure branches currently at 63.6%.

The branches we are covering (from reading `internal/livemode/openai_realtime/client.go:398`):

| Branch | How to induce it |
|---|---|
| `handleToolCall` returns `(result, err)` with non-nil err | Same shape as the Google test in Task 7 |
| First `writeJSON` (conversation.item.create) fails | Server closes after sending the tool-call event but before reading the response |
| Second `writeJSON` (response.create) fails | Server reads the first response, then closes before reading the second |

- [ ] **Step 1: Add the `errors` import to the existing import block, then append the three tests**

The existing `internal/livemode/openai_realtime/client_test.go` does **not** import `errors`. The new tests use `errors.New("openai-handler-failure")`, so it must be added. The current import block is:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

Add `"errors"` so the block becomes:

```go
import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"
	"memo/internal/livemode"
)
```

After the import block change, append the three tests after the last existing test function:

```go
func TestClient_ToolCall_HandlerErrorPropagatesAsToolResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		defer conn.Close(websocket.StatusNormalClosure, "")

		// Read the client's session.update.
		_, _, err = conn.Read(context.Background())
		if err != nil {
			return
		}

		// Send a function_call_arguments.done server event asking the
		// client to invoke 'probe' with the given CallID.
		event := `{"type":"response.function_call_arguments.done","call_id":"call-1","name":"probe","arguments":"{}"}`
		if err := conn.Write(context.Background(), websocket.MessageText, []byte(event)); err != nil {
			return
		}

		// Read the client's conversation.item.create response.
		_, data, err := conn.Read(context.Background())
		if err != nil {
			return
		}
		openAIToolResponseSink = string(data)
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	handlerErr := errors.New("openai-handler-failure")
	c := NewClient("any-key", "any-model", "", nil,
		func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			return "", handlerErr
		})
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if openAIToolResponseSink != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if openAIToolResponseSink == "" {
		t.Fatal("timed out waiting for the toolResponse to be received by the server")
	}
	if !strings.Contains(openAIToolResponseSink, "Error: openai-handler-failure") {
		t.Errorf("expected toolResponse to contain the handler error string, got: %s", openAIToolResponseSink)
	}
}

func TestClient_ToolCall_ResponseCreateWriteFailsEmitsErrorEvent(t *testing.T) {
	// Server reads the conversation.item.create, then closes. The
	// client's subsequent response.create write must fail and the client
	// must emit an EventError.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		// Read session.update.
		_, _, _ = conn.Read(context.Background())
		// Send the tool-call event.
		event := `{"type":"response.function_call_arguments.done","call_id":"call-1","name":"probe","arguments":"{}"}`
		_ = conn.Write(context.Background(), websocket.MessageText, []byte(event))
		// Read the first response (conversation.item.create).
		_, _, _ = conn.Read(context.Background())
		// Close before the second write.
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil,
		func(ctx context.Context, name string, args json.RawMessage) (string, error) {
			return "ok", nil
		})
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer c.Close()

	// Wait for an EventError on the events channel.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case ev := <-c.Events():
			if ev.Type == livemode.EventError {
				return // success
			}
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("timed out waiting for an EventError after response.create write failed")
}
```

- [ ] **Step 2: Add the test sink variable**

Append at the end of the file:

```go
// Package-level test sink written by the fake server and read by the
// test goroutine after a deadline.
var openAIToolResponseSink string
```

- [ ] **Step 3: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestClient_ToolCall_(HandlerErrorPropagatesAsToolResponse|ResponseCreateWriteFailsEmitsErrorEvent)' ./internal/livemode/openai_realtime/`
Expected: both PASS.

- [ ] **Step 4: Confirm `runToolCall` coverage improved significantly**

Run: `bash scripts/coverage-livemode.sh`
Expected: `runToolCall` row shows 90%+ (was 63.6%).

- [ ] **Step 5: Commit**

```bash
git add internal/livemode/openai_realtime/client_test.go
git commit -m "test(livemode/openai_realtime): cover runToolCall handler-error and write-failure branches

Two new tests:
- TestClient_ToolCall_HandlerErrorPropagatesAsToolResponse: handler returns
  an error; the response contains 'Error: openai-handler-failure'
- TestClient_ToolCall_ResponseCreateWriteFailsEmitsErrorEvent: server
  closes between the two writeJSON calls; the client must emit EventError

Together with the existing nil-handler test, this lifts runToolCall
coverage from 63.6% (the worst in the three packages) to 90%+ without
production-code changes."
```

---

## Task 12: OpenAI client Close idempotency + readLoop context-cancel tests (mirror of Task 8)

**Files:**
- Modify: `internal/livemode/openai_realtime/client_test.go` (append)

- [ ] **Step 1: Append the two tests**

```go
func TestClient_Close_Idempotent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		_, _, _ = conn.Read(context.Background())
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	c := NewClient("any-key", "any-model", "", nil, nil)
	if err := c.Start(context.Background()); err != nil {
		t.Fatalf("Start: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("first Close: %v", err)
	}
	if err := c.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

func TestClient_ReadLoop_ContextCancelClosesEventsChannel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, nil)
		if err != nil {
			return
		}
		for {
			if _, _, err := conn.Read(context.Background()); err != nil {
				return
			}
		}
	}))
	defer srv.Close()

	original := SessionBaseURL
	SessionBaseURL = "ws" + strings.TrimPrefix(srv.URL, "http")
	defer func() { SessionBaseURL = original }()

	ctx, cancel := context.WithCancel(context.Background())
	c := NewClient("any-key", "any-model", "", nil, nil)
	if err := c.Start(ctx); err != nil {
		t.Fatalf("Start: %v", err)
	}

	time.Sleep(50 * time.Millisecond)
	cancel()

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		select {
		case _, ok := <-c.Events():
			if !ok {
				return
			}
		default:
			time.Sleep(20 * time.Millisecond)
		}
	}
	t.Fatal("timed out waiting for events channel to close after context cancel")
}
```

- [ ] **Step 2: Run the new tests**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run 'TestClient_(Close_Idempotent|ReadLoop_ContextCancel)' ./internal/livemode/openai_realtime/`
Expected: both PASS.

- [ ] **Step 3: Confirm `readLoop` coverage improved**

Run: `bash scripts/coverage-livemode.sh`
Expected: `readLoop` row shows 90%+ (was 74.1%).

- [ ] **Step 4: Commit**

```bash
git add internal/livemode/openai_realtime/client_test.go
git commit -m "test(livemode/openai_realtime): cover Close idempotency and readLoop context cancel

Mirror of the Google client tests from Task 8, applied to the OpenAI
client. Lifts readLoop coverage from 74.1% to 90%+ and Close coverage
to 100% without production-code changes."
```

---

## Task 13: OpenAI models test (decode-failure path, mirror of Task 9)

**Files:**
- Modify: `internal/livemode/openai_realtime/models_test.go` (append)

- [ ] **Step 1: Append the test**

```go
func TestListRealtimeModels_DecodeFailureReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("{not valid json"))
	}))
	defer srv.Close()

	original := DiscoveryBaseURL
	DiscoveryBaseURL = srv.URL
	defer func() { DiscoveryBaseURL = original }()

	_, err := ListRealtimeModels(context.Background(), "any-key")
	if err == nil {
		t.Fatal("expected an error from ListRealtimeModels on malformed JSON, got nil")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Errorf("expected the error to mention 'decode', got: %v", err)
	}
}
```

- [ ] **Step 2: Add `strings` to the import block**

The existing `internal/livemode/openai_realtime/models_test.go` does **not** import `strings`. The new test uses `strings.Contains(err.Error(), "decode")`, so it must be added. The current import block is:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)
```

Add `"strings"` so the block becomes:

```go
import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)
```

- [ ] **Step 3: Run the new test**

Run: `CGO_ENABLED=1 go test -tags "sqlite_fts5" -run TestListRealtimeModels_DecodeFailureReturnsError ./internal/livemode/openai_realtime/`
Expected: PASS.

- [ ] **Step 4: Confirm `ListRealtimeModels` is now 100%**

Run: `bash scripts/coverage-livemode.sh`
Expected: `ListRealtimeModels` row is gone (was 84.2%).

- [ ] **Step 5: Commit**

```bash
git add internal/livemode/openai_realtime/models_test.go
git commit -m "test(livemode/openai_realtime): cover ListRealtimeModels decode-failure branch

Mirror of the Google ListLiveModels test from Task 9. Lifts
ListRealtimeModels from 84.2% to 100% without production-code changes."
```

---

## Task 14: Final verification + handoff

**Files:**
- Modify: `handoff.md` (prepend a new entry per AGENTS.md rule 2)
- No code changes — this task is documentation only.

- [ ] **Step 1: Run the full AGENTS.md verification suite on the touched packages**

```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet   -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test  -tags "sqlite_fts5" ./internal/livemode/... -race
```

Expected: all three commands exit 0. The test run should include all the new tests from Tasks 2-13 and they should all pass under `-race`.

- [ ] **Step 2: Run the coverage script and capture the final output**

Run: `bash scripts/coverage-livemode.sh`
Expected: 90%+ on each of the three packages. Paste the output into the handoff entry.

- [ ] **Step 3: Confirm no production-code files were modified**

Run: `git diff --name-only origin/feature/live-mode-v2..HEAD | grep -v '_test.go$' | grep -v '^scripts/coverage-livemode.sh$' | grep -v '^docs/' | grep -v '^handoff.md$'`
Expected: empty output. If anything appears, STOP and revert it (it should not be possible given the task constraints, but verify).

- [ ] **Step 4: Prepend the handoff entry**

Add a new entry at the TOP of `handoff.md` (above the previous "devam 23" entry). The entry should include:
- Date and session label
- Branch name (`feature/live-mode-v2`)
- Number of new test functions added
- Coverage numbers (before → after, for each of the three packages)
- The full output of the final `bash scripts/coverage-livemode.sh` run
- Confirmation that the user is concurrently editing WIP files and this session did not touch them
- One-line summary of next-session options (Flutter coverage, `internal/api`, untested provider clients, etc., per the spec §8 future-work list)

- [ ] **Step 5: Commit the handoff entry**

```bash
git add handoff.md
git commit -m "docs(handoff): record the Live Mode Go test coverage session"
```

- [ ] **Step 6: Final verification per AGENTS.md rule 3**

```bash
git log --oneline origin/feature/live-mode-v2..HEAD
```

Expected: shows the 14 new commits in order (1 for the script, ~12 for the test files, 1 for the handoff). Confirm no `Co-Authored-By` or `Generated with` lines in any of them:

```bash
git log origin/feature/live-mode-v2..HEAD | grep -iE 'co-authored|generated with'
```

Expected: empty output. If anything appears, fix the offending commit (amend with `--no-edit` and remove the line; for already-pushed commits this is only a problem if the branch is shared).

---

## Self-Review (per writing-plans skill)

**1. Spec coverage:**

- [x] Spec §5.1 (`scripts/coverage-livemode.sh`) → Task 1
- [x] Spec §5.2 (config.go tests) → Tasks 2, 3, 4
- [x] Spec §5.3 (echo_session.go tests) → Task 5 (deferred with comment per spec §6)
- [x] Spec §5.4 (google client tests) → Tasks 6, 7, 8, 9
- [x] Spec §5.5 (openai_realtime client tests) → Tasks 10, 11, 12, 13
- [x] Spec §7 verification → Task 14
- [x] Spec §8 handoff/future-work → Task 14, Step 4

**2. Placeholder scan:** No "TBD", "TODO", "implement later", "add appropriate error handling", "similar to Task N", or unfilled code blocks. Every step that says "run X" has an actual command and an expected result. Every test function has its full Go code inline.

**3. Type consistency check:**

- `NewClient` signature used throughout: `(apiKey, model, systemInstruction string, tools []livemode.ToolSpec, handleToolCall livemode.ToolCallHandler, voice ...string)` for Google (Task 6, 7, 8); `(apiKey, model, instructions string, tools []livemode.ToolSpec, handleToolCall livemode.ToolCallHandler, voice ...string)` for OpenAI (Task 10, 11, 12) — both verified against the actual `client.go` files in the exploration phase above.
- `SessionBaseURL` (Google and OpenAI) and `DiscoveryBaseURL` (both) are package `var`s — the save/restore pattern in every test mirrors existing tests in the same files.
- `livemode.SessionEvent` and `livemode.EventError` and `livemode.EventAudioOut` are used in test assertions — these are the same exported names referenced by the existing test files.
- Test sink variables (`toolResponseSink`, `toolCallSink`, `openAIToolResponseSink`) are package-level `string` vars declared at the bottom of their respective test files — no name collision with existing test code (verified: existing test files do not declare any of these names).
- The `handler` callback signature for `NewClient` is `func(ctx context.Context, name string, args json.RawMessage) (string, error)` — verified against `livemode.ToolCallHandler` in `internal/livemode/delegate_tool.go`.

**4. Spec gap I noticed during planning:** Task 5 was originally a real test in the spec (§5.3 second row). On inspection of `echo_session.go`, `Start` does not use the context at all and does not spawn a goroutine — so a "context cancel during start" test would have no observable behavior. The plan replaces it with a documentation comment, which is the resolution the spec §6 already allows. This is the only deviation from the spec, and it is in the direction the spec explicitly permits.

---

## Execution

After saving this plan, the user has approved the spec. Two execution options:

1. **Subagent-Driven (recommended)** — dispatch a fresh subagent per task, review between tasks, fast iteration with high-isolation context.
2. **Inline Execution** — execute tasks in this session using executing-plans, batch execution with checkpoints for review.
