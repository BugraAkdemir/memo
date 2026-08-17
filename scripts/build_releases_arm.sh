#!/bin/bash
set -e

# Linux arm64 release packaging — kept fully separate from
# build_releases.sh (x64/Windows/macOS) so that script never has to change
# for arm64's sake. Native build only, no cross-compiling: run this ON an
# actual arm64 Linux host (e.g. Raspberry Pi, or CI's ubuntu-24.04-arm
# runner). CPU-only — ARM boards/NAS realistically never carry a discrete
# GPU, so unlike build_releases.sh's Linux section there's no nvidia/amd
# variant here.
#
# Binaries come from binaries/linux/cpu-arm64/ (see download_binaries.sh),
# which download_binaries.sh populates with llama.cpp's own arm64 release
# asset + sqlite-vec's arm64 loadable extension — both already published
# upstream, no cross-compiling needed there either.

APP_NAME="Memo"
VERSION=$(cat version 2>/dev/null || echo "3.0.0")
VERSION=$(echo $VERSION | awk '{print $1}' | tr -d 'Vv')

BUILD_APPIMAGE=true
BUILD_TARGZ=true

# Auto-detect Flutter SDK
if ! command -v flutter &>/dev/null; then
    for p in "$HOME/Belgeler/flutter/bin" "$HOME/Belgeler/src/flutter/bin" "$HOME/.local/share/flutter/bin" "$HOME/snap/flutter/common/flutter/bin"; do
        if [ -x "$p/flutter" ]; then
            export PATH="$p:$PATH"
            break
        fi
    done
fi

echo "=========================================================="
echo "🚀 $APP_NAME V$VERSION Linux arm64 Paketleme İşlemi 🚀"
echo "=========================================================="

LINUX_ARCH="$(uname -m)"
if [ "$LINUX_ARCH" != "aarch64" ] && [ "$LINUX_ARCH" != "arm64" ]; then
    echo "❌ Bu script sadece arm64 host'ta çalışır (tespit edilen: $LINUX_ARCH)."
    echo "   x86_64 için build_releases.sh kullanın."
    exit 1
fi

if [ ! -d "binaries/linux/cpu-arm64" ]; then
    echo "❌ binaries/linux/cpu-arm64/ bulunamadı."
    echo "   Önce ./download_binaries.sh çalıştırın (llama.cpp + vec0 arm64 asset'lerini indirir)."
    exit 1
fi

rm -rf build_output/dist
rm -rf build_output/stage
mkdir -p build_output/dist
mkdir -p build_output/stage

STAGEDIR="build_output/stage/$APP_NAME"
mkdir -p "$STAGEDIR/data"
mkdir -p "$STAGEDIR/config"

# 1. Build Backend
echo "🔨 1. Go Backend Derleniyor (arm64)..."
go mod download
GOARCH=arm64 go build -tags "sqlite_fts5" -o "$STAGEDIR/memo-backend" .
# Same binary, shipped under the plain "memo" name too — this is what
# gets symlinked onto PATH by run_memo.sh so the terminal REPL is
# reachable by typing `memo`, independent of the desktop app launcher.
cp "$STAGEDIR/memo-backend" "$STAGEDIR/memo"
chmod +x "$STAGEDIR/memo"

