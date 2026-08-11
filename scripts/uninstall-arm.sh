#!/usr/bin/env bash
# Memo — uninstaller for Linux arm64: cleans up whichever install method(s)
# are actually present on this machine — a curl/get_memo_arm.sh (native)
# install, a Docker install (docker/docker-compose.yml, CasaOS included), or
# both. One script, not two: a Pi being torn down doesn't care which one put
# Memo there, and someone who tried the native install then switched to
# Docker (or vice versa) shouldn't need to know which cleanup script to run.
#
#   curl -fsSL https://download.bugradev.com/uninstall-arm.sh | bash
#
# Optionally backs up your memory data (from either source) before removal.
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
echo -e "  ${BOLD}Uninstaller (Linux arm64)${NC}"
echo ""

# ── detect what's installed ──────────────────────────────────────────────────
MEMO_HOME="$HOME/.memo"
SERVICE_FILE="$HOME/.config/systemd/user/memo.service"
DOCKER_IMAGE_REF="ghcr.io/bugraakdemir/memo-backend"

# Not using `systemctl --user list-unit-files memo.service` to detect this:
# it exits 0 and just prints an empty listing when the unit doesn't exist —
# there's nothing to check the exit code of. The unit file's own presence on
# disk is the actual signal, and get_memo_arm.sh always writes it to exactly
# this path, so checking that is both simpler and correct.
HAS_NATIVE=false
if [ -d "$MEMO_HOME" ] || [ -f "$HOME/.local/bin/memo" ] || [ -f "$SERVICE_FILE" ]; then
    HAS_NATIVE=true
fi

# Docker detection: a container is matched by which image it was actually
# created from, not by assuming it's named "memo" — docker-compose.yml does
# set container_name: memo, but someone could have renamed it in their own
# copy, and `docker inspect memo` would then either 404 or (worse) match an
# unrelated container that happens to share the name. Checked both ways:
# ancestor filter first (fast, correct for an unrenamed container), name
# fallback second (only trusted if its image also actually matches).
HAS_DOCKER=false
DOCKER_CONTAINER=""
DOCKER_DATA_DIR=""
DOCKER_IMAGE_IDS=""
if command -v docker >/dev/null 2>&1; then
    DOCKER_CONTAINER="$(docker ps -a --filter "ancestor=${DOCKER_IMAGE_REF}" --format '{{.Names}}' 2>/dev/null | head -n1 || true)"
    if [ -z "$DOCKER_CONTAINER" ] && docker inspect memo >/dev/null 2>&1; then
        case "$(docker inspect -f '{{.Config.Image}}' memo 2>/dev/null || true)" in
            *memo-backend*) DOCKER_CONTAINER="memo" ;;
        esac
    fi
    DOCKER_IMAGE_IDS="$(docker images "$DOCKER_IMAGE_REF" -q 2>/dev/null | sort -u || true)"
    if [ -n "$DOCKER_CONTAINER" ] || [ -n "$DOCKER_IMAGE_IDS" ]; then
        HAS_DOCKER=true
    fi
    if [ -n "$DOCKER_CONTAINER" ]; then
        # docker-compose.yml's own /memo mount is a bind mount (a plain host
        # directory, e.g. /DATA/AppData/memo under CasaOS), not a named
        # Docker volume — so once we know the real host path, the container
        # doesn't even need to be running to read or back up its data; it's
        # just files. Discovered rather than assumed, since CasaOS/users can
        # point it at a different host path.
        DOCKER_DATA_DIR="$(docker inspect -f '{{ range .Mounts }}{{ if eq .Destination "/memo" }}{{ .Source }}{{ end }}{{ end }}' "$DOCKER_CONTAINER" 2>/dev/null || true)"
    fi
fi

if ! $HAS_NATIVE && ! $HAS_DOCKER; then
    echo -e "${YELLOW}Memo does not appear to be installed (no curl/get_memo_arm.sh install, no Docker container or image). Nothing to do.${NC}"
    exit 0
fi

echo -e "${BOLD}This will remove:${NC}"
if $HAS_NATIVE; then
    echo -e "  ${RED}▸${NC} The memo.service systemd user service (stopped + disabled first)"
    echo -e "  ${RED}▸${NC} ~/.memo/         (app, config, data, engine binaries)"
    echo -e "  ${RED}▸${NC} ~/.local/bin/memo  (CLI wrapper)"
    echo -e "  ${RED}▸${NC} ~/.local/share/applications/memo.desktop  (if present)"
    echo -e "  ${RED}▸${NC} ~/.local/share/icons/hicolor/*/apps/memo.png  (if present)"
