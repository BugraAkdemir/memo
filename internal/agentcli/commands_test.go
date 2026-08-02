// SPDX-License-Identifier: AGPL-3.0-or-later

package agentcli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/provider"
)

// writeFile creates path (and its parents) with the given content.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// isolatedHome points os.UserHomeDir at an empty temp directory, so a test's
// results never depend on whatever commands the developer running it happens
// to have in their own ~/.claude or ~/.codex.
func isolatedHome(t *testing.T) string {
	t.Helper()
	home := t.TempDir()
	t.Setenv("HOME", home)
	return home
}

func findCommand(cmds []Command, name string) *Command {
	for i := range cmds {
		if cmds[i].Name == name {
			return &cmds[i]
		}
	}
	return nil
}

func TestListCommands_ClaudeFindsProjectUserAndSkillCommands(t *testing.T) {
	home := isolatedHome(t)
	workdir := t.TempDir()

	writeFile(t, filepath.Join(workdir, ".claude", "commands", "deploy.md"),
		"---\ndescription: Ship the current branch\n---\n\nDo the deploy.\n")
	writeFile(t, filepath.Join(home, ".claude", "commands", "notes.md"),
		"Just take some notes.\n")
	writeFile(t, filepath.Join(home, ".claude", "skills", "frontend-design", "SKILL.md"),
		"---\nname: frontend-design\ndescription: Distinctive visual design guidance\n---\n\nBody.\n")

	cmds := ListCommands(provider.ProviderClaudeCodeCLI, workdir)

	deploy := findCommand(cmds, "deploy")
	if deploy == nil {
		t.Fatalf("project command not found in %+v", cmds)
	}
	if deploy.Description != "Ship the current branch" || deploy.Source != "project" {
		t.Errorf("deploy = %+v", *deploy)
	}

	notes := findCommand(cmds, "notes")
	if notes == nil || notes.Source != "user" {
		t.Fatalf("user command not found/mislabeled in %+v", cmds)
	}
	// No frontmatter — the first prose line stands in as the description.
	if notes.Description != "Just take some notes." {
		t.Errorf("notes description = %q", notes.Description)
	}

	skill := findCommand(cmds, "frontend-design")
	if skill == nil || skill.Source != "skill" {
		t.Fatalf("skill command not found/mislabeled in %+v", cmds)
	}
	if skill.Description != "Distinctive visual design guidance" {
		t.Errorf("skill description = %q", skill.Description)
	}
}

func TestListCommands_ProjectShadowsUserWithSameName(t *testing.T) {
	home := isolatedHome(t)
	workdir := t.TempDir()

	writeFile(t, filepath.Join(workdir, ".claude", "commands", "build.md"),
		"---\ndescription: PROJECT VERSION\n---\nbody\n")
	writeFile(t, filepath.Join(home, ".claude", "commands", "build.md"),
		"---\ndescription: USER VERSION\n---\nbody\n")

	build := findCommand(ListCommands(provider.ProviderClaudeCodeCLI, workdir), "build")
	if build == nil {
		t.Fatal("build command missing")
	}
	if build.Description != "PROJECT VERSION" || build.Source != "project" {
		t.Errorf("project-level command must win over the same-named user one, got %+v", *build)
	}
}

func TestListCommands_NamespacedSubdirectory(t *testing.T) {
	isolatedHome(t)
	workdir := t.TempDir()
	writeFile(t, filepath.Join(workdir, ".claude", "commands", "git", "commit.md"),
		"---\ndescription: Write a commit\n---\nbody\n")

	if c := findCommand(ListCommands(provider.ProviderClaudeCodeCLI, workdir), "git:commit"); c == nil {
		t.Errorf("expected nested command to be namespaced as git:commit, got %+v",
			ListCommands(provider.ProviderClaudeCodeCLI, workdir))
	}
}

func TestListCommands_CodexReadsPromptsDirNotClaudeDir(t *testing.T) {
	home := isolatedHome(t)
	workdir := t.TempDir()

	writeFile(t, filepath.Join(home, ".codex", "prompts", "audit.md"), "Audit it.\n")
	// A Claude command must NOT leak into codex's list — the two CLIs have
	// entirely separate command directories.
	writeFile(t, filepath.Join(home, ".claude", "commands", "claude-only.md"), "Nope.\n")

	cmds := ListCommands(provider.ProviderCodexCLI, workdir)
	if findCommand(cmds, "audit") == nil {
		t.Errorf("codex prompt not found in %+v", cmds)
	}
	if findCommand(cmds, "claude-only") != nil {
		t.Errorf("claude command leaked into codex's list: %+v", cmds)
	}
}

func TestListCommands_IncludesBuiltinsAndIsSorted(t *testing.T) {
	isolatedHome(t)
	cmds := ListCommands(provider.ProviderClaudeCodeCLI, "")

	if findCommand(cmds, "init") == nil {
		t.Errorf("expected builtin /init in %+v", cmds)
	}
	for i := 1; i < len(cmds); i++ {
		if cmds[i-1].Name > cmds[i].Name {
			t.Fatalf("commands not sorted by name: %q before %q", cmds[i-1].Name, cmds[i].Name)
		}
	}
}

