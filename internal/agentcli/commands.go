// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"bufio"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"memo/internal/provider"
)

// Command is one slash command offered by a CLI-backed provider, as shown in
// the composer's "/" dropdown.
type Command struct {
	// Name is the command without its leading slash ("review",
	// "frontend-design"). Nested command directories are namespaced with ":"
	// the way both CLIs themselves do it (commands/git/commit.md -> "git:commit").
	Name string `json:"name"`
	// Description is a one-line summary from the command file's YAML
	// frontmatter `description:`, falling back to its first prose line.
	Description string `json:"description"`
	// Source is where it came from: "project" (the chat's working directory),
	// "user" (the home config directory), "skill", or "builtin".
	Source string `json:"source"`
	// Path is the backing .md file, empty for builtins. Only used internally
	// for expansion (see ExpandCommand) — the frontend has no filesystem
	// access of its own and never needs it.
	Path string `json:"-"`
}

// maxCommandFiles caps how many command files are scanned per directory, so a
// pathological directory can't stall the composer's dropdown.
const maxCommandFiles = 200

// commandDir is one directory to scan, tagged with the Source it produces.
type commandDir struct {
	path   string
	source string
	// skills is true for a directory laid out as <dir>/<name>/SKILL.md
	// (one directory per command) rather than <dir>/<name>.md.
	skills bool
}

// commandDirs returns where a CLI type keeps its slash commands, most
// specific first — a project-level command wins over a user-level one with
// the same name, matching how both CLIs resolve them themselves.
func commandDirs(cliType provider.ProviderType, workdir string) []commandDir {
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	var dirs []commandDir
	add := func(base, sub, source string, skills bool) {
		if base == "" {
			return
		}
		dirs = append(dirs, commandDir{path: filepath.Join(base, sub), source: source, skills: skills})
	}

	switch cliType {
	case provider.ProviderClaudeCodeCLI:
		add(workdir, filepath.Join(".claude", "commands"), "project", false)
		add(workdir, filepath.Join(".claude", "skills"), "skill", true)
		add(home, filepath.Join(".claude", "commands"), "user", false)
		add(home, filepath.Join(".claude", "skills"), "skill", true)
	case provider.ProviderCodexCLI:
		add(workdir, filepath.Join(".codex", "prompts"), "project", false)
		add(home, filepath.Join(".codex", "prompts"), "user", false)
	}
	return dirs
}

// builtinCommands lists the CLI's own built-in commands that actually do
// something useful in headless (non-interactive) mode. Deliberately a short
// curated list rather than everything the CLI reports: most built-ins
// (/clear, /model, /config, /compact, /usage, ...) only manipulate an
// interactive session's own state and would be meaningless — or actively
// confusing — sent through Memo's chat, which has no such session to act on.
func builtinCommands(cliType provider.ProviderType) []Command {
	switch cliType {
	case provider.ProviderClaudeCodeCLI:
		return []Command{
			{Name: "init", Description: "Create a CLAUDE.md with codebase documentation", Source: "builtin"},
			{Name: "review", Description: "Review a GitHub pull request", Source: "builtin"},
			{Name: "security-review", Description: "Security review of pending changes on this branch", Source: "builtin"},
		}
	case provider.ProviderCodexCLI:
		return []Command{
			{Name: "review", Description: "Run a code review against this repository", Source: "builtin"},
		}
	}
	return nil
}

