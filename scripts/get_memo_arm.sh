#!/usr/bin/env bash
# Memo — one-line installer / updater for Linux arm64 (Raspberry Pi, ARM
# NAS/CasaOS boxes, ARM cloud servers).
#
#   curl -fsSL https://download.bugradev.com/get_memo_arm.sh | bash
#
# This is get-memo.sh's arm64 sibling, not a rewrite — same install/update
# logic, same $MEMO_HOME layout, same PATH wrapper and desktop entry. The
# only real differences: no OS branch (Linux arm64 only, x86_64/macOS users
# should use get-memo.sh instead) and the archive is memo_arm.zip (.zip, not
# .tar.gz — matches how this specific package is produced, unzip is more
# universally preinstalled on minimal ARM board/NAS images than tar+gzip
# extras sometimes are).
#
# Auto-detects existing installs:
#   • Fresh install — seeds configs, copies engine binaries, sets up PATH & app menu
#   • Update        — refreshes all binaries, engine included (config & data preserved)
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
echo -e "${BOLD}  Local-first, privacy-focused AI assistant (Linux arm64)${NC}"
echo -e "  ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""

# ── arch guard ───────────────────────────────────────────────────────────────
DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"
MEMO_HOME="$HOME/.memo"

os="$(uname -s)"
arch="$(uname -m)"
if [ "$os" != "Linux" ]; then
    echo -e "${RED}This installer is Linux arm64-only. On macOS/other, use get-memo.sh instead.${NC}" >&2
    exit 1
fi
case "$arch" in
    aarch64|arm64) ;;
    *)
        echo -e "${RED}This installer is for arm64 (detected: $arch). Use get-memo.sh for x86_64.${NC}" >&2
        exit 1
        ;;
esac

url="$DOMAIN/memo_arm.zip"

# ── detect mode ──────────────────────────────────────────────────────────────
IS_UPDATE=false
if [ -d "$MEMO_HOME" ]; then
    IS_UPDATE=true
    echo -e "${YELLOW}Existing install detected — updating...${NC}"
    # Stop running instance so binary files aren't locked
    if curl -s -X POST "http://localhost:8090/api/shutdown" --max-time 3 >/dev/null 2>&1; then
        echo -e "  ${GREEN}▸${NC} Stopped running backend"
    else
        pkill -TERM -f "memo-backend" 2>/dev/null || true
        pkill -TERM -f "llama-server" 2>/dev/null || true
    fi
    sleep 2
else
    echo -e "${BOLD}Fresh install — setting up...${NC}"
fi

# ── dependencies ─────────────────────────────────────────────────────────────
for bin in curl unzip; do
    command -v "$bin" >/dev/null 2>&1 || {
        echo -e "${RED}Error: '$bin' not found. Install it and try again.${NC}" >&2
        exit 1
    }
done

# ── download ─────────────────────────────────────────────────────────────────
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

archive="$work_dir/$(basename "$url")"
echo ""
echo -e "${BOLD}Downloading:${NC} $url"
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
unzip -o "$archive" -d "$work_dir"

# Don't assume the archive's top-level folder is literally named "$APP_NAME"
# — unlike get-memo.sh's memo.tar.gz (always produced by build_releases.sh,
# always "Memo/"), memo_arm.zip right now is hand-assembled (CI's build
# artifact plus binaries/linux/cpu-arm64/ merged in by hand), so its wrapper
# folder can be named anything. If memo-backend isn't directly in work_dir,
# use whatever single subdirectory unzip actually created instead of
# guessing a name.
if [ -f "$work_dir/memo-backend" ]; then
    src="$work_dir"
else
    src="$(find "$work_dir" -mindepth 1 -maxdepth 1 -type d | head -n1)"
    [ -n "$src" ] || src="$work_dir"
fi

# ── install / update ─────────────────────────────────────────────────────────
echo ""
echo -e "${BOLD}$( $IS_UPDATE && echo 'Updating...' || echo 'Installing...' )${NC}"

