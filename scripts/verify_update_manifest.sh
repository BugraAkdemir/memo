#!/usr/bin/env bash
# After a release's CI has finished: check that what an installed Memo will fetch is
# consistent — for every platform, the signed manifest is there, its signature
# verifies with the key compiled into the app, and the file it names exists at that
# URL with exactly the size the manifest promises.
#
#   scripts/verify_update_manifest.sh [V4.7.0] [platform ...]
#
# Defaults to the version in the `version` file and every platform CI publishes.
# Meant to run BEFORE the beacon (version.json) is bumped: the beacon is what sends
# apps looking, so it must never point at a release that does not check out.
# Exit status is non-zero if anything is missing or inconsistent. It downloads only
# manifests and issues HEAD requests — never the 800 MB archives.
#
#   BASE=https://data.memocpp.com/update   (override to test against a local server)
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BASE="${BASE:-https://data.memocpp.com/update}"

ver="${1:-$(tr -d '[:space:]' < version)}"; shift || true
dir="${ver#[Vv]}"
platforms=("$@"); [ "${#platforms[@]}" -gt 0 ] || platforms=(linux-amd64 darwin-arm64 windows-amd64)

pub="$(sed -n 's/^const publicKeyB64 = "\(.*\)"$/\1/p' internal/updater/pubkey.go)"
[ -n "$pub" ] || { echo "internal/updater/pubkey.go has no public key" >&2; exit 1; }
tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
go build -o "$tmp/memo-sign" ./cmd/memo-sign

fail=0
for p in "${platforms[@]}"; do
  m="$tmp/$p.json"
  if ! curl -fsS -m 30 -o "$m" "$BASE/app/$dir/$p.json?cb=$(date +%s%N)" || ! curl -fsS -m 30 -o "$m.sig" "$BASE/app/$dir/$p.json.sig?cb=$(date +%s%N)"; then
    echo "MISSING  $p: no manifest at $BASE/app/$dir/$p.json" >&2; fail=1; continue
  fi
  if ! "$tmp/memo-sign" verify -pub "$pub" "$m" >/dev/null 2>&1; then
    echo "BAD SIG  $p: the manifest is not signed by the key compiled into the app" >&2; fail=1; continue
  fi
  url="$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["url"])' "$m")"
  want="$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["size"])' "$m")"
  mver="$(python3 -I -c 'import json,sys; print(json.load(open(sys.argv[1]))["version"])' "$m")"
  if [ "${mver#[Vv]}" != "$dir" ]; then echo "WRONG VERSION  $p: manifest says $mver, expected $ver" >&2; fail=1; continue; fi
  got="$(curl -fsSI -m 30 "$url?cb=$(date +%s%N)" | tr -d '\r' | awk 'tolower($1)=="content-length:"{print $2}' | tail -n1)"
  if [ "$got" != "$want" ]; then
    echo "SIZE     $p: $url is ${got:-unreachable} bytes, the manifest promises $want" >&2; fail=1; continue
  fi
  echo "ok       $p: $ver  $want bytes  $url"
done

if [ "$fail" -ne 0 ]; then echo "NOT consistent: do not bump the beacon." >&2; exit 1; fi
echo "all ${#platforms[@]} platform manifest(s) for $ver verified."
