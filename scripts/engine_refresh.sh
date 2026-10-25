#!/usr/bin/env bash
# Bring the engines in an installed binaries/ tree up to date WHILE the installer runs.
#
#   scripts/engine_refresh.sh <binaries-dir>        # stand-alone (also what the tests drive)
#
# Why: an installer unpacks the engines that were bundled into the last Memo release
# (e.g. CLIProxyAPI v8.0.25, llama b9441). Engines are published on their own between
# releases (scripts/publish_component.sh -> version.json "components"), so a fresh
# install used to start out of date until the app's own engine updater ran. This asks
# the same beacon the app asks, compares it with the version markers in the folders
# that were just installed (LLAMA_VERSION, WHISPER_VERSION, VERSION) and replaces any
# older folder with the published one — before Memo ever starts.
#
# Same rules as the in-app updater (internal/app/update.go), minus the signature: the
# app checks an Ed25519 signature on the manifest; a shell script has no key to check it
# with, so it trusts the same HTTPS origin as the Memo archive it just downloaded and
# checks every archive against the SHA-256 in the manifest. A folder is replaced WHOLE,
# the new binary is run once, and on any failure the old folder is kept. A folder
# whose marker is missing is never touched ("unknown" is not "outdated"). NEVER fatal:
# the installer carries on with what it has.
#
# The installers (get-memo*.sh, get_memo_arm.sh, update.sh) embed the block between the
# markers below VERBATIM; internal/components/installer_refresh_test.go keeps every copy
# identical to this file. Edit THIS file, then re-embed (the test says how).
#
# Env: MEMO_SKIP_ENGINE_REFRESH=1 turns it off; MEMO_BEACON_URL / MEMO_COMPONENTS_BASE
# point it somewhere else (tests).

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

if [ "${BASH_SOURCE[0]}" = "$0" ]; then
    set -uo pipefail
    [ "$#" -eq 1 ] || { echo "usage: $0 <binaries-dir>" >&2; exit 2; }
    GREEN="\033[32m"; YELLOW="\033[33m"; NC="\033[0m"
    memo_refresh_engines "$1"
fi
