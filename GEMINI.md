# Memo: Local AI Memory Shell

Memo is a high-performance, privacy-focused "Memory Shell" for local Large Language Models (LLMs). It transforms standard AI interactions into a persistent "Second Brain" using Retrieval-Augmented Generation (RAG) and local vector indexing.

## 🏛️ Project Overview

-   **Purpose:** To provide a sovereign, offline-first interface for LLMs that learns from user interactions while keeping all data locally.
-   **Architecture:** A decoupled system consisting of a **Go Backend** (Headless REST API) and a **Flutter Frontend** (Native Desktop UI).
-   **Core Technologies:**
    -   **Backend:** Go (Golang)
    -   **Frontend:** Flutter (Dart)
    -   **Inference Engine:** `llama.cpp` (managed via internal wrappers)
    -   **Persistence:** SQLite + sqlite-vec (memory vector store)
    -   **Vector Search:** Cosine similarity with RAM-based indexing for high performance.
    -   **Cloud Sync:** E2E encrypted Google Drive integration.

## 🚀 Building and Running

### Development Mode
To run the project during development, you need two separate terminals:

**1. Start the Backend (Go):**
```bash
go run . --port 8090
```
-   The backend will start a REST API server on port 8090.
-   It manages `llama-server` processes and semantic memory in the background.

**2. Start the Frontend (Flutter):**
```bash
cd frontend
flutter run -d linux # or windows
```
-   The Flutter app will automatically connect to `localhost:8090`.

### Production Packaging
To create a standalone release package (Linux):
```bash
./package_linux.sh
```
-   This generates a `build_output/memo-linux-x64/` directory.
-   Use `./run_memo.sh` inside that directory to start the unified application.

## 🛠️ Key Components

-   **`main.go` / `app.go`:** Entry point and central "Bridge" logic connecting all modules.
-   **`internal/webserver/`:** REST API routing and handler management (using Gin/Mux).
-   **`internal/memory/`:** Semantic memory, vector database, and SQLite/sqlite-vec storage.
-   **`internal/llama/`:** Lifecycle management for `llama-server` (start/stop/install).
-   **`internal/cloudsync/`:** AES-256 encrypted synchronization with Google Drive.
-   **`frontend/lib/`:** Flutter source code organized by Material 3 design principles and Riverpod state management.

## 📝 Development Conventions

-   **Privacy First:** No telemetry or data leakage. All data stays local by default.
-   **Persistence:** Use SQLite + sqlite-vec for fast, atomic vector storage over traditional file-per-interaction formats.
-   **Decoupled Communication:** The Frontend must only communicate with the Backend via the REST API defined in `internal/webserver/server.go`.
-   **Performance Monitoring:** High-resolution latency logging is implemented across the RAG and LLM pipeline (look for `LATENCY` logs in the console).
-   **State Management:** Flutter uses `flutter_riverpod` for reactive UI state.

---
*Built with passion. Control your AI. Own your Memory.*
