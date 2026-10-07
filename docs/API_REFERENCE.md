# API Reference

Memo Backend runs a REST API on `localhost:8090` (default). Every `/api/*` route is also aliased under `/api/v1/*` (see `route()` in `internal/webserver/server.go`).

## Authentication
Local (`localhost`) connections are open, no token required. **Remote access (LAN, ngrok, or Tailscale) requires the access token** shown in Settings on every request — this was previously optional and is now enforced (v3.3.3 security fix). The mobile app already sends it; a custom tool talking to the remote API directly needs to add it too.

For a self-hosted server, this token model sits alongside a full **account system**: `token`, `password`, `token+password`, or (opt-in, loudly warned about) `none` auth mode, selectable per-server. A server can host more than one account — an admin plus any number of user accounts — each with its own password and its own set of seven granular permissions (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines). Permission checks are enforced per-request: `requirePermission` (GET/HEAD-exempt) or `requirePermissionStrict` (no exemption, used for Memory) wrap the relevant handlers below. See the **Accounts & Permissions** section and [Self-Hosting](SELF_HOSTED.md).

## Developer API Gateway (Anthropic-compatible)
`POST /v1/messages` implements the server side of Anthropic's Messages API wire format (`internal/anthropicapi/`), so tools that only know how to talk to Anthropic — most notably **Claude Code** via `ANTHROPIC_BASE_URL` — can point at Memo instead. Model selection uses a `type/model-id` format (`local/qwen2.5`, `openai/gpt-4o`, ...). See Sidebar → Developer for the base URL/token/live request log.

`POST /v1/models` and `POST /v1/chat/completions` (`internal/openaiapi/`) are the OpenAI-compatible sibling of the same gateway, for tools that only speak OpenAI's wire format. Both this and the Anthropic-compatible endpoint above now **enforce the configured API key for any non-loopback caller** (v4.5.0 security fix) — previously, with "Require API Key" left off and remote access on, either could be reached by anything else able to reach the port, no credential at all.

This list below is not exhaustive — there are 180+ registered endpoints as of v4.6.0. It groups the major ones by area; see `internal/webserver/server.go`'s `route(...)` calls for the full, current list.

## Endpoints