# 2. Build Frontend
echo "🔨 2. Flutter Frontend Derleniyor (arm64)..."
cd frontend
flutter build linux --release
cd ..
cp -r frontend/build/linux/arm64/release/bundle/* "$STAGEDIR/"

# 3. Copy Assets
echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp + vec0, arm64 CPU-only)..."
# binaries/linux/cpu-arm64/'in içeriği pakette düz "cpu" ismiyle gidiyor —
# internal/llama'nın binary resolver'ı sadece GOOS + GPU mode biliyor
# (binaries/linux/cpu/...), mimari sonekli bir dizini hiç aramıyor.
mkdir -p "$STAGEDIR/binaries/linux/cpu"
cp -r binaries/linux/cpu-arm64/* "$STAGEDIR/binaries/linux/cpu/" 2>/dev/null || true
find "$STAGEDIR/binaries/linux/cpu" -name "llama-server*" -exec chmod +x {} \;
find "$STAGEDIR/binaries/linux/cpu" -name "*.so*" -exec chmod +x {} \;
# ngrok deliberately NOT bundled — bu repodaki binaries/linux/ngrok
# x86_64-only, internal/ngrok/installer.go zaten gömülü binary yoksa
# linux/arm64 build'ini bin.ngrok.com'dan indiriyor (sadece ilk
# kullanımda internet ister, kırık değil).

# Config
cp config/config.yaml.example "$STAGEDIR/config/config.yaml" 2>/dev/null || true
cp config/config.yaml.example "$STAGEDIR/config/config.yaml.example" 2>/dev/null || true
cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
echo '[]' > "$STAGEDIR/data/permissions.json"
mkdir -p "$STAGEDIR/data/models"
mkdir -p "$STAGEDIR/data/memory"
mkdir -p "$STAGEDIR/data/sessions"
mkdir -p "$STAGEDIR/data/agent-backups"
mkdir -p "$STAGEDIR/data/skills"
mkdir -p "$STAGEDIR/data/whatsapp"
touch "$STAGEDIR/data/whatsapp/.gitkeep"

# Create Runner Script — identical to build_releases.sh's Linux run_memo.sh
cat << 'RUNNER' > "$STAGEDIR/run_memo.sh"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Writable workspace
MEMO_HOME="$HOME/.memo"
export MEMO_DATA_DIR="$MEMO_HOME/data"
mkdir -p "$MEMO_HOME/data/bin"
mkdir -p "$MEMO_HOME/data/models"
mkdir -p "$MEMO_HOME/data/memory"
mkdir -p "$MEMO_HOME/data/sessions"
mkdir -p "$MEMO_HOME/data/agent-backups"
mkdir -p "$MEMO_HOME/data/skills"
mkdir -p "$MEMO_HOME/data/whatsapp"

# Copy llama.cpp binaries if not already present (first run)
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$DIR/binaries" ]; then
    echo "📦 İlk çalıştırma: engine binary'leri kopyalanıyor..."
    mkdir -p "$MEMO_HOME/binaries"
    cp -r "$DIR/binaries/"* "$MEMO_HOME/binaries/"
    # Archives don't reliably preserve the execute bit — force it.
    find "$MEMO_HOME/binaries" -name "llama-server*" -exec chmod +x {} \;
    find "$MEMO_HOME/binaries" -name "*.so*" -exec chmod +x {} \;
fi

# Install/refresh the `memo` CLI onto PATH. $DIR is a read-only mount for
# AppImage (unmounted once this process exits), so the binary is copied into
# the persistent $MEMO_HOME instead of symlinked straight to $DIR. Runs on
# every launch (cheap overwrite) so the CLI always matches the installed app.
if [ -f "$DIR/memo" ]; then
    mkdir -p "$MEMO_HOME/bin"
    rm -f "$MEMO_HOME/bin/memo"
    cp -f "$DIR/memo" "$MEMO_HOME/bin/memo"
    chmod +x "$MEMO_HOME/bin/memo"
    mkdir -p "$HOME/.local/bin"

    rm -f "$HOME/.local/bin/memo"
    cat > "$HOME/.local/bin/memo" <<WRAPPER
#!/bin/bash
export MEMO_DATA_DIR="$MEMO_HOME/data"
exec "$MEMO_HOME/bin/memo" "\$@"
WRAPPER
    chmod +x "$HOME/.local/bin/memo"

    case ":$PATH:" in
        *":$HOME/.local/bin:"*) ;; # already present
        *)
            for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
                if [ -f "$rc" ] && ! grep -q '\.local/bin' "$rc" 2>/dev/null; then
                    echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc"
                fi
            done
            if [ -d "$HOME/.config/fish" ] && ! grep -rq '\.local/bin' "$HOME/.config/fish/config.fish" 2>/dev/null; then
                echo 'fish_add_path $HOME/.local/bin' >> "$HOME/.config/fish/config.fish"
            fi
            ;;
    esac
fi

# Copy default config if not present
if [ ! -f "$MEMO_HOME/config/config.yaml" ] && [ -d "$DIR/config" ]; then
    mkdir -p "$MEMO_HOME/config"
    cp -r "$DIR/config/"* "$MEMO_HOME/config/"
fi

# Copy .env if not present
if [ ! -f "$MEMO_HOME/.env" ] && [ -f "$DIR/.env" ]; then
    cp "$DIR/.env" "$MEMO_HOME/.env"
fi

# Copy provider example config if no providers.json exists
if [ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$DIR/data/providers.example.json" ]; then
    cp "$DIR/data/providers.example.json" "$MEMO_HOME/data/providers.json"
fi

# Copy orchestra config if not present
if [ ! -f "$MEMO_HOME/data/orchestra.json" ] && [ -f "$DIR/data/orchestra.json" ]; then
    cp "$DIR/data/orchestra.json" "$MEMO_HOME/data/orchestra.json"
fi

# Create empty permissions if not present (Agent mode)
[ ! -f "$MEMO_HOME/data/permissions.json" ] && echo '[]' > "$MEMO_HOME/data/permissions.json"

cd "$MEMO_HOME"

# Set library paths
export LD_LIBRARY_PATH="$MEMO_HOME/data/bin:$DIR/lib:$LD_LIBRARY_PATH"

# Graceful shutdown helper: SIGTERM → wait → SIGKILL
_graceful_kill() {
    local pattern="$1"
    pkill -TERM -f "$pattern" 2>/dev/null || true
    local i=0
    while pgrep -f "$pattern" >/dev/null 2>&1 && [ $i -lt 5 ]; do
        sleep 1; i=$((i+1))
    done
    pkill -9 -f "$pattern" 2>/dev/null || true
}

# Check if a backend is already running — e.g. started via the `memo`
# terminal CLI. Attach to it instead of killing it and starting a second
# one, which caused a port-bind conflict and crashed both.
BACKEND_ALREADY_RUNNING=false
if curl -s -o /dev/null --max-time 2 "http://localhost:8090/api/status"; then
    BACKEND_ALREADY_RUNNING=true
    echo "ℹ Zaten çalışan bir Memo backend'i bulundu, ona bağlanılıyor."
fi

BACKEND_PID=""
if [ "$BACKEND_ALREADY_RUNNING" = false ]; then
    # Stop stale processes
    _graceful_kill "memo-backend"
    _graceful_kill "llama-server"

    # Start backend from writable directory
    "$DIR/memo-backend" > "$MEMO_HOME/backend.log" 2>&1 &
    BACKEND_PID=$!
    sleep 1
fi

# Start Flutter frontend
"$DIR/memo_flutter" "$@"

# Cleanup — only stop the backend if THIS script started it. If we attached
# to an already-running instance (e.g. the terminal CLI), leave it alone.
if [ -n "$BACKEND_PID" ] && kill -0 "$BACKEND_PID" 2>/dev/null; then
    curl -s -X POST "http://localhost:8090/api/shutdown" --max-time 5 >/dev/null 2>&1 || true
    sleep 3
    kill -TERM "$BACKEND_PID" 2>/dev/null || true
    sleep 2
    kill -9 "$BACKEND_PID" 2>/dev/null || true
fi
if [ "$BACKEND_ALREADY_RUNNING" = false ]; then
    _graceful_kill "llama-server"
fi
RUNNER

chmod +x "$STAGEDIR/run_memo.sh"

# 4. tar.gz
if [ "$BUILD_TARGZ" = true ]; then
    echo "📦 4. tar.gz Paketi Oluşturuluyor..."
    cd build_output/stage
    tar czf "../dist/${APP_NAME}-linux-arm64-v${VERSION}.tar.gz" "$APP_NAME"
    cd ../..
fi

# 5. AppImage
if [ "$BUILD_APPIMAGE" = true ]; then
    echo "📦 5. AppImage Paketi Oluşturuluyor..."
    APPDIR="build_output/stage/${APP_NAME}.AppDir"
    mkdir -p "$APPDIR"

    cp -r "$STAGEDIR/"* "$APPDIR/"

    ln -sf "run_memo.sh" "$APPDIR/AppRun"
    chmod +x "$APPDIR/AppRun"

    cat << DESKTOP > "$APPDIR/${APP_NAME}.desktop"
[Desktop Entry]
Name=${APP_NAME}
Exec=run_memo.sh
Icon=${APP_NAME}
Type=Application
Categories=Utility;
DESKTOP

    if [ -f "$APPDIR/icon.png" ]; then
        cp "$APPDIR/icon.png" "$APPDIR/${APP_NAME}.png"
    fi

    # arm64 appimagetool — different binary than build_releases.sh's x64 one
    if [ ! -s "appimagetool-aarch64.AppImage" ]; then
        echo "⬇️ appimagetool indiriliyor (aarch64)..."
        rm -f appimagetool-aarch64.AppImage
        wget -qO appimagetool-aarch64.AppImage https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-aarch64.AppImage
        chmod +x appimagetool-aarch64.AppImage
    fi
    ARCH=aarch64 ./appimagetool-aarch64.AppImage --appimage-extract-and-run "$APPDIR" "build_output/dist/${APP_NAME}-linux-arm64-v${VERSION}.AppImage" 2>&1 || echo "⚠️ AppImage oluşturulamadı."
fi

echo "🎉 LİNUX ARM64 PAKETLEMESİ TAMAMLANDI! Çıktılar 'build_output/dist' klasöründe."
echo "=========================================================="
echo "📁 Tüm Derleme Dosyaları: build_output/dist/"
ls -lh build_output/dist/
echo "=========================================================="
