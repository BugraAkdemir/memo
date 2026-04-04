#!/usr/bin/env bash
set -e

APP_NAME="Memo"
EXEC_NAME="memo"
BUILD_BIN="build/bin/local-llmmmemory"

# Hedef yollar (User-local installation)
INSTALL_DIR="$HOME/.local/share/MemoApp"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"

echo "==== $APP_NAME Kurulumu Başlıyor ===="

# 1. Build kontrolü
if [ ! -f "$BUILD_BIN" ]; then
    echo "Hata: Binary bulunamadı. Lütfen önce 'wails build' komutunu çalıştırın."
    exit 1
fi

# 2. Dizinleri oluştur
echo "-> Dizinler oluşturuluyor..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$DESKTOP_DIR"
mkdir -p "$ICON_DIR"

# 3. Binary'yi kopyala
echo "-> Binary kopyalanıyor..."
cp "$BUILD_BIN" "$INSTALL_DIR/$EXEC_NAME"
chmod +x "$INSTALL_DIR/$EXEC_NAME"

# 4. Icon olayını çöz (Eğer build/appicon.png veya benzeri varsa, yoksa atla)
# Wails genelde build/appicon.png yaratır
ICON_PATH="build/appicon.png"
if [ -f "$ICON_PATH" ]; then
    cp "$ICON_PATH" "$ICON_DIR/memo.png"
    DESKTOP_ICON="memo"
else
    DESKTOP_ICON="utilities-terminal"
fi

# 5. .desktop dosyasını oluştur
echo "-> Masaüstü kısayolu (.desktop) oluşturuluyor..."
cat <<EOF > "$DESKTOP_DIR/memo.desktop"
[Desktop Entry]
Name=$APP_NAME
Comment=Local AI Memory Shell
Exec="$INSTALL_DIR/$EXEC_NAME"
Path=$INSTALL_DIR
Icon=$DESKTOP_ICON
Terminal=false
Type=Application
Categories=Utility;Development;
EOF

chmod +x "$DESKTOP_DIR/memo.desktop"

# Uygulama menüsünü güncelle
if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$DESKTOP_DIR" || true
fi

echo "==== Kurulum Başarılı! ===="
echo "Uygulamayı artık sistem menüsünde 'Memo' adıyla bulabilir ve başlatabilirsiniz."
echo "Silmek için uninstall.sh kullanabilirsiniz."
