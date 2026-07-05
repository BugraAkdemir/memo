#!/usr/bin/env bash
# Memo — uninstaller for Linux / macOS.
#
#   curl -fsSL https://download.bugradev.com/uninstall.sh | bash
#
# Removes all Memo files. Optionally backs up your memory data before removal.
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
echo -e "  ${BOLD}Uninstaller${NC}"
echo ""

# ── check install ────────────────────────────────────────────────────────────
MEMO_HOME="$HOME/.memo"

if [ ! -d "$MEMO_HOME" ] && [ ! -f "$HOME/.local/bin/memo" ]; then
    echo -e "${YELLOW}Memo does not appear to be installed. Nothing to do.${NC}"
    exit 0
fi

echo -e "${BOLD}This will remove:${NC}"
echo -e "  ${RED}▸${NC} ~/.memo/         (app, config, data, engine binaries)"
echo -e "  ${RED}▸${NC} ~/.local/bin/memo  (CLI wrapper)"
echo -e "  ${RED}▸${NC} ~/.local/share/applications/memo.desktop"
echo -e "  ${RED}▸${NC} ~/.local/share/icons/hicolor/*/apps/memo.png"
echo ""

# ── memory backup ────────────────────────────────────────────────────────────
HAS_MEMORY=false
if [ -d "$MEMO_HOME/data/memory" ] && [ -n "$(ls -A "$MEMO_HOME/data/memory" 2>/dev/null)" ]; then
    HAS_MEMORY=true
fi

DO_BACKUP=false
if $HAS_MEMORY; then
    echo -ne "${YELLOW}Save your memory data before uninstalling? (yes/no) [yes]: ${NC}"
    read -r answer
    case "${answer:-yes}" in
        [Yy]|[Yy][Ee][Ss]) DO_BACKUP=true ;;
        *)                  DO_BACKUP=false ;;
    esac
fi

if $DO_BACKUP; then
    # find a writable Documents folder
    DOCS_DIR=""
    for d in "$HOME/Documents" "$HOME/Belgeler" "$HOME/Documentos" "$HOME/Dokumente"; do
        if [ -d "$d" ]; then DOCS_DIR="$d"; break; fi
    done
    [ -z "$DOCS_DIR" ] && DOCS_DIR="$HOME"

    BACKUP_FILE="$DOCS_DIR/memo-memory-$(date +%Y%m%d-%H%M%S).zip"
    echo -e "\n${BOLD}Backing up memory to:${NC} ${GREEN}$BACKUP_FILE${NC}"

    if command -v zip >/dev/null 2>&1; then
        (cd "$MEMO_HOME/data" && zip -qr "$BACKUP_FILE" memory/ sessions/ 2>/dev/null || true)
    elif command -v python3 >/dev/null 2>&1; then
        python3 -c "
import zipfile, os, sys
src = os.path.join('$MEMO_HOME', 'data')
zf = zipfile.ZipFile('$BACKUP_FILE', 'w', zipfile.ZIP_DEFLATED)
for root, dirs, files in os.walk(src):
    for f in files:
        fp = os.path.join(root, f)
        zf.write(fp, os.path.relpath(fp, src))
zf.close()
" 2>/dev/null
    else
        echo -e "${YELLOW}Warning: 'zip' not found. Copying memory folder as-is...${NC}"
        cp -r "$MEMO_HOME/data/memory" "$DOCS_DIR/memo-memory-backup/"
    fi

    if [ -f "$BACKUP_FILE" ]; then
        echo -e "${GREEN}  Memory saved: $BACKUP_FILE${NC}"
    fi
    echo ""
fi

# ── confirm ──────────────────────────────────────────────────────────────────
echo -ne "${RED}${BOLD}Proceed with uninstall? (yes/no) [no]: ${NC}"
read -r confirm
case "${confirm:-no}" in
    [Yy]|[Yy][Ee][Ss]) ;;
    *)
        echo -e "\n${YELLOW}Cancelled.${NC}"
        exit 0
        ;;
esac

echo ""

# ── remove ───────────────────────────────────────────────────────────────────
echo -e "${BOLD}Removing Memo...${NC}"

# Kill running processes first
pkill -TERM -f "memo-backend" 2>/dev/null || true
pkill -TERM -f "llama-server" 2>/dev/null || true
sleep 1

echo -e "  ${RED}▸${NC} ~/.memo/"
rm -rf "$MEMO_HOME"

echo -e "  ${RED}▸${NC} CLI wrapper"
rm -f "$HOME/.local/bin/memo"

echo -e "  ${RED}▸${NC} App menu entry"
rm -f "$HOME/.local/share/applications/memo.desktop"

echo -e "  ${RED}▸${NC} Icons"
rm -f "$HOME/.local/share/icons/hicolor/"*"/apps/memo.png" 2>/dev/null || true

# Clean PATH entries from shell configs
for rc in "$HOME/.bashrc" "$HOME/.zshrc" "$HOME/.config/fish/config.fish"; do
    if [ -f "$rc" ]; then
        sed -i.bak '/\.local\/bin/d' "$rc" 2>/dev/null || true
        rm -f "${rc}.bak" 2>/dev/null || true
    fi
done

command -v update-desktop-database >/dev/null 2>&1 && \
    update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}  Memo has been uninstalled.${NC}"
echo ""
echo -e "  ${BOLD}Guide:${NC}  ${BLUE}https://memo.bugradev.com/guide${NC}"

if $DO_BACKUP && [ -f "$BACKUP_FILE" ]; then
    echo -e "  ${BOLD}Memory:${NC} $BACKUP_FILE"
fi

echo ""
echo -e "  ${BOLD}Thank you for using Memo. See you again!${NC}"