fi
if $HAS_DOCKER; then
    [ -n "$DOCKER_CONTAINER" ]  && echo -e "  ${RED}▸${NC} Docker container '${DOCKER_CONTAINER}'"
    [ -n "$DOCKER_IMAGE_IDS" ]  && echo -e "  ${RED}▸${NC} Docker image(s): ${DOCKER_IMAGE_REF} (all tags pulled locally)"
    [ -n "$DOCKER_DATA_DIR" ]   && echo -e "  ${RED}▸${NC} ${DOCKER_DATA_DIR}  (mounted /memo data — asked about separately below, not deleted by default)"
fi
echo ""
if $HAS_NATIVE; then
    echo -e "  ${YELLOW}Note:${NC} if 'loginctl enable-linger' was turned on for this user during"
    echo -e "  install, it's left as-is — it may be relied on by other services besides"
    echo -e "  Memo, so this script doesn't touch it. Disable it yourself if you want to:"
    echo -e "  ${CYAN}sudo loginctl disable-linger $(whoami)${NC}"
    echo ""
fi

# ── memory backup (shared helper, used by both native and Docker paths) ─────
# $1 = source "data" directory (contains memory/, sessions/), $2 = label for
# the prompt (so the user can tell which install's data they're backing up
# if, unusually, both are present).
backup_memory_dir() {
    local src_data="$1" label="$2" answer backup_file docs_dir
    [ -d "$src_data/memory" ] && [ -n "$(ls -A "$src_data/memory" 2>/dev/null)" ] || return 1

    if [ -t 0 ]; then
        echo -ne "${YELLOW}Save ${label} memory data before uninstalling? (yes/no) [yes]: ${NC}"
        read -r answer
    else
        echo -ne "${YELLOW}Save ${label} memory data before uninstalling? (yes/no) [yes]: ${NC}" 2>/dev/null >/dev/tty || true
        read -r answer 2>/dev/null </dev/tty || true
    fi
    case "${answer:-yes}" in
        [Yy]|[Yy][Ee][Ss]) ;;
        *) return 1 ;;
    esac

    docs_dir=""
    for d in "$HOME/Documents" "$HOME/Belgeler" "$HOME/Documentos" "$HOME/Dokumente"; do
        [ -d "$d" ] && { docs_dir="$d"; break; }
    done
    [ -z "$docs_dir" ] && docs_dir="$HOME"

    backup_file="$docs_dir/memo-memory-$(echo "$label" | tr -cs 'a-zA-Z0-9' '-')-$(date +%Y%m%d-%H%M%S).zip"
    echo -e "\n${BOLD}Backing up ${label} memory to:${NC} ${GREEN}$backup_file${NC}"

    if command -v zip >/dev/null 2>&1; then
        (cd "$src_data" && zip -qr "$backup_file" memory/ sessions/ 2>/dev/null || true)
    elif command -v python3 >/dev/null 2>&1; then
        SRC_DATA="$src_data" BACKUP_FILE="$backup_file" python3 -c "
import zipfile, os
src = os.environ['SRC_DATA']
zf = zipfile.ZipFile(os.environ['BACKUP_FILE'], 'w', zipfile.ZIP_DEFLATED)
for root, dirs, files in os.walk(src):
    for f in files:
        fp = os.path.join(root, f)
        zf.write(fp, os.path.relpath(fp, src))
zf.close()
" 2>/dev/null
    else
        echo -e "${YELLOW}Warning: 'zip' not found. Copying memory folder as-is...${NC}"
        cp -r "$src_data/memory" "$docs_dir/memo-memory-backup-$(echo "$label" | tr -cs 'a-zA-Z0-9' '-')/" 2>/dev/null || {
            echo -e "${RED}Could not copy — check permissions (Docker's bind-mounted data is often root-owned; try running this script with sudo).${NC}"
            return 1
        }
    fi

    [ -f "$backup_file" ] && echo -e "${GREEN}  Saved: $backup_file${NC}"
    echo ""
    return 0
}