// ListCommands discovers the slash commands available to cliType for a chat
// working in workdir. Never fails: a missing/unreadable directory just
// contributes nothing, since "this CLI has no custom commands" is an
// ordinary state, not an error the caller can act on.
func ListCommands(cliType provider.ProviderType, workdir string) []Command {
	// Keyed by name so a project-level command shadows a same-named user-level
	// one — first writer wins, and commandDirs is ordered most-specific first.
	seen := make(map[string]Command)
	claim := func(c Command) {
		if c.Name == "" {
			return
		}
		if _, exists := seen[c.Name]; exists {
			return
		}
		seen[c.Name] = c
	}

	// Scanned directories first: only these carry a real description and an
	// accurate source, and a project-level file must shadow anything below.
	for _, dir := range commandDirs(cliType, workdir) {
		for _, c := range scanCommandDir(dir) {
			if isSessionOnlyCommand(c.Name) {
				continue
			}
			claim(c)
		}
	}
	for _, c := range builtinCommands(cliType) {
		claim(c)
	}
	// Then whatever the CLI itself reported on its last init event — the
	// only source that includes skills bundled inside the CLI package and
	// loaded plugins. Names only, so these fill in without a description
	// unless a scanned file already supplied one above.
	for _, name := range reportedCommandNames(cliType) {
		claim(Command{Name: name, Source: "builtin"})
	}

	out := make([]Command, 0, len(seen))
	for _, c := range seen {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// scanCommandDir reads one directory's command files. Skills directories
// (<dir>/<name>/SKILL.md) and plain command directories (<dir>/<name>.md,
// plus one level of namespacing) are both handled here.
func scanCommandDir(dir commandDir) []Command {
	entries, err := os.ReadDir(dir.path)
	if err != nil {
		return nil
	}
	var out []Command
	for _, e := range entries {
		if len(out) >= maxCommandFiles {
			break
		}
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		if dir.skills {
			if !e.IsDir() {
				continue
			}
			manifest := filepath.Join(dir.path, e.Name(), "SKILL.md")
			if _, err := os.Stat(manifest); err != nil {
				continue
			}
			out = append(out, Command{
				Name:        e.Name(),
				Description: describeCommandFile(manifest),
				Source:      dir.source,
				Path:        manifest,
			})
			continue
		}
		if e.IsDir() {
			out = append(out, scanNamespacedDir(dir, e.Name())...)
			continue
		}
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		full := filepath.Join(dir.path, e.Name())
		out = append(out, Command{
			Name:        strings.TrimSuffix(e.Name(), ".md"),
			Description: describeCommandFile(full),
			Source:      dir.source,
			Path:        full,
		})
	}
	return out
}

// scanNamespacedDir handles one level of nesting (commands/git/commit.md ->
// "git:commit"). Deeper nesting is not walked — neither CLI's own docs use
// it, and stopping here keeps a deep tree under an unrelated directory from
// being enumerated.
func scanNamespacedDir(parent commandDir, name string) []Command {
	sub := filepath.Join(parent.path, name)
	entries, err := os.ReadDir(sub)
	if err != nil {
		return nil
	}
	var out []Command
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") || strings.HasPrefix(e.Name(), ".") {
			continue
		}
		full := filepath.Join(sub, e.Name())
		out = append(out, Command{
			Name:        name + ":" + strings.TrimSuffix(e.Name(), ".md"),
			Description: describeCommandFile(full),
			Source:      parent.source,
			Path:        full,
		})
	}
	return out
}

// maxDescriptionLen keeps a runaway first paragraph from becoming the whole
// dropdown row.
const maxDescriptionLen = 160

// describeCommandFile returns a one-line summary of a command file: its
// frontmatter `description:` if present, otherwise its first line of prose
// (skipping headings and blank lines). Returns "" if it can't read anything
// useful — the dropdown simply shows the name alone in that case.
func describeCommandFile(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 8*1024), 256*1024)

	inFrontmatter := false
	firstLine := true
	var fallback string
	for sc.Scan() {
		line := strings.TrimRight(sc.Text(), "\r")
		trimmed := strings.TrimSpace(line)

		if firstLine {
			firstLine = false
			if trimmed == "---" {
				inFrontmatter = true
				continue
			}
		}
		if inFrontmatter {
			if trimmed == "---" {
				inFrontmatter = false
				continue
			}
			if desc, ok := frontmatterDescription(trimmed); ok {
				return truncateDescription(desc)
			}
			continue
		}
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if fallback == "" {
			fallback = trimmed
			// Keep scanning only while still inside frontmatter; past it the
			// first prose line is all we wanted.
			break
		}
	}
	return truncateDescription(fallback)
}

// frontmatterDescription pulls the value out of a `description: ...` line,
// tolerating the quoting styles YAML allows for a plain scalar.
func frontmatterDescription(line string) (string, bool) {
	lower := strings.ToLower(line)
	if !strings.HasPrefix(lower, "description:") {
		return "", false
	}
	val := strings.TrimSpace(line[len("description:"):])
	if len(val) >= 2 {
		if (val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'') {
			val = val[1 : len(val)-1]
		}
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return "", false
	}
	return val, true
}

