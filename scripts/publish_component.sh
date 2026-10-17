#!/usr/bin/env bash
# Publish a new release of an engine COMPONENT (llama.cpp, whisper.cpp, the
# Subscriptions sidecar) so installed apps can fetch it without a new Memo release.
#
#   scripts/publish_component.sh [--dry-run] [--requires-app 4.7.0] <component> <version> <os/folder>=<dir> ...
#
#   component   llama | whisper | cliproxy
#   version     the release tag: b11456 / 1.9.0 / v8.0.26 (must equal the marker inside)
#   os/folder   the engine folder it replaces, e.g. linux/cpu, windows/amd, linux/cliproxy
#   dir         the COMPLETE new folder contents (what binaries/<os>/<folder>/ should
#               hold after the update), e.g. the output of scripts/vendor_llama.sh with
#               its helpers rebuilt, already stamped (scripts/stamp_engine_versions.sh)
#
# A folder is replaced as a whole, so it must contain everything that lives in it:
# linux/cpu carries llama.cpp AND whisper.cpp built against the same libraries —
# publish the whole folder, with BOTH markers, or the one you did not mean to touch
# stops working (AGENTS.md "llama.cpp upgrade traps"). Folders you do not list are
# not updated: that is how NVIDIA stays on an older llama.cpp than the others.
#
# For each folder this makes <os>-<folder>.tar.gz (.zip for windows), signs a
# manifest listing them with SHA-256 and size, verifies the signature with the key
# compiled into the app, and uploads everything to
#   R2:memo/update/components/<component>/<version>/
# It does NOT announce the release: that is the line you add to version.json's
# "components" afterwards (printed at the end), which is what makes apps look.
#
# The signing key is MEMO_UPDATE_SIGNING_KEY, or the file written by `memo-sign
# keygen` at ~/.config/memo-release/update-signing.key. Uploading uses the
# `memo-r2` rclone remote (see /home/bugra/Documents/r2-memo-push/upload-memo.sh);
# override with MEMO_R2_REMOTE. --dry-run builds and verifies into
# build_output/components/ and uploads nothing.
set -euo pipefail

dry=false; requires=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run) dry=true; shift ;;
    --requires-app) requires="${2:-}"; shift 2 ;;
    --) shift; break ;;
    -*) echo "unknown option $1" >&2; exit 2 ;;
    *) break ;;
  esac
done
[ "$#" -ge 3 ] || { sed -n '2,32p' "$0" >&2; exit 2; }
component="$1"; version="$2"; shift 2

case "$component" in llama|whisper|cliproxy) ;; *) echo "unknown component '$component'" >&2; exit 2 ;; esac
[[ "$version" =~ ^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$ ]] || { echo "bad version '$version'" >&2; exit 2; }
case "$component" in
  llama)    marker=LLAMA_VERSION;   bin_prefix=llama-server;   [[ "$version" =~ ^b[0-9]+$ ]] || { echo "a llama version looks like b11456" >&2; exit 2; } ;;
  whisper)  marker=WHISPER_VERSION; bin_prefix=whisper-server ;;
  cliproxy) marker=VERSION;         bin_prefix=cli-proxy-api ;;
esac

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
out="$ROOT/build_output/components/$component/$version"
rm -rf "$out"; mkdir -p "$out"

# A zip of the folder's contents (no wrapping directory), with `zip` if there is
# one and Python's zipfile otherwise.
make_zip() { # dir out
  if command -v zip >/dev/null 2>&1; then
    (cd "$1" && zip -qr "$2" .)
  else
    python3 -I - "$1" "$2" <<'PY'
import os, sys, zipfile
src, dst = sys.argv[1], sys.argv[2]
with zipfile.ZipFile(dst, "w", zipfile.ZIP_DEFLATED) as z:
    for base, _, names in os.walk(src, followlinks=True):
        for n in sorted(names):
            p = os.path.join(base, n)
            z.write(p, os.path.relpath(p, src))
PY
  fi
}