$HAS_NATIVE  && backup_memory_dir "$MEMO_HOME/data" "native"
$HAS_DOCKER && [ -n "$DOCKER_DATA_DIR" ] && backup_memory_dir "$DOCKER_DATA_DIR/data" "Docker"

# Data written by the container is very often root-owned on the host (this
# repo's own Dockerfile never drops to a non-root USER) — the backup step
# above already surfaces a permission failure with a "try sudo" hint, but a
# Docker data directory is deliberately never auto-deleted below regardless
# of backup outcome (unlike ~/.memo/, which the user owns outright): it's
# usually a bind mount CasaOS itself manages (/DATA/AppData/memo), and this
# script has no reliable way to tell "safe to delete" from "CasaOS still
# expects this path to exist." Removed by hand if wanted, see the closing
# message.

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

# ── remove: native ───────────────────────────────────────────────────────────
if $HAS_NATIVE; then
    echo -e "${BOLD}Removing native install...${NC}"

    # Systemd service first — has to be stopped/disabled before its unit file
    # is deleted, otherwise it'd keep restarting itself (Restart=on-failure)
    # against a memo-backend binary that's about to be rm -rf'd out from
    # under it.
    if command -v systemctl >/dev/null 2>&1; then
        echo -e "  ${RED}▸${NC} systemd service"
        systemctl --user stop memo.service >/dev/null 2>&1 || true
        systemctl --user disable memo.service >/dev/null 2>&1 || true
        rm -f "$SERVICE_FILE"
        systemctl --user daemon-reload >/dev/null 2>&1 || true
        systemctl --user reset-failed memo.service >/dev/null 2>&1 || true
    fi

    # Belt-and-braces in case the service was somehow gone but the process
    # wasn't (e.g. it was started by hand rather than through the service).
    pkill -TERM -f "memo-backend" 2>/dev/null || true
    pkill -TERM -f "llama-server" 2>/dev/null || true
    sleep 1

    echo -e "  ${RED}▸${NC} ~/.memo/"
    rm -rf "$MEMO_HOME"

    echo -e "  ${RED}▸${NC} CLI wrapper"
    rm -f "$HOME/.local/bin/memo"

    echo -e "  ${RED}▸${NC} App menu entry (if any)"
    rm -f "$HOME/.local/share/applications/memo.desktop"

    echo -e "  ${RED}▸${NC} Icons (if any)"
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
fi

# ── remove: Docker ───────────────────────────────────────────────────────────
if $HAS_DOCKER; then
    echo -e "${BOLD}Removing Docker install...${NC}"

    if [ -n "$DOCKER_CONTAINER" ]; then
        echo -e "  ${RED}▸${NC} Container '$DOCKER_CONTAINER'"
        if ! docker rm -f "$DOCKER_CONTAINER" >/dev/null 2>&1; then
            echo -e "    ${YELLOW}⚠${NC}  Couldn't remove it — permission denied? Try: ${CYAN}sudo docker rm -f $DOCKER_CONTAINER${NC}"
        fi
    fi

    if [ -n "$DOCKER_IMAGE_IDS" ]; then
        echo -e "  ${RED}▸${NC} Image(s): $DOCKER_IMAGE_REF"
        # shellcheck disable=SC2086
        if ! docker rmi $DOCKER_IMAGE_IDS >/dev/null 2>&1; then
            echo -e "    ${YELLOW}⚠${NC}  Couldn't remove one or more — still referenced by another container/tag? Try: ${CYAN}docker rmi -f $DOCKER_IMAGE_REF${NC}"
        fi
    fi

    if [ -n "$DOCKER_DATA_DIR" ]; then
        echo -e "  ${YELLOW}i${NC}  Data left in place: $DOCKER_DATA_DIR"
        echo -e "     (bind-mounted, likely CasaOS-managed — not auto-deleted. Remove it"
        echo -e "     yourself if you're sure: ${CYAN}rm -rf \"$DOCKER_DATA_DIR\"${NC} — may need sudo.)"
    fi
fi

# ── done ─────────────────────────────────────────────────────────────────────
echo ""
echo -e "${GREEN}${BOLD}  Memo has been uninstalled.${NC}"
echo ""
echo -e "  ${BOLD}Guide:${NC}  ${BLUE}https://memo.bugradev.com/guide${NC}"
echo ""
echo -e "  ${BOLD}Thank you for using Memo. See you again!${NC}"