func truncateDescription(s string) string {
	s = strings.TrimSpace(s)
	if len(s) <= maxDescriptionLen {
		return s
	}
	return strings.TrimSpace(s[:maxDescriptionLen]) + "…"
}

// SplitCommand splits a message like "/review the auth code" into the
// command name ("review") and its arguments ("the auth code"). ok is false
// when the message isn't a slash command at all, or is a bare "/".
func SplitCommand(message string) (name, args string, ok bool) {
	trimmed := strings.TrimLeft(message, " \t")
	if !strings.HasPrefix(trimmed, "/") {
		return "", "", false
	}
	body := trimmed[1:]
	if body == "" {
		return "", "", false
	}
	if i := strings.IndexAny(body, " \t\n"); i >= 0 {
		name = body[:i]
		args = strings.TrimSpace(body[i+1:])
	} else {
		name = body
	}
	if name == "" {
		return "", "", false
	}
	return name, args, true
}

// ExpandCommand turns "/name args" into the command file's actual body, for
// CLIs that do NOT expand slash commands themselves.
//
// Verified 2026-08-02 against the real binaries: `claude -p "/some-command"`
// genuinely runs the command (its init event even reports the full
// slash_commands list), so Claude Code needs no expansion here and this is
// never called for it. `codex exec "/some-command"` does NOT — codex only
// resolves ~/.codex/prompts/*.md in its interactive TUI, and in exec mode
// passes the text through to the model verbatim, which then improvises
// something plausible instead of running the prompt. Expanding it on Memo's
// side is what makes a codex chat's "/" commands actually work.
//
// Returns ok=false when message isn't a slash command or names one that
// doesn't exist, in which case the caller must send the original text
// unchanged rather than silently swallowing it.
func ExpandCommand(cliType provider.ProviderType, workdir, message string) (string, bool) {
	name, args, ok := SplitCommand(message)
	if !ok {
		return message, false
	}
	var match *Command
	for _, c := range ListCommands(cliType, workdir) {
		if c.Name == name && c.Path != "" {
			cc := c
			match = &cc
			break
		}
	}
	if match == nil {
		return message, false
	}
	body, err := os.ReadFile(match.Path)
	if err != nil {
		return message, false
	}
	return substituteArgs(stripFrontmatter(string(body)), args), true
}

// stripFrontmatter drops a leading `---`-delimited YAML block, which is
// metadata for the CLI, not part of the prompt the model should read.
func stripFrontmatter(body string) string {
	trimmed := strings.TrimLeft(body, "\ufeff \t\r\n")
	if !strings.HasPrefix(trimmed, "---") {
		return body
	}
	rest := trimmed[3:]
	// The opening --- must be its own line.
	if i := strings.IndexByte(rest, '\n'); i >= 0 {
		rest = rest[i+1:]
	} else {
		return body
	}
	end := strings.Index(rest, "\n---")
	if end < 0 {
		return body
	}
	after := rest[end+len("\n---"):]
	if i := strings.IndexByte(after, '\n'); i >= 0 {
		after = after[i+1:]
	} else {
		after = ""
	}
	return strings.TrimLeft(after, "\r\n")
}

// substituteArgs fills in the argument placeholders both CLIs' command files
// use: $ARGUMENTS for everything the user typed after the command name, and
// $1..$9 for individual whitespace-separated words. A command file with no
// placeholders gets the arguments appended instead, so typing "/review the
// login flow" never silently drops "the login flow".
func substituteArgs(body, args string) string {
	hasPlaceholder := strings.Contains(body, "$ARGUMENTS")
	fields := strings.Fields(args)

	body = strings.ReplaceAll(body, "$ARGUMENTS", args)
	for i := 1; i <= 9; i++ {
		token := "$" + strconv.Itoa(i)
		if !strings.Contains(body, token) {
			continue
		}
		hasPlaceholder = true
		val := ""
		if i <= len(fields) {
			val = fields[i-1]
		}
		body = strings.ReplaceAll(body, token, val)
	}

	if !hasPlaceholder && args != "" {
		body = strings.TrimRight(body, "\n") + "\n\n" + args
	}
	return body
}
