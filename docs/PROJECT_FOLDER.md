# Project folders (`.memo/`)

When you open a folder as a project (an agent chat), Memo can keep its own small
folder in it: working rules, notes, project skills — and give that project a
memory of its own. Design record: `PLAN_project_folders.md`.

## What the person sees

- Picking a folder for a new agent chat asks **"Set up Memo for this project?"** —
  *Set up*, *Not now*, *Never for this folder*. Nothing is written unless you answer
  *Set up*. The question is not asked in Minimal Mode, for a folder
  that can never be a project (see below), or for a folder you said "never" to.
- In a set-up project the model is told, in one line, to read `.memo/AGENTS.md` first.
  That file points on to the project's `AGENTS.md`, `handoff.md` and `.memo/notes.md`.
  None of these files is pasted into the prompt; the model reads them with its tools
  when it needs them, so the prompt stays small and cache-friendly.
- Project skills are added with `/skill add <source>` (see below).

## What is created

```
<project>/AGENTS.md          project facts: structure, commands, conventions, pitfalls
<project>/handoff.md         session log, newest on top
<project>/.memo/AGENTS.md    generic working rules (Memo-managed)
<project>/.memo/project.json id + settings
<project>/.memo/notes.md     a few durable facts
<project>/.memo/.gitignore   keeps per-user files out of the repo
<project>/.memo/skills/      project skills (created on first install)
```

Nothing that already exists is overwritten. If the project already has a `CLAUDE.md`
no second `AGENTS.md` is created. The root `AGENTS.md` skeleton never mentions Memo's
folder, because other tools read that file too. `.memo/AGENTS.md` is the one file Memo
owns: it is refreshed on an update only while its hash still equals what Memo wrote
(`rules_hash` in `project.json`), i.e. while nobody has edited it.

The templates are tech-agnostic and about quality: understand before changing, fix root
causes, keep scope tight, never say "done" without running the project's real
verification commands, say what was *not* verified, never commit unless asked.

## `~/.memo` is not a project

Memo's own install lives in `~/.memo` (`bin/`, `binaries/`, `data/`…), and the updater
and the uninstaller manage it. A project's folder has the same name, so a project rooted
at your home folder would put project files into the install — and an update would
delete them. `internal/project.Guard` therefore refuses: the home folder and every parent
of it, anything inside Memo's install/data/config folders, system folders, and any folder
whose `.memo` has `bin|binaries|data|config` in it. A `.memo` without a valid
`project.json` (`"kind":"project"` and a `p_…` id) is foreign and Memo never writes to
it; a symlinked `.memo` is treated the same way, so Memo can never write through it.

## Project id and memory

Each project has a stable id (`p_` + 128 random bits) in `.memo/project.json`. It travels
with the folder, so chats and memory follow a project that is moved or renamed. A
per-device registry (`data/projects.json`, never synced) maps folder → id, hands out an
id before the folder is set up (setup writes that same id), notices a move (old folder
gone → same id) and a copy (the same id in a second live folder → the copy gets a new id,
so two projects never share one memory).

Memory is partitioned by `memories.project_id`; the empty id is the general pool
(everything from ordinary chats and everything saved before this feature).

| Chat | Reads | Writes |
|---|---|---|
| Ordinary chat | every pool | the general pool |
| Project (agent) chat | **only its own project's pool** | its own project's pool |

A project chat never receives your general memory, pinned facts, name, learned style
notes or custom persona text; its mood is neither shown nor fed; it learns no personal
facts and its turns do not go to the optional decision model. A project whose id cannot
be determined gets **no** memory, never the general pool. Code Mode used to have no
memory at all; a project chat now reads and writes its own pool whether or not Code Mode is
on: Code Mode only strips what is personal, and the project's own pool is not. The
Memory switch still decides whether any memory is used. A coding turn's reply is
stored as an excerpt (1500 characters), since retrieval only shows the user's side.
Self-Driving task steps with "task memory" off neither read nor write it.

### The one-time database change

The upgrade adds `memories.project_id` (`ADD COLUMN … DEFAULT ''`, idempotent). Before
touching a database that has rows, Memo copies `memory.db` to
`data/backups/memory-pre-project-id-<time>.db`. If that copy cannot be written the change
is skipped: ordinary chats behave exactly as before and project chats get no memory until
the next start. The existing `INSERT`s are unchanged, so an older Memo can still read and
write the upgraded file (its rows land in the general pool).

## Minimal Mode

Minimal Mode is a pure wrapper: no session-start line, no setup question, nothing read.
The memory switch is independent: with memory on, only the (scoped) memory block is added.

## Project skills

