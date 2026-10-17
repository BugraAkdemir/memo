#!/usr/bin/env bash
# Write (or verify) the version marker files in the engine folders under a
# binaries/ tree — the files internal/components reads and the component
# updater compares against (PLAN_auto_update.md).
#
#   scripts/stamp_engine_versions.sh stamp [--llama bNNNN] [--whisper X.Y.Z] [--only OS/FLAVOR]... <binaries-dir>
#   scripts/stamp_engine_versions.sh check [--llama bNNNN] [--whisper X.Y.Z] [--only OS/FLAVOR]... <binaries-dir>
#   scripts/stamp_engine_versions.sh stamp|check --from internal/components/ENGINES.txt <binaries-dir>
#
#   <binaries-dir>   the directory that holds linux/ windows/ darwin/
#                    (i.e. the repo's ./binaries, or the R2 staging copy)
#   --only OS/FLAVOR restrict to those folders (repeatable), e.g. --only linux/cpu.
#                    The flavors do NOT all hold the same llama.cpp build (NVIDIA is
#                    pinned to an older one, macOS has its own), so each group is
#                    stamped with the tag it really holds.
#   --from FILE      the declaration of what each folder holds: one line per
#                    component, "<llama|whisper> <version> <os/flavor>...". It is the
#                    single place that says what ships; CI stamps from it and then
#                    checks, so a wrong declaration fails the build.
#
# stamp writes the given component's marker where that component's binary exists:
#   LLAMA_VERSION    in every selected folder that has a llama-server
#   WHISPER_VERSION  in every selected folder that has a whisper-server
# check (with a version) requires the marker to equal it; with no version it only
# requires the marker to be well-formed. The cliproxy folders already carry VERSION
# (scripts/vendor_cliproxy.sh).
#
# Why the version is an explicit input: the llama.cpp release tarballs carry no
# version file, and the build number stops showing in library file names after
# b11xxx. The one cross-check that is possible is made — a Linux
# libllama.so.0.0.<N> with a real build number (N >= 1000) must agree with --llama —
# otherwise the stamp would claim a version the files deny and the updater would
# skip (or repeat) an update.
set -euo pipefail

usage() { sed -n '2,34p' "$0" >&2; exit 2; }

[ "$#" -ge 2 ] || usage
cmd="$1"; shift
case "$cmd" in stamp|check) ;; *) usage ;; esac

llama=""; whisper=""; from=""; only=()
while [ "$#" -gt 1 ]; do
  case "$1" in
    --llama)   llama="${2:-}"; shift 2 ;;
    --whisper) whisper="${2:-}"; shift 2 ;;
    --from)    from="${2:-}"; shift 2 ;;
    --only)    only+=("${2:-}"); shift 2 ;;
    *) usage ;;
  esac
done
[ "$#" -eq 1 ] || usage
root="$1"
[ -d "$root" ] || { echo "not a directory: $root" >&2; exit 2; }

# --from: run once per declared line, via ourselves.
if [ -n "$from" ]; then
  [ -f "$from" ] || { echo "no such declaration file: $from" >&2; exit 2; }
  [ -z "$llama$whisper" ] && [ "${#only[@]}" -eq 0 ] || { echo "--from cannot be combined with --llama/--whisper/--only" >&2; exit 2; }
  seen=0; rc=0
  while read -r comp ver folders; do
    case "$comp" in ''|'#'*) continue ;; esac
    case "$comp" in llama|whisper) ;; *) echo "$from: unknown component '$comp'" >&2; exit 2 ;; esac
    [ -n "$ver" ] && [ -n "$folders" ] || { echo "$from: line for '$comp' needs a version and folders" >&2; exit 2; }
    args=("--$comp" "$ver")
    for f in $folders; do args+=(--only "$f"); done
    "$0" "$cmd" "${args[@]}" "$root" || rc=1
    seen=$((seen+1))
  done < "$from"
  [ "$seen" -gt 0 ] || { echo "$from declares nothing" >&2; exit 2; }
  exit "$rc"
fi

token_re='^[A-Za-z0-9][A-Za-z0-9._+-]{0,31}$'
llama_re='^b[0-9]+$'

if [ -n "$llama" ] && ! [[ "$llama" =~ $llama_re ]]; then echo "--llama must look like b9441 (got '$llama')" >&2; exit 2; fi
if [ -n "$whisper" ] && ! [[ "$whisper" =~ $token_re ]]; then echo "--whisper is not a version token (got '$whisper')" >&2; exit 2; fi
if [ "$cmd" = "stamp" ] && [ -z "$llama$whisper" ]; then echo "stamp needs --llama and/or --whisper" >&2; exit 2; fi

# Engine folders hold a llama-server* or whisper-server*; cliproxy, ngrok and vec0 never match.
has_bin() { compgen -G "$1/$2*" >/dev/null; }

fail=0
matched=0

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
      # Only the components we were told about are touched or judged.
      do_llama=false; do_whisper=false
      if $has_llama && { [ -n "$llama" ] || { [ "$cmd" = "check" ] && [ -z "$whisper" ]; }; }; then do_llama=true; fi
      if $has_whisper && { [ -n "$whisper" ] || { [ "$cmd" = "check" ] && [ -z "$llama" ]; }; }; then do_whisper=true; fi
      $do_llama || $do_whisper || continue
      [ "$pass" = "validate" ] || [ "$pass" = "check" ] && matched=$((matched+1)) || true

      if [ "$cmd" = "stamp" ]; then
        if $do_llama; then
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
        if [ "$pass" = "write" ]; then
          $do_llama   && printf '%s\n' "$llama"   > "$dir/LLAMA_VERSION"
          $do_whisper && printf '%s\n' "$whisper" > "$dir/WHISPER_VERSION"
          echo "stamped $rel:$($do_llama && echo " llama=$llama")$($do_whisper && echo " whisper=$whisper")"
        fi
      else
        if $do_llama; then
          v="$(head -n1 "$dir/LLAMA_VERSION" 2>/dev/null || true)"
          if ! [[ "$v" =~ $llama_re ]]; then
            echo "MISSING/BAD $rel/LLAMA_VERSION ('$v')" >&2; fail=1
          elif [ -n "$llama" ] && [ "$v" != "$llama" ]; then
            echo "WRONG $rel/LLAMA_VERSION is '$v', expected '$llama'" >&2; fail=1
          fi
        fi
        if $do_whisper; then
          v="$(head -n1 "$dir/WHISPER_VERSION" 2>/dev/null || true)"
          if ! [[ "$v" =~ $token_re ]]; then
            echo "MISSING/BAD $rel/WHISPER_VERSION ('$v')" >&2; fail=1
          elif [ -n "$whisper" ] && [ "$v" != "$whisper" ]; then
            echo "WRONG $rel/WHISPER_VERSION is '$v', expected '$whisper'" >&2; fail=1
          fi
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

# A selection that matched nothing is a mistake (a typo in --only, an empty
# download), never a pass: CI must not go green because nothing was looked at.
if [ "${#only[@]}" -gt 0 ] && [ "$matched" -eq 0 ]; then
  echo "nothing matched ${only[*]} under $root" >&2; exit 1
fi

exit "$fail"
