# 📡 API Documentation

The Memo Backend provides a comprehensive REST API for the Flutter Frontend or third-party clients. It runs on `localhost:8090` by default.

## Core Endpoints

### Chat and Messaging
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/send` | Standard message submission (non-streaming) |
| `POST` | `/api/send/stream` | Streaming (SSE) message submission — see "Marker chunks" below |
| `GET` | `/api/chats/streaming` | `{chat_ids}` — chats generating a reply right now (the sidebar spinner) (v4.6.0) |
| `GET` | `/api/image?path=` | `{data: base64}` of a stored chat image; only the backend's own image folders are served (v4.6.0) |
| `POST` | `/api/send_file` | File/image message submission (Multipart) |
| `GET` | `/api/chats` | List all chat sessions |
| `POST` | `/api/chats/new` | Create new chat session |
| `POST` | `/api/chats/switch` | Switch active session |
| `POST` | `/api/chats/delete` | Delete session |
| `GET` | `/api/messages` | Get active chat history |

**Marker chunks on a chat SSE stream (v4.6.0).** A chunk is reply text unless its `finish_reason` is a marker; clients ignore unknown markers and never print them. `heartbeat` (empty, every 10 s of silence, keeps the client's idle timeout from firing), `agent_event` (JSON tool event), `browser_frame` (JSON `{screenshot, timestamp}`, live pane only, never saved to history), `quota_exhausted` / `quota_low` (JSON `QuotaSignal`: `kind`, `provider`, `model`, `vendor`, `remaining_percent`, `reset_at`, `window`). A plain chat turn ends after 300 s of silence or 30 min in total — not after a fixed 300 s. `GET /api/messages` and its update/delete siblings take `chat_id`; without it they act on the global active chat.

### Memory Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/status` | System status + memory count |
| `POST` | `/api/incognito` | Toggle incognito mode |
| `GET`/`DELETE` | `/api/memory/files` | List/delete memory files |
| `POST` | `/api/memory/clear` | Clear all memory |
| `GET` | `/api/memory/known-facts` | Every pinned fact ("what Memo knows about you"), no query needed (v4.6.0) |
| `GET` | `/api/memory/conversation?limit=&offset=` | A page of ordinary conversation memories: `{results, total}` (v4.6.0) |
| `POST` | `/api/memory/delete-by-ids` | `{ids}` — delete exactly those records, never a pattern; returns `{deleted}` (v4.6.0) |
| `POST` | `/api/memory/pinned/update` | `{id, content, tags}` — rewrite one pinned fact (v4.6.0) |
| `POST` | `/api/memory/explicit/save`, `/api/memory/explicit/delete` | Save / remove an explicit "remember this" memory |
| `GET`/`PUT` | `/api/system-prompt` | Get/update system prompt |

### Model Control
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`DELETE` | `/api/models/local` | List/delete local models |
| `POST` | `/api/models/start` | Start a model |
| `POST` | `/api/models/stop` | Stop running model |
| `GET` | `/api/models/status` | Model runtime status |
| `GET` | `/api/gpu` | GPU/VRAM detection info |
| `POST` | `/api/models/search` | Search HuggingFace for GGUF |
| `POST` | `/api/models/download` | Start model download |
| `GET` | `/api/models/download/progress` | Download progress stream |
| `GET` | `/api/models/llama/check` | Check llama.cpp installed |

### External Providers (NEW)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT`/`DELETE` | `/api/providers` | List/update/delete provider configs |
| `POST` | `/api/providers/test` | Test provider connection |
| `GET`/`PUT` | `/api/providers/active` | Get/set active provider |
| `GET` | `/api/kilo/models` | Live model list from Kilo Code, free models flagged (v3.9.0) |
| `GET` | `/api/opencode-zen/models` | Live model list from OpenCode Zen, free models flagged by `-free` id suffix (v3.9.0) |
| `GET`/`PUT` | `/api/providers/model` | Model selector (v4.6.0): `GET ?name=` lists the provider's live models (Subscriptions models carry `remaining`, `reset_at`, `quota_window`; `fresh=1` waits ≤3 s for current quota), `PUT {name, model, activate}` switches only its model. Spends a stored key, so it needs the models permission |
| `GET` | `/api/providers/models` | Model list for a key the caller supplies |
| `GET`/`POST` | `/api/subscriptions` | Subscriptions sidecar (v4.6.0): state + `login` (`antigravity` \| `claude` \| `codex`) / `cancel_login` / `logout`. POST is admin-only; GET never starts the sidecar |

