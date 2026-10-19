# 📁 Project Folders (`.memo/`)

> **Package:** `internal/project/` (guard, ids, scaffold, skills), `internal/memory/scope.go` (memory pools), bridged into `internal/app/` via `project_scope.go`, `project_setup.go`, `project_skills.go`
> Design record: `PLAN_project_folders.md`; full page: `docs/PROJECT_FOLDER.md`
> **Data:** `<project>/.memo/`, `data/projects.json` (per device), `data/backups/memory-pre-project-id-*.db`
> **API endpoints:** `/api/projects/status`, `/api/projects/setup`, `/api/projects/decline`

Opening a folder as a project ([[Agent Mode]] chat) can give it a small `.memo/` folder with working rules and notes, a memory of its own, and project skills.

## What is created
`AGENTS.md` and `handoff.md` in the project root (never overwriting; no second `AGENTS.md` next to a `CLAUDE.md`), and `.memo/AGENTS.md` (generic, tech-agnostic working rules, Memo-managed), `.memo/project.json` (id + settings), `.memo/notes.md`, `.memo/.gitignore`. A set-up project's chat gets one prompt line: read `.memo/AGENTS.md` first — the files themselves are not pasted into the prompt.

## `~/.memo` is Memo's own install
A project's `.memo` has the same name as Memo's install folder, so the guard refuses the home folder and its parents, Memo's install/data/config folders, system folders and any folder whose `.memo` looks like an install. A `.memo` without a valid `project.json` (or one that is a symlink) is foreign and never written to.

## Project id and memory
A stable id lives in `project.json` and travels with the folder; a per-device registry notices moves (same id) and copies (new id). Memory is partitioned by `memories.project_id` (empty = general pool). An ordinary chat reads every pool; a **project chat reads and writes only its own pool and never sees the general memory, pinned facts, your name, learned style notes or custom persona** (see [[RAG and Semantic Memory]]). A project whose id cannot be determined gets no memory at all. The upgrade adds the column additively after a full copy of `memory.db`; if the copy fails the change is skipped.

## Minimal Mode
A pure wrapper: no session-start line, no setup question. The memory switch is independent.

## Project skills
`.memo/skills/<name>/SKILL.md` — **instructions only** (declared tools are dropped, because a cloned repo's skill is untrusted text) and **not in the prompt until `/skill:on <name>`**. `/skill add <source>` (or the permission-asking `install_project_skill` tool) installs from GitHub: `owner/repo`, a URL, or a pasted `npx skills add …` line — Memo only reads the source and downloads a plain archive over HTTPS with strict size limits; it never runs npx, never makes anything executable and never overwrites a skill.

## Not done / not verified
Per-project command permissions are not implemented; real GitHub downloads were only tested against a local stand-in; the setup dialog was widget-tested only; a real model's compliance with the session-start line was not measured.
