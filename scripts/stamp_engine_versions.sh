#!/usr/bin/env bash
# Write (or verify) the version marker files in the engine folders under a
# binaries/ tree — the files internal/components reads and the component
# updater compares against (PLAN_auto_update.md).
#
#   scripts/stamp_engine_versions.sh stamp --llama bNNNN [--whisper X.Y.Z] [--only OS/FLAVOR]... <binaries-dir>
#   scripts/stamp_engine_versions.sh check [--only OS/FLAVOR]... <binaries-dir>
#
#   <binaries-dir>   the directory that holds linux/ windows/ darwin/
#                    (i.e. the repo's ./binaries, or the R2 staging copy)
#   --only OS/FLAVOR restrict to those folders (repeatable), e.g. --only linux/cpu.
#                    The flavors do NOT all hold the same llama.cpp build (NVIDIA is
#                    pinned to an older one, macOS has its own), so stamp each group
#                    with the tag it really holds.
#
# Markers are written per engine folder, and only where the thing exists:
#   LLAMA_VERSION    in every folder that has a llama-server
#   WHISPER_VERSION  in every folder that has a whisper-server
# The cliproxy folders already carry VERSION (scripts/vendor_cliproxy.sh).
#
# Why a script that is told the version instead of detecting it: the llama.cpp
# release tarballs carry no version file, and the build number stops showing in
# library file names after b11xxx. So the version is an explicit input, and the
# one cross-check that is possible is made: a Linux libllama.so.0.0.<N> with a
# real build number (N >= 1000) must agree with --llama, otherwise the stamp
# would claim a version the files deny and the updater would skip (or repeat)
# an update.
set -euo pipefail

usage() { sed -n '2,28p' "$0" >&2; exit 2; }

[ "$#" -ge 2 ] || usage
cmd="$1"; shift

llama=""; whisper=""; only=()
while [ "$#" -gt 1 ]; do
  case "$1" in
    --llama)   llama="${2:-}"; shift 2 ;;
    --whisper) whisper="${2:-}"; shift 2 ;;
    --only)    only+=("${2:-}"); shift 2 ;;
    *) usage ;;
  esac
done
[ "$#" -eq 1 ] || usage
root="$1"
[ -d "$root" ] || { echo "not a directory: $root" >&2; exit 2; }

token_re='^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$'

# Engine folders: <root>/<os>/<flavor>/ that hold a llama-server* or whisper-server*.
# (cliproxy folders, ngrok, vec0 are not engines and never match.)
has_bin() { # dir prefix
  compgen -G "$1/$2*" >/dev/null
}

fail=0

case "$cmd" in
  stamp)
    [[ "$llama" =~ ^b[0-9]+$ ]] || { echo "--llama must look like b9441 (got '$llama')" >&2; exit 2; }
    if [ -n "$whisper" ] && ! [[ "$whisper" =~ $token_re ]]; then
      echo "--whisper is not a version token (got '$whisper')" >&2; exit 2
    fi
    ;;
  check) ;;
  *) usage ;;
esac

walk() {
  local pass="$1"
  for osdir in "$root"/*/; do
    for dir in "$osdir"*/; do
      dir="${dir%/}"
      [ -d "$dir" ] || continue
      rel="${dir#"$root"/}"
      if [ "${#only[@]}" -gt 0 ]; then
        picked=false
        for o in "${only[@]}"; do [ "$o" = "$rel" ] && picked=true; done
        $picked || continue
      fi
      has_llama=false; has_whisper=false
      has_bin "$dir" "llama-server" && has_llama=true
      has_bin "$dir" "whisper-server" && has_whisper=true
      $has_llama || $has_whisper || continue

      if [ "$cmd" = "stamp" ]; then
        if $has_llama; then
          # Cross-check against a real build number in the library names.
          nums="$(cd "$dir" && ls 2>/dev/null | sed -n 's/^libllama\.so\.0\.0\.\([0-9][0-9]*\)$/\1/p' | awk '$1>=1000' | sort -u)"
          if [ -n "$nums" ]; then
            if [ "$(printf '%s\n' "$nums" | wc -l)" -ne 1 ]; then
              echo "$rel: libllama carries several build numbers ($(echo $nums)); the folder is mixed" >&2; exit 1
            fi
            if [ "b$nums" != "$llama" ]; then
              echo "$rel: libllama.so.0.0.$nums says b$nums but --llama is $llama" >&2; exit 1
            fi
          fi
        fi
        if $has_whisper && [ -z "$whisper" ]; then
          echo "$rel has a whisper-server but --whisper was not given" >&2; exit 2
        fi
        if [ "$pass" = "write" ]; then
          $has_llama   && printf '%s\n' "$llama"   > "$dir/LLAMA_VERSION"
          $has_whisper && printf '%s\n' "$whisper" > "$dir/WHISPER_VERSION"
          echo "stamped $rel:$($has_llama && echo " llama=$llama")$($has_whisper && echo " whisper=$whisper")"
        fi
      else
        if $has_llama; then
          v="$(head -n1 "$dir/LLAMA_VERSION" 2>/dev/null || true)"
          if ! [[ "$v" =~ ^b[0-9]+$ ]]; then echo "MISSING/BAD $rel/LLAMA_VERSION ('$v')" >&2; fail=1; fi
        fi
        if $has_whisper; then
          v="$(head -n1 "$dir/WHISPER_VERSION" 2>/dev/null || true)"
          if ! [[ "$v" =~ $token_re ]]; then echo "MISSING/BAD $rel/WHISPER_VERSION ('$v')" >&2; fail=1; fi
        fi
      fi
    done
  done
}

# stamp validates every folder before writing any marker, so a refusal in the
# last folder cannot leave the tree half-stamped.
if [ "$cmd" = "stamp" ]; then
  walk validate
  walk write
else
  walk check
fi

exit "$fail"