### Accounts & Permissions (self-hosted, v3.5.5 + v3.9.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/setup/status` | Whether a self-hosted server has an admin account yet |
| `POST` | `/api/setup/create-admin` | First-run: create the admin account |
| `POST` | `/api/setup/create-device` | Pair a new device/token under an account |
| `GET`/`POST` | `/api/accounts` | List accounts / create a new account |
| `GET`/`PUT`/`DELETE` | `/api/accounts/{id}` | Get/update/delete one account |
| `PUT` | `/api/accounts/{id}/password` | Change an account's password |
| `GET`/`PUT` | `/api/accounts/{id}/permissions` | Get/update an account's 7 granular permissions (Faz 5.1.1) |

Details: [[Remote Access & Self-Hosting]]

### WhatsApp
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/whatsapp/status` | Pairing/connection status |
| `POST` | `/api/whatsapp/start` / `/api/whatsapp/stop` / `/api/whatsapp/logout` | Client lifecycle |
| `POST` | `/api/whatsapp/send` | Send a message |
| `GET` | `/api/whatsapp/search` / `/api/whatsapp/chats` / `/api/whatsapp/messages` | Search/browse message history |
| `GET` | `/api/whatsapp/avatar` / `/api/whatsapp/stats` | Contact avatar / counters |
| `PUT` | `/api/whatsapp/chat-mode` | Configure the dedicated WhatsApp-only chat executor |
| `POST` | `/api/whatsapp/chat-stream` | SSE stream for the WhatsApp-only chat mode |
| `POST` | `/api/whatsapp/self-chat-assistant` | Enable/configure the self-chat assistant (v3.9.0) |

Details: [[WhatsApp Integration]]

### Telegram (v3.9.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/telegram/status` | Bot connection/owner-link status |
| `POST` | `/api/telegram/connect` | Connect with a bot token, start long-polling |
| `POST` | `/api/telegram/stop` / `/api/telegram/disconnect` | Stop, or stop and clear the stored token/owner link |

Details: [[Telegram Integration]]

### Agent Mode (NEW)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/agent/enabled` | Get/set agent mode |
| `POST` | `/api/agent/permission` | Respond to permission request |
| `GET`/`DELETE` | `/api/agent/permissions` | List/revoke permissions |

### Orchestra Mode (NEW)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/orchestra/config` | Get/update orchestra config |

### Routines (v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`POST` | `/api/routines` | List / create routines |
| `POST` | `/api/routines/parse` | Turn plain-language text into a draft routine |
| `GET`/`PUT`/`DELETE` | `/api/routines/{id}` | Get/update/delete a routine |
| `POST` | `/api/routines/sync-offset` | Resync a client's UTC offset |

Details: [[Proactive Learning and Calendar]]

### Self-Insight & Memory Import (v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/memory/insight` | On-demand `/insight` generation |
| `POST` | `/api/memory/import-text` | Import-Memory-From-Another-AI: submit the pasted text |
| `POST` | `/api/memory/import` | Process the imported text into atomic facts + a communication-style summary |

### Live Mode / TTS (Beta, v3.3.4)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/tts/synthesize` | Synthesize speech for a reply |
| `POST` | `/api/tts/filler` | Locally-synthesized "thinking" filler sound |
| `GET` | `/api/tts/providers` | List configured TTS providers |
| `POST` | `/api/tts/providers/test` | Test a TTS provider |
| `GET` | `/api/tts/voices` | List available (downloaded + downloadable) Piper voices |
| `POST` | `/api/tts/voices/download` | Download an offline voice |
| `POST` | `/api/tts/voices/select` | Switch active voice |

Details: [[Multimodal Capabilities (Vision and Voice)]]

### Claude Code CLI / Codex CLI Providers (Beta, v3.3.4)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/cli/status?type=` | Whether `claude`/`codex` is installed, with version |
| `GET` | `/api/cli/running` | Currently running CLI jobs |
| `GET` | `/api/cli/commands?type=&chat_id=` | The CLI's own slash commands (project/personal/skill/built-in) |
| `POST` | `/api/chats/cli-provider` | Set a chat's CLI provider |
| `POST` | `/api/chats/cli-workdir` | Set a chat's CLI working directory |
| `POST` | `/api/chats/cli-model` | Set a chat's CLI model |
| `GET` | `/api/cli/model-options` | Available CLI model options |
| `POST` | `/api/send/cli-stream` | Send a message through the active CLI provider |
| `POST` | `/api/cli/remove` / `/api/cli/reinstall` | Manage the installed CLI binary |

Details: [[External Providers]]

