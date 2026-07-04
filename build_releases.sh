#!/bin/bash
set -e

APP_NAME="Memo"
APP_EXEC="memo_flutter"
VERSION=$(cat version 2>/dev/null || echo "3.0.0")

# Paket türleri
BUILD_DEB=false          # .deb paketi oluştur (dpkg-deb gerekli)
BUILD_APPIMAGE=true      # .AppImage paketi oluştur
BUILD_TARGZ=true         # .tar.gz paketi oluştur

# Auto-detect Flutter SDK
if ! command -v flutter &>/dev/null; then
    for p in "$HOME/Belgeler/flutter/bin" "$HOME/Belgeler/src/flutter/bin" "$HOME/.local/share/flutter/bin" "$HOME/snap/flutter/common/flutter/bin"; do
        if [ -x "$p/flutter" ]; then
            export PATH="$p:$PATH"
            break
        fi
    done
fi
VERSION=$(echo $VERSION | awk '{print $1}' | tr -d 'Vv') # Clean version string, e.g. 3.0.0

echo "=========================================================="
echo "🚀 $APP_NAME V$VERSION Paketleme İşlemi (Linux & Windows & macOS) 🚀"
echo "=========================================================="

# Check OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "darwin"* ]]; then
    OS="darwin"
elif [[ "$OSTYPE" == "msys"* || "$OSTYPE" == "cygwin"* || "$OSTYPE" == "win32"* ]]; then
    OS="windows"
else
    echo "❌ Desteklenmeyen işletim sistemi: $OSTYPE"
    exit 1
fi

rm -rf build_output/dist
rm -rf build_output/stage
mkdir -p build_output/dist
mkdir -p build_output/stage

STAGEDIR="build_output/stage/$APP_NAME"
mkdir -p "$STAGEDIR/data"
mkdir -p "$STAGEDIR/config"

