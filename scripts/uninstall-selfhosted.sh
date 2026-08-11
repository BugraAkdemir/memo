#!/usr/bin/env bash
# Memo — self-hosted SERVER uninstaller (no desktop app).
#
#   curl -fsSL https://download.bugradev.com/uninstall-selfhosted.sh | bash
#   bash uninstall-selfhosted.sh [-y|--yes]   # skip the confirmation prompt
#
# Removes everything the self-hosted installer (get-memo-server.sh /
# get-memo-server-beta.sh) puts on the machine:
#   • the systemd --user memo.service unit (if `memo service install` was
#     used) — leaves the machine with no auto-started backend,
#   • every running Memo process started from $MEMO_HOME (manual
#     `memo --headless` runs included),
#   • ~/.memo/ itself (backend, CLI, config, ALL data — memory, sessions,
#     models, skills, WhatsApp store),
#   • the ~/.local/bin/memo PATH wrapper,
#   • PATH lines the installer appended to shell rc files.
# Optionally backs up memory + sessions first (same zip fallback chain as
# uninstall.sh). Desktop-app leftovers (menu entry, icons, Flutter prefs)
# are also cleaned in case a desktop install ever shared this $MEMO_HOME.
set -euo pipefail

# ── colours ──────────────────────────────────────────────────────────────────
BOLD="\033[1m"
GREEN="\033[32m"
CYAN="\033[36m"
YELLOW="\033[33m"
RED="\033[31m"
BLUE="\033[34m"
NC="\033[0m"

ASSUME_YES=false
case "${1:-}" in
    -y|--yes) ASSUME_YES=true ;;
    "")       ;;
    *) echo -e "${RED}Unknown argument: $1${NC}" >&2; exit 1 ;;
esac

MEMO_HOME="${MEMO_HOME:-$HOME/.memo}"

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
echo -e "${BOLD}  Self-hosted server uninstaller${NC}"
echo ""

# ── check install ────────────────────────────────────────────────────────────
UNIT_FILE="$HOME/.config/systemd/user/memo.service"
if [ ! -d "$MEMO_HOME" ] && [ ! -f "$HOME/.local/bin/memo" ] && [ ! -f "$UNIT_FILE" ]; then
    echo -e "${YELLOW}No self-hosted Memo server install found. Nothing to do.${NC}"
    exit 0
fi

echo -e "${BOLD}This will remove:${NC}"
echo -e "  ${RED}▸${NC} $MEMO_HOME/          (backend, CLI, config, ALL data, engine binaries)"
echo -e "  ${RED}▸${NC} ~/.local/bin/memo    (PATH wrapper)"
if [ -f "$UNIT_FILE" ]; then
    echo -e "  ${RED}▸${NC} $UNIT_FILE  (systemd --user service — stops & disables it)"
fi
echo -e "  ${RED}▸${NC} PATH lines added to ~/.bashrc / ~/.zshrc / fish config"
echo -e "  ${RED}▸${NC} desktop leftovers (app-menu entry, icons, Flutter prefs)"
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
    DOCS_DIR=""
    for d in "$HOME/Documents" "$HOME/Belgeler" "$HOME/Documentos" "$HOME/Dokumente"; do
        if [ -d "$d" ]; then DOCS_DIR="$d"; break; fi
    done
    [ -z "$DOCS_DIR" ] && DOCS_DIR="$HOME"

    BACKUP_FILE="$DOCS_DIR/memo-server-data-$(date +%Y%m%d-%H%M%S).zip"
    echo -e "\n${BOLD}Backing up to:${NC} ${GREEN}$BACKUP_FILE${NC}"

    if command -v zip >/dev/null 2>&1; then
        (cd "$MEMO_HOME/data" && zip -qr "$BACKUP_FILE" memory/ sessions/ 2>/dev/null || true)
    elif command -v python3 >/dev/null 2>&1; then
        MEMO_HOME="$MEMO_HOME" BACKUP_FILE="$BACKUP_FILE" python3 -c "
import zipfile, os
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
        echo -e "${GREEN}  Data saved: $BACKUP_FILE${NC}"
    fi
    echo ""
fi

# ── confirm ──────────────────────────────────────────────────────────────────
if ! $ASSUME_YES; then
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
fi

echo ""

# ── systemd --user service, if `memo service install` was used ─────────────
if command -v systemctl >/dev/null 2>&1 && [ -f "$UNIT_FILE" ]; then
    echo -e "${BOLD}Stopping and removing the memo.service unit...${NC}"
    # Prefer the CLI's own uninstall path (cli_service.go) — it does
    # `disable --now` and removes the unit in the right order. Fall back to
    # raw systemctl if the wrapper is already gone.
    if [ -x "$HOME/.local/bin/memo" ]; then
        "$HOME/.local/bin/memo" service uninstall >/dev/null 2>&1 || \
            systemctl --user disable --now memo.service >/dev/null 2>&1 || true
    else
        systemctl --user disable --now memo.service >/dev/null 2>&1 || true
    fi
    rm -f "$UNIT_FILE"
    systemctl --user daemon-reload >/dev/null 2>&1 || true
    echo -e "  ${GREEN}▸${NC} Service removed."
fi

# ── kill running processes started from $MEMO_HOME ──────────────────────────
# Scoped to the actual executable paths on purpose: `memo --headless`
# (RPi-style manual runs), the $MEMO_HOME/binaries engine servers, and the
# CLI all run from these paths. No bare "memo" or whole-$MEMO_HOME pattern
# — something unrelated (e.g. the "memos" notes app) must never be caught,
# and this script's own invoking shell must not match itself.
echo -e "${BOLD}Stopping running Memo processes...${NC}"
for pat in "$MEMO_HOME/memo-backend" "$MEMO_HOME/bin/memo" "$MEMO_HOME/binaries" "$MEMO_HOME/memo"; do
    pkill -TERM -f "$pat" 2>/dev/null || true
done
sleep 1
for pat in "$MEMO_HOME/memo-backend" "$MEMO_HOME/bin/memo" "$MEMO_HOME/binaries" "$MEMO_HOME/memo"; do
    pkill -KILL -f "$pat" 2>/dev/null || true
done
echo -e "  ${GREEN}▸${NC} Processes stopped."

# ── remove files ─────────────────────────────────────────────────────────────
echo -e "${BOLD}Removing files...${NC}"
rm -rf "$MEMO_HOME"
echo -e "  ${RED}▸${NC} $MEMO_HOME/"

rm -f "$HOME/.local/bin/memo"
echo -e "  ${RED}▸${NC} CLI wrapper"

# Desktop leftovers — the self-hosted installer never creates these, but a
# desktop install sharing the same $MEMO_HOME might have. Same cleanups as
# uninstall.sh.
rm -f "$HOME/.local/share/applications/memo.desktop"
rm -f "$HOME/.local/share/icons/hicolor/"*"/apps/memo.png" 2>/dev/null || true
rm -rf "$HOME/.local/share/com.memo.memo_flutter"
rm -f "$HOME/Library/Preferences/com.bugrakaptan.memo.plist" 2>/dev/null || true
echo -e "  ${RED}▸${NC} Desktop leftovers"

# PATH lines the installer appended to shell configs
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
echo -e "${GREEN}${BOLD}  Memo self-hosted server has been uninstalled.${NC}"
echo ""
if $DO_BACKUP && [ -f "$BACKUP_FILE" ]; then
    echo -e "  ${BOLD}Data backup:${NC} $BACKUP_FILE"
fi
echo ""
echo -e "  ${BOLD}Thank you for using Memo. See you again!${NC}"