### Memo Swarm (Beta, v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `/api/swarm/*` | Room create/join, worker registration, share %, start/stop — see [[Memo Swarm]] | |

### Usage Stats (v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/stats/usage?days=N` | Usage stats (tokens, speed, model breakdown, daily series) — defaults to 30 days. v4.6.0 adds the prompt-cache split: `total_cached_prompt_tokens`, `total_cache_write_tokens`, and `cached_prompt_tokens` / `cache_write_tokens` per model and per category; zero means "not reported", not a measured 0% |

Details: [[Features Catalog]]

### Developer API Gateway (v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/dev-gateway/config` | Get/update `require_api_key`/`use_memory`, returns the token |
| `GET` | `/api/dev-gateway/models` | Lists every available `"type/model-id"` |
| `GET` | `/api/dev-gateway/logs` | Live request/response log (Developer screen, 200 entries, not persisted) |
| `POST` | `/v1/messages` | Anthropic Messages API-compatible endpoint — deliberately NOT under `/api/`, matching the real Anthropic path exactly so Claude Code's `ANTHROPIC_BASE_URL` can point straight at Memo |
| `POST` | `/v1/chat/completions` | OpenAI-compatible endpoint (v3.9.0) — same auth/routing/memory/system-prompt pipeline as `/v1/messages`, for tools that only support an OpenAI-shaped base URL |
| `GET` | `/v1/models` | OpenAI-compatible model list (v3.9.0) |

Details: [[Developer API Gateway]]

### Synchronization
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/sync/settings` | Get/update Cloud Sync settings |

### Configuration
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/config/llama` | Get/update llama configuration |
| `POST` | `/api/image` | Read image (path-restricted) |
| `POST` | `/api/embed/start` | Start embedding server |
| `POST` | `/api/embed/stop` | Stop embedding server |

### Self-Driving Task Loop (v4.4.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`POST` | `/api/tasklists` | List task lists / create one from a `Task.md` |
| `GET`/`PUT`/`DELETE` | `/api/tasklists/{id}` | Get/update/delete a task list |
| `POST` | `/api/tasklists/{id}/plan` | Trigger a planning turn (planner mode) |
| `POST` | `/api/tasklists/{id}/approve-plan` | Approve a pending `Plan.md` |
| `GET` | `/api/tasks/running` | Live view of the currently-running task list |
| `GET` | `/api/tasks/events` | SSE stream of task-loop activity |
| `POST` | `/api/tasks/{id}/{pause,resume,cancel,skip,inject}` | Control a running task list from outside the model |
| `GET`/`PUT` | `/api/taskloop/settings` | Persistent task-loop configuration |

Details: [[Self-Driving Task Loop]]

### Interactive Browser Pane (v4.6.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/browser/session/navigate` | `{url}` (scheme optional) |
| `POST` | `/api/browser/session/click` | `{x, y}` in the screenshot's pixel space |
| `POST` | `/api/browser/session/type` | `{text, enter}` |
| `POST` | `/api/browser/session/scroll` | `{dx, dy}` |
| `GET` | `/api/browser/session/status` | `{active, url}` |
| `POST` | `/api/browser/session/close` | End the session |

All need the agent permission, accept only `http`/`https`/blank, and answer `{screenshot_base64, url, error}`. Details: [[Agent Mode]]

### Skills (per chat, v4.6.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/skills/active-list?chat_id=` | Skills active in one chat |
| `PUT` | `/api/skills/active` | `{chat_id, names}` — set that chat's active skills; a new chat starts with none |

### Code Mode (v4.5.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`POST` | `/api/code-mode/prompt` | Get/set a Code Mode sub-mode's system prompt (`plan`/`auto`/`build`) |
| `POST` | `/api/code-mode/prompt/reset` | Reset a sub-mode's prompt to its default |

### Desktop Mascot (v4.5.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/mascot/activity` | App-wide activity signal the mascot window (and terminal REPL) polls |

Details: [[Desktop Mascot]]

### Live Mode v2 (v4.3.0)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/livemode/engines` | List/select the configured voice engine |
| `GET` | `/api/livemode/engines/models` | Live model list for the selected engine |
| `POST` | `/api/livemode/session` | Start/manage a Live Mode v2 session |
| `GET`/`PUT` | `/api/livemode/active` | Get/set whether Live Mode is the active surface for a chat |

Details: [[Multimodal Capabilities (Vision and Voice)]]

---
> **Note:** For more details on API usage, examine `internal/webserver/server.go` and `internal/webserver/handlers_flutter.go`.
