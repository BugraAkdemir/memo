# Contributing to Memo

First of all, thank you for considering contributing to Memo! This is an independently developed project, and contributions of any kind are genuinely appreciated to make this "Second Brain" even better.

## How Can You Help?

### 1. Bug Reports
If you find a bug, please open an issue with:
- Steps to reproduce.
- Your OS and Hardware (GPU/RAM).
- Logs from `server.log`.

### 2. Feature Requests
Have a great idea? Open an issue and describe how it would benefit the "local-first" philosophy.

### 3. Code Contributions
- Fork the repository.
- Create a feature branch (`git checkout -b feature/amazing-feature`).
- Ensure your Go code is formatted (`go fmt ./...`).
- Ensure your Flutter code follows Material 3 guidelines.
- Submit a Pull Request.

## Development Standards
- **Privacy First:** Never add code that sends data to external servers without explicit user consent.
- **Performance:** Prefer efficient algorithms (SQLite + sqlite-vec for vector storage) to keep RAM usage low.
- **Documentation:** Every new feature should be documented in the `/docs` folder.
- **Testing:** Backend tests use standard `go test -tags "sqlite_fts5" -race` (the build tag matters — see `docs/CGO_FLAGS.md`). `internal/provider/`, `internal/agent/`, and `internal/orchestra/` now have solid test coverage; some newer packages (`internal/cloudsync`, `internal/skill`, `internal/proactive`, `internal/observer`) are still lighter on tests — contributions welcome.

## Key Areas for Contribution
- **Test coverage:** previously-untested areas noted in `versinNote/v3.3.4.md`'s "Planned" section: `handlers_oauth.go`, `handlers_proactive.go`, `internal/cloudsync/drive.go`, `hardwareID()`.
- **Multi-Step Planning:** Plan creation, batch permission, step-by-step execution (agent mode currently runs tool-by-tool, up to 20 iterations).
- **Git Tools:** git_status, git_diff, git_commit, git_push integration for agent mode (not yet built-in).
- **Full duplex audio for Live Mode:** current beta has one-directional barge-in and no echo cancellation.
- **CLI provider review UI:** Claude Code/Codex CLI providers are an early integration — there's no UI yet to review what the CLI actually did (file edits, commands run) beyond its final text reply.

## Philosophy
Memo is built on **Sovereignty**. Users should own their AI, their data, and their hardware. Keep it local, keep it fast, keep it yours.
