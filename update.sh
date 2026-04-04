#!/usr/bin/env bash
set -e

APP_NAME="Memo"
EXEC_NAME="memo"
BUILD_BIN="build/bin/local-llmmmemory"
INSTALL_DIR="$HOME/.local/share/MemoApp"

echo "==== $APP_NAME Güncellemesi Başlıyor ===="

if [ ! -f "$BUILD_BIN" ]; then
    echo "Hata: Yeni binary bulunamadı. Lütfen önce 'wails build' komutunu çalıştırın."
    exit 1
fi

if [ ! -d "$INSTALL_DIR" ]; then
    echo "Hata: Uygulama kurulu değil. Lütfen önce install.sh kullanın."
    exit 1
fi

echo "-> Arka planda çalışan Memo varsa kapatılıyor..."
pkill -f "$EXEC_NAME" 2>/dev/null || true
pkill -f "local-llmmmemory" 2>/dev/null || true

echo "-> Yeni binary kopyalanıyor..."
cp "$BUILD_BIN" "$INSTALL_DIR/$EXEC_NAME"
chmod +x "$INSTALL_DIR/$EXEC_NAME"

echo "==== Güncelleme Başarılı! ===="
echo "Sisteminize yeni Memo versiyonu kuruldu."
