# Memo on CasaOS / Docker

This is a **backend-only** image: Memo's headless Go REST API. There's no
Flutter GUI in it (that stays a separate, dart:io-based desktop app), but the
backend now serves its own small, built-in browser client — open
`http://<casaos-ip>:8090` in any browser and you get a chat screen plus the
handful of settings you need before anything else is possible (starting/
stopping the local model, connecting a provider). See "Open it in a browser"
under step 4.

For everything else — memory, identity, orchestra, calendar, WhatsApp, and
so on — point the full Flutter desktop app at this container instead
(Settings → Remote Access, same as step 4 below); once it's connected, it can
manage the whole backend remotely with no extra code on this side. The
built-in browser client is intentionally not a port of that app.

## 1. The image is already built for you

`docker-compose.yml` points at `ghcr.io/bugraakdemir/memo-backend:latest` —
a **multi-arch image (amd64 + arm64)** built and published automatically by
`.github/workflows/build-docker.yml`. Nothing to build or push yourself for
a normal install: `docker compose up -d` (or CasaOS's "Install a customized
app") pulls the right architecture for your machine on its own, Raspberry
Pi/ARM NAS included.

Tag scheme, same beta/stable split the rest of this repo's release pipeline
uses: `:latest` (and `:vX.Y.Z`) only ever move on a real tagged release;
`:beta` tracks every push to `main` if you want the bleeding edge instead.

The image bundles the **CPU** llama.cpp backend only (no GPU passthrough
assumed on a typical NAS/home-server box). Local GGUF chat/embedding models
work, just at CPU speed — for anything beyond a small model, configuring an
external provider (OpenAI, Claude, Gemini, OpenRouter, ...) in Settings after
first boot is the realistic choice on this class of hardware anyway. Voice
transcription (Whisper) isn't bundled — the model wasn't shipped in the image
to keep it small; that feature stays unavailable in this deployment for now.

**arm64 status:** the image itself is built and pushed for arm64 (native, on
GitHub's hosted arm64 runner — see the workflow), but `x-casaos.architectures`
in `docker-compose.yml` still only lists `amd64` until someone actually boots
it on real ARM hardware and confirms it works end to end (CasaOS uses that
field to decide which architectures to even offer the app on — listing one
that was never live-tested risks shipping something silently broken to every
CasaOS/ARM user at once). If you're testing this on a Raspberry Pi: `docker
pull ghcr.io/bugraakdemir/memo-backend:latest` already gets you the arm64
image directly (Docker resolves the manifest automatically), you don't need
to wait for that flag — just report back whether it actually works.

### Building your own image instead

Only needed for a fork, or testing a local change you haven't pushed:

```bash
docker build -t <your-registry>/memo-backend:latest .
docker push <your-registry>/memo-backend:latest
```

then point `docker-compose.yml`'s `image:` at what you pushed instead of the
GHCR one.

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

**Open it in a browser** — go to `http://<casaos-ip>:8090`. First visit shows
a token-entry screen (paste the token from step 3, stored in the browser's
own `localStorage` so you only enter it once per browser); after that you
land on a chat screen with a Settings tab for starting/stopping the local
model and connecting an external provider. This is the built-in web UI
(`internal/webserver/webui/`), served by the backend itself — no separate
container or port.

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

**`.github/workflows/build-docker.yml` itself is also unverified against a
real run** — YAML-syntax-checked only (no GitHub Actions runner available in
the environment it was written in), same limitation as this Dockerfile had
before someone actually ran it. First real push to `main` is the actual
test. One manual, one-time step it can't do for you: a package pushed with
the workflow's own `GITHUB_TOKEN` is **private** by default — go to the
repo's GitHub page → Packages → `memo-backend` → Package settings → Change
visibility → Public, once, after the first successful push, or nobody
without repo access can `docker pull` it.
