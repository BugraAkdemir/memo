#!/bin/bash
# Windows Paketleme Script'i — Linux üzerinden cross-compile
# Gereksinim: mingw-w64  (sudo pacman -S mingw-w64-gcc  veya  sudo apt install mingw-w64)
# NOT: NSIS gerekmez — portable .exe üretilir, kullanıcı çift tıklar ve çalışır.

set -e

APP_NAME="Memo"
EXEC_NAME="local-llmmmemory"
VER=2.5.0

echo "========================================="
echo "Memo Windows Paketleme İşlemi Başladı"
echo "========================================="

# Mingw cross-compiler kontrolü
if ! command -v x86_64-w64-mingw32-gcc &>/dev/null; then
    echo "HATA: mingw-w64 kurulu değil."
    echo "  Arch:   sudo pacman -S mingw-w64-gcc"
    echo "  Debian: sudo apt install mingw-w64"
    exit 1
fi

mkdir -p dist_packages

# Wails Windows Build (portable — NSIS gerektirmez)
echo "--> Wails ile Windows/amd64 build alınıyor..."
PATH="$HOME/go/bin:$PATH" wails build \
    -platform windows/amd64 \
    -ldflags="-s -w" \
    -clean

PORTABLE_SRC="build/bin/${EXEC_NAME}.exe"
PORTABLE_DST="dist_packages/${APP_NAME}_${VER}_Windows.exe"

if [ ! -f "${PORTABLE_SRC}" ]; then
    echo "HATA: Build çıktısı bulunamadı: ${PORTABLE_SRC}"
    exit 1
fi

cp "${PORTABLE_SRC}" "${PORTABLE_DST}"

echo ""
echo "========================================="
echo "BAŞARILI!"
echo "  Çıktı: ${PORTABLE_DST}"
echo ""
echo "Kullanım: Dosyayı Windows'a kopyala, çift tıkla — bitti."
echo "Windows 10/11 gereksinim: WebView2 (genellikle zaten yüklü)."
echo "========================================="
