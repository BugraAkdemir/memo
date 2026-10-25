#!/usr/bin/env bash
# Memo — updater for Linux / macOS.
#
#   curl -fsSL https://data.memocpp.com/update.sh | bash
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
    echo -e "  ${CYAN}curl -fsSL https://data.memocpp.com/get-memo.sh | bash${NC}"
    exit 1
fi

# ── os detection ─────────────────────────────────────────────────────────────
DOMAIN="https://data.memocpp.com"
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

# Engines: the archive carries the engines of the last Memo release, and newer ones may have
# been published since (llama.cpp, whisper.cpp, the Subscriptions connector). Refresh any older
# engine folder right now, while installing — never fatal, the bundled engine stays on failure.
# Set MEMO_SKIP_ENGINE_REFRESH=1 to skip.
# >>> memo-engine-refresh (embedded from scripts/engine_refresh.sh — do not edit in the installers)
# 0 when $1 (installed) is OLDER than $2 (latest). llama builds are bNNNN (numeric);
# everything else is a dotted version (v8.0.25, 1.8.6).
memo_engine_older() {
    local a="${1#[bBvV]}" b="${2#[bBvV]}"
    [ -n "$a" ] && [ -n "$b" ] && [ "$a" != "$b" ] || return 1
    case "$a$b" in
        *[!0-9]*) ;;
        *) [ "$a" -lt "$b" ]; return ;;
    esac
    [ "$(printf '%s\n%s\n' "$a" "$b" | sort -V | head -n1)" = "$a" ]
}

# First string value of "$1" in the flattened JSON on stdin ("#" delimits sed: keys hold "/").
memo_json_str() {
    sed -n 's#.*"'"$1"'"[[:space:]]*:[[:space:]]*"\([^"]*\)".*#\1#p' | head -n1
}

# The {...} body of object "$1" in the flattened JSON on stdin.
memo_json_obj() {
    sed -n 's#.*"'"$1"'"[[:space:]]*:[[:space:]]*{\([^}]*\)}.*#\1#p' | head -n1
}

memo_sha256() {
    if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1
    else shasum -a 256 "$1" | cut -d' ' -f1; fi
}

# Run the freshly unpacked engine once; 0 when it starts.
memo_engine_runs() { # <component> <dir>
    local comp="$1" dir="$2" out="" t=""
    command -v timeout >/dev/null 2>&1 && t="timeout 20"
    case "$comp" in
        llama)
            [ -f "$dir/llama-server" ] || return 1
            out="$(cd "$dir" && LD_LIBRARY_PATH="$dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" $t ./llama-server --version 2>&1)" || return 1
            printf '%s' "$out" | grep -q "version:" ;;
        whisper)
            [ -f "$dir/whisper-server" ] || return 1
            out="$(cd "$dir" && LD_LIBRARY_PATH="$dir${LD_LIBRARY_PATH:+:$LD_LIBRARY_PATH}" $t ./whisper-server --help 2>&1)" || true
            printf '%s' "$out" | grep -qi "usage" ;;
        cliproxy)
            [ -f "$dir/cli-proxy-api" ] || return 1
            out="$($t "$dir/cli-proxy-api" -h 2>&1)" || true
            printf '%s' "$out" | grep -qi "usage" ;;
        *) return 1 ;;
    esac
}