files=()
for spec in "$@"; do
  folder="${spec%%=*}"; dir="${spec#*=}"
  [[ "$folder" =~ ^(linux|darwin|windows)/[a-z0-9][a-z0-9-]*$ ]] || { echo "bad folder '$folder' (want os/name)" >&2; exit 2; }
  [ -d "$dir" ] || { echo "no such directory: $dir" >&2; exit 2; }
  # What is inside must be what the manifest will promise.
  got="$(head -n1 "$dir/$marker" 2>/dev/null | tr -d '[:space:]' || true)"
  [ "$got" = "$version" ] || { echo "$dir/$marker says '${got:-<missing>}', not '$version' — stamp the folder first" >&2; exit 1; }
  compgen -G "$dir/$bin_prefix*" >/dev/null || { echo "$dir has no $bin_prefix*" >&2; exit 1; }
  name="${folder//\//-}"
  case "$folder" in
    windows/*) make_zip "$dir" "$out/$name.zip"; files+=(-file "$folder=$out/$name.zip:zip") ;;
    # -h follows symlinks and --hard-dereference stores every name as its own
    # regular file: without it two links to one library become a tar HARD LINK,
    # which the app's unpacker (rightly) refuses.
    *)         tar -C "$dir" -h --hard-dereference -czf "$out/$name.tar.gz" .; files+=(-file "$folder=$out/$name.tar.gz") ;;
  esac
  echo "packed $folder  ($(du -h "$out/$name".* | cut -f1))"
done

key_args=()
if [ -z "${MEMO_UPDATE_SIGNING_KEY:-}" ]; then
  keyfile="${MEMO_UPDATE_KEY_FILE:-$HOME/.config/memo-release/update-signing.key}"
  [ -f "$keyfile" ] || { echo "no signing key: set MEMO_UPDATE_SIGNING_KEY or create $keyfile (memo-sign keygen)" >&2; exit 1; }
  key_args=(-key-file "$keyfile")
fi

sign="$(mktemp -d)/memo-sign"
go build -o "$sign" ./cmd/memo-sign
pub="$(sed -n 's/^const publicKeyB64 = "\(.*\)"$/\1/p' internal/updater/pubkey.go)"
[ -n "$pub" ] || { echo "internal/updater/pubkey.go has no public key" >&2; exit 1; }

base_url="https://data.memocpp.com/update/components/$component/$version"
req=(); [ -n "$requires" ] && req=(-requires-app "$requires")
"$sign" component -component "$component" -version "$version" ${req[@]+"${req[@]}"} -base-url "$base_url" -out "$out" ${key_args[@]+"${key_args[@]}"} "${files[@]}" >/dev/null
"$sign" verify -pub "$pub" "$out/manifest.json"
echo "manifest:"; sed 's/^/  /' "$out/manifest.json"

if $dry; then
  echo "dry run: nothing uploaded. Built in $out"
else
  remote="${MEMO_R2_REMOTE:-memo-r2}"
  bucket="${R2_BUCKET:-memo}"
  # Archives first, manifest last: nothing points at a file that is not there yet.
  for f in "$out"/*.tar.gz "$out"/*.zip; do
    [ -e "$f" ] || continue
    rclone copyto "$f" "$remote:$bucket/update/components/$component/$version/$(basename "$f")" --s3-acl public-read --progress
  done
  rclone copyto "$out/manifest.json"     "$remote:$bucket/update/components/$component/$version/manifest.json"     --s3-acl public-read --header-upload "Cache-Control: no-cache"
  rclone copyto "$out/manifest.json.sig" "$remote:$bucket/update/components/$component/$version/manifest.json.sig" --s3-acl public-read --header-upload "Cache-Control: no-cache"
  echo "uploaded to $remote:$bucket/update/components/$component/$version/"
fi

echo
echo "To announce it, set in version.json (github.com/BugraAkdemir/version):"
echo "  \"components\": { \"$component\": \"$version\" }"
