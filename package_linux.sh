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
go build -o build_output/memo-linux-x64/memo .

echo ""
echo "⚙️ 2. Building Flutter Frontend..."
cd frontend
flutter build linux --release
cd ..

echo ""
echo "⚙️ 3. Copying Frontend Assets..."
cp -r frontend/build/linux/x64/release/bundle/* build_output/memo-linux-x64/

echo ""
echo "⚙️ 4. Copying Backend Data & Config..."
cp -r data/* build_output/memo-linux-x64/data/ 2>/dev/null || true
cp -r config/* build_output/memo-linux-x64/config/ 2>/dev/null || true
cp .env build_output/memo-linux-x64/ 2>/dev/null || true

# Create a runner script
cat << 'EOF' > build_output/memo-linux-x64/run_memo.sh
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

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

echo "Shutting down backend..."
kill -9 $BACKEND_PID 2>/dev/null
wait $BACKEND_PID 2>/dev/null
pkill -9 -f "llama-server" 2>/dev/null || true
EOF

chmod +x build_output/memo-linux-x64/run_memo.sh

echo ""
echo "✅ Packaging complete!"
echo "📍 Output directory: build_output/memo-linux-x64/"
echo "🚀 To run: cd build_output/memo-linux-x64 && ./run_memo.sh"
