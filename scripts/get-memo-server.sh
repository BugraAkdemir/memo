#!/usr/bin/env bash
# Memo — self-hosted SERVER-ONLY installer for Linux (x86_64/arm64) / macOS.
#
#   curl -fsSL https://download.bugradev.com/get-memo-server.sh | bash
#
# This is get-memo.sh's headless sibling, not a rewrite: same release
# archives (memo.tar.gz / memo_arm.zip / memo-mac.zip already bundle both
# the backend and the Flutter desktop app — see build-linux.yml/
# build-macos.yml), same $MEMO_HOME layout, same PATH wrapper. The one real
# difference is what gets installed FROM that archive: the Flutter desktop
# binary, its `lib/`/`flutter_assets`, and the app-menu (.desktop) entry are
# deliberately skipped — this script is for the "just run it as a service on
# a box with no display" case (yapacam.md's self-hosted roadmap): a
# Raspberry Pi, a home server, a VPS. You manage it entirely over SSH via
# the `memo` CLI (`memo remote`, `memo config`, `memo service`) and connect
# to it from your desktop/mobile Memo app elsewhere, exactly like a remote
# backend — never from a Flutter window on the server itself.
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
echo -e "${BOLD}  Self-hosted server install — no desktop app, SSH/CLI-managed${NC}"
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
            aarch64|arm64) url="$DOMAIN/memo_arm.zip"; archive_kind="zip" ;;
            *)             url="$DOMAIN/memo.tar.gz" ;;
        esac
        ;;
    Darwin) url="$DOMAIN/memo-mac.zip"; archive_kind="zip" ;;
    *)
        echo -e "${RED}Unsupported OS: $os${NC}" >&2
        exit 1
        ;;
esac

# ── detect mode ──────────────────────────────────────────────────────────────
IS_UPDATE=false
HAD_SYSTEMD_SERVICE=false
if [ -d "$MEMO_HOME" ]; then
    IS_UPDATE=true
    echo -e "${YELLOW}Existing install detected — updating...${NC}"
    # POST /api/shutdown used to be the primary stop mechanism here, but it
    # goes through the same auth gate as every other /api/ route — once the
    # backend is bound to 0.0.0.0 (--lan, which the systemd install prompt
    # below defaults to), an unauthenticated POST just gets a 401 and stops
    # nothing. Worse, plain `curl -s` (no `-f`) treats that 401 as success
    # at the transport level, so the old `if curl ...; then echo "Stopped
    # running backend"` check printed a false positive while the old
    # process kept running — the update below would replace the binary
    # *on disk* out from under a process that never actually stopped, so
    # the "update" silently never took effect until something else
    # restarted the service. If systemd --user owns it, stop it the way
    # systemd expects — no HTTP credential needed for that.
    if command -v systemctl >/dev/null 2>&1 && systemctl --user is-enabled memo.service >/dev/null 2>&1; then
        HAD_SYSTEMD_SERVICE=true
        echo -e "  ${GREEN}▸${NC} Stopping systemd --user service"
        systemctl --user stop memo.service >/dev/null 2>&1 || true
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
        # Archives don't reliably preserve the execute bit (the arm64
        # build_releases_arm.sh bundle used to ship llama-server as 0644,
        # which made every model start fail with EACCES) — force it.
        find "$MEMO_HOME/binaries" -name "llama-server*" -exec chmod +x {} \;
        find "$MEMO_HOME/binaries" -name "*.so*" -exec chmod +x {} \;
    elif [ ! -d "$MEMO_HOME/binaries" ]; then
        echo -e "  ${GREEN}▸${NC} Engine binaries (llama.cpp + vec0)"
        cp -r "$src/binaries" "$MEMO_HOME/binaries"
        find "$MEMO_HOME/binaries" -name "llama-server*" -exec chmod +x {} \;
        find "$MEMO_HOME/binaries" -name "*.so*" -exec chmod +x {} \;
    fi
fi

# Backend
if [ -f "$src/memo-backend" ]; then
    echo -e "  ${GREEN}▸${NC} Backend"
    if $IS_UPDATE; then rm -f "$MEMO_HOME/memo-backend"; fi
    cp -f "$src/memo-backend" "$MEMO_HOME/memo-backend"
    chmod +x "$MEMO_HOME/memo-backend"
fi

