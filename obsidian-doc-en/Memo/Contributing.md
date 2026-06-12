# Contributing to Memo

Thank you for considering contributing to Memo! As a solo developer project, any help is appreciated.

---

## How Can You Help?

### 1. Bug Reports
Open an issue with:
- Steps to reproduce
- Your OS and Hardware (GPU/RAM)
- Logs from `server.log`

### 2. Feature Requests
Open an issue describing how it benefits the "local-first" philosophy.

### 3. Code Contributions
1. Fork the repository
2. Create a feature branch: `git checkout -b feature/amazing-feature`
3. Format Go code: `go fmt ./...`
4. Follow Material 3 guidelines for Flutter
5. Submit a Pull Request

## Development Standards

- **Privacy First**: Never send data to external servers without user consent
- **Performance**: Prefer efficient algorithms (SQLite + sqlite-vec)
- **Documentation**: Every feature documented in `docs/` and Obsidian vault
- **Testing**: `go test ./...` for backend; `flutter test` for frontend

## Key Areas for Contribution

| Area | Description |
|------|-------------|
| Agent Frontend UI | Permission dialog, tool call cards, agent mode toggle (Flutter) |
| Tests | Unit tests for `internal/provider/`, `internal/agent/`, `internal/orchestra/` |
| Multi-Step Planning | Plan creation, batch permission, step-by-step execution |
| File Edit Tool | Line-based editing with diff preview and undo |
| Git Tools | git_status, git_diff, git_commit, git_push |

## Philosophy

Memo is built on **Sovereignty**. Users should own their AI, their data, and their hardware. Keep it local, keep it fast, keep it yours.
