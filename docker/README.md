# Memo on CasaOS / Docker

This is a **backend-only** image: Memo's headless Go REST API, no Flutter
GUI, no browser page to open. You get a running Memo brain on your CasaOS
box, and you talk to it with the same Flutter desktop app or `memo` CLI you'd
otherwise run locally — pointed at the container instead of at localhost.

If what you actually want is "open a browser tab and chat" on CasaOS itself,
that needs a Flutter *web* build (a much larger, separate project — the
Flutter frontend currently only targets linux/macos/windows, and several
screens use `dart:io` directly). This image does not attempt that.

## 1. Build and push the image

CasaOS's "Install a customized app" screen installs a `docker-compose.yml`
that references an already-built `image:` — it does not build from a
Dockerfile itself. Build it wherever you have Docker, push it to a registry
your CasaOS box can pull from (Docker Hub, GHCR, a private registry), then
point `docker-compose.yml` at it.

```bash
docker build -t <your-registry>/memo-backend:latest .
docker push <your-registry>/memo-backend:latest
```

The image bundles the **CPU** llama.cpp backend only (no GPU passthrough
assumed on a typical NAS/home-server box). Local GGUF chat/embedding models
work, just at CPU speed — for anything beyond a small model, configuring an
external provider (OpenAI, Claude, Gemini, OpenRouter, ...) in Settings after
first boot is the realistic choice on this class of hardware anyway. Voice
transcription (Whisper) isn't bundled — the model wasn't shipped in the image
to keep it small; that feature stays unavailable in this deployment for now.

Not built for arm64 — the bundled llama.cpp/vec0 binaries under
`binaries/linux/cpu` in this repo are x86_64 only, and `docker-compose.yml`'s
`x-casaos.architectures` reflects that (amd64 only). A Raspberry Pi or other
ARM CasaOS box needs its own arm64 llama.cpp/vec0 build substituted in first.

## 2. Install on CasaOS

App Store → **Install a customized app** → paste `docker/docker-compose.yml`
(after editing the `image:` line to the one you pushed) → adjust the
`/DATA/AppData/memo` volume path if you want it elsewhere → Install.

Without CasaOS, plain `docker compose up -d` from this directory works the
same way.

## 3. Get the access token

The container binds `0.0.0.0:8090` (needed for Docker's own port-forwarding
to reach it at all — a loopback-only bind is unreachable through `-p`/CasaOS
port mapping) and, because of that, requires an `X-Memo-Token` header on
*every* request, including from other containers/localhost. This is the same
token/middleware Memo's desktop "Remote Access" feature already uses — see
`AGENTS.md`'s Security section — just turned on unconditionally at boot via
the `--lan` flag instead of through a Settings toggle, since there is no GUI
to click here.

The token is generated on first boot and persisted into the mounted volume
(`/DATA/AppData/memo/config/config.yaml`, survives restarts/updates). Read it
from the logs:

```bash
docker logs memo | grep "X-Memo-Token required"
```

## 4. Connect a client

**Flutter desktop app** — Settings → Remote Access → "Backend Server URL"
section:
- Backend URL: `http://<casaos-ip>:8090`
- Access Token: the `memo-...` token from step 3
- Apply, then reconnect

(This token field is new — added alongside this Docker image, since the
existing "Backend Server URL" box had no way to authenticate against a
backend that requires the token from its very first request. The desktop
app's own *self*-hosted remote access, by contrast, learns its token
automatically because the app is on the same origin as the backend it
spawned — see `savedRemoteToken`/`onRemoteTokenLearned` in
`frontend/lib/core/api_client.dart` — that shortcut doesn't exist when
you're a pure client of somebody else's backend.)

**Terminal `memo` CLI** — not supported today. `internal/replcli`'s client
always talks to `127.0.0.1`; there is no flag to point it at a remote host.
Out of scope for this change; flagged here so it isn't assumed to work.

**Direct REST/scripting** — any HTTP client, `X-Memo-Token: <token>` header
on every request:

```bash
curl -H "X-Memo-Token: memo-..." http://<casaos-ip>:8090/api/status
```

## Data & updates

Everything persistent lives under the single `/memo` volume
(`/DATA/AppData/memo` on the host by default) — `data/` and `config/` as
usual, just rooted there instead of next to the binary. Re-pulling a newer
image and recreating the container keeps all of it; nothing above ever
touches that volume except to seed a clean `config.yaml`/`providers.json` the
very first time it's empty.

## What hasn't been verified live

This Dockerfile/compose file was written and reviewed against the real
codebase (`internal/config` path resolution, `internal/webserver`'s auth
middleware, the actual `ldd` output of the bundled `llama-server` binary,
etc.) but **`docker build`/`docker run` were never actually executed** — no
Docker daemon is available in the environment this was written in. Before
trusting it in production:

```bash
docker build -t memo-backend:test .
docker run --rm -p 8090:8090 -v memo-test-data:/memo memo-backend:test
docker logs <container> # confirm it boots, binds 0.0.0.0, prints a token
curl -i http://127.0.0.1:8090/api/status                          # expect 401
curl -i -H "X-Memo-Token: <token>" http://127.0.0.1:8090/api/status # expect 200
```

If the build fails on a missing apt package or an unexpected `ldd` dependency
on your build machine's exact `llama-server` build, that's the first place to
look — the runtime stage's package list was derived from `ldd` against the
binary already committed in this repo, not guessed.
