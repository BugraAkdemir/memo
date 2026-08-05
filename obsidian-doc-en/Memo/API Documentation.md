# 📡 API Documentation

The Memo Backend provides a comprehensive REST API for the Flutter Frontend or third-party clients. It runs on `localhost:8090` by default.

## Core Endpoints

### Chat and Messaging
| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/api/send` | Standard message submission (non-streaming) |
| `POST` | `/api/send/stream` | Streaming (SSE) message submission |
| `POST` | `/api/send_file` | File/image message submission (Multipart) |
| `GET` | `/api/chats` | List all chat sessions |
| `POST` | `/api/chats/new` | Create new chat session |
| `POST` | `/api/chats/switch` | Switch active session |
| `POST` | `/api/chats/delete` | Delete session |
| `GET` | `/api/messages` | Get active chat history |

### Memory Management
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET` | `/api/status` | System status + memory count |
| `POST` | `/api/incognito` | Toggle incognito mode |
| `GET`/`DELETE` | `/api/memory/files` | List/delete memory files |
| `POST` | `/api/memory/clear` | Clear all memory |
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
| `GET` | `/api/routines/mobile-ready` | Mobile polling endpoint for pre-scheduled local notifications |
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
| `GET` | `/api/stats/usage?days=N` | Usage stats (tokens, speed, model breakdown, daily series) — defaults to 30 days |

Details: [[Features Catalog]]

### Developer API Gateway (v3.3.3)
| Method | Endpoint | Description |
|--------|----------|-------------|
| `GET`/`PUT` | `/api/dev-gateway/config` | Get/update `require_api_key`/`use_memory`, returns the token |
| `GET` | `/api/dev-gateway/models` | Lists every available `"type/model-id"` |
| `GET` | `/api/dev-gateway/logs` | Live request/response log (Developer screen, 200 entries, not persisted) |
| `POST` | `/v1/messages` | Anthropic Messages API-compatible endpoint — deliberately NOT under `/api/`, matching the real Anthropic path exactly so Claude Code's `ANTHROPIC_BASE_URL` can point straight at Memo |

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

---
> **Note:** For more details on API usage, examine `internal/webserver/server.go` and `internal/webserver/handlers_flutter.go`.
