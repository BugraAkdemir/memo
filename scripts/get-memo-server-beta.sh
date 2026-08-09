#!/usr/bin/env bash
# Memo BETA — self-hosted SERVER-ONLY installer for Linux (x86_64/arm64) / macOS.
#
#   curl -fsSL https://download.bugradev.com/get-memo-server-beta.sh | bash
#
# This is get-memo-server.sh's beta sibling, not a rewrite — same install/
# update logic, same $MEMO_HOME layout, same PATH wrapper, same "skip the
# desktop app entirely" behavior. The one difference is which R2 archives it
# pulls: memo_beta.tar.gz / memo_arm_beta.zip / memo-mac_beta.zip instead of
# the stable memo.tar.gz / memo_arm.zip / memo-mac.zip.
#
# Why this matters here specifically: build-linux.yml/build-macos.yml/
# build-windows.yml overwrite the *_beta.* filenames on every single push to
# main, but only overwrite the stable (no-suffix) filenames when a real
# `vX.Y.Z` tag is pushed (see the memo-release skill). Faz 1-4 of
# yapacam.md's self-hosted roadmap (Docker/ARM CI, the 4-mode auth system,
# `memo config`/`memo remote`/`memo service`) all currently live only on
# `main` — no tagged release includes any of it yet. This script is
# therefore the only way to actually test any of that self-hosted work on a
# real device (a Raspberry Pi, in particular) before the next real release.
#
# Auto-detects existing installs:
#   • Fresh install — seeds configs, copies engine binaries, sets up PATH
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

clear

# ── banner ───────────────────────────────────────────────────────────────────
echo -e "${CYAN}${BOLD}"
echo "  __  __ _____ __  __  ___  "
echo " |  \\/  | ____|  \\/  |/ _ \\ "
echo " | |\\/| |  _| | |\\/| | | | |"
echo " | |  | | |___| |  | | |_| |"
echo " |_|  |_|_____|_|  |_|\\___/ "
echo -e "${NC}"
echo -e "${BOLD}  Self-hosted server install — no desktop app, SSH/CLI-managed ${YELLOW}(BETA)${NC}"
echo -e "  ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""

# ── os/arch detection ────────────────────────────────────────────────────────
DOMAIN="https://download.bugradev.com"
APP_NAME="Memo"
MEMO_HOME="$HOME/.memo"

os="$(uname -s)"
arch="$(uname -m)"
archive_kind="tar" # tar for .tar.gz, zip for .zip
case "$os" in
    Linux)
        case "$arch" in
            aarch64|arm64) url="$DOMAIN/memo_arm_beta.zip"; archive_kind="zip" ;;
            *)             url="$DOMAIN/memo_beta.tar.gz" ;;
        esac
        ;;
    Darwin) url="$DOMAIN/memo-mac_beta.zip"; archive_kind="zip" ;;
    *)
        echo -e "${RED}Unsupported OS: $os${NC}" >&2
        exit 1
        ;;
esac

# ── detect mode ──────────────────────────────────────────────────────────────
IS_UPDATE=false
if [ -d "$MEMO_HOME" ]; then
    IS_UPDATE=true
    echo -e "${YELLOW}Existing install detected — updating...${NC}"
    if curl -s -X POST "http://localhost:8090/api/shutdown" --max-time 3 >/dev/null 2>&1; then
        echo -e "  ${GREEN}▸${NC} Stopped running backend"
    else
        pkill -TERM -f "memo-backend" 2>/dev/null || true
    fi
    sleep 2
else
    echo -e "${BOLD}Fresh install — setting up...${NC}"
fi

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
echo ""
echo -e "${BOLD}Downloading:${NC} $url"
curl -fSL -# -o "$archive" "$url"
echo ""

# ── extract ──────────────────────────────────────────────────────────────────
echo -e "${BOLD}Extracting...${NC}"
if [ "$archive_kind" = "zip" ]; then
    command -v unzip >/dev/null 2>&1 || {
        echo -e "${RED}Error: 'unzip' not found.${NC}" >&2
        exit 1
    }
    unzip -o "$archive" -d "$work_dir"
else
    tar xzf "$archive" -C "$work_dir"
fi

src="$work_dir/$APP_NAME"
[ -d "$src" ] || src="$work_dir"

# ── install / update (backend + CLI + engine only — no desktop app) ────────
echo ""
echo -e "${BOLD}$( $IS_UPDATE && echo 'Updating...' || echo 'Installing...' )${NC}"

mkdir -p "$MEMO_HOME/bin" "$MEMO_HOME/config" \
    "$MEMO_HOME/data/models" "$MEMO_HOME/data/memory" "$MEMO_HOME/data/sessions" \
    "$MEMO_HOME/data/agent-backups" "$MEMO_HOME/data/skills" "$MEMO_HOME/data/whatsapp"

