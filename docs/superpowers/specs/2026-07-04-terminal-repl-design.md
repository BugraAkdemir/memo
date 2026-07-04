# Memo Terminal REPL — Design

## Problem

Memo currently requires the Flutter desktop app for any interaction. Some users
want a lighter, dependency-free way to chat with Memo directly from a terminal
by typing `memo` — similar in spirit to how `claude` opens Claude Code, but
intentionally much simpler: no panes, no scrollback management, no session
switching.

## Goals

- Typing `memo` in an interactive terminal opens a simple text chat loop
  against the existing Go backend.
- Zero new runtime dependencies (no TUI framework).
- Reuses the existing REST API exactly as the Flutter app does — no new
  backend logic, no duplicated chat/agent pipeline code.
- Does not change behavior for any non-interactive invocation of `memo`
  (background/launcher/scripts), since it's unclear today how every existing
  launch path invokes the binary.

## Non-goals

- No session history / switching inside the REPL (every run is a fresh chat).
- No full-screen TUI, no scrolling panes, no mouse support.
- No nuanced permission policies (session/forever) inside the REPL — only
  allow-once / deny-once.
- No changes to the Flutter app or its API contract.

## Behavior switch in `main.go`

- If stdin is **not** an interactive TTY, or `--headless` is passed: unchanged
  today's behavior (start the headless web server, block on SIGINT/SIGTERM).
  This protects any existing non-interactive launch path regardless of how it
  actually invokes the binary.
- If stdin **is** a TTY and `--headless` is not passed:
  1. `GET /api/status` (short timeout) against the target port to check
     whether a backend is already running.
  2. If not running, start it in-process exactly as today
     (`a.Startup(ctx)` + `a.StartWebServerHTTP(port)` in a goroutine), then
     poll `/api/status` until it responds.
  3. If already running, skip starting a new one (avoids a port-bind
     conflict) and just connect as a client.
  4. Run the new REPL in the foreground against `http://localhost:<port>`.
  5. On REPL exit: if this process started the backend, shut it down
     (context cancel, mirrors today's defer chain); if it only attached to an
     already-running backend, leave that backend alone.

## New component: `internal/replcli`

`Run(baseURL string) error` — the entire REPL loop. Plain stdlib only
(`net/http`, `bufio`, `encoding/json`), no new dependency.

**Startup (once per process):**
1. `os.Getwd()` → `POST /api/agent/chat {"project_path": cwd}` → chat `id`.
2. `POST /api/chats/switch {"id": id}`.
3. `PUT /api/agent/enabled {"enabled": true}` — agent mode is on by default
   for the whole REPL session.

**Loop:**
1. Print `> ` prompt, read one line via `bufio.Scanner` on stdin.
   - EOF (Ctrl+D) or the literal input `/exit` → exit the loop.
   - Ctrl+C (SIGINT) at any point → process exits immediately (no
     in-flight-request-cancel-only behavior — kept simple on purpose).
2. `POST /api/send/stream {"message": line}`, parse the response body as
   Server-Sent Events: lines starting with `data: ` contain a JSON object
   with the same fields the Flutter client already relies on
   (`content`, `thinking`, `finish_reason`, `done`, `error`) — this is the
   same wire format as `sendMessageStream` in
   `frontend/lib/core/api_client.dart`.
3. As `content` arrives, write it to stdout immediately (token-by-token
   streaming).
4. Handle `finish_reason`:
   - `"agent_event"` → decode `content` as JSON into a small local struct
     mirroring `frontend/lib/models/agent.dart`'s `AgentEvent`
     (`type`, `request_id`, `tool`, `args`, `result`, `error`,
     `danger_level`). Dispatch on `type`:
     - `tool_executing` → print `⚙ <tool> çalışıyor...`
     - `tool_result` / `tool_error` → print a short one-line outcome
     - `permission_request` → print the tool name and a short args preview,
       prompt `İzin ver mi? [y/n]`, read a line, then
       `POST /api/agent/permission {"request_id": ..., "policy": "allow_once"|"deny_once"}`
       and keep reading the same SSE stream (the backend resumes generation
       on the same request once the permission POST lands, exactly as it
       does for the Flutter permission dialog today).
   - `"status"` → print the status line as-is (e.g. a web-search notice).
   - anything else (`"usage"`, `"activity"`, unrecognized) → ignored.
5. `done: true` → print a trailing newline, go back to the prompt.
6. A non-empty `error` field, a malformed SSE line, or a transport-level
   error → print `Hata: ...` (malformed lines are skipped silently, matching
   the Flutter client's existing tolerance) and return to the prompt without
   crashing the REPL.

## Data flow

```
terminal input
  → POST /api/send/stream          (existing endpoint, unchanged)
  → existing chat.go / llm.go / agent pipeline
  → SSE chunks
  → replcli prints tokens + dispatches agent_event
  → (mid-stream) POST /api/agent/permission for tool approvals
```

No new backend endpoints, no new persistence. `replcli` is purely an HTTP
client of the API surface the Flutter app already uses.

## Error handling

- Target port already held by a non-Memo process when trying to start the
  backend → clear error message to stderr, exit code 1.
- `/api/status` never comes up after starting → timeout with a clear error,
  exit code 1.
- Mid-stream network error → print `Hata: ...`, return to prompt (REPL keeps
  running; user can retry).
- Malformed individual SSE `data:` line → skip that line only, matching
  existing Flutter behavior — a single bad chunk must not abort the turn.

## Testing

- Unit tests (table-driven) for the SSE-line-parsing function — pure
  function, no network needed.
- `go build ./...`, `go vet ./...`, `go test ./...` must stay green; the
  non-TTY / `--headless` path is byte-for-byte the same code path as today,
  so no existing test should need to change.
- Manual smoke test after implementation:
  1. Run `memo` in a terminal → send a plain message → verify streamed reply.
  2. Send a message that triggers a permission-gated tool → verify the y/n
     prompt appears and both allow and deny paths behave correctly.
  3. `/exit` and Ctrl+D both exit cleanly.
  4. Confirm the backend process is shut down on exit when this REPL process
     started it, and left running when it only attached to one already
     running.
