# API Reference

Memo Backend runs a REST API on `localhost:8090` (default). Every `/api/*` route is also aliased under `/api/v1/*` (see `route()` in `internal/webserver/server.go`).

## Authentication
Local (`localhost`) connections are open, no token required. **Remote access (LAN, ngrok, or Tailscale) requires the access token** shown in Settings on every request — this was previously optional and is now enforced (v3.3.3 security fix). The mobile app already sends it; a custom tool talking to the remote API directly needs to add it too.

For a self-hosted server, this token model sits alongside a full **account system**: `token`, `password`, `token+password`, or (opt-in, loudly warned about) `none` auth mode, selectable per-server. A server can host more than one account — an admin plus any number of user accounts — each with its own password and its own set of seven granular permissions (Models, Memory, Agent, Calendar, WhatsApp, Telegram, Routines). Permission checks are enforced per-request: `requirePermission` (GET/HEAD-exempt) or `requirePermissionStrict` (no exemption, used for Memory) wrap the relevant handlers below. See the **Accounts & Permissions** section and [Self-Hosting](SELF_HOSTED.md).

## Developer API Gateway (Anthropic-compatible)
`POST /v1/messages` implements the server side of Anthropic's Messages API wire format (`internal/anthropicapi/`), so tools that only know how to talk to Anthropic — most notably **Claude Code** via `ANTHROPIC_BASE_URL` — can point at Memo instead. Model selection uses a `type/model-id` format (`local/qwen2.5`, `openai/gpt-4o`, ...). See Sidebar → Developer for the base URL/token/live request log.

This list below is not exhaustive — there are 180+ registered endpoints as of v3.9.0. It groups the major ones by area; see `internal/webserver/server.go`'s `route(...)` calls for the full, current list.

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
| `GET` | `/api/messages` | Get active chat history |
| `GET` | `/api/status` | System status + memory count |
| `POST` | `/api/incognito` | Toggle incognito mode |
| `GET`/`PUT` | `/api/system-prompt` | Get/update system prompt |

### 🧠 Memory
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/memory/files` | List memory files |
| `DELETE` | `/api/memory/files` | Delete a memory file |
| `POST` | `/api/memory/clear` | Clear all memory |

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
| `POST` | `/api/image` | Read image (⚠️ path-restricted to `data/`) |
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
| `GET` | `/api/proactive/pending` | Poll for a pending suggestion (used by mobile) |
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
| `GET` | `/api/stats/usage` | Requests, tokens, avg tok/s, per-model breakdown, 30-day history |

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
| `GET`/`PUT` | `/api/dev-gateway/config` | Get/update the Anthropic-compatible gateway's config (API key requirement, memory integration) |
| `GET` | `/api/dev-gateway/models` | List `type/model-id` selectable models across local + configured providers |
| `GET` | `/api/dev-gateway/logs` | Live request log |
| `GET` | `/api/dev-gateway/claude-code-cli` | Claude Code CLI connection helper/status |

### 🗂️ Skills
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/skills/list` | List installed skills |
| `POST` | `/api/skills/install` | Install a skill |
| `DELETE` | `/api/skills/remove/{name}` | Remove a skill |
| `GET` | `/api/skills/get/{name}` | Get one skill's manifest |
| `GET`/`PUT` | `/api/skills/active-list`, `/api/skills/active` | Get/set active skills |

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

*For detailed JSON payloads, refer to `internal/webserver/handlers_flutter.go`, the other `handlers_*.go` files, and `internal/webserver/server.go`.*
