# Memo — backend-only image (headless REST API, no Flutter GUI).
#
# Built for running Memo as a LAN service (CasaOS, a home server, a spare
# machine) rather than as a local desktop app. There is no browser UI here —
# connect to it with the Flutter desktop app or the `memo` terminal client
# pointed at this container's address, using the X-Memo-Token printed to the
# container logs on first boot (see docker/README.md).
#
# Bundles only the CPU llama.cpp backend (binaries/linux/cpu) for local GGUF
# inference — no GPU passthrough assumed. External providers (OpenAI,
# Claude, Gemini, ...) configured after boot work regardless, and are the
# realistic choice on typical NAS/home-server hardware anyway.

# ---- build stage --------------------------------------------------------
FROM golang:1.26-bookworm AS builder

WORKDIR /src

# Dependencies first so `go build` doesn't refetch modules on every source
# change during local iteration.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO is required (mattn/go-sqlite3); -tags sqlite_fts5 is required too — see
# docs/CGO_FLAGS.md and AGENTS.md's "Memory / Vector Store" section. Without
# it FTS5 silently never activates and memory retrieval permanently degrades
# to vector-only search, with no error surfaced anywhere.
RUN CGO_ENABLED=1 go build -tags "sqlite_fts5" -ldflags="-s -w" -o /out/memo .

# ---- runtime stage -------------------------------------------------------
FROM debian:bookworm-slim

# Shared libraries the bundled llama-server binary links against (verified
# via ldd against the real binary: libstdc++, libssl/libcrypto, libgomp,
# zlib, brotli, zstd — libc/libm/libgcc_s ship in the base image already).
# ca-certificates is needed for HTTPS calls to external LLM providers and
# model downloads; curl backs the HEALTHCHECK below.
RUN apt-get update && apt-get install -y --no-install-recommends \
        ca-certificates \
        curl \
        libstdc++6 \
        libssl3 \
        libgomp1 \
        zlib1g \
        libbrotli1 \
        libzstd1 \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app

COPY --from=builder /out/memo /app/memo
COPY config/config.yaml.example /app/config.yaml.example
COPY data/providers.example.json /app/providers.example.json
COPY binaries/linux/cpu /app/binaries/linux/cpu

# Trim the CPU engine bundle down to what a headless server actually runs:
# llama-server + vec0 (the sqlite-vec extension memory retrieval needs) plus
# their shared libraries. The rest (llama-cli/-bench/-imatrix/-quantize/
# -tokenize/-tts/-finetune, rpc-server, memo-lora-train, whisper-server +
# libwhisper — the Whisper STT model itself is excluded earlier by
# .dockerignore, so the server binary alone is dead weight) are llama.cpp's
# own dev/debug tools and an unrelated speech-to-text stack, not used by
# Memo's own server process. Also drops the duplicate/legacy-named library
# copies (*-new, *.so.0.0.1, the nested llama-new/ dir) that accumulated in
# this directory across llama.cpp version bumps during development.
RUN find /app/binaries/linux/cpu -mindepth 1 -maxdepth 1 \
        ! -name 'llama-server' \
        ! -name 'vec0.so' \
        ! -name '*.so' \
        ! -name '*.so.*' \
        ! -name 'LICENSE' \
        -exec rm -rf {} + \
    && rm -f /app/binaries/linux/cpu/*-new /app/binaries/linux/cpu/*.so.0.0.1 \
             /app/binaries/linux/cpu/libllama-common.so.0.0.1 \
    && chmod +x /app/binaries/linux/cpu/llama-server

# /memo is the single persistent volume: MEMO_DATA_DIR=/memo/data makes
# internal/config.ConfigDir() resolve to /memo/config automatically (it is
# always the "config" sibling of the data dir's parent) — one mount covers
# both, matching the existing data/+config/ sibling layout used everywhere
# else in this codebase, just rooted under /memo instead of the cwd.
RUN mkdir -p /memo/data /memo/config
ENV MEMO_DATA_DIR=/memo/data
VOLUME ["/memo"]

EXPOSE 8090

# Seeds a clean config.yaml/providers.json into the mounted volume on first
# boot only — never overwrites a config a previous run (or the user) already
# wrote there. See docker/README.md for what --lan does and how to retrieve
# the generated X-Memo-Token from `docker logs`.
COPY docker/entrypoint.sh /app/entrypoint.sh
RUN chmod +x /app/entrypoint.sh

HEALTHCHECK --interval=30s --timeout=5s --start-period=20s --retries=3 \
    CMD curl -fs -o /dev/null http://127.0.0.1:8090/api/status || [ "$(curl -s -o /dev/null -w '%{http_code}' http://127.0.0.1:8090/api/status)" = "401" ]

ENTRYPOINT ["/app/entrypoint.sh"]
