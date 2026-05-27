# 🖥️ Backend (Go) Architecture

Memo's backend is a "Headless" server written in Go for speed, reliability, and low resource consumption.

## Modular Structure (`internal/`)
The codebase is divided into modules, each with a specific responsibility:

### 1. `webserver`
- **Role:** Manages the REST API layer.
- **Technology:** Gin Gonic framework.
- **Feature:** Secure local communication with Flutter and optional remote access support.

### 2. `llama`
- **Role:** Manages `llama-server` (C++) processes.
- **Feature:** Automatic installation (`llamaInstaller`), GPU/VRAM detection, and multi-process (chat + embedding) management.

### 3. `memory`
- **Role:** Semantic memory and vector database.
- **Feature:** Cosine similarity calculations, `.gob` serialization, and fast RAM-based indexing.

### 4. `cloudsync`
- **Role:** Google Drive integration.
- **Feature:** AES-256 E2E (End-to-End) encryption for data backup.

### 5. `identity`
- **Role:** Manages system prompts and user identities (personas).

## Bridge Pattern
The `app.go` file acts as the main "Brain" that brings all these modules together. The web server communicates with this engine via the `AppBridge` interface.

### Linked Notes:
- [[Architecture]]
- [[API Documentation]]
- [[Llama.cpp Integration]]
