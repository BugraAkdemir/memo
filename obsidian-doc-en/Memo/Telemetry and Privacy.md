# 📊 Telemetry and Privacy

> **Package:** `internal/telemetry/` (schema, collect, state, manager, send, install, sanitize)
> **API:** `GET /api/telemetry/status`, `GET|POST /api/telemetry/preview`, `POST /api/telemetry/consent|reset-id|delete`
> **CLI:** `memo telemetry status|preview|on|off|reset-id|delete`
> **Data:** `data/install_id`, `data/telemetry_state.json`
> Full inventory: `docs/TELEMETRY.md`

> [!IMPORTANT]
> In **official builds** Memo sends a small, anonymous, content-free daily usage summary **by default** — also when you update from a version that had no such feature, and without a question. It takes one switch to turn off: the privacy step of the [[Setup Wizard]], or **Settings → Privacy**. A previously saved "no" is kept. Builds made from source send nothing unless you turn it on; `MEMO_TELEMETRY=off` also switches it off.

## What it contains
A random 128-bit install ID (not tied to you or your hardware), version and platform, interface language, the class of your GPU and rounded memory sizes, how much each kind of model and provider was used (types and catalogue names, counts, token totals, speed ranges), classes of provider errors, update results, where Memo itself crashed (function names and file/line from Memo's own code, hashed), which features were used, and a size range for your memory.

## What it never contains
Messages, replies, chat titles, memory contents, contacts, calendar, WhatsApp or Telegram content, pictures or audio, file or project names and paths, keys, passwords, e-mail addresses, your user or computer name. Private chats and Minimal Mode turns are not recorded.

## When and how
Only completed days after sharing started are reported, about every three hours (offline days are retried up to eight days back) to `https://tel.memocpp.com`. `memo telemetry preview` shows the exact report before anything is sent. A separate **install count** is sent once per data folder by official builds, with only version, system and architecture — no ID — even when daily sharing is off. The report carries no IP address, but like any web request the server's connection sees where it came from.

## The rest of the privacy model
See [[Incognito Mode]], [[Cloud Sync]] (encrypted before upload), [[Developer API Gateway]] (keys enforced for non-loopback callers), and [[Digital Twin and Mail]] (mail treated as hostile text).