### 💬 Chat
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/send` | Send a standard JSON message (non-streaming) |
| `POST` | `/api/send/stream` | SSE streaming response |
| `POST` | `/api/send_file` | File/image message (Multipart) |
| `GET` | `/api/chats` | List all sessions |
| `POST` | `/api/chats/new` | Create new session |
| `POST` | `/api/chats/switch` | Switch active session |
| `POST` | `/api/chats/delete` | Delete session |
| `GET` | `/api/messages` | Get a chat's history. Pass `chat_id` — without it the call acts on the global active chat, which another client can switch at any moment. `/api/messages/update` and `/api/messages/delete` take `chat_id` too |
| `GET` | `/api/chats/streaming` | `{"chat_ids": [...]}` — every chat that is generating a reply right now (the sidebar's "still working" spinner; covers background tasks, WhatsApp/Telegram replies and other browser tabs) |
| `GET` | `/api/status` | System status + memory count |
| `POST` | `/api/incognito` | Toggle incognito mode |
| `GET`/`PUT` | `/api/system-prompt` | Get/update system prompt |

#### Marker chunks on a chat SSE stream (`/api/send/stream`, `/api/send/file/stream`)
A streamed chunk is ordinary reply text unless its `finish_reason` names one of these markers. Clients must treat an unknown marker as "ignore", never print it.

| `finish_reason` | `content` | Meaning |
| :--- | :--- | :--- |
| `heartbeat` | empty | Sent after 10 s of silence so a long, quiet thinking phase is not mistaken for a dropped connection. A client's own timeout should be an *idle* timeout that these reset |
| `agent_event` | JSON | A tool event (call, result, permission request) — render as a badge, never as text. Parse defensively |
| `browser_frame` | JSON `{screenshot, timestamp}` | A fresh screenshot of the interactive browser after a page-changing tool. Streams to the live panel only and is never saved into the chat history |
| `quota_exhausted` | JSON `QuotaSignal` | The turn failed because the allowance behind the model ran out; arrives just before the error chunk |
| `quota_low` | JSON `QuotaSignal` | The turn worked but 10% or less is left; sent once per allowance window |

`QuotaSignal` (`internal/models/quota_signal.go`): `kind` (`exhausted` \| `low`), `provider`, `model`, `vendor`, `remaining_percent` (0–100, `-1` unknown), `reset_at` (RFC 3339, empty when unknown), `window` (`5h`, `7d` …). The quota markers are added in one place, `Server.withQuotaSignals`; CLI-agent streams, WhatsApp streams and the Self-Driving loop are not wrapped. A plain chat turn is ended by *silence* (300 s with no chunk, first word included) plus a 30-minute cap, not by a fixed total length.

### 🧠 Memory
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/memory/files` | List memory files |
| `DELETE` | `/api/memory/files` | Delete a memory file |
| `POST` | `/api/memory/clear` | Clear all memory |
| `GET` | `/api/memory/known-facts` | Every currently pinned fact ("what Memo knows about you") — same shape as the debug-search results, no query needed |
| `GET` | `/api/memory/conversation?limit=&offset=` | A page of ordinary (non-pinned) conversation memories: `{results, total}` |
| `POST` | `/api/memory/delete-by-ids` | `{ids: [...]}` — delete exactly those records (never a pattern); returns `{deleted: n}` |
| `POST` | `/api/memory/pinned/update` | `{id, content, tags}` — rewrite one pinned fact |
| `POST` | `/api/memory/explicit/save`, `/api/memory/explicit/delete` | Save / remove an explicit "remember this" memory (works with no embedding model configured) |

### 🏭 Models
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/models/local` | List downloaded .gguf files |
| `DELETE` | `/api/models/local` | Delete a local model |
| `POST` | `/api/models/start` | Start a model (spawn llama-server) |
| `POST` | `/api/models/stop` | Stop running model |
| `GET` | `/api/models/status` | Model runtime status |
| `GET` | `/api/gpu` | GPU detection info (NVIDIA/AMD/Metal) |
| `POST` | `/api/models/search` | Search HuggingFace for GGUF files |
| `POST` | `/api/models/download` | Start model download |
| `GET` | `/api/models/download/progress` | Download progress stream |
| `GET` | `/api/models/llama/check` | Check if llama.cpp binary exists |

### 🔌 External Providers
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/providers` | List all provider configs |
| `PUT` | `/api/providers` | Add/update a provider config |
| `DELETE` | `/api/providers` | Delete a provider config |
| `POST` | `/api/providers/test` | Test provider connection |
| `GET` | `/api/providers/active` | Get active provider type |
| `PUT` | `/api/providers/active` | Set active provider |
| `GET` | `/api/providers/effort-levels` | Reasoning-effort levels for the active model, if supported |
| `POST` | `/api/openrouter/connect` | OAuth connect flow for OpenRouter |
| `GET` | `/api/kilo/models` | Live model list from Kilo Code (app.kilo.ai), free models flagged |
| `GET` | `/api/opencode-zen/models` | Live model list from OpenCode Zen, free models flagged (`-free` id suffix) |

### 👤 Accounts & Permissions (self-hosted)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/setup/status` | Whether a self-hosted server has an admin account yet |
| `POST` | `/api/setup/create-admin` | First-run: create the admin account |
| `POST` | `/api/setup/create-device` | Pair a new device/token under an account |
| `GET`/`POST` | `/api/accounts` | List accounts / create a new account |
| `GET`/`PUT`/`DELETE` | `/api/accounts/{id}` | Get/update/delete one account |
| `PUT` | `/api/accounts/{id}/password` | Change an account's password |
| `GET`/`PUT` | `/api/accounts/{id}/permissions` | Get/update an account's 7 granular permissions |