if [ "$OS" == "linux" ]; then
    echo "✅ İşletim Sistemi: Linux tespit edildi. (tar.gz, AppImage, deb oluşturulacak)"

    # 1. Build Backend
    echo "🔨 1. Go Backend Derleniyor..."
    go mod download
    go build -o "$STAGEDIR/memo-backend" .
    # Same binary, shipped under the plain "memo" name too — this is what
    # gets symlinked onto PATH by run_memo.sh so the terminal REPL is
    # reachable by typing `memo`, independent of the desktop app launcher.
    cp "$STAGEDIR/memo-backend" "$STAGEDIR/memo"
    chmod +x "$STAGEDIR/memo"

    # 2. Build Frontend
    echo "🔨 2. Flutter Frontend Derleniyor..."
    cd frontend
    flutter build linux --release
    cd ..
    cp -r frontend/build/linux/x64/release/bundle/* "$STAGEDIR/"

    # 3. Copy Assets
    echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp + vec0)..."

    # Binaries (llama-server + vec0 extension) — only this OS's binaries,
    # not Windows/macOS ones too (keeps package size down). Runtime code
    # (ngrok/whisper) expects the "binaries/<GOOS>/..." layout, so keep
    # the linux/ subdir instead of flattening it.
    mkdir -p "$STAGEDIR/binaries/linux"
    cp -r binaries/linux/* "$STAGEDIR/binaries/linux/" 2>/dev/null || true

	# Config
    # Ship ONLY the clean example as config.yaml — never the developer's real
    # config.yaml, which holds personal tokens/keys (ngrok, tailscale, sync...).
    cp config/config.yaml.example "$STAGEDIR/config/config.yaml" 2>/dev/null || true
    cp config/config.yaml.example "$STAGEDIR/config/config.yaml.example" 2>/dev/null || true
    # .env.example'ı .env olarak kopyala (gerçek .env değil)
    cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
    # Provider & Orchestra example configs
    cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
    # Orchestra config is NOT bundled — the app generates clean defaults on
    # first run, so we never ship the developer's personal orchestra setup.
    # Agent permissions (boş başlangıç)
    echo '[]' > "$STAGEDIR/data/permissions.json"
    # Boş klasörleri hazırla ki uygulama hatasız başlasın
    mkdir -p "$STAGEDIR/data/models"
    mkdir -p "$STAGEDIR/data/memory"
    mkdir -p "$STAGEDIR/data/sessions"
    mkdir -p "$STAGEDIR/data/agent-backups"
    mkdir -p "$STAGEDIR/data/skills"
    mkdir -p "$STAGEDIR/data/whatsapp"
    touch "$STAGEDIR/data/whatsapp/.gitkeep"

    # Create Runner Script
    cat << 'RUNNER' > "$STAGEDIR/run_memo.sh"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Writable workspace
MEMO_HOME="$HOME/.memo"
# Pin the app's data directory to this writable workspace (overrides the
# OS default so data lands here regardless of launch CWD).
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

    # A plain symlink would run with MEMO_DATA_DIR unset, and the binary
    # falls back to a relative "data" dir in that case — meaning `memo` run
    # from some other project directory would create a stray ./data there
    # instead of using $MEMO_HOME/data. Write a small wrapper instead so the
    # data dir is always pinned, while cwd (used for agent file access)
    # stays whatever directory the user actually ran `memo` from.
    #
    # rm -f first: an older version of this script left a plain symlink at
    # this path (straight to $MEMO_HOME/bin/memo). `cat >` follows an
    # existing symlink instead of replacing it, so without this it would
    # write the wrapper text straight through the symlink and clobber the
    # real binary at $MEMO_HOME/bin/memo with wrapper source.
    rm -f "$HOME/.local/bin/memo"
    cat > "$HOME/.local/bin/memo" <<WRAPPER
#!/bin/bash
export MEMO_DATA_DIR="$MEMO_HOME/data"
exec "$MEMO_HOME/bin/memo" "\$@"
WRAPPER
    chmod +x "$HOME/.local/bin/memo"

    # Make sure ~/.local/bin is actually on PATH for new shells.
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
        tar czf "../dist/${APP_NAME}-linux-x64-v${VERSION}.tar.gz" "$APP_NAME"
        cd ../..
    fi

    # 5. AppImage
    if [ "$BUILD_APPIMAGE" = true ]; then
        echo "📦 5. AppImage Paketi Oluşturuluyor..."
        APPDIR="build_output/stage/${APP_NAME}.AppDir"
        mkdir -p "$APPDIR"

        cp -r "$STAGEDIR/"* "$APPDIR/"

        # Use run_memo.sh as AppRun
        ln -sf "run_memo.sh" "$APPDIR/AppRun"
        chmod +x "$APPDIR/AppRun"

        # Create Desktop entry
        cat << DESKTOP > "$APPDIR/${APP_NAME}.desktop"
[Desktop Entry]
Name=${APP_NAME}
Exec=run_memo.sh
Icon=${APP_NAME}
Type=Application
Categories=Utility;
DESKTOP

        # Copy app icon
        if [ -f "$APPDIR/icon.png" ]; then
            cp "$APPDIR/icon.png" "$APPDIR/${APP_NAME}.png"
        fi

        # Download appimagetool if not exists or is empty
        if [ ! -s "appimagetool-x86_64.AppImage" ]; then
            echo "⬇️ appimagetool indiriliyor..."
            rm -f appimagetool-x86_64.AppImage
            wget -qO appimagetool-x86_64.AppImage https://github.com/AppImage/appimagetool/releases/download/continuous/appimagetool-x86_64.AppImage
            chmod +x appimagetool-x86_64.AppImage
        fi
        # Try with --appimage-extract-and-run for systems without FUSE
        ARCH=x86_64 ./appimagetool-x86_64.AppImage --appimage-extract-and-run "$APPDIR" "build_output/dist/${APP_NAME}-linux-x64-v${VERSION}.AppImage" 2>&1 || echo "⚠️ AppImage oluşturulamadı."
    fi

    # --- DEB ---
    if [ "$BUILD_DEB" = true ]; then
        echo "📦 6. .deb Paketi Oluşturuluyor..."
        DEBDIR="build_output/stage/${APP_NAME}_${VERSION}_amd64"
        mkdir -p "$DEBDIR/opt/$APP_NAME"
        mkdir -p "$DEBDIR/usr/bin"
        mkdir -p "$DEBDIR/usr/share/applications"
        mkdir -p "$DEBDIR/usr/share/icons/hicolor/1024x1024/apps"
        mkdir -p "$DEBDIR/DEBIAN"

        cp -r "$STAGEDIR/"* "$DEBDIR/opt/$APP_NAME/"
        ln -s "/opt/$APP_NAME/run_memo.sh" "$DEBDIR/usr/bin/${APP_NAME,,}"
        if [ -f "$STAGEDIR/icon.png" ]; then
            cp "$STAGEDIR/icon.png" "$DEBDIR/usr/share/icons/hicolor/1024x1024/apps/${APP_NAME,,}.png"
        fi

        cat << CONTROL > "$DEBDIR/DEBIAN/control"
Package: ${APP_NAME,,}
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: amd64
Maintainer: Bugra Akdemir <bugrakaptan5@gmail.com>
Description: Local LLM Memory Shell — Privacy-first AI assistant with RAG, Agent mode, Orchestra, and External Providers
CONTROL

        cat << DEBDESKTOP > "$DEBDIR/usr/share/applications/${APP_NAME}.desktop"
[Desktop Entry]
Name=${APP_NAME}
Exec=/opt/${APP_NAME}/run_memo.sh
Icon=${APP_NAME}
Type=Application
Categories=Utility;
DEBDESKTOP

        if command -v dpkg-deb &>/dev/null; then
            dpkg-deb --build "$DEBDIR" "build_output/dist/" >/dev/null
        else
            echo "⚠️ dpkg-deb bulunamadı. .deb paketi atlanıyor. Kurmak için: sudo apt install dpkg"
        fi
    fi

    echo "🎉 LİNUX PAKETLEMESİ TAMAMLANDI! Çıktılar 'build_output/dist' klasöründe."

elif [ "$OS" == "windows" ]; then
    echo "✅ İşletim Sistemi: Windows tespit edildi. (.exe oluşturulacak)"

    echo "🔨 1. Go Backend Derleniyor..."
    go build -o "$STAGEDIR/memo-backend.exe" .
    # Same binary, shipped under the plain "memo.exe" name too — installer.iss
    # adds {app} to PATH, so typing `memo` in cmd/PowerShell resolves to this.
    cp "$STAGEDIR/memo-backend.exe" "$STAGEDIR/memo.exe"

    echo "🔨 2. Flutter Frontend Derleniyor..."
    cd frontend
    flutter build windows --release
    cd ..
    cp -r frontend/build/windows/x64/release/runner/Release/* "$STAGEDIR/"

    echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp + vec0)..."

    # Binaries (llama-server DLL'leri + vec0 extension) — sadece Windows'a
    # ait binary'ler, Linux/macOS'unkiler değil (paket boyutunu şişirmesin).
    # Runtime kod "binaries/<GOOS>/..." yapısını beklediği için windows/
    # alt klasörünü düzleştirmeden koruyoruz.
    mkdir -p "$STAGEDIR/binaries/windows"
    cp -r binaries/windows/* "$STAGEDIR/binaries/windows/" 2>/dev/null || true

    # Windows için vec0.dll yoksa uyar
    if [ ! -f "$STAGEDIR/binaries/windows/vec0.dll" ] && [ ! -f "$STAGEDIR/binaries/windows/cpu/vec0.dll" ]; then
        echo "⚠️  vec0.dll bulunamadı! Windows'ta vektör arama çalışmayacak."
        echo "   İndirmek için: https://github.com/asg017/sqlite-vec/releases"
    fi

    # Ship ONLY the clean example as config.yaml — never the developer's real
    # config.yaml, which holds personal tokens/keys (ngrok, tailscale, sync...).
    cp config/config.yaml.example "$STAGEDIR/config/config.yaml" 2>/dev/null || true
    cp config/config.yaml.example "$STAGEDIR/config/config.yaml.example" 2>/dev/null || true
    # .env.example'ı .env olarak kopyala (gerçek .env değil)
    cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
    # Provider & Orchestra example configs
    cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
    # Orchestra config is NOT bundled — the app generates clean defaults on
    # first run, so we never ship the developer's personal orchestra setup.
    echo '[]' > "$STAGEDIR/data/permissions.json"
    mkdir -p "$STAGEDIR/data/models"
    mkdir -p "$STAGEDIR/data/memory"
    mkdir -p "$STAGEDIR/data/sessions"
    mkdir -p "$STAGEDIR/data/agent-backups"
    mkdir -p "$STAGEDIR/data/skills"
    mkdir -p "$STAGEDIR/data/whatsapp"
    touch "$STAGEDIR/data/whatsapp/.gitkeep"

    # Create batch runner for Windows
    cat << 'RUNNERWIN' > "$STAGEDIR/run_memo.bat"
@echo off
setlocal enabledelayedexpansion
cd /d "%~dp0"
set "APP_DIR=%~dp0"
set "MEMO_HOME=%USERPROFILE%\.memo"
set "MEMO_DATA_DIR=%MEMO_HOME%\data"

REM First-run: copy example configs if needed
if not exist "%MEMO_HOME%\data" mkdir "%MEMO_HOME%\data"
if not exist "%MEMO_HOME%\data\providers.json" (
    if exist "%APP_DIR%data\providers.example.json" (
        copy "%APP_DIR%data\providers.example.json" "%MEMO_HOME%\data\providers.json" >nul
    )
)

REM Check if a backend is already running (e.g. started via the `memo`
REM terminal CLI) - attach to it instead of killing it and starting a
REM second one, which caused a port-bind conflict and crashed both.
set "BACKEND_ALREADY_RUNNING="
powershell -NoProfile -Command "try { Invoke-WebRequest -Uri 'http://localhost:8090/api/status' -Method GET -TimeoutSec 2 -UseBasicParsing -ErrorAction Stop | Out-Null; exit 0 } catch { exit 1 }" >nul 2>&1
if %ERRORLEVEL% EQU 0 (
    set "BACKEND_ALREADY_RUNNING=1"
    echo Zaten calisan bir Memo backend'i bulundu, ona baglaniliyor.
)

set "BACKEND_PID="
if not defined BACKEND_ALREADY_RUNNING (
    REM Stop stale instances gracefully (shutdown API, then force)
    powershell -NoProfile -Command "try { Invoke-WebRequest -Uri 'http://localhost:8090/api/shutdown' -Method POST -TimeoutSec 3 -ErrorAction Stop } catch {}" >nul 2>&1
    timeout /t 2 /nobreak >nul
    taskkill /F /IM memo-backend.exe >nul 2>&1
    taskkill /F /IM llama-server.exe >nul 2>&1

    REM Start backend, capture PID for targeted cleanup
    for /f %%i in ('powershell -NoProfile -Command "& { $p = Start-Process -FilePath '%APP_DIR%memo-backend.exe' -WorkingDirectory '%MEMO_HOME%' -PassThru; $p.Id }"') do set BACKEND_PID=%%i
    timeout /t 1 /nobreak >nul
)

REM Start Flutter frontend (blocks until closed)
start "" /WAIT "%APP_DIR%memo_flutter.exe"

REM Cleanup — only stop the backend if THIS script started it. If we
REM attached to an already-running instance (e.g. the terminal CLI),
REM leave it alone.
if defined BACKEND_PID (
    powershell -NoProfile -Command "try { Invoke-WebRequest -Uri 'http://localhost:8090/api/shutdown' -Method POST -TimeoutSec 5 -ErrorAction Stop } catch {}" >nul 2>&1
    timeout /t 3 /nobreak >nul
    taskkill /F /PID %BACKEND_PID% >nul 2>&1
)
if not defined BACKEND_ALREADY_RUNNING (
    taskkill /F /IM llama-server.exe >nul 2>&1
)
RUNNERWIN

    echo "📦 4. Windows ZIP Paketi Oluşturuluyor..."
    cd build_output/stage
    powershell -Command "Compress-Archive -Path '${APP_NAME}\*' -DestinationPath '..\dist\${APP_NAME}-windows-x64-v${VERSION}.zip' -Force"
    cd ../..

    echo "🎉 WINDOWS PAKETLEMESİ TAMAMLANDI! Çıktılar 'build_output/dist' klasöründe."

elif [ "$OS" == "darwin" ]; then
    echo "✅ İşletim Sistemi: macOS tespit edildi. (.app + .zip oluşturulacak)"

    # 1. Build Backend
    echo "🔨 1. Go Backend Derleniyor (darwin)..."
    go mod download
    MAC_ARCH=$(uname -m)  # arm64 on Apple Silicon, x86_64 on Intel
    GOARCH=$MAC_ARCH go build -o "$STAGEDIR/memo-backend" .

    # 2. Build Frontend
    echo "🔨 2. Flutter macOS Derleniyor..."
    cd frontend
    flutter build macos --release
    cd ..
    # Flutter outputs a full .app bundle
    APP_BUNDLE_SRC="frontend/build/macos/Build/Products/Release/${APP_NAME}.app"
    if [ ! -d "$APP_BUNDLE_SRC" ]; then
        echo "❌ Flutter .app bundle bulunamadı: $APP_BUNDLE_SRC"
        exit 1
    fi
    cp -r "$APP_BUNDLE_SRC" "$STAGEDIR/${APP_NAME}.app"

    # 3. Copy Assets
    echo "📂 3. Gömülü Dosyalar Kopyalanıyor..."
    # Only macOS's own binaries, not Linux/Windows ones (keeps package size
    # down). Runtime code expects "binaries/<GOOS>/...", so keep the darwin/
    # subdir instead of flattening it.
    mkdir -p "$STAGEDIR/binaries/darwin"
    cp -r binaries/darwin/* "$STAGEDIR/binaries/darwin/" 2>/dev/null || true

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

    # Create Runner Script
    cat << 'RUNNER_MAC' > "$STAGEDIR/run_memo.sh"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

MEMO_HOME="$HOME/.memo"
export MEMO_DATA_DIR="$MEMO_HOME/data"
mkdir -p "$MEMO_HOME/data/"{models,memory,sessions,agent-backups,skills,whatsapp}
mkdir -p "$MEMO_HOME/data/bin"

# First-run: copy binaries
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$DIR/binaries" ]; then
    echo "📦 İlk çalıştırma: engine binary'leri kopyalanıyor..."
    mkdir -p "$MEMO_HOME/binaries"
    cp -r "$DIR/binaries/"* "$MEMO_HOME/binaries/"
fi

# First-run: copy configs
if [ ! -f "$MEMO_HOME/config/config.yaml" ] && [ -d "$DIR/config" ]; then
    mkdir -p "$MEMO_HOME/config"
    cp -r "$DIR/config/"* "$MEMO_HOME/config/"
fi
[ ! -f "$MEMO_HOME/.env" ] && [ -f "$DIR/.env" ] && cp "$DIR/.env" "$MEMO_HOME/.env"
[ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$DIR/data/providers.example.json" ] && \
    cp "$DIR/data/providers.example.json" "$MEMO_HOME/data/providers.json"
[ ! -f "$MEMO_HOME/data/permissions.json" ] && echo '[]' > "$MEMO_HOME/data/permissions.json"

cd "$MEMO_HOME"

# Graceful shutdown helper
_graceful_kill() {
    local pattern="$1"
    pkill -TERM -f "$pattern" 2>/dev/null || true
    local i=0
    while pgrep -f "$pattern" >/dev/null 2>&1 && [ $i -lt 5 ]; do
        sleep 1; i=$((i+1))
    done
    pkill -9 -f "$pattern" 2>/dev/null || true
}

# Check if a backend is already running (e.g. started via the `memo`
# terminal CLI) — attach to it instead of killing it and starting a second
# one, which caused a port-bind conflict and crashed both.
BACKEND_ALREADY_RUNNING=false
if curl -s -o /dev/null --max-time 2 "http://localhost:8090/api/status"; then
    BACKEND_ALREADY_RUNNING=true
    echo "ℹ Zaten çalışan bir Memo backend'i bulundu, ona bağlanılıyor."
fi

BACKEND_PID=""
if [ "$BACKEND_ALREADY_RUNNING" = false ]; then
    _graceful_kill "memo-backend"
    _graceful_kill "llama-server"

    # Start backend
    "$DIR/memo-backend" > "$MEMO_HOME/backend.log" 2>&1 &
    BACKEND_PID=$!
    sleep 1
fi

# Launch Flutter .app (run binary directly so the shell waits)
"$DIR/Memo.app/Contents/MacOS/memo_flutter" "$@"

# Cleanup — only stop the backend if THIS script started it.
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
RUNNER_MAC

    chmod +x "$STAGEDIR/run_memo.sh"

    # 4. Create .zip
    echo "📦 4. macOS ZIP Paketi Oluşturuluyor..."
    cd build_output/stage
    zip -qr "../dist/${APP_NAME}-macos-${MAC_ARCH}-v${VERSION}.zip" "$APP_NAME"
    cd ../..

    echo "🎉 MACOS PAKETLEMESİ TAMAMLANDI! Çıktılar 'build_output/dist' klasöründe."

fi

echo "=========================================================="
echo "📁 Tüm Derleme Dosyaları: build_output/dist/"
ls -lh build_output/dist/
echo "=========================================================="