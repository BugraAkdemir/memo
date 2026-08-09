#!/usr/bin/env bash
set -e

APP_NAME="Memo"
EXEC_NAME="memo"

# Source bundle produced by package_linux.sh (memo backend + memo_flutter + binaries + run_memo.sh)
BUNDLE_DIR="build_output/memo-linux-x64"
RUNNER="run_memo.sh"

# Target paths (user-local installation)
INSTALL_DIR="$HOME/.local/share/MemoApp"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/512x512/apps"

echo "==== $APP_NAME Kurulumu Başlıyor ===="

# 1. Build kontrolü — bundle yoksa otomatik üret
if [ ! -d "$BUNDLE_DIR" ] || [ ! -f "$BUNDLE_DIR/$RUNNER" ]; then
    echo "-> Derlenmiş bundle bulunamadı ($BUNDLE_DIR)."
    if [ -f "package_linux.sh" ]; then
        echo "-> 'package_linux.sh' çalıştırılıyor..."
        bash package_linux.sh
    else
        echo "Hata: $BUNDLE_DIR yok ve package_linux.sh bulunamadı. Önce 'bash package_linux.sh' çalıştırın."
        exit 1
    fi
fi

# 2. Dizinleri oluştur
echo "-> Dizinler oluşturuluyor..."
mkdir -p "$INSTALL_DIR"
mkdir -p "$DESKTOP_DIR"
mkdir -p "$ICON_DIR"

# 3. Bundle'ı kopyala (backend, frontend, binaries, runner, config, data)
echo "-> Uygulama dosyaları kopyalanıyor..."
cp -r "$BUNDLE_DIR/." "$INSTALL_DIR/"
chmod +x "$INSTALL_DIR/$EXEC_NAME" 2>/dev/null || true
chmod +x "$INSTALL_DIR/$RUNNER" 2>/dev/null || true

# 4. Icon
ICON_PATH="build/appicon.png"
if [ -f "$ICON_PATH" ]; then
    cp "$ICON_PATH" "$ICON_DIR/memo.png"
    DESKTOP_ICON="memo"
else
    DESKTOP_ICON="utilities-terminal"
fi

# 5. .desktop dosyasını oluştur — run_memo.sh backend + frontend'i birlikte başlatır
echo "-> Masaüstü kısayolu (.desktop) oluşturuluyor..."
cat <<EOF > "$DESKTOP_DIR/memo.desktop"
[Desktop Entry]
Name=$APP_NAME
Comment=Local AI Memory Shell
Exec="$INSTALL_DIR/$RUNNER"
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