### 🤖 Agent Mode
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/agent/enabled` | Get agent mode status |
| `PUT` | `/api/agent/enabled` | Enable/disable agent mode |
| `POST` | `/api/agent/permission` | Respond to a permission request |
| `GET` | `/api/agent/permissions` | List permanent permissions |
| `DELETE` | `/api/agent/permissions` | Revoke (with `?id=`) or clear all permissions |

### 💚 WhatsApp
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/whatsapp/status` | Pairing/connection status |
| `POST` | `/api/whatsapp/start` | Start the client, generate a QR pairing code |
| `POST` | `/api/whatsapp/stop` | Stop the client |
| `POST` | `/api/whatsapp/logout` | Log out and clear the paired session |
| `POST` | `/api/whatsapp/send` | Send a message |
| `GET` | `/api/whatsapp/search` | Search WhatsApp message history |
| `GET` | `/api/whatsapp/chats` | List recent chats |
| `GET` | `/api/whatsapp/messages` | Get messages for a chat |
| `GET` | `/api/whatsapp/avatar` | Fetch a contact's avatar |
| `GET` | `/api/whatsapp/stats` | Message/contact counters |
| `PUT` | `/api/whatsapp/chat-mode` | Configure the dedicated WhatsApp-only chat executor |
| `POST` | `/api/whatsapp/chat-stream` | SSE stream for the WhatsApp-only chat mode |
| `POST` | `/api/whatsapp/self-chat-assistant` | Enable/configure the self-chat assistant (message your own number, get a full Memo assistant back) |

