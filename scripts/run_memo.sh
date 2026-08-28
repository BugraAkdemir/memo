#!/bin/bash
# Dev launcher: start the headless backend, WAIT until it actually answers,
# then start the Flutter desktop app. Without the wait the app races ahead
# of the backend (which needs a few seconds to compile under `go run`, spawn
# whisper/llama, and time out any offline Telegram/WhatsApp auto-connect),
# fires its init requests into a dead port, caches "cannot connect", and
# shows the connection dialog even though the backend comes up moments later.

set -u
PORT=8090
ROOT="/home/bugra/Documents/memo"

cd "$ROOT" && go run -tags "sqlite_fts5" . --headless --port "$PORT" &
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
