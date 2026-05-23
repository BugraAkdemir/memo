#!/bin/bash
set -e

APP_NAME="Memo"
APP_EXEC="memo_flutter"
VERSION=$(cat version 2>/dev/null || echo "3.5.0")
VERSION=$(echo $VERSION | awk '{print $1}' | tr -d 'Vv') # Clean version string, e.g. 3.5.0

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
    go build -o "$STAGEDIR/memo-backend" .
    
    # 2. Build Frontend
    echo "🔨 2. Flutter Frontend Derleniyor..."
    cd frontend
    flutter build linux --release
    cd ..
    cp -r frontend/build/linux/x64/release/bundle/* "$STAGEDIR/"
    
    # 3. Copy Assets (data, config)
    echo "📂 3. Gömülü Dosyalar Kopyalanıyor (llama.cpp, stt, modeller)..."
    cp -r data/* "$STAGEDIR/data/" 2>/dev/null || true
    cp -r config/* "$STAGEDIR/config/" 2>/dev/null || true
    cp .env "$STAGEDIR/" 2>/dev/null || true
    
    # Create Runner Script
    cat << 'RUNNER' > "$STAGEDIR/run_memo.sh"
#!/bin/bash
DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
cd "$DIR"

# LD_LIBRARY_PATH'i ayarla ki llama.cpp ve diğer paylaşımlı kütüphaneler bulunsun
export LD_LIBRARY_PATH="$DIR/data/bin:$LD_LIBRARY_PATH"

pkill -9 -f "./memo-backend" 2>/dev/null || true
pkill -9 -f "llama-server" 2>/dev/null || true
sleep 1

./memo-backend > backend.log 2>&1 &
BACKEND_PID=$!

sleep 1
./memo_flutter

kill -9 $BACKEND_PID 2>/dev/null
pkill -9 -f "llama-server" 2>/dev/null || true
RUNNER
    chmod +x "$STAGEDIR/run_memo.sh"
    
    # --- TAR.GZ ---
    echo "📦 4. tar.gz Paketi Oluşturuluyor..."
    cd build_output/stage
    tar -czvf "../dist/${APP_NAME}-linux-x64-v${VERSION}.tar.gz" $APP_NAME >/dev/null
    cd ../..
    
    # --- APPIMAGE ---
    echo "📦 5. AppImage Paketi Oluşturuluyor..."
    APPDIR="build_output/stage/AppDir"
    mkdir -p "$APPDIR/usr/bin"
    cp -r "$STAGEDIR/"* "$APPDIR/usr/bin/"
    
    # Create AppRun
    cat << 'APPRUN' > "$APPDIR/AppRun"
#!/bin/bash
HERE="$(dirname "$(readlink -f "${0}")")"
exec "$HERE/usr/bin/run_memo.sh" "$@"
APPRUN
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
    
    # Create a dummy icon if doesn't exist
    touch "$APPDIR/${APP_NAME}.png"
    
    # Download appimagetool if not exists
    if [ ! -f "appimagetool-x86_64.AppImage" ]; then
        echo "⬇️ appimagetool indiriliyor..."
        wget -qO appimagetool-x86_64.AppImage https://github.com/AppImage/AppImageKit/releases/download/13/appimagetool-x86_64.AppImage
        chmod +x appimagetool-x86_64.AppImage
    fi
    ./appimagetool-x86_64.AppImage "$APPDIR" "build_output/dist/${APP_NAME}-linux-x64-v${VERSION}.AppImage" >/dev/null 2>&1 || echo "⚠️ AppImage oluşturulamadı (FUSE sorunu olabilir)."
    
    # --- DEB ---
    echo "📦 6. .deb Paketi Oluşturuluyor..."
    DEBDIR="build_output/stage/${APP_NAME}_${VERSION}_amd64"
    mkdir -p "$DEBDIR/opt/$APP_NAME"
    mkdir -p "$DEBDIR/usr/bin"
    mkdir -p "$DEBDIR/usr/share/applications"
    mkdir -p "$DEBDIR/DEBIAN"
    
    cp -r "$STAGEDIR/"* "$DEBDIR/opt/$APP_NAME/"
    ln -s "/opt/$APP_NAME/run_memo.sh" "$DEBDIR/usr/bin/${APP_NAME,,}"
    
    cat << CONTROL > "$DEBDIR/DEBIAN/control"
Package: ${APP_NAME,,}
Version: ${VERSION}
Section: utils
Priority: optional
Architecture: amd64
Maintainer: Bugra Akdemir <bugrakaptan5@gmail.com>
Description: Local LLM Memory Application with Flutter and Go
CONTROL

    cat << DEBDESKTOP > "$DEBDIR/usr/share/applications/${APP_NAME}.desktop"
[Desktop Entry]
Name=${APP_NAME}
Exec=/opt/${APP_NAME}/run_memo.sh
Icon=${APP_NAME}
Type=Application
Categories=Utility;
DEBDESKTOP

    dpkg-deb --build "$DEBDIR" "build_output/dist/" >/dev/null
    
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
    
    echo "📂 3. Gömülü Dosyalar Kopyalanıyor..."
    cp -r data/* "$STAGEDIR/data/" 2>/dev/null || true
    cp -r config/* "$STAGEDIR/config/" 2>/dev/null || true
    cp .env "$STAGEDIR/" 2>/dev/null || true
    
    # Create batch runner for Windows
    cat << 'RUNNERWIN' > "$STAGEDIR/run_memo.bat"
@echo off
cd /d "%~dp0"
set PATH=%~dp0data\bin;%PATH%

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