`.memo/skills/<name>/SKILL.md`, same format as global skills, with two differences:

- **Instructions only.** Tools a project skill declares are dropped. A skill that
  arrives with a cloned repository is untrusted text; letting it register commands
  (with a danger level it writes itself) would let a repo run commands on the machine of
  whoever opens it.
- A skill is **not in the prompt** until someone switches it on with `/skill:on <name>`
  (the model may also read the file when the task matches its description).

Add one from GitHub with `/skill add <source>` or by asking the agent (tool
`install_project_skill`, which asks permission). A source can be `owner/repo`,
`owner/repo/some/folder`, a `github.com` URL (also `/tree/<ref>/<path>`), or the line you
pasted from a skill's README such as `npx skills add owner/repo --skill name`. **Memo
never runs npx**: it reads the source out of the line and downloads a plain archive over
HTTPS (25 MB compressed / 60 MB unpacked / 5000 entries / 5 MB per file; symlinks,
absolute and `..` paths are refused; nothing is made executable). If a place holds
several skills the error lists them. An installed skill is never overwritten.
`/skill remove <name>` removes it.

## Permissions

A project can pre-approve a few things so Memo stops asking about them, in
`.memo/permissions.json` (kept out of version control by `.memo/.gitignore`):

```json
{ "version": 1, "allowed_commands": ["go test", "git status"], "allowed_tools": ["edit_file"] }
```

- **A command rule approves exactly one plain command, never what is chained to it.**
  `run_command` hands the line to `bash -c`, so "starts with `go test`" is not enough:
  `go test ./... ; rm -rf ~` also starts with it. A line is approved only if it is made of
  letters, digits, blanks and `_ . / : = , @ + -` and quotes — nothing that can chain
  (`; & |`), redirect (`< >`), substitute (`$ \``), expand (`* ? ~ { }`) or comment (`#`) —
  and it starts with an allowed prefix at a word boundary (`go test` matches `go test ./...`,
  not `go testing`). A command that fails this simply asks as before, for example
  `-run 'A|B'`; write `-run A` or ask once.
- **Rules that are almost everything are refused** when saved or approved: shells and
  interpreters (`bash`, `python3`, `node`…), wrappers (`env`, `xargs`, `sudo`, `time`,
  `find`…), network and container tools (`curl`, `nc`, `ssh`, `docker`, `npx`…), destructive
  ones (`rm`, `mv`, `dd`, `chmod`…), a bare `git` / `go` / `npm` / `cargo` (name the
  sub-command: `git status`), and flags that make a command run another program
  (`-exec`, `-toolexec`, `-ldflags`…). The list is best effort; the character check is the
  main defence.
- **Only file tools can be pre-approved by name:** `read_file`, `list_directory`,
  `get_file_info`, `search_files`, `search_content`, `write_file`, `edit_file`,
  `insert_line`, `delete_lines`, `create_task_md`, `edit_task_md`, `save_code_plan`.
  `run_command`, `delete_file`, anything that reaches the network, your accounts or Memo's
  own settings always asks.
- **Trust on first use.** A permissions file that arrives with a cloned repository, or that
  the model wrote itself, is ignored until *you* approve it (`/api/projects/permissions/trust`
  or saving through `/api/projects/permissions/save`). Memo pins the file's SHA-256 in the
  per-device registry (`data/projects.json`); if the file changes afterwards, by anyone, it is
  ignored again. A file with even one rule that breaks the constraints is ignored as a whole.
- The project's own `AGENTS.md` and `.memo/AGENTS.md` can *not* grant any of this; only the
  approved file can.

## API

| Method | Endpoint | |
|---|---|---|
| `POST` | `/api/projects/status` | `{path}` → `{root, id, name, set_up, declined, allowed, reason, enabled, should_ask}`; writes nothing |
| `POST` | `/api/projects/setup` | `{path}` → creates the files; 403 for home / Memo's folders / a foreign `.memo` |
| `POST` | `/api/projects/decline` | `{path, declined?}` → remember "never here" (`false` undoes it) |
| `POST` | `/api/projects/permissions` | `{path}` → `{permissions, status: {exists, trusted, hash}}` |
| `POST` | `/api/projects/permissions/save` | `{path, permissions}` → writes and trusts `.memo/permissions.json` |
| `POST` | `/api/projects/permissions/trust` | `{path}` → pins the existing file hash as trusted |

## Not done / not verified

- Downloading from real GitHub was only tested against a local stand-in server.
- The setup dialog and the new flow were tested with widget tests only, not on a real
  window; a real model's compliance with the session-start line was not measured.
- A project skill has no picker in the app yet; use `/skill`, `/skill:on`, `/skill add`.