### ✈️ Telegram
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/telegram/status` | Bot connection/owner-link status |
| `POST` | `/api/telegram/connect` | Connect with a bot token, start long-polling |
| `POST` | `/api/telegram/stop` | Stop the client without clearing the token |
| `POST` | `/api/telegram/disconnect` | Disconnect and clear the stored token/owner link |

### 🚗 Self-Driving Task Loop
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`POST` | `/api/tasklists` | List task lists / create one from a `Task.md` |
| `GET`/`PUT`/`DELETE` | `/api/tasklists/{id}` | Get/update/delete a task list |
| `POST` | `/api/tasklists/{id}/plan` | Trigger a planning turn (planner mode) |
| `POST` | `/api/tasklists/{id}/approve-plan` | Approve a pending `Plan.md` |
| `GET` | `/api/tasks/running` | Live view of the currently-running task list |
| `GET` | `/api/tasks/events` | SSE stream of task-loop activity |
| `POST` | `/api/tasks/{id}/pause`, `/resume`, `/cancel`, `/skip`, `/inject` | Control a running task list from outside the model (pause/resume/cancel current item, skip it, or inject a message) |
| `GET`/`PUT` | `/api/taskloop/settings` | Persistent task-loop configuration |

### 🛠️ Code Mode
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`POST` | `/api/code-mode/prompt` | Get/set the system prompt for a Code Mode sub-mode (`plan`/`auto`/`build`) |
| `POST` | `/api/code-mode/prompt/reset` | Reset a sub-mode's prompt to its default |

### 🐾 Desktop Mascot
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/mascot/activity` | App-wide "what is Memo doing right now" activity signal the mascot window polls (also consumed by the terminal REPL's spinner) |

### 🎙️ Live Mode v2 (native audio-to-audio)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/livemode/engines` | List/select the configured voice engine (Google Live, OpenAI Realtime, local fallback) |
| `GET` | `/api/livemode/engines/models` | Live model list for the selected engine |
| `POST` | `/api/livemode/session` | Start/manage a Live Mode v2 session |
| `GET`/`PUT` | `/api/livemode/active` | Get/set whether Live Mode is the active surface for a chat |

### 🎵 Orchestra Mode
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/orchestra/config` | Get orchestra configuration |
| `PUT` | `/api/orchestra/config` | Update orchestra configuration |

### ☁️ Cloud Sync
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/sync/settings` | Get/update Google Drive sync settings |

### ⚙️ Config
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/config/llama` | Get/update llama configuration |
| `GET` | `/api/image?path=` | `{data: <base64>}` for an image a chat message stored. Only files under the backend's own image directories are served (`..` and everything else is refused). Clients must load chat images through this, never from a local path — the file is on the backend's disk |
| `POST` | `/api/models/embedding/start` | Start embedding server |
| `POST` | `/api/models/embedding/stop` | Stop embedding server |

### ⏰ Routines
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`POST` | `/api/routines` | List / create routines |
| `POST` | `/api/routines/parse` | Turn a plain-language description into a routine config |
| `GET`/`PUT`/`DELETE` | `/api/routines/{id}` | Get/update/delete a routine |
| `POST` | `/api/routines/sync-offset` | Resync a device's timezone offset for its routines |

### 🔔 Proactive Learning & Self-Insight
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/proactive/settings` | Get/update proactive learning level |
| `GET` | `/api/proactive/pending` | Poll for a pending suggestion |
| `POST` | `/api/proactive/respond` | Accept/dismiss/suppress a suggestion |
| `GET` | `/api/proactive/patterns` | List learned patterns |
| `POST` | `/api/proactive/patterns/forget` | Forget one pattern |
| `POST` | `/api/proactive/clear` | Clear all patterns |
| `GET` | `/api/memory/insight` | `/insight` — summarize recent mood/memory patterns |

### 🎙️ Live Mode / Text-to-Speech
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/tts/synthesize` | Synthesize speech for a chat reply |
| `POST` | `/api/tts/filler` | Short "thinking" filler sound |
| `GET`/`PUT` | `/api/tts/providers` | List/select TTS provider (local Piper or external) |
| `POST` | `/api/tts/providers/test` | Test an external TTS provider |
| `GET` | `/api/tts/voices` | List downloadable/local Piper voices |
| `POST` | `/api/tts/voices/download` | Download a voice |
| `POST` | `/api/tts/voices/select` | Select active voice |

### 🖧 Memo Swarm (beta)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/swarm/status` | Current room/worker status |
| `POST` | `/api/swarm/host/create` | Create a Swarm room (become Host) |
| `POST` | `/api/swarm/host/workers/add`/`remove`/`reorder`/`share` | Manage joined workers and their compute share |
| `POST` | `/api/swarm/host/start`/`stop`/`close` | Control the Swarm session |
| `POST` | `/api/swarm/join`/`leave` | Join/leave a room with a room code |

### 📊 Usage Stats
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/stats/usage` | Requests, tokens, avg tok/s, per-model breakdown, 30-day history, and the prompt-cache split: `total_cached_prompt_tokens`, `total_cache_write_tokens` plus `cached_prompt_tokens` / `cache_write_tokens` on each per-model and per-category row. A zero means "the provider reported nothing", not a measured 0% |

### 🖥️ Claude Code / Codex CLI Providers (beta)
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/cli/status` | Whether `claude`/`codex` are installed, with version |
| `GET` | `/api/cli/running` | CLI jobs currently running in the background |
| `GET` | `/api/cli/commands` | The CLI's own `/` commands (project/personal/skill/built-in) |
| `GET`/`PUT` | `/api/chats/cli-provider`, `/api/chats/cli-workdir`, `/api/chats/cli-model` | Per-chat CLI provider/working-dir/model |
| `POST` | `/api/send/cli-stream` | Send a message to a CLI-provider chat (SSE) |

### 🛠️ Developer API Gateway
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/dev-gateway/config` | Get/update the gateway's config (API key requirement, memory integration) — shared by both the Anthropic- and OpenAI-compatible endpoints |
| `GET` | `/api/dev-gateway/models` | List `type/model-id` selectable models across local + configured providers |
| `GET` | `/api/dev-gateway/logs` | Live request log |
| `GET` | `/api/dev-gateway/claude-code-cli` | Claude Code CLI connection helper/status |
| `POST` | `/api/dev-gateway/token/rotate` | Rotate the gateway's API key |

### 🔑 Subscriptions & model selector
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` / `POST` | `/api/subscriptions` | Subscriptions (the bundled CLIProxyAPI sidecar): state (bundled?, running?, accounts, models, sign-in in flight) and the actions `login` (`provider`: `antigravity` \| `claude` \| `codex`), `cancel_login`, `logout`. POST is admin-only; GET never starts the sidecar and waits at most ~2 s for quota figures |
| `GET` | `/api/providers/model?name=<provider>` | That provider's live models. For `Subscriptions` each model may carry `remaining` (0–1), `reset_at` and `quota_window`. Answers from a quota cache at once; add `fresh=1` (the picker's background refresh) to wait up to 3 s for current figures. Needs the models permission — it spends a stored key |
| `PUT` | `/api/providers/model` | `{name, model, activate}` — switch only that provider's model (every other setting survives); `activate` also makes it the active provider, so a selector needs one call |
| `GET` | `/api/providers/models` | Model list for a key the *caller* supplies (never a stored one) |

