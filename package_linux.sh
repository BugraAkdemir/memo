#!/bin/bash
set -e

echo "====================================="
echo "📦 Memo Linux Packaging (Flutter + Go)"
echo "====================================="

# Clean build directory
rm -rf build_output
mkdir -p build_output/memo-linux-x64/data
mkdir -p build_output/memo-linux-x64/config

echo ""
echo "⚙️ 1. Building Go Backend (Headless)..."
go build -tags "sqlite_fts5" -o build_output/memo-linux-x64/memo .

echo ""
echo "⚙️ 2. Building Flutter Frontend..."
cd frontend
flutter build linux --release
cd ..

echo ""
echo "⚙️ 3. Copying Frontend Assets..."
cp -r frontend/build/linux/x64/release/bundle/* build_output/memo-linux-x64/

echo ""
echo "⚙️ 4. Copying Bundled Binaries (llama-server engines)..."
mkdir -p build_output/memo-linux-x64/binaries/linux
cp -r binaries/linux/* build_output/memo-linux-x64/binaries/linux/
# Ensure execution permissions for the engines
find build_output/memo-linux-x64/binaries/linux -name "llama-server" -exec chmod +x {} \;
find build_output/memo-linux-x64/binaries/linux -name "*.so*" -exec chmod +x {} \;

echo ""
echo "⚙️ 5. Copying Backend Data & Config..."
cp -r data/* build_output/memo-linux-x64/data/ 2>/dev/null || true
cp -r config/* build_output/memo-linux-x64/config/ 2>/dev/null || true
cp .env build_output/memo-linux-x64/ 2>/dev/null || true

# Create a runner script
cat << 'EOF' > build_output/memo-linux-x64/run_memo.sh
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# Single-instance lock: double-click koruması — aynı anda iki Memo başlatılamaz.
LOCK_FILE="/tmp/memo.lock"
exec {LOCK_FD}>"$LOCK_FILE"
if ! flock -n "$LOCK_FD"; then
    echo "Memo zaten çalışıyor. (lock: $LOCK_FILE)" >&2
    exit 0
fi

# The frontend exits with code 42 to request a full, clean restart (e.g. after
# "Wipe all data", so every backend subsystem re-initializes from scratch with
# no stale file handles). Any other exit code ends the session normally.
RESTART_CODE=42

while true; do
  echo "🧹 Cleaning up old processes..."
  pkill -9 -f "./memo --headless" 2>/dev/null || true
  pkill -9 -f "llama-server" 2>/dev/null || true
  sleep 1

  echo "Starting backend..."
  ./memo --headless --port 8090 > backend.log 2>&1 &
  BACKEND_PID=$!

  sleep 1
  echo "Starting frontend..."
  ./memo_flutter
  EXIT_CODE=$?

  echo "Shutting down backend..."
  kill -9 $BACKEND_PID 2>/dev/null
  wait $BACKEND_PID 2>/dev/null
  pkill -9 -f "llama-server" 2>/dev/null || true

  if [ "$EXIT_CODE" = "$RESTART_CODE" ]; then
    echo "♻️  Restart requested, relaunching..."
    sleep 1
    continue
  fi
  break
done

# Release lock on exit
flock -u "$LOCK_FD" 2>/dev/null || true
EOF

chmod +x build_output/memo-linux-x64/run_memo.sh

echo ""
echo "✅ Packaging complete!"
echo "📍 Output directory: build_output/memo-linux-x64/"
echo "🚀 To run: cd build_output/memo-linux-x64 && ./run_memo.sh"
