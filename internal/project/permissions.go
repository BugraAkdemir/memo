// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"memo/internal/fileutil"
)

const (
	// PermissionsFile is the project permissions file in .memo/.
	PermissionsFile = "permissions.json"
	// PermissionsSchema is the current schema version.
	PermissionsSchema = 1
)

var (
	// ErrUntrustedPermissions indicates the project's permissions.json has not been
	// trusted by the user on this device yet, or has changed since it was trusted.
	ErrUntrustedPermissions = errors.New("project permissions are untrusted")
	// ErrInvalidRule indicates a permission rule violates safety constraints (e.g. wildcards).
	ErrInvalidRule = errors.New("invalid permission rule")
)

// Permissions contains the per-project tool execution rules stored in
// `<root>/.memo/permissions.json`.
type Permissions struct {
	Version         int      `json:"version"`
	AllowedCommands []string `json:"allowed_commands,omitempty"`
	AllowedTools    []string `json:"allowed_tools,omitempty"`
}

// PermissionsPath returns `<root>/.memo/permissions.json`.
func PermissionsPath(root string) string {
	return filepath.Join(root, DirName, PermissionsFile)
}

// HashPermissions calculates the SHA-256 hash of a permissions file's raw bytes.
func HashPermissions(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

// PreapprovableTools are the only tools a project may pre-approve by name. They
// edit or read files inside the project's sandbox. Everything that runs commands,
// reaches the network or the person's accounts, spends money or changes Memo
// itself (run_command, delete_file, fetch_page, web_search, configure_provider,
// start_self_driving_task, install_project_skill, whatsapp_*, …) always asks.
var PreapprovableTools = map[string]bool{
	"read_file": true, "list_directory": true, "get_file_info": true,
	"search_files": true, "search_content": true,
	"write_file": true, "edit_file": true, "insert_line": true, "delete_lines": true,
	"create_task_md": true, "edit_task_md": true, "save_code_plan": true,
}

// neverPreapprovedExecutables may not start a command rule. Each one runs
// arbitrary code, reads or sends data anywhere, or runs another program given as
// an argument, so "always allow" on it is "always allow everything".
var neverPreapprovedExecutables = map[string]bool{
	// shells and interpreters
	"sh": true, "bash": true, "zsh": true, "fish": true, "dash": true, "ksh": true, "csh": true, "tcsh": true,
	"cmd": true, "cmd.exe": true, "powershell": true, "powershell.exe": true, "pwsh": true,
	"python": true, "python2": true, "python3": true, "node": true, "nodejs": true, "deno": true, "bun": true,
	"perl": true, "ruby": true, "php": true, "lua": true, "java": true, "osascript": true,
	// wrappers that run whatever they are given
	"env": true, "xargs": true, "sudo": true, "su": true, "doas": true, "time": true, "timeout": true,
	"nohup": true, "nice": true, "command": true, "exec": true, "eval": true, "watch": true, "strace": true, "find": true,
	// remote code and containers
	"npx": true, "bunx": true, "pnpx": true, "docker": true, "podman": true, "kubectl": true,
	// destructive, permission or network tools
	"rm": true, "rmdir": true, "mv": true, "dd": true, "shred": true, "chmod": true, "chown": true, "mkfs": true,
	"curl": true, "wget": true, "nc": true, "ncat": true, "netcat": true, "ssh": true, "scp": true, "sftp": true, "rsync": true, "ftp": true,
}

// needsASubcommand are tools that run many different things depending on their
// first argument; a rule must name the sub-command ("git status", not "git").
var needsASubcommand = map[string]bool{
	"git": true, "go": true, "npm": true, "yarn": true, "pnpm": true, "cargo": true,
	"dotnet": true, "pip": true, "pip3": true, "gem": true, "gh": true, "make": false,
}

// unsafeFlags (names without their leading dashes) make an otherwise harmless
// command run another program. Best effort: the main defences are the
// metacharacter check and the executable lists.
var unsafeFlags = map[string]bool{
	"exec": true, "toolexec": true, "ldflags": true, "gcflags": true, "asmflags": true,
	"upload-pack": true, "receive-pack": true, "extld": true, "overlay": true,
}

// shellSafe reports whether cmd consists only of characters that cannot chain,
// redirect, substitute, expand or comment in a shell: letters, digits, blanks and
// _ . / : = , @ + - and quotes. run_command hands the line to `bash -c`, so a
// prefix match alone would let "go test ./... ; rm -rf ~" through. A pattern such
// as -run 'A|B' is refused on purpose; such a command simply asks.
func shellSafe(cmd string) bool {
	for _, r := range cmd {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		case r == ' ' || r == '\t' || strings.ContainsRune("_./:=,@+-'\"", r):
		default:
			return false
		}
	}
	return true
}

// firstWord returns the executable of a command line, without its directory.
func firstWord(cmd string) string {
	f := strings.Fields(cmd)
	if len(f) == 0 {
		return ""
	}
	return strings.ToLower(path.Base(strings.ReplaceAll(f[0], "\\", "/")))
}

func hasUnsafeFlag(cmd string) bool {
	for _, tok := range strings.Fields(cmd) {
		if !strings.HasPrefix(tok, "-") {
			continue
		}
		name := strings.SplitN(strings.TrimLeft(tok, "-"), "=", 2)[0]
		if unsafeFlags[name] {
			return true
		}
	}
	return false
}

