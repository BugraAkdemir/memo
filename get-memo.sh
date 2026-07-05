#!/usr/bin/env bash
# Memo — one-line full installer for Linux / macOS.
#
#   curl -fsSL https://download.bugradev.com/get-memo.sh | bash
#
# Complete install: CLI, Flutter desktop app, app menu entry, engine binaries.
# Safe to re-run — existing config/data are preserved, only binaries refreshed.
set -euo pipefail

# ── colours ──────────────────────────────────────────────────────────────────
BOLD="\033[1m"
GREEN="\033[32m"
CYAN="\033[36m"
YELLOW="\033[33m"
RED="\033[31m"
BLUE="\033[34m"
NC="\033[0m"

clear

# ── banner ───────────────────────────────────────────────────────────────────
echo -e "${CYAN}${BOLD}"
echo "  __  __ _____ __  __  ___  "
echo " |  \\/  | ____|  \\/  |/ _ \\ "
echo " | |\\/| |  _| | |\\/| | | | |"
echo " | |  | | |___| |  | | |_| |"
echo " |_|  |_|_____|_|  |_|\\___/ "
echo -e "${NC}"
echo -e "${BOLD}  Local-first, privacy-focused AI assistant${NC}"
echo -e "  ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""

# ── os detection ─────────────────────────────────────────────────────────────
DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"
MEMO_HOME="$HOME/.memo"

os="$(uname -s)"
case "$os" in
    Linux)  url="$DOMAIN/memo.tar.gz" ;;
    Darwin) url="$DOMAIN/memo-mac.zip" ;;
    *)
        echo -e "${RED}Unsupported OS: $os${NC}" >&2
        exit 1
        ;;
esac

# ── dependencies ─────────────────────────────────────────────────────────────
for bin in curl; do
    command -v "$bin" >/dev/null 2>&1 || {
        echo -e "${RED}Error: '$bin' not found. Install it and try again.${NC}" >&2
        exit 1
    }
done

# ── download ─────────────────────────────────────────────────────────────────
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

archive="$work_dir/$(basename "$url")"
echo -e "${BOLD}Downloading:${NC} $url"
curl -fSL -# -o "$archive" "$url"
echo ""

# ── extract ──────────────────────────────────────────────────────────────────
echo -e "${BOLD}Extracting...${NC}"
case "$os" in
    Linux)  tar xzf "$archive" -C "$work_dir" ;;
    Darwin)
        command -v unzip >/dev/null 2>&1 || {
            echo -e "${RED}Error: 'unzip' not found.${NC}" >&2
            exit 1
        }
        unzip -o "$archive" -d "$work_dir"
        ;;
esac

src="$work_dir/$APP_NAME"
[ -d "$src" ] || src="$work_dir"

# ── install ──────────────────────────────────────────────────────────────────
echo -e "${BOLD}Installing...${NC}"

mkdir -p "$MEMO_HOME/bin" "$MEMO_HOME/config" \
    "$MEMO_HOME/data/models" "$MEMO_HOME/data/memory" "$MEMO_HOME/data/sessions" \
    "$MEMO_HOME/data/agent-backups" "$MEMO_HOME/data/skills" "$MEMO_HOME/data/whatsapp"

# Engine binaries (first-install only)
if [ ! -d "$MEMO_HOME/binaries" ] && [ -d "$src/binaries" ]; then
    echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
    cp -r "$src/binaries" "$MEMO_HOME/binaries"
fi

# Backend
if [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} Backend"
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# Flutter frontend
if [ -f "$src/memo_flutter" ]; then
    echo -e "  ${GREEN}▸${NC} Desktop app"
    cp -f "$src/memo_flutter" "$MEMO_HOME/memo_flutter"
    chmod +x "$MEMO_HOME/memo_flutter"
    if [ -d "$src/lib" ]; then
        cp -rf "$src/lib" "$MEMO_HOME/lib"
    fi
    [ -f "$src/data/icudtl.dat" ] && cp -f "$src/data/icudtl.dat" "$MEMO_HOME/data/icudtl.dat"
    [ -d "$src/data/flutter_assets" ] && cp -rf "$src/data/flutter_assets" "$MEMO_HOME/data/flutter_assets"
fi

# Runner script
if [ -f "$src/run_memo.sh" ]; then
    echo -e "  ${GREEN}▸${NC} Launcher"
    cp -f "$src/run_memo.sh" "$MEMO_HOME/run_memo.sh"
    chmod +x "$MEMO_HOME/run_memo.sh"
fi

# CLI binary
if [ -f "$src/memo" ]; then
    echo -e "  ${GREEN}▸${NC} CLI"
    rm -f "$MEMO_HOME/bin/memo"
    cp -f "$src/memo" "$MEMO_HOME/bin/memo"
elif [ -f "$src/memo-backend" ]; then
    cp -f "$src/memo-backend" "$MEMO_HOME/bin/memo"
else
    echo -e "${RED}Error: memo binary not found in archive.${NC}" >&2
    exit 1
fi
chmod +x "$MEMO_HOME/bin/memo"

# ── config seeding ───────────────────────────────────────────────────────────
if [ ! -f "$MEMO_HOME/config/config.yaml" ]; then
    if [ -f "$src/config/config.yaml" ]; then
        cp "$src/config/config.yaml" "$MEMO_HOME/config/config.yaml"
    elif [ -f "$src/config/config.yaml.example" ]; then
        cp "$src/config/config.yaml.example" "$MEMO_HOME/config/config.yaml"
    fi
fi
[ ! -f "$MEMO_HOME/.env" ]       && [ -f "$src/.env" ]                     && cp "$src/.env" "$MEMO_HOME/.env"
[ ! -f "$MEMO_HOME/data/providers.json" ] && [ -f "$src/data/providers.example.json" ] && \
    cp "$src/data/providers.example.json" "$MEMO_HOME/data/providers.json"
[ ! -f "$MEMO_HOME/data/permissions.json" ] && echo '[]' > "$MEMO_HOME/data/permissions.json"

# ── CLI wrapper ──────────────────────────────────────────────────────────────
echo -e "  ${GREEN}▸${NC} PATH wrapper"
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

# ── app menu entry ───────────────────────────────────────────────────────────
echo -e "  ${GREEN}▸${NC} App menu entry"
DESKTOP_DIR="$HOME/.local/share/applications"
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
mkdir -p "$DESKTOP_DIR" "$ICON_DIR"

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

command -v update-desktop-database >/dev/null 2>&1 && \
    update-desktop-database "$DESKTOP_DIR" 2>/dev/null || true

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}  Installation complete!${NC}"
echo ""
echo -e "  ${BOLD}Terminal:${NC}  ${CYAN}memo${NC}"
echo -e "  ${BOLD}Desktop:${NC}   find ${CYAN}Memo${NC} in your app menu"
echo -e "  ${BOLD}Guide:${NC}     ${BLUE}https://memo.bugradev.com/guide${NC}"

if command -v memo >/dev/null 2>&1; then
    echo ""
    echo -e "  Run ${CYAN}memo${NC} to get started."
else
    echo ""
    echo -e "  ${YELLOW}Open a new terminal (or restart your shell) and run 'memo'.${NC}"
fi

echo ""
echo -e "  ${BOLD}Thank you for choosing Memo!${NC}"
