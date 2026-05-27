# API Reference

Memo Backend runs a REST API on `localhost:8090` (default).

## Authentication
Currently, the API is open for `localhost` connections. In future versions, a Bearer Token system will be implemented for Remote Access.

## Endpoints

### 💬 Chat
| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/send` | `POST` | Send a standard JSON message. |
| `/api/send/stream` | `POST` | SSE (Server-Sent Events) streaming response. |
| `/api/messages` | `GET` | Retrieve history for current session. |
| `/api/chats` | `GET` | List all available sessions. |
| `/api/chats/new` | `POST` | Create a new session. |

### 🧠 Memory
| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/status` | `GET` | Get total memory count & system health. |
| `/api/incognito` | `POST` | Toggle Incognito Mode. |
| `/api/memory/clear` | `POST` | Wipe all local memories. |
| `/api/system-prompt` | `PUT` | Update the AI personality. |

### 🏭 Models
| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/models/local` | `GET` | List downloaded .gguf files. |
| `/api/models/start` | `POST` | Spawn a `llama-server` instance. |
| `/api/models/stop` | `POST` | Terminate the active model process. |
| `/api/gpu` | `GET` | Detect CUDA/ROCm and VRAM stats. |

### ☁️ Sync
| Endpoint | Method | Description |
| :--- | :--- | :--- |
| `/api/sync/settings` | `GET` | Get Google Drive sync status. |
| `/api/sync/start` | `POST` | Trigger a manual E2E encrypted sync. |

---
*For detailed JSON payloads, refer to `internal/webserver/handlers_flutter.go`.*
