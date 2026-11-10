# Contributing to Memo

Thanks for wanting to help. Memo is a local-first assistant: a Go backend
(REST + SSE on `:8090`) and a Flutter app (desktop, web, mobile) that talks to
it. This page is the short version. The full engineering reference — module
map, gotchas that have caused real bugs, and the house rules — is
[AGENTS.md](AGENTS.md). Please read it before a non-trivial change.

- What Memo does: [README.md](README.md) ([Türkçe](READmeTR.md))
- Feature and design docs: [`docs/`](docs/)
- Plans for work in progress: [`docs/plans/`](docs/plans/)

## Set up

You need:

- **Go 1.26** (see `go.mod`) with a C compiler — `CGO_ENABLED=1` is required
  (SQLite).
- **Flutter** with Dart SDK ^3.10 (see `frontend/pubspec.yaml`).
- On Linux, for the desktop app: `ayatana-appindicator3-0.1` (or
  `appindicator3-0.1`) for the tray icon.

Every Go build, run and test needs the `sqlite_fts5` tag. Without it FTS5 is
silently missing and memory search falls back to vector-only, with no error
([docs/CGO_FLAGS.md](docs/CGO_FLAGS.md)).

```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5" -o memo .
```

### Engines (`binaries/`)

The engine binaries (llama.cpp, whisper.cpp, sqlite-vec, the Subscriptions
sidecar) are not in git. The backend runs without them — you just have no local
model — so for most changes you do not need them: use a cloud provider, or the
scripted fake provider below. To run a local model, copy the `binaries/` folder
of an installed Memo (e.g. `~/.memo/binaries` after
`curl -fsSL https://data.memocpp.com/get-memo.sh | bash`) into the repo root.

## Run

```bash
./memo --headless --port 8090          # backend only
cd frontend && flutter run -d linux     # or -d windows / -d macos / -d chrome
```

`./memo` without `--headless` opens the terminal client (REPL) against the same
backend.

### Trying things without a real model

`cmd/fakeprovider` is a scripted OpenAI-compatible server: replies, tool calls,
errors and slow streams are steered by directives in your message. Run it, then
add a `custom` provider with base URL `http://127.0.0.1:4959/v1`:

```bash
go build -o /tmp/fp/fakeprovider ./cmd/fakeprovider
/tmp/fp/fakeprovider -addr 127.0.0.1:4959 -log /tmp/fp/fake.jsonl
```

The directives are listed in the file's doc comment. Use a separate data folder
(`MEMO_DATA_DIR=/tmp/fp/data ./memo --headless --port 18090`) so experiments do
not touch your own chats.

## Before you open a pull request

Run what CI runs:

```bash
CGO_ENABLED=1 go build -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go vet   -tags "sqlite_fts5" ./...
CGO_ENABLED=1 go test  -tags "sqlite_fts5" ./... -race
cd frontend && flutter analyze lib/ && flutter test
```

`flutter analyze` currently reports a few `info`-level findings
(`use_build_context_synchronously`); anything new must be fixed.

House rules (details in AGENTS.md):

- **No hardcoded text in the Flutter UI.** Everything a user can read goes
  through `L10n.t('key')`, with both a Turkish and an English entry in
  `frontend/lib/core/l10n.dart` in the same change.
- **Format only what you changed.** Run `dart format` / `gofmt` on the files you
  touched, never on the whole tree, and never on `l10n.dart` (it rewrites the
  whole file).
- **Small files, small changes.** Aim for files under 500 lines; fix the root
  cause instead of papering over it; add a test that fails without your fix.
- **Database writes** go through `database.DB.Write()`; **data paths** through
  `config.DataPath()`; **stored pictures** through `writeImageFile` /
  `readImageFile`. AGENTS.md explains why each of these exists.
- New Go files carry the `// SPDX-License-Identifier: AGPL-3.0-or-later`
  header the existing ones have.

## Commits and pull requests

- [Conventional Commits](https://www.conventionalcommits.org/):
  `fix(frontend): ...`, `feat(chat): ...`, `docs: ...`. The body explains *why*:
  the root cause, what changed, what it fixes.
- Enable the repo's hook once: `git config core.hooksPath .githooks`. It
  rejects AI-attribution trailers (`Co-Authored-By: …`, "Generated with …"),
  which this project does not accept in commit messages.
- One topic per pull request. Say what you changed, why, and how you tested it
  (commands run, platforms tried). Note anything you could not test — for
  example Windows or macOS behaviour checked only by a build.

## Reporting bugs and security problems

- Bugs and ideas: open a [GitHub issue](https://github.com/BugraAkdemir/memo/issues)
  with the version (`/api/version` or the About screen), the platform, and what
  you did, expected and saw.
- **Security problems: do not put details in a public issue.** Open an issue
  titled "Security contact request" with no details, and the maintainer will
  reply with a private channel.

## License

Memo is licensed under the [GNU AGPL v3.0 or later](LICENSE). By contributing
you agree that your contribution is released under the same license.
