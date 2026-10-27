# Anonymous daily summary (telemetry)

Memo can send **one small, anonymous summary per day** so we can learn which computers and models people really use, what is too slow, and what breaks. It is **optional and off until you say yes**. Nothing is counted or sent before you answer, and you can change your mind at any time.

This page is the whole contract: what is asked, what is sent, what is never sent, and how to look at it, turn it off or delete it. The list of fields below is checked by a test against the code, so it cannot drift.

## The choice

- **Official builds** ask once, on first run after the setup wizard, with two buttons of equal weight: *Yes, share* and *No, do not share*. Nothing is preselected and there is no "later". If you close Memo without answering, nothing is counted or sent and the question comes back next launch.
- **An update from a version that had no telemetry** is treated the same way: you have not been asked yet, so nothing is sent until you answer.
- **Builds from source never ask** and stay off unless you turn them on (they then report `build: "source"`, so community builds do not mix into the official numbers).
- **`MEMO_TELEMETRY=off`** in the environment beats every answer: nothing is counted, nothing is sent, no network request is made.

Change it any time in **Settings › Privacy**, or from a terminal:

```bash
memo telemetry status            # where things stand
memo telemetry preview [--raw]   # the exact content of the next summary
memo telemetry on                # share the anonymous daily summary
memo telemetry off               # do not share
memo telemetry reset-id          # a new random id (the server is asked to forget the old one)
memo telemetry delete            # delete your data on the server and stop sharing
```

## Look before you decide

`memo telemetry preview --raw` (and *Settings › Privacy › Show the content*) prints **the exact bytes that would be sent**. The preview and the sender call the same function, so what you read is what leaves. Looking sends nothing.

## When and how it is sent

- At most **one report per completed UTC day**, covering that day only. Never today, and never a day on or before the day you said yes.
- Only the day is sent, never a time of day.
- A report that could not be delivered is retried for at most **8 days back**; older days are dropped.
- Over HTTPS to `tel.memocpp.com`. The server answers only to a fixed, validated format and refuses anything else.
- The server stores a **keyed hash** of your random id, never the id itself. Your IP address is never read, logged or stored; only a country, which Cloudflare's edge adds, is used.

## Never sent

- The content of messages, replies and chat titles
- Memories, contacts, calendar, WhatsApp and Telegram messages
- File names, folder paths, project names
- API keys, passwords, tokens, sessions
- Your IP address
- Your user name, computer name, e-mail
- The time of day (only the day, UTC)
- Graphics driver version, serial numbers, processor model name
- Names of models you made yourself (anything not in a known list becomes `other`)
- Crash error messages, variable values and any code that is not Memo's
- **Incognito and Minimal Mode turns, and every number derived from them**

## What is sent

Every field is a number, a coarse class, or a name picked from a fixed list. There is no field that could hold free text.

- **Identity and time.** A random installation id, the day, Memo's version, official or source build, operating system, architecture, interface language. Used so the same device is not counted twice and problems can be tied to a version.
- **Hardware (coarse).** Graphics card class and a name from a known catalogue (anything else is `other_<vendor>`), video memory, which engine the local model runs on, processor cores and threads, RAM rounded to a standard size.
- **Usage counts.** Per day: how many turns and tokens per kind (local, own API key, subscription), provider type, model (from a catalogue, else `other`) and purpose. Counts only, never messages.
- **Speed.** For local models: how many samples fell in each (model size, hardware, tokens-per-second, time-to-first-token) bucket.
- **Failures.** Local engine start failures and unexpected exits by a fixed reason code; provider errors by class (`401`, `429`, `5xx`, `quota`, `timeout`, `network`); update outcomes (applied, failed, rolled back; checksum or signature as the reason).
- **Crashes.** The *type* of the error and up to five of Memo's own function names, plus a hash of them. The panic message is never sent.
- **Features and milestones.** Which features were used that day, and the dates of three firsts (installed, first provider added, first message).

### Every field

<!-- fields:begin -->
- `schema` — the report format number
- `tid` — random installation id (the server stores only a keyed hash)
- `day` — the UTC day the report covers, no time
- `app_version` — Memo's version
- `build` — `official` or `source`
- `os` — linux, windows or darwin
- `arch` — amd64 or arm64
- `ui_lang` — the interface language
- `hw.gpu_class` — none, nvidia, amd, intel or apple
- `hw.gpu_model` — a catalogue name, or `other_<vendor>`
- `hw.vram_gb` — video memory in GB
- `hw.engine` — the hardware path the local model uses
- `hw.cpu_cores` — processor cores
- `hw.cpu_threads` — processor threads
- `hw.ram_gb` — RAM rounded to a standard size
- `usage[].kind` — local, api or sub
- `usage[].provider` — provider type or subscription vendor
- `usage[].model` — a catalogue name, or `other`
- `usage[].category` — what the call was for (chat, agent, title, …)
- `usage[].turns` — number of turns
- `usage[].in_tokens` — input tokens
- `usage[].out_tokens` — output tokens
- `usage[].cached_tokens` — input tokens served from a cache
- `purpose.{key}.turns` — turns per purpose (text or code)
- `purpose.{key}.in_tokens` — input tokens per purpose
- `purpose.{key}.out_tokens` — output tokens per purpose
- `perf[].size_bucket` — local model size bucket
- `perf[].hw_class` — hardware class of the sample
- `perf[].tps_bucket` — tokens-per-second range
- `perf[].ttft_bucket` — time-to-first-token range
- `perf[].n` — samples in that bucket
- `local_start_fail[].component` — llama, embedding or whisper
- `local_start_fail[].code` — permission, port, oom, missing_file, crash_at_load, timeout or unknown
- `local_start_fail[].n` — how many times
- `llama_exit[].component` — llama, embedding or whisper
- `llama_exit[].code` — oom_killed, signal, exit_nonzero or unknown
- `llama_exit[].n` — how many times
- `provider_err[].provider` — provider type
- `provider_err[].code` — 401, 429, 5xx, quota, timeout or network
- `provider_err[].n` — how many times
- `update[].event` — offered, downloaded, applied, rolled_back or failed
- `update[].code` — checksum, signature, needs_newer_app or not_for_this_machine
- `update[].n` — how many times
- `crash[].sig` — hash of the frames
- `crash[].component` — go, dart or llama
- `crash[].exc_type` — the error type name, never its message
- `crash[].frames[]` — up to five of Memo's own function names
- `crash[].n` — how many times
- `features[]` — features used that day
- `milestones.installed` — the day of the first run
- `milestones.provider_added` — the day the first provider was saved
- `milestones.first_message` — the day of the first chat turn
- `memory_rows_bucket` — a coarse size range of the memory store
<!-- fields:end -->

Not everything the format allows is sent yet. This version does not send the `offered` and `downloaded` update events or the `needs_newer_app` and `not_for_this_machine` reasons, does not report `whisper` failures, and reports only crashes of Memo's backend (`go`), not of the app window or the engine process; image and voice use appear in `features[]`, not in `purpose`. A quiet day (no activity) sends nothing.

## Reset and delete

- **Renew the id** gives this install a new random id and a fresh history, and asks the server to forget the old one. If you are offline, the request is kept and retried.
- **Delete my data on the server** asks the server to delete everything belonging to this installation and turns sharing off. The local state file is not part of your backups.

## Where it lives

The local state (your answer, the random id, counters for the current days) is one file, `telemetry_state.json`, in Memo's data folder with owner-only permissions. Counters are kept only while sharing is on.

The server side is a separate, open project: a write-only endpoint (`tel.memocpp.com`) and a read-only admin panel. See its repository for the database schema and what is stored.