Quota comes from the vendor with the token in the credential file, which goes only to the vendor — never to this API, logs or UI. Antigravity reports it per model; Codex and Claude meter the account in windows, so the tightest window is applied to all of that vendor's models. The Claude parser is written from the endpoint's known shape and has not been verified against a live Claude sign-in.

### 🗂️ Skills
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/skills/list` | List installed skills |
| `POST` | `/api/skills/install` | Install a skill |
| `DELETE` | `/api/skills/remove/{name}` | Remove a skill |
| `GET` | `/api/skills/get/{name}` | Get one skill's manifest |
| `GET` / `PUT` | `/api/skills/active-list?chat_id=`, `/api/skills/active` (`{chat_id, names}`) | Get/set the skills active **in one chat**. Activation is per chat; a new chat starts with none |

### 🌐 Interactive Browser Pane
The live browser panel next to the chat drives one isolated Chromium session (`internal/browserengine/`). All session routes need the agent (tool-execution) permission, accept only `http`, `https` and a blank page (no `file://`), and answer `{screenshot_base64, url, error}` so the panel can repaint from a single round trip. `GET`/`PUT /api/browser`, `POST /api/browser/install` and `GET /api/browser/install/progress` manage the optional browser install.

| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/browser/session/navigate` | `{url}` — an address without a scheme is accepted |
| `POST` | `/api/browser/session/click` | `{x, y}` in the screenshot's pixel space |
| `POST` | `/api/browser/session/type` | `{text, enter}` — typed into whatever the last click focused |
| `POST` | `/api/browser/session/scroll` | `{dx, dy}` |
| `GET` | `/api/browser/session/status` | `{active, url}` |
| `POST` | `/api/browser/session/close` | End the session |

### 💾 Backup / Export / Wipe
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `POST` | `/api/export` | Full `.memo` backup (now includes calendar, routines, task lists, permissions, skills, `machine.key`) |
| `POST` | `/api/import` | Restore from a `.memo` backup |
| `POST` | `/api/wipe` | Factory reset (all internal DBs closed before file removal, fixed on Windows) |

### 🌐 Remote Access
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET`/`PUT` | `/api/remote-access` | LAN/ngrok/Tailscale config, requires the access token on remote requests |

---

**Gating.** Every new route needs an explicit decision: destructive actions (export, import, wipe, uninstall, shutdown) are admin-only; state-changing admin settings whose GET is read ambiently are admin-write with a redacted GET; a GET that carries a credential is redacted for callers without the permission. The single-user desktop (no credential) is never affected.

*For detailed JSON payloads, refer to `internal/webserver/handlers_flutter.go`, the other `handlers_*.go` files, and `internal/webserver/server.go`.*
