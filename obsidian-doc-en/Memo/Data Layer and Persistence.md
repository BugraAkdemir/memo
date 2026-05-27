# 💾 Data Layer and Persistence

Memo uses a specialized file structure for local performance and data integrity instead of traditional databases (SQL/NoSQL).

## .gob Format: Binary-Atomic Persistence
All of Memo's memories and settings are stored in Go's native binary format, `.gob`.

### Advantages:
- **Speed:** Much faster serialization and restoration compared to JSON or XML.
- **Atomic Writes:** Each interaction is saved as an independent file. This way, the corruption of one file does not affect the entire database.
- **Type Safety:** Maintains the Go object structure exactly.

## Folder Structure (`data/`)
- `data/memory/`: Semantic vector files and memories.
- `data/sessions/`: Chat history (in JSON format).
- `data/models/`: Downloaded GGUF model files.
- `data/sync_token.json`: Cloud sync authorization data.

## Memory (RAM) Management
Memo ensures low RAM usage even with thousands of memories:
1. At startup, only vectors (numerical data) are loaded into RAM.
2. Text content remains on the disk.
3. When a search is performed, only the text of the 5-10 most relevant memories is read from the disk.

### Linked Notes:
- [[RAG and Semantic Memory]]
- [[Vector Search Logic]]
