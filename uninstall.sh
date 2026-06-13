#!/usr/bin/env bash
set -e

APP_NAME="Memo"

INSTALL_DIR="$HOME/.local/share/MemoApp"
DESKTOP_FILE="$HOME/.local/share/applications/memo.desktop"
ICON_FILE="$HOME/.local/share/icons/hicolor/512x512/apps/memo.png"

echo "==== $APP_NAME Kaldırılıyor ===="

rm -rf "$INSTALL_DIR"
rm -f "$DESKTOP_FILE"
rm -f "$ICON_FILE"

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$HOME/.local/share/applications" || true
fi

echo "-> Uygulama dosyaları kaldırıldı."
echo "Not: Kullanıcı verileriniz (~/.memo) korundu. Tamamen silmek için: rm -rf ~/.memo"
echo "==== Kaldırma Tamamlandı ===="
