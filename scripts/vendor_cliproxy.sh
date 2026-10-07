#!/usr/bin/env bash
# Fetch the pinned CLIProxyAPI release (internal/cliproxy/PINNED.txt), verify each
# archive against the pinned SHA-256, and lay the binaries out the way Memo and
# the build workflows expect:
#
#   <out>/<os>/cliproxy[-arm64]/<binary>  +  SHA256  VERSION  LICENSE
#
# Usage: scripts/vendor_cliproxy.sh [OUT_DIR] [ONLY_DEST ...]
#   OUT_DIR    defaults to ./binaries (the local dev tree, gitignored)
#   ONLY_DEST  optional dest dirs (e.g. linux/cliproxy) to limit the download
#
# Memo never downloads this at runtime — the zero-external-dependency promise —
# so this script (via the vendor-cliproxy.yml workflow) is the only place the
# network is touched, and the pinned digests are the only trust anchor.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PIN="$ROOT/internal/cliproxy/PINNED.txt"
OUT="${1:-$ROOT/binaries}"
shift || true
ONLY=("$@")

VERSION="$(awk '$1=="version"{print $2}' "$PIN")"
[ -n "$VERSION" ] || { echo "no version in $PIN" >&2; exit 1; }
BASE="https://github.com/router-for-me/CLIProxyAPI/releases/download/$VERSION"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

wanted() {
  [ "${#ONLY[@]}" -eq 0 ] && return 0
  for d in "${ONLY[@]}"; do [ "$d" = "$1" ] && return 0; done
  return 1
}

while read -r kind asset sha dest name binsha; do
  binsha="${binsha%$'\r'}" # tolerate a CRLF checkout of PINNED.txt
  [ "$kind" = "asset" ] || continue
  wanted "$dest" || continue
  echo "==> $asset -> $dest/$name"
  curl -fL --retry 3 -sS -o "$WORK/$asset" "$BASE/$asset"
  echo "$sha  $WORK/$asset" | sha256sum -c --quiet - || { echo "CHECKSUM MISMATCH for $asset — refusing to use it" >&2; exit 1; }

  X="$WORK/x_$asset"; mkdir -p "$X"
  case "$asset" in
    *.zip)    unzip -q "$WORK/$asset" -d "$X"; src="$X/cli-proxy-api.exe" ;;
    *)        tar -xzf "$WORK/$asset" -C "$X";  src="$X/cli-proxy-api" ;;
  esac
  [ -f "$src" ] || { echo "binary not found inside $asset" >&2; exit 1; }
  got="$(sha256sum "$src" | cut -d' ' -f1)"
  [ "$got" = "$binsha" ] || { echo "BINARY CHECKSUM MISMATCH for $name: got $got, PINNED.txt says $binsha" >&2; exit 1; }

  D="$OUT/$dest"; mkdir -p "$D"
  cp "$src" "$D/$name"; chmod +x "$D/$name"
  cp "$X/LICENSE" "$D/LICENSE"
  echo "$VERSION" > "$D/VERSION"
  # one SHA256 line per binary in this dir (darwin carries two)
  ( cd "$D" && { grep -v "  $name\$" SHA256 2>/dev/null || true; sha256sum "$name"; } > SHA256.new && mv SHA256.new SHA256 )
done < "$PIN"
echo "done -> $OUT ($VERSION)"
