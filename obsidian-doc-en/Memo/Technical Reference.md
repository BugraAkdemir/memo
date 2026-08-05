# Technical Reference

Map of Content for in-depth technical documentation about Memo's architecture, APIs, and internals.

---

## 🏛️ Architecture

| Page | Description |
|------|-------------|
| [[Architecture]] | System architecture overview |
| [[System Overview]] | High-level component interaction |
| [[Backend (Go) Architecture]] | App bridge pattern, handler structure |
| [[Frontend (Flutter) Design]] | Riverpod state management, widget tree |
| [[Data Layer and Persistence]] | SQLite, sessions, memory store |
| [[Technical Deep Dive]] | Bridge pattern, SQLite+vec0, llama lifecycle, E2E sync, provider system, agent engine, orchestra internals |

## 📡 API Reference

| Page | Description |
|------|-------------|
| [[API Documentation]] | Complete REST API endpoint reference (160+ routes) |
| [[Event System]] | Ring buffer event system for background notifications |
| [[SSE Streaming Protocol]] | Server-Sent Events message format and flow |

## 🔧 Internals

| Page | Description |
|------|-------------|
| [[Llama.cpp Integration]] | Subprocess lifecycle, health checks, port management |
| [[Vector Search Logic]] | Cosine similarity, parallel workers, Top-K |
| [[CGO Flags]] | Build requirements, sqlite-vec compilation |
| [[Default System Prompt]] | Memo's identity directives and anti-hallucination rules |
| [[Known Issues]] | Design-level technical debt and architectural risk (0 open tracked bugs — see `BUG_REPORT.md`) |
| [[Resolved Issues]] | 61 documented fixes with code references |

## 🚀 Guides

| Page | Description |
|------|-------------|
| [[Developer Setup Guide]] | Environment setup, quick start |
| [[Build and Packaging]] | Cross-platform build instructions |
| [[Troubleshooting]] | Common problems and solutions |
| [[Contributing]] | How to contribute to the project |
