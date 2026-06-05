# API Reference

Memo Backend runs a REST API on `localhost:8090` (default).

## Authentication
Currently, the API is open for `localhost` connections. Remote access is disabled in v3.0.0.

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

### 🤖 Agent Mode
| Method | Endpoint | Description |
| :--- | :--- | :--- |
| `GET` | `/api/agent/enabled` | Get agent mode status |
| `PUT` | `/api/agent/enabled` | Enable/disable agent mode |
| `POST` | `/api/agent/permission` | Respond to a permission request |
| `GET` | `/api/agent/permissions` | List permanent permissions |
| `DELETE` | `/api/agent/permissions` | Revoke (with `?id=`) or clear all permissions |

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
| `POST` | `/api/embed/start` | Start embedding server |
| `POST` | `/api/embed/stop` | Stop embedding server |

---

*For detailed JSON payloads, refer to `internal/webserver/handlers_flutter.go` and `internal/webserver/server.go`.*
