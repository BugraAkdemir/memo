#!/usr/bin/env bash
set -e

APP_NAME="Memo"
EXEC_NAME="memo"

# Hedef yollar
INSTALL_DIR="$HOME/.local/share/MemoApp"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"

echo "==== $APP_NAME Kaldırma Başlıyor ===="

# 1. Kısayolu ve iconu sil
echo "-> Kısayol siliniyor..."
rm -f "$DESKTOP_DIR/memo.desktop"
rm -f "$ICON_DIR/memo.png"

# 2. Sadece binary uygulamasını sil (verileri koru)
echo "-> Uygulama dosyası siliniyor..."
rm -f "$INSTALL_DIR/$EXEC_NAME"

# Update desktop db
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$DESKTOP_DIR" || true
fi

echo "==== Kaldırma Tamamlandı ===="
echo "Not: Geçmiş sohbetleriniz ve ayarlarınız korunmuştur."
echo "Hafıza ve ayarları da tamamen silmek istiyorsanız şu komutu çalıştırın:"
echo "rm -rf $INSTALL_DIR"