mkdir -p "$MEMO_HOME/bin" "$MEMO_HOME/config" \
    "$MEMO_HOME/data/models" "$MEMO_HOME/data/memory" "$MEMO_HOME/data/sessions" \
    "$MEMO_HOME/data/agent-backups" "$MEMO_HOME/data/skills" "$MEMO_HOME/data/whatsapp"

# Engine binaries — update always refreshes these
if [ -d "$src/binaries" ]; then
    if $IS_UPDATE || [ ! -d "$MEMO_HOME/binaries" ]; then
        echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
        rm -rf "$MEMO_HOME/binaries"
        cp -r "$src/binaries" "$MEMO_HOME/binaries"
        # Defensive flatten: internal/llama's binary resolver only ever
        # looks under binaries/linux/{cpu,nvidia,amd}/ — it has no idea an
        # architecture-suffixed "cpu-arm64" exists (that name only exists
        # as a *source*-side staging convention, see download_binaries.sh/
        # build_releases_arm.sh). A hand-assembled archive that still has
        # binaries sitting under cpu-arm64/ would otherwise install fine
        # and then silently never find llama-server at runtime.
        if [ -d "$MEMO_HOME/binaries/linux/cpu-arm64" ] && [ ! -d "$MEMO_HOME/binaries/linux/cpu" ]; then
            mv "$MEMO_HOME/binaries/linux/cpu-arm64" "$MEMO_HOME/binaries/linux/cpu"
        fi
    fi
else
    echo -e "  ${YELLOW}⚠${NC}  No bundled engine binaries in this archive — llama-server/vec0 will need to be installed separately (Settings → AI Engine)."
fi

# Backend
if [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} Backend"
    if $IS_UPDATE; then rm -f "$MEMO_HOME/memo-backend"; fi
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# Flutter frontend
if [ -f "$src/memo_flutter" ]; then
    echo -e "  ${GREEN}▸${NC} Desktop app"
    if $IS_UPDATE; then
        rm -rf "$MEMO_HOME/memo_flutter" "$MEMO_HOME/lib" 2>/dev/null || true
        rm -f "$MEMO_HOME/data/icudtl.dat" 2>/dev/null || true
        rm -rf "$MEMO_HOME/data/flutter_assets" 2>/dev/null || true
    fi
    cp -f "$src/memo_flutter" "$MEMO_HOME/memo_flutter"
    chmod +x "$MEMO_HOME/memo_flutter"
    if [ -d "$src/lib" ]; then
        cp -rf "$src/lib" "$MEMO_HOME/lib"
    fi
    [ -f "$src/data/icudtl.dat" ]    && cp -f "$src/data/icudtl.dat" "$MEMO_HOME/data/icudtl.dat"
    [ -d "$src/data/flutter_assets" ] && cp -rf "$src/data/flutter_assets" "$MEMO_HOME/data/flutter_assets"
fi

# Runner script
if [ -f "$src/run_memo.sh" ]; then
    echo -e "  ${GREEN}▸${NC} Launcher"
    if $IS_UPDATE; then rm -f "$MEMO_HOME/run_memo.sh"; fi
    cp -f "$src/run_memo.sh" "$MEMO_HOME/run_memo.sh"
    chmod +x "$MEMO_HOME/run_memo.sh"
fi

# CLI binary
if [ -f "$src/memo" ]; then
    echo -e "  ${GREEN}▸${NC} CLI"
    rm -f "$MEMO_HOME/bin/memo"
    cp -f "$src/memo" "$MEMO_HOME/bin/memo"
elif [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} CLI (from memo-backend — this archive has no separate memo binary)"
    cp -f "$src/memo-backend" "$MEMO_HOME/bin/memo"
else
    echo -e "${RED}Error: memo binary not found in archive.${NC}" >&2
    exit 1
fi
chmod +x "$MEMO_HOME/bin/memo"

# ── config seeding (only on fresh install) ──────────────────────────────────
if ! $IS_UPDATE; then
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
fi

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