# Deliberately NOT copied, unlike get-memo.sh: memo_flutter, lib/,
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
    echo -e "${GREEN}${BOLD}  Update complete!${NC}"
    echo ""
    echo -e "  ${BOLD}Updated:${NC}    backend, engine, CLI"
    echo -e "  ${BOLD}Preserved:${NC}  config, memory, models, sessions, providers, skills, systemd unit (if any)"
else
    echo -e "${GREEN}${BOLD}  Installation complete!${NC}"
fi
echo ""

# ── systemd: restart after an update, or offer to set it up on fresh
# installs (Linux only; skipped on macOS — launchd isn't wired up yet, see
# yapacam.md Faz 3) ──────────────────────────────────────────────────────────
if [ "$os" = "Linux" ] && command -v systemctl >/dev/null 2>&1 && $IS_UPDATE && $HAD_SYSTEMD_SERVICE; then
    echo -e "${BOLD}Restarting the systemd --user service with the updated binary...${NC}"
    systemctl --user daemon-reload >/dev/null 2>&1 || true
    if systemctl --user restart memo.service >/dev/null 2>&1; then
        echo -e "  ${GREEN}▸${NC} Restarted — now running the updated version."
    else
        echo -e "  ${YELLOW}⚠${NC}  Could not restart it automatically — run this yourself:"
        echo -e "    ${CYAN}systemctl --user restart memo${NC}"
    fi
    echo ""
elif [ "$os" = "Linux" ] && command -v systemctl >/dev/null 2>&1 && ! $IS_UPDATE; then
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

# ── tell the user exactly where to point their browser (BUG-ONB1) ──────────
# Inspects the actual persisted unit file (if any) instead of tracking
# shell state across the branches above — this correctly covers every path:
# a fresh --lan install, an update that restarted an existing --lan
# service, a loopback-only install, and "no service configured at all"
# (nothing printed in that last case — there's no running backend yet to
# point at). Deliberately not npm/hostname -I based: `ip route get` returns
# the real source IP for the box's default outbound route, which sidesteps
# Docker/Podman/libvirt bridge IPs entirely (they're never used for
# outbound routing) instead of listing every interface and leaving the
# user to guess which one is real.
UNIT_FILE="$HOME/.config/systemd/user/memo.service"
if [ -f "$UNIT_FILE" ] && command -v systemctl >/dev/null 2>&1 && systemctl --user is-active --quiet memo.service 2>/dev/null; then
    unit_port="$(sed -n 's/.*--port \([0-9]*\).*/\1/p' "$UNIT_FILE" | head -1)"
    [ -n "$unit_port" ] || unit_port="8090"
    echo ""
    if grep -q -- '--lan' "$UNIT_FILE"; then
        lan_ip="$(ip -4 route get 1.1.1.1 2>/dev/null | sed -n 's/.* src \([0-9.]*\).*/\1/p')"
        if [ -n "$lan_ip" ]; then
            echo -e "  ${GREEN}${BOLD}➜ Open http://$lan_ip:$unit_port in your browser to get started${NC}"
        else
            echo -e "  ${YELLOW}Could not auto-detect this device's LAN IP — find it with 'ip addr' or"
            echo -e "  'hostname -I', then open http://<that-ip>:$unit_port in your browser.${NC}"
        fi
    else
        echo -e "  ${GREEN}${BOLD}➜ Open http://127.0.0.1:$unit_port in your browser on this machine to get started${NC}"
    fi
fi

echo ""
echo -e "  ${BOLD}Manage over SSH:${NC}"
echo -e "    ${CYAN}memo service status${NC}          — is it running?"
echo -e "    ${CYAN}memo service restart${NC}         — restart it (always with --user, see below)"
echo -e "    ${CYAN}memo remote status${NC}           — auth mode, addresses, warnings"
echo -e "    ${CYAN}memo config get/set <key>${NC}    — edit config.yaml from the command line"
echo -e "    ${YELLOW}Note:${NC} the service is a systemd ${CYAN}--user${NC} unit (no root/sudo) — if you use"
echo -e "    plain systemctl yourself, include ${CYAN}--user${NC} too (bare/sudo systemctl targets"
echo -e "    system-wide units and will fail with 'Unit memo.service not found')."
echo -e "  ${BOLD}Guide:${NC}     ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""

if command -v memo >/dev/null 2>&1; then
    echo -e "  Run ${CYAN}memo --help${NC} to see everything."
else
    echo -e "  ${YELLOW}Open a new terminal (or restart your shell) and run 'memo --help'.${NC}"
fi

echo ""
echo -e "  ${BOLD}Thank you for choosing Memo!${NC}"