# Engine binaries — update always refreshes these (GPU drivers may differ)
if [ -d "$src/binaries" ]; then
    if $IS_UPDATE; then
        echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
        rm -rf "$MEMO_HOME/binaries"
        cp -r "$src/binaries" "$MEMO_HOME/binaries"
    elif [ ! -d "$MEMO_HOME/binaries" ]; then
        echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
        cp -r "$src/binaries" "$MEMO_HOME/binaries"
    fi
fi

# Backend
if [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} Backend"
    if $IS_UPDATE; then rm -f "$MEMO_HOME/memo-backend"; fi
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# Deliberately NOT copied, unlike get-memo-beta.sh: memo_flutter, lib/,
# data/icudtl.dat, data/flutter_assets, run_memo.sh (the desktop launcher),
# the .desktop app-menu entry, and its icon. This is the entire point of
# this script — a server has no display to show a Flutter window on.

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
export PATH="$HOME/.local/bin:$PATH"

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
if $IS_UPDATE; then
    echo -e "${GREEN}${BOLD}  Update complete! ${YELLOW}(BETA)${NC}"
    echo ""
    echo -e "  ${BOLD}Updated:${NC}    backend, engine, CLI"
    echo -e "  ${BOLD}Preserved:${NC}  config, memory, models, sessions, providers, skills, systemd unit (if any)"
else
    echo -e "${GREEN}${BOLD}  Installation complete! ${YELLOW}(BETA)${NC}"
fi
echo ""

# ── systemd offer (Linux only; skipped on macOS — launchd isn't wired up
# yet, see yapacam.md Faz 3) ─────────────────────────────────────────────────
if [ "$os" = "Linux" ] && command -v systemctl >/dev/null 2>&1 && ! $IS_UPDATE; then
    echo -e "${BOLD}Run Memo as a background service (systemd --user)?${NC}"
    echo -e "It'll auto-restart if it crashes, and can be set to survive a reboot."
    echo ""
    answer=""
    # Reading from /dev/tty, not stdin: this script is normally invoked as
    # "curl ... | bash", which means stdin is the piped script itself, not
    # the terminal — a plain "read" here would silently consume none of the
    # (already-exhausted) pipe and immediately return empty. Redirecting
    # from /dev/tty directly can still fail (no controlling terminal at
    # all — CI, a non-interactive provisioning script), so both the
    # redirection and the read itself are allowed to fail silently; either
    # way "answer" just stays empty and the default (--lan, below) applies.
    read -r -p "Bind to 0.0.0.0 so other devices can reach it? [Y/n] " answer 2>/dev/null < /dev/tty || true
    lan_flag="--lan"
    if [ "$answer" = "n" ] || [ "$answer" = "N" ]; then
        lan_flag=""
    fi
    if "$HOME/.local/bin/memo" service install $lan_flag; then
        echo ""
        echo -e "  ${GREEN}▸${NC} Service installed and started."
        if [ -n "$lan_flag" ]; then
            echo -e "  ${YELLOW}Default auth mode is 'token' — a device token was just generated,"
            echo -e "  but once bound to 0.0.0.0 the API requires a credential for every"
            echo -e "  request, including local CLI calls — so it can't be read back via"
            echo -e "  'memo remote status' (that itself would 401 without a token yet)."
            echo -e "  Find it in the service log instead:"
            echo -e "    ${CYAN}journalctl --user -u memo.service --no-pager | grep -i token${NC}"
            echo -e "  Or skip tokens entirely and set a password:"
            echo -e "    ${CYAN}memo remote set-mode password --username you --password ...${NC}"
        fi
        echo -e "  For it to also start on boot without a login session: ${CYAN}loginctl enable-linger $(whoami)${NC}"
    else
        echo -e "  ${YELLOW}Skipped — you can run this manually anytime: memo service install${NC}"
    fi
    echo ""
fi

echo -e "  ${BOLD}Manage over SSH:${NC}"
echo -e "    ${CYAN}memo service status${NC}          — is it running?"
echo -e "    ${CYAN}memo remote status${NC}           — auth mode, addresses, warnings"
echo -e "    ${CYAN}memo config get/set <key>${NC}    — edit config.yaml from the command line"
echo -e "  ${BOLD}Guide:${NC}     ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""

if command -v memo >/dev/null 2>&1; then
    echo -e "  Run ${CYAN}memo --help${NC} to see everything."
else
    echo -e "  ${YELLOW}Open a new terminal (or restart your shell) and run 'memo --help'.${NC}"
fi

echo ""
echo -e "  ${BOLD}Thank you for trying the Memo BETA!${NC}"
