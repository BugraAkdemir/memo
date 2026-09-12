#!/bin/bash
# Dev launcher: stop any Memo backend left over from a previous run (frees
# port 8090 so this run doesn't fail to bind or, worse, talk to a stale
# process), build a fresh backend binary, wait until it actually answers,
# then start the Flutter desktop app. Without the wait the app races ahead
# of the backend (which needs a few seconds to start, spawn whisper/llama,
# and time out any offline Telegram/WhatsApp auto-connect), fires its init
# requests into a dead port, caches "cannot connect", and shows the
# connection dialog even though the backend comes up moments later.

set -u
export CGO_ENABLED=1
PORT=8090
ROOT="/home/bugra/Documents/memo"
BIN="$ROOT/memo"

cd "$ROOT"

# --kill talks to whatever's already on the port (POST /api/shutdown first,
# then a port-based kill, then a platform process-name sweep as a last
# resort) — the same safe teardown `memo --kill` always does, so a backend
# left running from an earlier run_memo.sh (or a crashed one whose process
# didn't exit) can't collide with the fresh one this script is about to
# start. Runs via `go run` here specifically because it must work even
# before $BIN exists yet (a first-ever run) — it's a one-shot command that
# exits immediately either way, so the extra compile cost is paid once, not
# on every loop iteration below.
echo "run_memo: stopping any previous Memo backend / freeing port $PORT ..."
go run -tags "sqlite_fts5" . --kill

echo "run_memo: building backend ..."
if ! go build -tags "sqlite_fts5" -o "$BIN" .; then
  echo "run_memo: backend build failed" >&2
  exit 1
fi

"$BIN" --headless --port "$PORT" &
BACKEND_PID=$!
# Kill the backgrounded backend when this script exits (flutter run quits).
trap 'kill "$BACKEND_PID" 2>/dev/null' EXIT

echo "run_memo: waiting for backend on :$PORT ..."
for _ in $(seq 1 120); do
  if curl -sf -o /dev/null "http://127.0.0.1:$PORT/api/version"; then
    echo "run_memo: backend up."
    break
  fi
  # bail out early if the backend process already died
  kill -0 "$BACKEND_PID" 2>/dev/null || { echo "run_memo: backend exited before it was ready"; exit 1; }
  sleep 0.5
done

cd "$ROOT/frontend" && flutter run -d linux
