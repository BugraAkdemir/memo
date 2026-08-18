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

# clear fails (nonzero exit) when $TERM isn't set - happens any time this
# runs without a real pty (piped curl | bash over some SSH/provisioning
# setups, cron). Under 'set -e' that would kill the WHOLE script right
# here, before anything is downloaded/installed/removed - never let it
# be fatal.
clear 2>/dev/null || true

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
echo -e "  ${RED}▸${NC} ~/.local/share/com.memo.memo_flutter/  (app preferences: language, theme, setup wizard)"
echo ""

# ── memory backup ────────────────────────────────────────────────────────────
HAS_MEMORY=false
if [ -d "$MEMO_HOME/data/memory" ] && [ -n "$(ls -A "$MEMO_HOME/data/memory" 2>/dev/null)" ]; then
    HAS_MEMORY=true
fi

DO_BACKUP=false
if $HAS_MEMORY; then
    if [ -t 0 ]; then
        echo -ne "${YELLOW}Save your memory data before uninstalling? (yes/no) [yes]: ${NC}"
        read -r answer
    else
        echo -ne "${YELLOW}Save your memory data before uninstalling? (yes/no) [yes]: ${NC}" 2>/dev/null >/dev/tty || true
        read -r answer 2>/dev/null </dev/tty || true
    fi
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
        # providers.json holds configured API keys — encrypted at rest
        # (internal/provider/config.go), so backing it up alongside
        # memory/sessions is no less safe than the file already sitting on
        # disk. Without it, a routine uninstall+reinstall silently drops
        # every configured provider with no warning — the user finds out
        # only after reinstalling, when chat no longer works and they have
        # to dig up (or regenerate) their API keys from scratch.
        (cd "$MEMO_HOME/data" && zip -qr "$BACKUP_FILE" memory/ sessions/ providers.json 2>/dev/null || true)
    elif command -v python3 >/dev/null 2>&1; then
        MEMO_HOME="$MEMO_HOME" BACKUP_FILE="$BACKUP_FILE" python3 -c "
import zipfile, os, sys
src = os.path.join(os.environ['MEMO_HOME'], 'data')
zf = zipfile.ZipFile(os.environ['BACKUP_FILE'], 'w', zipfile.ZIP_DEFLATED)
for root, dirs, files in os.walk(src):
    for f in files:
        fp = os.path.join(root, f)
        zf.write(fp, os.path.relpath(fp, os.environ['MEMO_HOME'] + '/data'))
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
if [ -t 0 ]; then
    echo -ne "${RED}${BOLD}Proceed with uninstall? (yes/no) [no]: ${NC}"
    read -r confirm
else
    echo -ne "${RED}${BOLD}Proceed with uninstall? (yes/no) [no]: ${NC}" 2>/dev/null >/dev/tty || true
    read -r confirm 2>/dev/null </dev/tty || true
fi
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

# Flutter's shared_preferences_linux plugin stores app-level UI prefs
# (setup wizard completed, language, theme, onboarding tour seen) here,
# keyed by the app's fixed APPLICATION_ID (frontend/linux/CMakeLists.txt),
# independent of which Memo build/version is installed. Without removing
# this, those prefs silently survive a full uninstall+reinstall and the
# setup wizard never reappears on what looks like a fresh install.
echo -e "  ${RED}▸${NC} App preferences"
rm -rf "$HOME/.local/share/com.memo.memo_flutter"

# Same data on macOS lives in NSUserDefaults, backed by this plist.
rm -f "$HOME/Library/Preferences/com.bugrakaptan.memo.plist" 2>/dev/null || true

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
