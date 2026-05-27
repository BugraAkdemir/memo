# 📡 API Documentation

The Memo Backend provides a comprehensive REST API for the Flutter Frontend or third-party clients. It runs on `localhost:8090` by default.

## Core Endpoints

### Chat and Messaging
- `POST /api/send`: Standard message submission.
- `POST /api/send/stream`: Streaming (SSE) message submission.
- `POST /api/send_file`: Submission of messages containing files or images.
- `GET /api/messages`: Fetches the message history of the current session.

### Memory Management
- `GET /api/status`: Returns the total number of memories and system status.
- `POST /api/incognito`: Toggles Incognito Mode.
- `GET /api/memory/files`: Lists semantic memory files.
- `POST /api/memory/clear`: Resets all memory.

### Model Control
- `GET /api/models/local`: Lists downloaded models.
- `POST /api/models/start`: Starts a specific model.
- `POST /api/models/stop`: Stops the running model.
- `GET /api/gpu`: Returns GPU and VRAM information.

### Synchronization
- `GET /api/sync/settings`: Fetches Cloud Sync settings.
- `PUT /api/sync/settings`: Updates settings.

---
> **Note:** For more details on API usage, you can examine the `internal/webserver/server.go` file.
