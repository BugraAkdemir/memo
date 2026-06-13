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
    for p in "$HOME/Belgeler/src/flutter/bin" "$HOME/.local/share/flutter/bin" "$HOME/snap/flutter/common/flutter/bin"; do
        if [ -x "$p/flutter" ]; then
            export PATH="$p:$PATH"
            break
        fi
    done
fi
VERSION=$(echo $VERSION | awk '{print $1}' | tr -d 'Vv') # Clean version string, e.g. 3.0.0

echo "=========================================================="
echo "🚀 $APP_NAME V$VERSION Paketleme İşlemi (Linux & Windows) 🚀"
echo "=========================================================="

# Check OS
if [[ "$OSTYPE" == "linux-gnu"* ]]; then
    OS="linux"
elif [[ "$OSTYPE" == "msys"* || "$OSTYPE" == "cygwin"* || "$OSTYPE" == "win32"* ]]; then
    OS="windows"
else
    echo "❌ Desteklenmeyen işletim sistemi: $OSTYPE"
    exit 1
fi

rm -rf build_output/dist
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

    # 2. Build Frontend
    echo "🔨 2. Flutter Frontend Derleniyor..."
    cd frontend
    flutter build linux --release
    cd ..
    cp -r frontend/build/linux/x64/release/bundle/* "$STAGEDIR/"

    # 3. Copy Assets
    echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp + vec0)..."

    # Binaries (llama-server + vec0 extension)
    mkdir -p "$STAGEDIR/binaries"
    cp -r binaries/* "$STAGEDIR/binaries/" 2>/dev/null || true

    # stt_server varsa kopyala
    mkdir -p "$STAGEDIR/data/bin"
    cp data/bin/stt_server "$STAGEDIR/data/bin/" 2>/dev/null || true

    # Config
    cp -r config/* "$STAGEDIR/config/" 2>/dev/null || true
    # .env.example'ı .env olarak kopyala (gerçek .env değil)
    cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
    # Provider & Orchestra example configs
    cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
    cp data/orchestra.json "$STAGEDIR/data/orchestra.json" 2>/dev/null || true
    # Agent permissions (boş başlangıç)
    echo '[]' > "$STAGEDIR/data/permissions.json"
    # Boş klasörleri hazırla ki uygulama hatasız başlasın
    mkdir -p "$STAGEDIR/data/models"
    mkdir -p "$STAGEDIR/data/memory"
    mkdir -p "$STAGEDIR/data/sessions"
    mkdir -p "$STAGEDIR/data/agent-backups"
    mkdir -p "$STAGEDIR/data/whatsapp"
    touch "$STAGEDIR/data/whatsapp/.gitkeep"

    # Create Runner Script
    cat << 'RUNNER' > "$STAGEDIR/run_memo.sh"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"

# Writable workspace
MEMO_HOME="$HOME/.memo"
mkdir -p "$MEMO_HOME/data/bin"
mkdir -p "$MEMO_HOME/data/models"
mkdir -p "$MEMO_HOME/data/memory"
mkdir -p "$MEMO_HOME/data/sessions"
mkdir -p "$MEMO_HOME/data/agent-backups"
mkdir -p "$MEMO_HOME/data/whatsapp"

# Copy llama.cpp binaries if not already present (first run)
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$DIR/binaries" ]; then
    echo "📦 İlk çalıştırma: engine binary'leri kopyalanıyor..."
    mkdir -p "$MEMO_HOME/binaries"
    cp -r "$DIR/binaries/"* "$MEMO_HOME/binaries/"
fi

# Copy default config if not present
if [ ! -f "$MEMO_HOME/config/config.yaml" ] && [ -d "$DIR/config" ]; then
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

# Stop old processes
pkill -9 -f "memo-backend" 2>/dev/null || true
pkill -9 -f "llama-server" 2>/dev/null || true
sleep 0.5

# Start backend from writable directory
"$DIR/memo-backend" > "$MEMO_HOME/backend.log" 2>&1 &
BACKEND_PID=$!
sleep 1

# Start Flutter frontend
"$DIR/memo_flutter" "$@"

# Cleanup
kill $BACKEND_PID 2>/dev/null
pkill -9 -f "llama-server" 2>/dev/null || true
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

    echo "🔨 2. Flutter Frontend Derleniyor..."
    cd frontend
    flutter build windows --release
    cd ..
    cp -r frontend/build/windows/x64/release/runner/Release/* "$STAGEDIR/"

    echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp + vec0)..."

    # Binaries (llama-server DLL'leri + vec0 extension)
    mkdir -p "$STAGEDIR/binaries"
    cp -r binaries/* "$STAGEDIR/binaries/" 2>/dev/null || true

    # Windows için vec0.dll yoksa uyar
    if [ ! -f "$STAGEDIR/binaries/windows/vec0.dll" ] && [ ! -f "$STAGEDIR/binaries/windows/cpu/vec0.dll" ]; then
        echo "⚠️  vec0.dll bulunamadı! Windows'ta vektör arama çalışmayacak."
        echo "   İndirmek için: https://github.com/asg017/sqlite-vec/releases"
    fi

    cp -r config/* "$STAGEDIR/config/" 2>/dev/null || true
    # .env.example'ı .env olarak kopyala (gerçek .env değil)
    cp .env.example "$STAGEDIR/.env" 2>/dev/null || true
    # Provider & Orchestra example configs
    cp data/providers.example.json "$STAGEDIR/data/providers.example.json" 2>/dev/null || true
    cp data/orchestra.json "$STAGEDIR/data/orchestra.json" 2>/dev/null || true
    echo '[]' > "$STAGEDIR/data/permissions.json"
    mkdir -p "$STAGEDIR/data/models"
    mkdir -p "$STAGEDIR/data/memory"
    mkdir -p "$STAGEDIR/data/sessions"
    mkdir -p "$STAGEDIR/data/agent-backups"
    mkdir -p "$STAGEDIR/data/whatsapp"
    touch "$STAGEDIR/data/whatsapp/.gitkeep"

    # Create batch runner for Windows
    cat << 'RUNNERWIN' > "$STAGEDIR/run_memo.bat"
@echo off
cd /d "%~dp0"
set PATH=%~dp0data\bin;%PATH%

REM First-run: copy example configs if needed
if not exist "%USERPROFILE%\.memo\data\providers.json" (
    if exist "%~dp0data\providers.example.json" (
        mkdir "%USERPROFILE%\.memo\data" 2>nul
        copy "%~dp0data\providers.example.json" "%USERPROFILE%\.memo\data\providers.json" >nul
    )
)

taskkill /F /IM memo-backend.exe >nul 2>&1
taskkill /F /IM llama-server.exe >nul 2>&1

start "" /B memo-backend.exe
timeout /t 1 /nobreak >nul
start "" /WAIT memo_flutter.exe

taskkill /F /IM memo-backend.exe >nul 2>&1
taskkill /F /IM llama-server.exe >nul 2>&1
RUNNERWIN

    echo "📦 4. Windows ZIP Paketi Oluşturuluyor..."
    cd build_output/stage
    powershell -Command "Compress-Archive -Path '${APP_NAME}\*' -DestinationPath '..\dist\${APP_NAME}-windows-x64-v${VERSION}.zip' -Force"
    cd ../..

    echo "🎉 WINDOWS PAKETLEMESİ TAMAMLANDI! Çıktılar 'build_output/dist' klasöründe."
fi

echo "=========================================================="
echo "📁 Tüm Derleme Dosyaları: build_output/dist/"
ls -lh build_output/dist/
echo "=========================================================="