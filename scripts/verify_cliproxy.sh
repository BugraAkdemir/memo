#!/usr/bin/env bash
# Verify the CLIProxyAPI binaries under BINARIES_ROOT against the SHA-256s pinned
# in internal/cliproxy/PINNED.txt. Build workflows run this after staging so a
# package can never ship a sidecar that is not byte-for-byte the pinned release
# (a bad R2 sync, a half-downloaded file, a swapped object).
#
# Usage: scripts/verify_cliproxy.sh BINARIES_ROOT DEST [DEST ...]
#   DEST is a dest dir from PINNED.txt as it should exist under BINARIES_ROOT
#   (e.g. linux/cliproxy). A DEST given but not present is a failure: the whole
#   point is that a package without its sidecar must not be published silently.
#
# arm64 note: PINNED.txt lists linux/cliproxy-arm64, but the arm64 builds stage
# that folder as linux/cliproxy. Pass "linux/cliproxy=linux/cliproxy-arm64" to
# verify the staged dir against the arm64 row.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
PIN="$ROOT/internal/cliproxy/PINNED.txt"
BIN_ROOT="${1:?usage: verify_cliproxy.sh BINARIES_ROOT DEST [DEST ...]}"
shift
[ "$#" -gt 0 ] || { echo "no DEST given" >&2; exit 2; }

sha() { if command -v sha256sum >/dev/null 2>&1; then sha256sum "$1" | cut -d' ' -f1; else shasum -a 256 "$1" | cut -d' ' -f1; fi; }

fail=0
for spec in "$@"; do
  staged="${spec%%=*}"          # where it sits under BIN_ROOT
  pinned="${spec#*=}"           # which PINNED.txt dest it must match
  found=0
  while read -r kind asset asum dest name binsha; do
    # Git on Windows can check PINNED.txt out with CRLF line endings; without
    # this the last column carries a trailing \r and no hash ever matches.
    binsha="${binsha%$'\r'}"
    [ "$kind" = "asset" ] && [ "$dest" = "$pinned" ] || continue
    found=1
    f="$BIN_ROOT/$staged/$name"
    if [ ! -f "$f" ]; then echo "MISSING  $staged/$name" >&2; fail=1; continue; fi
    got="$(sha "$f")"
    if [ "$got" = "$binsha" ]; then echo "ok       $staged/$name ($(cat "$BIN_ROOT/$staged/VERSION" 2>/dev/null || echo '?'))"
    else echo "MISMATCH $staged/$name: got $got, pinned $binsha" >&2; fail=1; fi
  done < "$PIN"
  [ "$found" = 1 ] || { echo "no PINNED.txt row for dest $pinned" >&2; fail=1; }
done
exit "$fail"
