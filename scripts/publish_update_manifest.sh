#!/usr/bin/env bash
# Sign and publish the update manifest for one platform's release build.
#
#   scripts/publish_update_manifest.sh <platform> <kind> <format> <file> <public-url> [archive-root]
#
#   platform     linux-amd64 | darwin-arm64 | darwin-amd64 | windows-amd64
#   kind         archive | installer
#   format       tar.gz | zip | exe
#   file         the local build output (its size and SHA-256 go into the manifest)
#   public-url   where that very file is published (https://data.memocpp.com/memo.tar.gz …)
#   archive-root the top-level directory inside an archive ("Memo")
#
# Run by the build workflows AFTER the artifact itself has been uploaded to R2, on
# real release tags only. Needs MEMO_UPDATE_SIGNING_KEY (the Actions secret) and the
# RCLONE_CONFIG_R2_* environment the upload steps already use. Writes
#   R2:<bucket>/update/app/<X.Y.Z>/<platform>.json  and  .json.sig
# which is what an installed Memo fetches (internal/updater, Client.AppManifestURL).
#
# Three guards, because a bad manifest is worse than none:
#  1. the tag must match the `version` file — a checkpoint tag (v3.2.1 …) re-publishes
#     the stable files too, and must NOT announce itself as the version in `version`;
#  2. the signature is verified against the public key COMPILED INTO the app before
#     anything is uploaded, so a secret that does not match the build fails here and
#     not silently on every user's machine;
#  3. the manifest is parsed back by the app's own parser inside memo-sign.
set -euo pipefail

[ "$#" -ge 5 ] || { sed -n '2,25p' "$0" >&2; exit 2; }
platform="$1"; kind="$2"; format="$3"; file="$4"; url="$5"; root="${6:-}"

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
BUCKET="${R2_BUCKET:-memo}"

[ -f "$file" ] || { echo "no such file: $file" >&2; exit 1; }

ver_file="$(tr -d '[:space:]' < version)"           # V4.7.0
ver_dir="${ver_file#[Vv]}"                           # 4.7.0
tag="${GITHUB_REF_NAME:-}"
if [ -n "$tag" ]; then
  if [ "$(printf '%s' "${tag#[Vv]}" | tr 'A-Z' 'a-z')" != "$ver_dir" ]; then
    echo "::notice::tag '$tag' does not match the version file ($ver_file): this is a checkpoint tag, not a release. No update manifest published."
    exit 0
  fi
fi
[ -n "${MEMO_UPDATE_SIGNING_KEY:-}" ] || { echo "MEMO_UPDATE_SIGNING_KEY is not set; refusing to publish an unsigned release" >&2; exit 1; }

tmp="${RUNNER_TEMP:-$(mktemp -d)}"
sign="$tmp/memo-sign"
go build -o "$sign" ./cmd/memo-sign

pub="$(sed -n 's/^const publicKeyB64 = "\(.*\)"$/\1/p' internal/updater/pubkey.go)"
[ -n "$pub" ] || { echo "internal/updater/pubkey.go has no public key" >&2; exit 1; }

out="$ROOT/build_output/update/app/$ver_dir"
rm -rf "$ROOT/build_output/update"
args=(app -version "$ver_file" -platform "$platform" -kind "$kind" -format "$format" -url "$url" -file "$file" -out "$out")
[ -n "$root" ] && args+=(-root "$root")
"$sign" "${args[@]}"

# Guard 2: the key behind the secret must be the one compiled into the app.
"$sign" verify -pub "$pub" "$out/$platform.json"

dest="R2:$BUCKET/update/app/$ver_dir"
rclone copyto "$out/$platform.json"     "$dest/$platform.json"     --header-upload "Cache-Control: no-cache"
rclone copyto "$out/$platform.json.sig" "$dest/$platform.json.sig" --header-upload "Cache-Control: no-cache"
echo "published $dest/$platform.json ($(wc -c < "$file") bytes described, $url)"
