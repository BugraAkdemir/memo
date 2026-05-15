#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

echo "Starting backend..."
./memo --headless --port 8090 > backend.log 2>&1 &
BACKEND_PID=$!

sleep 1
echo "Starting frontend..."
./memo_flutter

echo "Shutting down backend..."
kill $BACKEND_PID 2>/dev/null
wait $BACKEND_PID 2>/dev/null