# memo_refresh_engines <binaries-dir>  — never fatal, never touches anything it cannot prove.
memo_refresh_engines() {
    local bin="$1" arch beacon comps comp latest dir folder marker prefix installed key
    local manifest entry url want got tmp x relevant gpu_amd gpu_nvidia gpus listing
    [ "${MEMO_SKIP_ENGINE_REFRESH:-0}" = "1" ] && return 0
    [ "$(uname -s)" = "Linux" ] || return 0
    [ -d "$bin/linux" ] || return 0
    command -v curl >/dev/null 2>&1 && command -v tar >/dev/null 2>&1 || return 0

    arch="$(uname -m)"
    case "$arch" in x86_64|amd64|aarch64|arm64) ;; *) return 0 ;; esac
    gpu_amd=false; gpu_nvidia=false
    if command -v nvidia-smi >/dev/null 2>&1; then gpu_nvidia=true; fi
    if command -v lspci >/dev/null 2>&1; then
        gpus="$(lspci 2>/dev/null | grep -iE 'vga|3d|display' | tr '[:upper:]' '[:lower:]' || true)"
        case "$gpus" in *nvidia*) gpu_nvidia=true ;; esac
        case "$gpus" in *amd*|*radeon*) gpu_amd=true ;; esac
    fi

    beacon="$(curl -fsSL --max-time 15 "${MEMO_BEACON_URL:-https://version-zeta.vercel.app/version.json}?cachebust=$(date +%s)" 2>/dev/null | tr -d '\n\r')" || beacon=""
    comps="$(printf '%s' "$beacon" | memo_json_obj components)"
    if [ -z "$comps" ]; then return 0; fi   # nothing announced: the bundled engines are the newest

    for comp in llama whisper cliproxy; do
        latest="$(printf '%s' "$comps" | memo_json_str "$comp")"
        [ -n "$latest" ] || continue
        case "$comp" in
            llama)    marker="LLAMA_VERSION";   prefix="llama-server" ;;
            whisper)  marker="WHISPER_VERSION"; prefix="whisper-server" ;;
            cliproxy) marker="VERSION";         prefix="cli-proxy-api" ;;
        esac
        for dir in "$bin"/linux/*/; do
            dir="${dir%/}"; [ -d "$dir" ] || continue
            folder="$(basename "$dir")"
            # Only folders that can run on THIS machine: no 1 GB ROCm engine on a laptop without an AMD GPU.
            relevant=false
            case "$arch" in
                x86_64|amd64)
                    case "$folder" in
                        cpu|cliproxy) relevant=true ;;
                        amd) $gpu_amd && relevant=true ;;
                        nvidia) $gpu_nvidia && relevant=true ;;
                    esac ;;
                *)
                    case "$folder" in cpu|cpu-arm64|cliproxy|cliproxy-arm64) relevant=true ;; esac ;;
            esac
            $relevant || continue
            compgen -G "$dir/$prefix*" >/dev/null || continue
            installed="$(head -n1 "$dir/$marker" 2>/dev/null | tr -d '[:space:]' || true)"
            [ -n "$installed" ] || continue                        # unknown is not outdated
            memo_engine_older "$installed" "$latest" || continue

            manifest="$(curl -fsSL --max-time 20 "${MEMO_COMPONENTS_BASE:-https://data.memocpp.com/update/components}/$comp/$latest/manifest.json?cachebust=$(date +%s)" 2>/dev/null | tr -d '\n\r')" || manifest=""
            entry=""
            for key in "linux/$folder" "linux/$folder-arm64"; do
                case "$arch" in aarch64|arm64) ;; *) [ "$key" = "linux/$folder-arm64" ] && continue ;; esac
                entry="$(printf '%s' "$manifest" | memo_json_obj "$key")"
                [ -n "$entry" ] && break
            done
            if [ -z "$entry" ]; then continue; fi                  # nothing published for this folder (e.g. nvidia llama)
            url="$(printf '%s' "$entry" | memo_json_str url)"
            want="$(printf '%s' "$entry" | memo_json_str sha256)"
            case "$url" in https://*|http://127.0.0.1:*|http://localhost:*) ;; *) continue ;; esac
            [ -n "$want" ] || continue

            echo -e "  ${GREEN:-}▸${NC:-} Engine ${comp} (${folder}): ${installed} → ${latest}"
            tmp="$(mktemp -d)" || continue
            if ! curl -fSL -# --max-time 3600 -o "$tmp/engine.tar.gz" "$url"; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  download failed — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            got="$(memo_sha256 "$tmp/engine.tar.gz")"
            if [ "$got" != "$want" ]; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  checksum mismatch for ${comp} ${latest} — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            # Refuse archives that could write outside the folder or smuggle links in.
            # (Listings go through a variable: `tar | grep -q` can die of SIGPIPE under pipefail
            # and then report "no match" for an archive that matched.)
            listing="$(tar -tzf "$tmp/engine.tar.gz" 2>/dev/null || true)"
            if grep -qE '(^/|(^|/)\.\.(/|$))' <<<"$listing"; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  unsafe paths in the ${comp} archive — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            listing="$(tar -tvzf "$tmp/engine.tar.gz" 2>/dev/null | cut -c1 || true)"
            if grep -qE '[lh]' <<<"$listing"; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  links in the ${comp} archive — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            x="$tmp/x"; mkdir -p "$x"
            if ! tar -xzf "$tmp/engine.tar.gz" -C "$x"; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  could not unpack ${comp} ${latest} — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            rm -f "$tmp/engine.tar.gz"
            if [ "$(head -n1 "$x/$marker" 2>/dev/null | tr -d '[:space:]')" != "$latest" ]; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  the ${comp} archive does not carry ${marker}=${latest} — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            find "$x" \( -name "llama-server*" -o -name "whisper-server*" -o -name "cli-proxy-api*" -o -name "*.so*" \) -type f -exec chmod +x {} \; 2>/dev/null || true
            if ! memo_engine_runs "$comp" "$x"; then
                echo -e "  ${YELLOW:-}⚠${NC:-}  the new ${comp} ${latest} did not start here — keeping ${installed}"; rm -rf "$tmp"; continue
            fi
            if mv "$dir" "$dir.memo-old" 2>/dev/null && mv "$x" "$dir"; then
                rm -rf "$dir.memo-old"
            else
                [ -d "$dir.memo-old" ] && [ ! -d "$dir" ] && mv "$dir.memo-old" "$dir"
                echo -e "  ${YELLOW:-}⚠${NC:-}  could not swap ${folder} — keeping ${installed}"
            fi
            rm -rf "$tmp"
        done
    done
    return 0
}
# <<< memo-engine-refresh
if [ -d "$MEMO_HOME/binaries" ]; then
    memo_refresh_engines "$MEMO_HOME/binaries" || true
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
echo -e "  ${BOLD}Guide:${NC}      ${BLUE}https://memocpp.com/guide${NC}"
echo ""
echo -e "  Run ${CYAN}memo${NC} to launch the updated version."
echo ""
echo -e "  ${BOLD}Thank you for using Memo!${NC}"
