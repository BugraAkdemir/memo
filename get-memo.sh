#!/usr/bin/env bash
# Memo — one-line full installer for Linux and macOS.
#
#   curl -fsSL https://download.bugradev.com/get-memo.sh | bash
#
# Downloads the packaged build for this OS and performs a complete install:
#   • CLI binary + wrapper on PATH  →  `memo` in any terminal
#   • Flutter desktop app           →  `~/.memo/memo_flutter`
#   • Application menu entry        →  .desktop file + icon
#   • Engine binaries (llama.cpp + vec0) — first-install only
#   • Config / provider defaults — never overwrites existing
#
# Safe to re-run — existing config/data are preserved, only binaries refreshed.
set -euo pipefail

clear

DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"
MEMO_HOME="$HOME/.memo"

os="$(uname -s)"
case "$os" in
    Linux)  url="$DOMAIN/memo.tar.gz" ;;
    Darwin) url="$DOMAIN/memo-mac.zip" ;;
    *)
        echo "Unsupported OS: $os" >&2
        exit 1
        ;;
esac

for bin in curl; do
    command -v "$bin" >/dev/null 2>&1 || { echo "Error: '$bin' not found. Install it and try again." >&2; exit 1; }
done

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

archive="$work_dir/$(basename "$url")"
echo "Downloading: $url"
curl -fSL -# -o "$archive" "$url"

echo ""
echo "Extracting..."
case "$os" in
    Linux)  tar xzf "$archive" -C "$work_dir" ;;
    Darwin)
        command -v unzip >/dev/null 2>&1 || { echo "Error: 'unzip' not found." >&2; exit 1; }
        unzip -o "$archive" -d "$work_dir"
        ;;
esac

src="$work_dir/$APP_NAME"
[ -d "$src" ] || src="$work_dir"

echo "Installing..."
mkdir -p "$MEMO_HOME/bin" "$MEMO_HOME/config" \
    "$MEMO_HOME/data/models" "$MEMO_HOME/data/memory" "$MEMO_HOME/data/sessions" \
    "$MEMO_HOME/data/agent-backups" "$MEMO_HOME/data/skills" "$MEMO_HOME/data/whatsapp"

# --- Engine binaries (llama-server + vec0) ---
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$src/binaries" ]; then
    echo "  Engine binaries (llama.cpp + vec0)..."
    cp -r "$src/binaries" "$MEMO_HOME/binaries"
fi

# --- Backend binary ---
if [ -f "$src/memo-backend" ]; then
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# --- Flutter frontend ---
if [ -f "$src/memo_flutter" ]; then
    cp -f "$src/memo_flutter" "$MEMO_HOME/memo_flutter"
    chmod +x "$MEMO_HOME/memo_flutter"
fi
if [ -d "$src/lib" ]; then
    cp -rf "$src/lib" "$MEMO_HOME/lib"
fi
# Flutter runtime assets (not user data — only the engine files)
if [ -f "$src/data/icudtl.dat" ]; then
    cp -f "$src/data/icudtl.dat" "$MEMO_HOME/data/icudtl.dat"
fi
if [ -d "$src/data/flutter_assets" ]; then
    cp -rf "$src/data/flutter_assets" "$MEMO_HOME/data/flutter_assets"
fi

# --- Runner script ---
if [ -f "$src/run_memo.sh" ]; then
    cp -f "$src/run_memo.sh" "$MEMO_HOME/run_memo.sh"
    chmod +x "$MEMO_HOME/run_memo.sh"
fi

# --- CLI binary ---
echo "  CLI..."
if [ -f "$src/memo" ]; then
    rm -f "$MEMO_HOME/bin/memo"
    cp -f "$src/memo" "$MEMO_HOME/bin/memo"
elif [ -f "$src/memo-backend" ]; then
    cp -f "$src/memo-backend" "$MEMO_HOME/bin/memo"
else
    echo "Error: memo binary not found in archive." >&2
    exit 1
fi
chmod +x "$MEMO_HOME/bin/memo"

# --- Config / env / provider defaults (never overwrite existing) ---
if [ ! -f "$MEMO_HOME/config/config.yaml" ]; then
    if [ -f "$src/config/config.yaml" ]; then
        cp "$src/config/config.yaml" "$MEMO_HOME/config/config.yaml"
    elif [ -f "$src/config/config.yaml.example" ]; then
        cp "$src/config/config.yaml.example" "$MEMO_HOME/config/config.yaml"
    fi
fi
if [ ! -f "$MEMO_HOME/.env" ] && [ -f "$src/.env" ]; then
    cp "$src/.env" "$MEMO_HOME/.env"
fi
if [ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$src/data/providers.example.json" ]; then
    cp "$src/data/providers.example.json" "$MEMO_HOME/data/providers.json"
fi
if [ ! -f "$MEMO_HOME/data/permissions.json" ]; then
    echo '[]' > "$MEMO_HOME/data/permissions.json"
fi

# --- CLI wrapper on PATH ---
echo "  PATH wrapper..."
mkdir -p "$HOME/.local/bin"
rm -f "$HOME/.local/bin/memo"
cat > "$HOME/.local/bin/memo" <<WRAPPER
#!/bin/bash
export MEMO_DATA_DIR="$MEMO_HOME/data"
exec "$MEMO_HOME/bin/memo" "\$@"
WRAPPER
chmod +x "$HOME/.local/bin/memo"

case ":$PATH:" in
    *":$HOME/.local/bin:"*) ;;
    *)
        for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
            if [ -f "$rc" ] && ! grep -q '\.local/bin' "$rc" 2>/dev/null; then
                echo 'export PATH="$HOME/.local/bin:$PATH"' >> "$rc"
            fi
        done
        if [ -d "$HOME/.config/fish" ] && ! grep -rq '\.local/bin' "$HOME/.config/fish/config.fish" 2>/dev/null; then
            mkdir -p "$HOME/.config/fish"
            echo 'fish_add_path $HOME/.local/bin' >> "$HOME/.config/fish/config.fish"
        fi
        ;;
esac

# --- Application menu entry (.desktop) ---
echo "  App menu entry..."
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
mkdir -p "$DESKTOP_DIR" "$ICON_DIR"

# Try to find an icon from the archive
if [ -f "$src/icon.png" ]; then
    cp "$src/icon.png" "$ICON_DIR/memo.png"
elif [ -f "$src/data/flutter_assets/assets/icon.png" ]; then
    cp "$src/data/flutter_assets/assets/icon.png" "$ICON_DIR/memo.png"
fi

cat > "$DESKTOP_DIR/memo.desktop" <<DESKTOP
[Desktop Entry]
Name=$APP_NAME
Comment=Local AI Memory Shell
Exec=$MEMO_HOME/run_memo.sh
Path=$MEMO_HOME
Icon=$ICON_DIR/memo.png
Terminal=false
Type=Application
Categories=Utility;Development;
DESKTOP
chmod +x "$DESKTOP_DIR/memo.desktop"

if command -v update-desktop-database >/dev/null 2>&1; then
    update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true
fi

echo ""
echo "  Memo installed successfully!"
echo ""
echo "  Terminal:  memo"
echo "  Desktop:   find 'Memo' in your app menu"
if command -v memo >/dev/null 2>&1; then
    echo ""
    echo "  Run 'memo' now to get started."
else
    echo "  Open a new terminal (or restart your shell) and run 'memo'."
fi
