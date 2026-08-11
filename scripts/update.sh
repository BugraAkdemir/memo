#!/usr/bin/env bash
# Memo — updater for Linux / macOS.
#
#   curl -fsSL https://download.bugradev.com/update.sh | bash
#
# Downloads the latest build and refreshes all binaries while keeping your
# data intact — configs, memory, models, sessions, providers are never touched.
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
echo -e "  ${BOLD}Updater${NC}"
echo ""

# ── check installed ──────────────────────────────────────────────────────────
MEMO_HOME="$HOME/.memo"

if [ ! -d "$MEMO_HOME" ]; then
    echo -e "${YELLOW}Memo is not installed. Run the installer first:${NC}"
    echo -e "  ${CYAN}curl -fsSL https://download.bugradev.com/get-memo.sh | bash${NC}"
    exit 1
fi

# ── os detection ─────────────────────────────────────────────────────────────
DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"

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
        echo -e "${RED}Error: '$bin' not found.${NC}" >&2
        exit 1
    }
done

# ── download ─────────────────────────────────────────────────────────────────
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

archive="$work_dir/$(basename "$url")"
echo -e "${BOLD}Downloading latest build:${NC} $url"
# Cache-busting query param: an ISP-level transparent HTTP cache along the
# way (not Cloudflare itself — confirmed live: the same URL with vs.
# without this query string returned two different binaries from the
# same network path at the same moment) can keep serving a stale archive
# indefinitely even long after a fresh CI upload. $url itself stays
# unbusted (used for archive's local filename and the printed message
# above) — only the actual request is.
curl -fSL -# -o "$archive" "${url}?cachebust=$(date +%s%N)"
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

# ── stop running backend ─────────────────────────────────────────────────────
echo -e "${BOLD}Stopping running instance...${NC}"
if curl -s -X POST "http://localhost:8090/api/shutdown" --max-time 3 >/dev/null 2>&1; then
    echo -e "  ${GREEN}▸${NC} Backend stopped gracefully"
else
    pkill -TERM -f "memo-backend" 2>/dev/null || true
    pkill -TERM -f "llama-server" 2>/dev/null || true
    echo -e "  ${YELLOW}▸${NC} Stale processes terminated"
fi
sleep 2

# ── update binaries (preserves data) ─────────────────────────────────────────
echo ""
echo -e "${BOLD}Updating...${NC}"

# Engine binaries — always refresh so GPU drivers match the new build
if [ -d "$src/binaries" ]; then
    echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
    rm -rf "$MEMO_HOME/binaries"
    cp -r "$src/binaries" "$MEMO_HOME/binaries"
    # Archives don't reliably preserve the execute bit (the arm64
    # build_releases_arm.sh bundle used to ship llama-server as 0644,
    # which made every model start fail with EACCES) — force it.
    find "$MEMO_HOME/binaries" -name "llama-server*" -exec chmod +x {} \;
    find "$MEMO_HOME/binaries" -name "*.so*" -exec chmod +x {} \;
fi

# Backend
if [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} Backend"
    rm -f "$MEMO_HOME/memo-backend"
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# Flutter frontend
if [ -f "$src/memo_flutter" ]; then
    echo -e "  ${GREEN}▸${NC} Desktop app"
    rm -rf "$MEMO_HOME/memo_flutter" "$MEMO_HOME/lib" 2>/dev/null || true
    rm -f "$MEMO_HOME/data/icudtl.dat" 2>/dev/null || true
    rm -rf "$MEMO_HOME/data/flutter_assets" 2>/dev/null || true
    cp -f "$src/memo_flutter" "$MEMO_HOME/memo_flutter"
    chmod +x "$MEMO_HOME/memo_flutter"
    [ -d "$src/lib" ]                && cp -rf "$src/lib" "$MEMO_HOME/lib"
    [ -f "$src/data/icudtl.dat" ]    && cp -f "$src/data/icudtl.dat" "$MEMO_HOME/data/icudtl.dat"
    [ -d "$src/data/flutter_assets" ] && cp -rf "$src/data/flutter_assets" "$MEMO_HOME/data/flutter_assets"
fi

# Runner script
if [ -f "$src/run_memo.sh" ]; then
    echo -e "  ${GREEN}▸${NC} Launcher"
    rm -f "$MEMO_HOME/run_memo.sh"
    cp -f "$src/run_memo.sh" "$MEMO_HOME/run_memo.sh"
    chmod +x "$MEMO_HOME/run_memo.sh"
fi

# CLI binary + wrapper
if [ -f "$src/memo" ]; then
    echo -e "  ${GREEN}▸${NC} CLI"
    rm -f "$MEMO_HOME/bin/memo"
    cp -f "$src/memo" "$MEMO_HOME/bin/memo"
    chmod +x "$MEMO_HOME/bin/memo"
fi

# ── app menu icon ────────────────────────────────────────────────────────────
ICON_DIR="$HOME/.local/share/icons/hicolor/256x256/apps"
mkdir -p "$ICON_DIR"
if [ -f "$src/icon.png" ]; then
    cp -f "$src/icon.png" "$ICON_DIR/memo.png"
elif [ -f "$src/data/flutter_assets/assets/icon.png" ]; then
    cp -f "$src/data/flutter_assets/assets/icon.png" "$ICON_DIR/memo.png"
fi

command -v update-desktop-database >/dev/null 2>&1 && \
    update-desktop-database "$HOME/.local/share/applications" 2>/dev/null || true

# ── preserved ────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}  Update complete!${NC}"
echo ""
echo -e "  ${BOLD}Updated:${NC}     binaries, engine, launcher, CLI"
echo -e "  ${BOLD}Preserved:${NC}   config, memory, models, sessions, providers, skills, WhatsApp data"
echo -e "  ${BOLD}Guide:${NC}      ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""
echo -e "  Run ${CYAN}memo${NC} to launch the updated version."
echo ""
echo -e "  ${BOLD}Thank you for using Memo!${NC}"