# ── systemd service (headless, auto-start on boot) ──────────────────────────
# ARM boards/NAS/servers are headless — there's no display for the bundled
# Flutter GUI (memo_flutter, still in the archive but unused here) and no app
# menu to put a launcher icon in. What this install actually needs is the
# backend running unattended: --headless (no REPL waiting on a TTY that will
# never exist) + --lan (binds 0.0.0.0 so the built-in web UI and any remote
# client can actually reach it — 127.0.0.1 is invisible from outside the box).
# A user systemd unit (not system-wide — matches the rest of this installer
# never asking for root) plus lingering so it survives reboots with no one
# logged in, which is the entire point on a box you SSH into once and forget.
SERVICE_PORT=8090
echo -e "  ${GREEN}▸${NC} Systemd service (port $SERVICE_PORT, --lan)"
if command -v systemctl >/dev/null 2>&1; then
    mkdir -p "$HOME/.config/systemd/user"
    cat > "$HOME/.config/systemd/user/memo.service" <<SERVICE
[Unit]
Description=Memo — local-first AI assistant (headless)
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
Environment=MEMO_DATA_DIR=%h/.memo/data
WorkingDirectory=%h/.memo
ExecStart=%h/.memo/memo-backend --headless --port $SERVICE_PORT --lan
Restart=on-failure
RestartSec=3

[Install]
WantedBy=default.target
SERVICE
    # Each systemctl/loginctl call can legitimately fail (no user session
    # bus in some minimal/container environments, no polkit rule for
    # lingering, etc.) — none of that should abort the whole install under
    # `set -e`, so every check here is explicit rather than relying on
    # bare command status.
    if systemctl --user daemon-reload 2>/dev/null \
        && systemctl --user enable memo.service >/dev/null 2>&1 \
        && systemctl --user restart memo.service 2>/dev/null; then
        # restart (not start): on an update the old process needs to
        # actually exit and relaunch to pick up the binary that was just
        # overwritten, not just "already active, nothing to do".
        if loginctl enable-linger "$(whoami)" >/dev/null 2>&1; then
            echo -e "    ${GREEN}✓${NC} Enabled — starts on boot even with no one logged in"
        else
            echo -e "    ${YELLOW}⚠${NC}  Service enabled, but couldn't turn on lingering (needs root)."
            echo -e "       Without it, this only runs while you're logged in / SSH'd in."
            echo -e "       Run this once to fix that:"
            echo -e "       ${CYAN}sudo loginctl enable-linger $(whoami)${NC}"
        fi
    else
        echo -e "    ${YELLOW}⚠${NC}  Unit written but couldn't start it via 'systemctl --user'"
        echo -e "       (no user session bus available right now?). Start it by hand:"
        echo -e "       ${CYAN}$MEMO_HOME/memo-backend --headless --port $SERVICE_PORT --lan${NC}"
    fi
else
    echo -e "  ${YELLOW}⚠${NC}  systemd not found — skipping service setup."
    echo -e "     Start it by hand: ${CYAN}$MEMO_HOME/memo-backend --headless --port $SERVICE_PORT --lan${NC}"
fi

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
if $IS_UPDATE; then
    echo -e "${GREEN}${BOLD}  Update complete!${NC}"
    echo ""
    echo -e "  ${BOLD}Updated:${NC}    binaries, engine, service, CLI"
    echo -e "  ${BOLD}Preserved:${NC}  config, memory, models, sessions, providers, skills"
else
    echo -e "${GREEN}${BOLD}  Installation complete!${NC}"
fi
echo ""
echo -e "  ${BOLD}Web UI:${NC}    ${CYAN}http://$(hostname -I 2>/dev/null | awk '{print $1}'):$SERVICE_PORT${NC} (or this box's IP)"
echo -e "  ${BOLD}Terminal:${NC}  ${CYAN}memo${NC} (attaches to the running service instead of starting a second one)"
echo -e "  ${BOLD}Guide:${NC}     ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""
echo -e "  This box now requires a token on every request (LAN mode). Get it with:"
echo -e "  ${CYAN}journalctl --user -u memo -n 50 --no-pager | grep 'X-Memo-Token'${NC}"
echo -e "  The web UI's first screen will ask for it."

echo ""
echo -e "  ${BOLD}Thank you for choosing Memo!${NC}"
