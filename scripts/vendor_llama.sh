#!/usr/bin/env bash
# Stage a llama.cpp upgrade for one or more bundled engine folders WITHOUT touching
# binaries/ itself, and refuse any folder where the upgrade would break something
# else that lives in it.
#
#   scripts/vendor_llama.sh [-t TAG] [-o OUT_DIR] FLAVOR [FLAVOR ...]
#
#   FLAVOR   linux/cpu  linux/amd  linux/cpu-arm64  windows/cpu  windows/amd
#            darwin/cpu darwin/metal      (NVIDIA is deliberately not here: the
#            CUDA 12.2 builds Memo bundles are no longer published)
#   -t TAG   llama.cpp release (default: $LLAMA_TAG or b11456)
#   -o OUT   staging dir (default ./binaries-staged); OUT/<flavor>/ = the current
#            folder with ONLY llama.cpp's own files replaced
#
# Why this is a script and not "just extract the tarball over it":
#  1. The bundled folders hold helpers that are not llama.cpp but are LINKED to the
#     folder's own libggml*/libllama* — whisper-server (speech-to-text) and
#     memo-lora-train (on-device training) are Memo's own builds. Dropping a newer
#     ggml next to them kills them at load ("libggml-cpu.so.0: cannot open shared
#     object file"; verified with b9441 -> b11456). After staging, every helper's
#     NEEDED libraries are checked against the new folder and the flavor is
#     REFUSED if any would go missing. Fix the helpers (rebuild against the new
#     ggml, or give them a folder of their own) before applying.
#  2. Release tarballs wrap everything in llama-<tag>/, ship lib symlinks (the bundle
#     is plain files: R2 and zips do not keep symlinks) and, from b11xxx on, call
#     the RPC worker ggml-rpc-server (Memo's lookup accepts both names).
#  3. Every archive is checked against the SHA-256 GitHub publishes for it.
set -euo pipefail

TAG="${LLAMA_TAG:-b11456}"
OUT="./binaries-staged"
while getopts "t:o:" o; do case "$o" in t) TAG="$OPTARG";; o) OUT="$OPTARG";; *) exit 2;; esac; done
shift $((OPTIND-1))
[ "$#" -gt 0 ] || { sed -n '2,30p' "$0" >&2; exit 2; }

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
REPO="ggml-org/llama.cpp"
WORK="$(mktemp -d)"; trap 'rm -rf "$WORK"' EXIT

asset_for() { # flavor -> release asset name
  case "$1" in
    linux/cpu)        echo "llama-$TAG-bin-ubuntu-x64.tar.gz" ;;
    linux/amd)        echo "llama-$TAG-bin-ubuntu-rocm-10.0-x64.tar.gz" ;;
    linux/cpu-arm64)  echo "llama-$TAG-bin-ubuntu-arm64.tar.gz" ;;
    windows/cpu)      echo "llama-$TAG-bin-win-cpu-x64.zip" ;;
    windows/amd)      echo "llama-$TAG-bin-win-vulkan-x64.zip" ;;
    darwin/cpu)       echo "llama-$TAG-bin-macos-x64.tar.gz" ;;
    darwin/metal)     echo "llama-$TAG-bin-macos-arm64.tar.gz" ;;
    *) echo "unknown flavor: $1" >&2; return 1 ;;
  esac
}

# Files that belong to llama.cpp (replaced); anything else in a folder is Memo's.
is_llama_file() {
  case "$(basename "$1")" in
    llama*|libllama*|libggml*|ggml*.dll|libmtmd*|mtmd*|ggml-rpc-server*|rpc-server*|llama.cpp.txt|LICENSE*|test-*|vulkan-shaders-gen*|*.metal|*.metallib) return 0 ;;
    *) return 1 ;;
  esac
}

# Libraries every Linux system provides; not ours to ship.
system_lib() { case "$1" in libc.so*|libm.so*|libdl.so*|libpthread.so*|librt.so*|libstdc++.so*|libgcc_s.so*|libgomp.so*|ld-linux*|linux-vdso*|libutil.so*) return 0;; *) return 1;; esac; }

refused=0
for flavor in "$@"; do
  asset="$(asset_for "$flavor")"
  echo "==> $flavor  <-  $asset ($TAG)"
  want="$(gh release view "$TAG" --repo "$REPO" --json assets --jq ".assets[]|select(.name==\"$asset\")|.digest|sub(\"sha256:\";\"\")")"
  [ -n "$want" ] || { echo "   no such asset in $TAG" >&2; exit 1; }
  gh release download "$TAG" --repo "$REPO" --pattern "$asset" --dir "$WORK" --clobber >/dev/null
  got="$(sha256sum "$WORK/$asset" | cut -d' ' -f1)"
  [ "$got" = "$want" ] || { echo "   CHECKSUM MISMATCH (got $got, GitHub says $want)" >&2; exit 1; }

  X="$WORK/x_$(echo "$flavor" | tr / _)"; mkdir -p "$X"
  case "$asset" in *.zip) unzip -q -o "$WORK/$asset" -d "$X" ;; *) tar -xzf "$WORK/$asset" -C "$X" ;; esac
  [ -d "$X/llama-$TAG" ] && SRC="$X/llama-$TAG" || SRC="$X"

  D="$OUT/$flavor"; rm -rf "$D"; mkdir -p "$D"
  [ -d "$ROOT/binaries/$flavor" ] && cp -a "$ROOT/binaries/$flavor/." "$D/"
  # drop llama.cpp's own (old) files, keep Memo's
  find "$D" -maxdepth 1 \( -type f -o -type l \) | while read -r f; do is_llama_file "$f" && rm -f "$f"; done
  cp -aL "$SRC/." "$D/"                       # -L: plain files, no symlinks
  echo "$TAG" > "$D/LLAMA_VERSION"

  # Back-compat: Memo accepts both names, but keep the long-standing one too.
  [ -f "$D/ggml-rpc-server" ] && [ ! -f "$D/rpc-server" ] && cp "$D/ggml-rpc-server" "$D/rpc-server"
  [ -f "$D/ggml-rpc-server.exe" ] && [ ! -f "$D/rpc-server.exe" ] && cp "$D/ggml-rpc-server.exe" "$D/rpc-server.exe"

  # Helper check (Linux ELF only): everything that is NOT llama.cpp must still find its libraries.
  case "$flavor" in linux/*)
    bad=0
    for f in "$D"/*; do
      [ -f "$f" ] && [ -x "$f" ] && ! is_llama_file "$f" || continue
      head -c4 "$f" 2>/dev/null | grep -q $'\x7fELF' || continue
      while read -r lib; do
        system_lib "$lib" && continue
        [ -e "$D/$lib" ] || { echo "   REFUSED: $(basename "$f") needs $lib, which the new folder does not have" >&2; bad=1; }
      done < <(readelf -d "$f" 2>/dev/null | sed -n 's/.*Shared library: \[\(.*\)\]/\1/p')
    done
    if [ "$bad" = 1 ]; then refused=1; rm -rf "$D"; echo "   $flavor NOT staged — see above" >&2; continue; fi
    ;;
  esac
  echo "   staged -> $D ($(du -sh "$D" | cut -f1))"
done
exit "$refused"