func TestListCommands_UnknownProviderTypeYieldsNothing(t *testing.T) {
	isolatedHome(t)
	if cmds := ListCommands(provider.ProviderOpenAI, ""); len(cmds) != 0 {
		t.Errorf("non-CLI provider should have no slash commands, got %+v", cmds)
	}
}

func TestSplitCommand(t *testing.T) {
	cases := []struct {
		in       string
		wantName string
		wantArgs string
		wantOK   bool
	}{
		{"/review", "review", "", true},
		{"/review the auth code", "review", "the auth code", true},
		{"  /review  spaced  ", "review", "spaced", true},
		{"/git:commit msg", "git:commit", "msg", true},
		{"not a command", "", "", false},
		{"/", "", "", false},
		{"", "", "", false},
	}
	for _, c := range cases {
		name, args, ok := SplitCommand(c.in)
		if name != c.wantName || args != c.wantArgs || ok != c.wantOK {
			t.Errorf("SplitCommand(%q) = (%q, %q, %v), want (%q, %q, %v)",
				c.in, name, args, ok, c.wantName, c.wantArgs, c.wantOK)
		}
	}
}

func TestExpandCommand_SubstitutesArgumentsPlaceholder(t *testing.T) {
	home := isolatedHome(t)
	writeFile(t, filepath.Join(home, ".codex", "prompts", "audit.md"),
		"---\ndescription: Audit\n---\nAudit this area: $ARGUMENTS\n")

	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", "/audit the login flow")
	if !ok {
		t.Fatal("expected expansion to succeed")
	}
	if !strings.Contains(got, "Audit this area: the login flow") {
		t.Errorf("got %q", got)
	}
	// Frontmatter is CLI metadata, not prompt text — it must not reach the model.
	if strings.Contains(got, "description: Audit") {
		t.Errorf("frontmatter leaked into the expanded prompt: %q", got)
	}
}

func TestExpandCommand_SubstitutesPositionalArgs(t *testing.T) {
	home := isolatedHome(t)
	writeFile(t, filepath.Join(home, ".codex", "prompts", "move.md"),
		"Move $1 to $2 please.\n")

	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", "/move a.txt b.txt")
	if !ok {
		t.Fatal("expected expansion to succeed")
	}
	if !strings.Contains(got, "Move a.txt to b.txt please.") {
		t.Errorf("got %q", got)
	}
}

func TestExpandCommand_AppendsArgsWhenTemplateHasNoPlaceholder(t *testing.T) {
	home := isolatedHome(t)
	writeFile(t, filepath.Join(home, ".codex", "prompts", "plain.md"), "Do the thing.\n")

	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", "/plain and also this")
	if !ok {
		t.Fatal("expected expansion to succeed")
	}
	// The user's extra words must never be silently dropped just because the
	// command file didn't declare a placeholder for them.
	if !strings.Contains(got, "Do the thing.") || !strings.Contains(got, "and also this") {
		t.Errorf("got %q", got)
	}
}

func TestExpandCommand_UnknownCommandPassesThroughUnchanged(t *testing.T) {
	isolatedHome(t)
	const msg = "/definitely-not-a-command hello"
	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", msg)
	if ok {
		t.Errorf("expected ok=false for an unknown command")
	}
	if got != msg {
		t.Errorf("unknown command must pass through verbatim, got %q", got)
	}
}

func TestExpandCommand_PlainMessageIsNotTouched(t *testing.T) {
	isolatedHome(t)
	const msg = "just a normal question"
	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", msg)
	if ok || got != msg {
		t.Errorf("got (%q, %v), want the message unchanged and ok=false", got, ok)
	}
}

func TestExpandCommand_BuiltinHasNoFileSoPassesThrough(t *testing.T) {
	isolatedHome(t)
	// /review is a codex builtin with no backing file — codex resolves it
	// itself as a subcommand, so Memo must not try to expand it.
	const msg = "/review"
	got, ok := ExpandCommand(provider.ProviderCodexCLI, "", msg)
	if ok || got != msg {
		t.Errorf("got (%q, %v), want the builtin passed through untouched", got, ok)
	}
}

func TestStripFrontmatter(t *testing.T) {
	in := "---\nname: x\ndescription: y\n---\nreal body here\n"
	if got := stripFrontmatter(in); got != "real body here\n" {
		t.Errorf("got %q", got)
	}
	// A body that merely contains --- later on must be left alone.
	noFM := "body\n---\nnot frontmatter\n"
	if got := stripFrontmatter(noFM); got != noFM {
		t.Errorf("got %q, want unchanged", got)
	}
}

func TestDescribeCommandFile_TruncatesRunawayDescription(t *testing.T) {
	dir := t.TempDir()
	long := strings.Repeat("x", maxDescriptionLen+50)
	path := filepath.Join(dir, "long.md")
	writeFile(t, path, "---\ndescription: "+long+"\n---\nbody\n")

	got := describeCommandFile(path)
	if len([]rune(got)) > maxDescriptionLen+1 {
		t.Errorf("description not truncated: %d chars", len([]rune(got)))
	}
	if !strings.HasSuffix(got, "…") {
		t.Errorf("expected an ellipsis marker, got %q", got)
	}
}
