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
#            linux/cpu also gets libggml-vulkan.so (same tag) so GPUs ROCm/CUDA
#            builds do not cover can still speed up prompt processing
#   -t TAG   llama.cpp release (default: $LLAMA_TAG or b11669)
#   -o OUT   staging dir (default ./binaries-staged); OUT/<flavor>/ = the current
#            folder with ONLY llama.cpp's own files replaced
#
# Why this is a script and not "just extract the tarball over it":
#  1. The bundled folders hold helpers that are not llama.cpp but are LINKED to the
#     folder's own libggml*/libllama* — whisper-server (speech-to-text) was one of
#     Memo's own builds (memo-lora-train, removed 2026-10-25, was another; whisper-server
#     is now static, see the memo-release skill). Dropping a newer
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

TAG="${LLAMA_TAG:-b11669}"
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

# GPU backend plugin shipped INTO a flavor that would not have it. ggml loads every
# libggml-*.so it finds next to llama-server, so one file turns the cpu engine into a
# Vulkan-capable one (any GPU with a Vulkan driver: Intel/AMD iGPUs, AMD cards ROCm does
# not support, NVIDIA without a matching CUDA build). It must come from the SAME tag as
# the engine: a plugin from another build is not ABI-compatible with libggml-base.
# Memo picks it up without a new flavor (internal/llama/device_probe.go asks the engine
# which devices it sees). Measured with Phi-3 Q4 on a Ryzen 5 5600H APU: prompt
# processing 2.2x faster, decode unchanged. Not for linux/amd: that build has its own GPU
# backend and the same card would show up twice. is_llama_file() below treats libggml*
# as llama.cpp's own, so without this step a restage would silently DROP the plugin.
plugin_asset_for() { # flavor -> asset holding the plugin, or empty
  case "$1" in
    linux/cpu) echo "llama-$TAG-bin-ubuntu-vulkan-x64.tar.gz" ;;
    *) echo "" ;;
  esac
}

# Download an asset into $WORK and check it against the SHA-256 GitHub publishes for it.
fetch_verified() { # asset
  local a="$1" want got
  want="$(gh release view "$TAG" --repo "$REPO" --json assets --jq ".assets[]|select(.name==\"$a\")|.digest|sub(\"sha256:\";\"\")")"
  [ -n "$want" ] || { echo "   no such asset in $TAG: $a" >&2; exit 1; }
  gh release download "$TAG" --repo "$REPO" --pattern "$a" --dir "$WORK" --clobber >/dev/null
  got="$(sha256sum "$WORK/$a" | cut -d' ' -f1)"
  [ "$got" = "$want" ] || { echo "   CHECKSUM MISMATCH for $a (got $got, GitHub says $want)" >&2; exit 1; }
}

refused=0
for flavor in "$@"; do
  asset="$(asset_for "$flavor")"
  echo "==> $flavor  <-  $asset ($TAG)"
  fetch_verified "$asset"

  X="$WORK/x_$(echo "$flavor" | tr / _)"; mkdir -p "$X"
  case "$asset" in *.zip) unzip -q -o "$WORK/$asset" -d "$X" ;; *) tar -xzf "$WORK/$asset" -C "$X" ;; esac
  [ -d "$X/llama-$TAG" ] && SRC="$X/llama-$TAG" || SRC="$X"

  D="$OUT/$flavor"; rm -rf "$D"; mkdir -p "$D"
  [ -d "$ROOT/binaries/$flavor" ] && cp -a "$ROOT/binaries/$flavor/." "$D/"
  # drop llama.cpp's own (old) files, keep Memo's
  # `if`, not `is_llama_file && rm`: a `&&` list that ends on a non-llama file exits 1, which with
  # `set -e` + a pipe ended the whole script silently (whisper-server happened to be listed last).
  find "$D" -maxdepth 1 \( -type f -o -type l \) | while read -r f; do if is_llama_file "$f"; then rm -f "$f"; fi; done
  cp -aL "$SRC/." "$D/"                       # -L: plain files, no symlinks
  echo "$TAG" > "$D/LLAMA_VERSION"

  plugin="$(plugin_asset_for "$flavor")"
  if [ -n "$plugin" ]; then
    echo "   + GPU backend plugin from $plugin"
    fetch_verified "$plugin"
    P="$WORK/p_$(echo "$flavor" | tr / _)"; mkdir -p "$P"
    tar -xzf "$WORK/$plugin" -C "$P"
    pl="$(find "$P" -name libggml-vulkan.so -type f | head -1)"
    [ -n "$pl" ] || { echo "   REFUSED: $plugin has no libggml-vulkan.so" >&2; refused=1; rm -rf "$D"; continue; }
    cp -L "$pl" "$D/libggml-vulkan.so"
  fi

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
