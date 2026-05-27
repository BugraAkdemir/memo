# 🏛️ Architecture

Unlike traditional monolithic applications, Memo is built on a **Decoupled** architecture for high performance and flexibility.

## Core Philosophy: Sovereign Interface
Memo's architecture serves a single principle: **User data stays with the user.** There is no telemetry, no cloud dependency, and no data leakage. The system is designed to work completely offline.

## Inter-Component Communication
The system consists of two main parts that communicate with each other over a standard REST API.

```mermaid
graph TD
    subgraph Frontend [Flutter Desktop Client]
        UI[User Interface] -->|State Management| States[Riverpod Providers]
        States -->|HTTP/JSON| API_Client[REST Client]
    end

    subgraph Backend [Headless Go Server]
        WebServer[Gin Web Server] -->|Bridge| AppGo[Core App Engine]
        AppGo -->|Vector Search| Memory[Semantic Memory]
        AppGo -->|Process Management| Llama[Llama.cpp Wrapper]
        AppGo -->|E2E Encryption| Sync[Google Drive Sync]
    end

    API_Client <-->|localhost:8090| WebServer
```

### Linked Notes:
- [[System Overview]]: General workflow diagram.
- [[Backend (Go) Architecture]]: Modular structure of the backend.
- [[Frontend (Flutter) Design]]: Modern Material 3 interface.
- [[Data Layer and Persistence]]: .gob format and atomic writes.
