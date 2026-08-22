# Self-Hosting Memo

Memo can run as a real, always-on service on its own machine — a Raspberry
Pi, a home server, a VPS — instead of being tied to one computer's desktop
session. This page covers everything that's specific to that setup: how to
install just the headless server, how to secure it, and how to manage it
without ever opening a desktop window on the server itself.

Everyday use is unchanged: point your regular desktop or mobile Memo app at
the server's address and it works exactly like a local instance — full
feature parity, nothing server-specific to learn on the client side.

---

## 1. Installing just the server

Two ways to get the headless backend with no Flutter desktop app installed
on the machine:

### Native install

```bash
curl -fsSL https://download.bugradev.com/get-memo-server.sh | bash
```

Auto-detects Linux x86_64, Linux arm64 (Raspberry Pi and other ARM boards),
and macOS. Re-running the same command later updates an existing install in
place (binaries refreshed, config/data preserved) — safe to re-run anytime.
On a fresh Linux install it offers to set the backend up as a systemd
service right away (see [§3](#3-running-as-a-service-systemd)).

This is `get-memo.sh`'s headless sibling — same release archives, same
`~/.memo` layout, same PATH wrapper — it just skips copying the desktop
binary, its Flutter assets, and the application-menu entry, since a
server has no display to show them on.

### Beta vs. stable channel

Two separate script/archive pairs exist, matching the desktop installer's
own `get-memo.sh` / `get-memo-beta.sh` split:

| Script | Archives it pulls | Updated by CI on |
|---|---|---|
| `get-memo-server.sh` (stable) | `memo.tar.gz` / `memo_arm.zip` / `memo-mac.zip` | A `vX.Y.Z` tag push (a real, tagged release) |
| `get-memo-server-beta.sh` | `memo_beta.tar.gz` / `memo_arm_beta.zip` / `memo-mac_beta.zip` | **Every push to `main`** |

This isn't a documentation detail — it changes which one you actually want.
Self-hosting-specific work (Docker/ARM CI, this four-mode auth system, the
`memo config`/`memo remote`/`memo service` commands) lands on `main` first
and only reaches a tagged release later. If a feature is described here but
`get-memo-server.sh` doesn't seem to have it yet, `get-memo-server-beta.sh`
almost certainly does — it's rebuilt on every single push. Re-running either
script later updates an existing install in place the same way; switching
from stable to beta (or back) is just running the other script once.

### Docker / CasaOS

```bash
docker compose -f docker/docker-compose.yml up -d
```

A multi-arch (amd64 + arm64) image is published automatically to
`ghcr.io/bugraakdemir/memo-backend` on every push. See
[`docker/README.md`](../docker/README.md) for the full compose file, volume
layout, and CasaOS-specific notes.

---

## 2. Securing it: auth modes

A server bound only to `127.0.0.1` (the desktop app's own local backend)
never requires a credential — that's unchanged. The moment it's reachable
from the network (`--lan`, Docker's port mapping, Tailscale, ngrok), one of
four auth modes applies:

| Mode | What it checks | When to use it |
|---|---|---|
| `none` | Nothing — anyone who can reach the port gets in | Never on a real network. A visible warning is shown everywhere (Settings, `--lan` startup log, `memo remote status`) whenever this is active on an exposed listener. |
| `token` (default) | A per-device token | The default and simplest option — pair each device once, revoke individually if one is lost. |
| `password` | Username + password (argon2id-hashed), a short-lived signed session token per login | When you'd rather type a password than copy a token to each device. |
| `token_password` | Either a valid device token **or** a valid session — either satisfies | Want both options available at once (e.g. tokens for your own devices, a password for occasional access from somewhere else). |

Password-mode logins are rate-limited independently of the general API
limiter: a couple of free attempts, then exponential backoff — a trivial
`admin`/`admin` script hits multi-minute waits within a dozen guesses.

Set the mode from Settings → Remote Access on desktop, or over SSH:

```bash
memo remote set-mode token_password --username you --password 'a real password'
memo remote list-devices
memo remote add-device "My Phone"      # prints its token once — copy it now
memo remote revoke-device <id>
```

On top of that, `password`/`token_password` modes support a real **multi-account
model**: an admin account plus any number of user accounts, each with its
own password and its own set of seven granular permissions (Models, Memory,
Agent, Calendar, WhatsApp, Telegram, Routines) an admin can grant or deny.
This is still access control to one shared backend, not per-account
separate memory/data (see [§5](#5-known-limitations)) — but it's no longer
just "how one person authenticates to their own server." Manage accounts
from Settings → Accounts on desktop, or over SSH:

```bash
memo remote list-accounts
memo remote add-account "friend" --role user --perm models,agent,routines
memo remote delete-account <id>
```

`add-account` defaults `--role` to the most restrictive `user` role with no
permissions granted; pass `--perm` to turn specific ones on. Omit
`--password` to be prompted for it interactively (hidden input) instead of
putting it in shell history.

> **Bootstrapping the very first token:** once bound to `0.0.0.0`
> (`--lan`), the API requires a credential on *every* request — including
> `memo remote` calls run locally, over SSH, on the server itself. The
> auth gate only checks the listener's bind address, not where the caller
> is. So the device token auto-generated on first `--lan` enable can't be
> read back with `memo remote status` (that call itself would 401,
> credential-less). It's only ever printed to the backend's own process
> log — under `memo service install`, that means the systemd journal, not
> your terminal:
> ```bash
> journalctl --user -u memo.service --no-pager | grep -i token
> ```
> `memo remote status`/`list-devices`/etc. also print this same hint
> automatically when they hit a 401, so this isn't something you have to
> remember — just easier to know up front. Setting a password instead
> (`memo remote set-mode password ...`) sidesteps the whole bootstrap
> problem, since you choose the credential yourself rather than reading
> one back.

---

## 3. Running as a service (systemd)

```bash
memo service install --lan     # installs, enables, and starts it now
memo service status
memo service uninstall
```

This installs a **systemd `--user`** service (`~/.config/systemd/user/
memo.service`) — no root/sudo needed, matching a single-user self-hosted
setup. It restarts automatically on failure. One thing a user service can't
do on its own: start before any login session exists at all (relevant right
after a headless Raspberry Pi reboot). For that, run once:

```bash
loginctl enable-linger $(whoami)
```

---

## 4. Managing it entirely over SSH

Everything above is reachable without ever installing a desktop app on the
server:

```bash
memo config get llama.port           # read any config.yaml key
memo config set llama.ctx_size 8192  # write one — takes effect on restart
memo remote status                   # auth mode, addresses, warnings
memo service status                  # is it running?
```

`memo config` deliberately refuses anything under `remote_access.*` — that
section has its own commands above, because a few of its fields (the
password hash, the device list) need real validation a raw config edit
would bypass.

For the rare moment nothing else is reachable, a minimal built-in web page
(no separate install — it's baked into the backend binary) is served at
`http://<server-ip>:<port>/`: chat, basic model/provider settings, and a
Remote Access panel with the current auth status and a restart button. It's
intentionally not a full admin panel — device/auth-mode management stays on
the desktop app or the CLI, both of which already do real validation this
static page would otherwise have to duplicate.

---

## 5. Known limitations

- **No built-in TLS yet.** Memo's own listener is plain HTTP. For traffic
  leaving your LAN, put it behind something that terminates TLS — your own
  reverse proxy, or Memo's built-in Tailscale/ngrok tunnels (both already
  encrypt the transport for you, no extra setup).
- **`memo service` is Linux-only** (systemd). No launchd unit for macOS yet.
- **No tunnel management from the CLI yet** — Tailscale/ngrok start/stop is
  still Settings-only; `memo remote`/`memo config`/`memo service` cover
  auth, config, and process lifecycle, not tunnels.
- **Accounts share one backend, not isolated data.** Multiple accounts on
  one server get their own login and their own permission toggles, but
  memory/chat history/models are still one shared store — this is
  permission-gating, not per-user data isolation. Real per-user isolation
  is a separate, not-yet-started roadmap item.
- **Agent permission is a single global flag.** Denying the `agent`
  permission on a user account turns off Agent mode entirely for that
  account; it can't yet scope down to individual tools (e.g. allow
  `web_search` but not `run_command`).

---

*See also: [Features](FEATURES.md#remote-access--self-hosting) ·
[Docker/CasaOS](../docker/README.md) · [API Reference](API_REFERENCE.md)*