// ValidateRules checks that rules satisfy the security constraints:
//   - A command prefix is not empty and is made only of shell-safe characters
//     (see shellSafe): no wildcards, chaining, redirection or substitution.
//   - Its executable is not a shell, an interpreter, a wrapper, a network or a
//     destructive tool (neverPreapprovedExecutables), and a multi-purpose tool
//     such as git or go names its sub-command.
//   - A tool name is one of PreapprovableTools; nothing that runs commands or
//     reaches outside the project can be pre-approved by name.
func (p *Permissions) ValidateRules() error {
	if p == nil {
		return nil
	}
	for _, cmd := range p.AllowedCommands {
		trimmed := strings.TrimSpace(cmd)
		if trimmed == "" {
			return fmt.Errorf("%w: empty command prefix is not allowed", ErrInvalidRule)
		}
		if !shellSafe(trimmed) {
			return fmt.Errorf("%w: command prefix %q contains characters a shell treats specially (wildcards, ; & | < > $ ` ( ) and so on)", ErrInvalidRule, cmd)
		}
		exe := firstWord(trimmed)
		if neverPreapprovedExecutables[exe] {
			return fmt.Errorf("%w: %q can run arbitrary code or reach outside the project and cannot be pre-approved", ErrInvalidRule, exe)
		}
		if needsASubcommand[exe] && len(strings.Fields(trimmed)) < 2 {
			return fmt.Errorf("%w: %q alone is too broad; name the sub-command, for example %q", ErrInvalidRule, exe, exe+" status")
		}
		if hasUnsafeFlag(trimmed) {
			return fmt.Errorf("%w: command prefix %q contains a flag that runs another program", ErrInvalidRule, cmd)
		}
	}
	for _, tool := range p.AllowedTools {
		trimmed := strings.TrimSpace(tool)
		if trimmed == "" {
			return fmt.Errorf("%w: empty tool name is not allowed", ErrInvalidRule)
		}
		if !PreapprovableTools[trimmed] {
			return fmt.Errorf("%w: tool %q cannot be pre-approved (only file read/edit tools can)", ErrInvalidRule, tool)
		}
	}
	return nil
}

// MatchesCommand reports whether cmd may run without asking.
//
// Two conditions: cmd is one plain command line (shellSafe: nothing that could
// chain a second command, redirect, substitute or expand) and it starts with an
// allowed prefix at a word boundary — prefix "go test" matches "go test",
// "go test ./..." and "go test -v", not "go testextra". Anything else is not a
// match and therefore asks.
func (p *Permissions) MatchesCommand(cmd string) bool {
	if p == nil {
		return false
	}
	trimmedCmd := strings.TrimSpace(cmd)
	if trimmedCmd == "" || !shellSafe(trimmedCmd) || hasUnsafeFlag(trimmedCmd) {
		return false
	}
	for _, prefix := range p.AllowedCommands {
		pfx := strings.TrimSpace(prefix)
		if pfx == "" || !shellSafe(pfx) || neverPreapprovedExecutables[firstWord(pfx)] {
			continue
		}
		if trimmedCmd == pfx {
			return true
		}
		if strings.HasPrefix(trimmedCmd, pfx+" ") || strings.HasPrefix(trimmedCmd, pfx+"\t") {
			return true
		}
	}
	return false
}

// AllowsTool reports whether toolName is listed in AllowedTools and may be
// pre-approved at all (PreapprovableTools). run_command is never allowed this way.
func (p *Permissions) AllowsTool(toolName string) bool {
	if p == nil {
		return false
	}
	name := strings.TrimSpace(toolName)
	if name == "" || !PreapprovableTools[name] {
		return false
	}
	for _, allowed := range p.AllowedTools {
		if strings.TrimSpace(allowed) == name {
			return true
		}
	}
	return false
}

// CheckToolCall reports whether toolName and args are authorized by these permissions.
func (p *Permissions) CheckToolCall(toolName string, args json.RawMessage) (allowed bool, matched bool) {
	if p == nil {
		return false, false
	}
	if toolName == "run_command" {
		var cmdArgs struct {
			Command string `json:"command"`
		}
		if err := json.Unmarshal(args, &cmdArgs); err == nil && cmdArgs.Command != "" {
			if p.MatchesCommand(cmdArgs.Command) {
				return true, true
			}
		}
		return false, false
	}
	if p.AllowsTool(toolName) {
		return true, true
	}
	return false, false
}

// ReadPermissionsFile reads `<root>/.memo/permissions.json` from disk.
// Returns nil, nil if the file does not exist.
func ReadPermissionsFile(root string) (*Permissions, []byte, error) {
	path := PermissionsPath(root)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil, nil
		}
		return nil, nil, err
	}
	var perms Permissions
	if err := json.Unmarshal(data, &perms); err != nil {
		return nil, data, fmt.Errorf("parsing %s: %w", path, err)
	}
	if err := perms.ValidateRules(); err != nil {
		return nil, data, err
	}
	return &perms, data, nil
}

// WritePermissionsFile writes perms to `<root>/.memo/permissions.json`.
func WritePermissionsFile(root string, perms *Permissions) ([]byte, error) {
	if perms == nil {
		return nil, errors.New("cannot write nil permissions")
	}
	if perms.Version == 0 {
		perms.Version = PermissionsSchema
	}
	if err := perms.ValidateRules(); err != nil {
		return nil, err
	}
	data, err := json.MarshalIndent(perms, "", "  ")
	if err != nil {
		return nil, err
	}
	data = append(data, '\n')
	path := PermissionsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := fileutil.AtomicWrite(path, data, 0o600); err != nil {
		return nil, err
	}
	return data, nil
}
