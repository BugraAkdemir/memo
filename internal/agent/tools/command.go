package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

// DefaultToolTimeout is the maximum execution time for any single tool call.
const DefaultToolTimeout = 60 * time.Second

// RunCommandArgs represents arguments for run_command tool.
type RunCommandArgs struct {
	Command string `json:"command"`
	CWD     string `json:"cwd"`
}

// rmTargetEnd matches what may legally follow a bare "/", "~", or "." target
// in an rm -rf command for it to still count as wiping that whole target:
// end of string, whitespace, or a shell operator/glob character. Using \b
// here (as the patterns below used to) is wrong — "/", "~", and "." are all
// non-word characters, so \b requires the *next* character to be a word
// character to form a boundary. At end of string (also non-word on that
// side) there is no boundary, so "rm -rf /", "rm -rf /*", "rm -rf ~", and
// "rm -rf ." never matched their own blacklist entry.
const rmTargetEnd = `(?:$|[\s;&|*])`

var blacklistedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+-rf\s+/` + rmTargetEnd),
	regexp.MustCompile(`\brm\s+-rf\s+~` + rmTargetEnd),
	regexp.MustCompile(`\brm\s+-rf\s+\.` + rmTargetEnd),
	regexp.MustCompile(`\bdd\s`), // dd (disk destroyer) followed by space
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bfdisk\b`),
	regexp.MustCompile(`\bparted\b`),
	regexp.MustCompile(`\bchmod\s+777\b`),
	regexp.MustCompile(`\bsudo\b`),
	regexp.MustCompile(`\bpkexec\b`),
	regexp.MustCompile(`:\{\s*:\|\s*:\s*&\s*;?\s*:\s*\}`), // fork bomb
	// Block interactive/code-exec shell invocations but NOT .sh file references.
	// Patterns like "sh -c" or "bash -i" are shell escape vectors;
	// references like "./build.sh" or "ls *.sh" are legitimate.
	// Also block absolute-path variants: /bin/sh -c, /usr/bin/bash -i, etc.
	regexp.MustCompile(`\bsh\s+-[ci]\b`),
	regexp.MustCompile(`\bbash\s+-[ci]\b`),
	regexp.MustCompile(`\bzsh\s+-[ci]\b`),
	regexp.MustCompile(`\bdash\s+-[ci]\b`),
	regexp.MustCompile(`(?:^|[\s;&|])(?:\S*[/\\])?sh\s+-[ci]\b`),
	regexp.MustCompile(`(?:^|[\s;&|])(?:\S*[/\\])?bash\s+-[ci]\b`),
	// Block code execution via interpreters.
	regexp.MustCompile(`\bpython[23]?\s+-c\b`),
	regexp.MustCompile(`\bnode\s+-e\b`),
	regexp.MustCompile(`\bperl\s+-e\b`),
	regexp.MustCompile(`\bruby\s+-e\b`),
	regexp.MustCompile(`\bphp\s+-r\b`),
	regexp.MustCompile(`\blua\s+-e\b`),
	// Block privilege escalation.
	regexp.MustCompile(`\bsu\b`),
	regexp.MustCompile(`\bdoas\b`),
	regexp.MustCompile(`\bnc\s+-[^ ]*e\b`),
	regexp.MustCompile(`\bmkfifo\b`),
	regexp.MustCompile(`\bshutdown\b`),
	regexp.MustCompile(`\breboot\b`),
	regexp.MustCompile(`\bhalt\b`),
	regexp.MustCompile(`\bpoweroff\b`),
	// Windows-specific destructive commands (run_command uses `cmd /C` there).
	regexp.MustCompile(`\bformat\s`),                        // format a volume
	regexp.MustCompile(`\bdiskpart\b`),                      // partition editor
	regexp.MustCompile(`\bdel\b[^\n]*\s/[sq]\b`),            // del /s or /q (recursive/quiet)
	regexp.MustCompile(`\b(rd|rmdir)\b[^\n]*\s/s\b`),        // rd /s (recursive dir delete)
	regexp.MustCompile(`\breg\s+delete\b`),                  // registry deletion
	regexp.MustCompile(`\bvssadmin\s+delete\b`),             // delete shadow copies
	regexp.MustCompile(`\bcipher\s+/w\b`),                   // wipe free space
	regexp.MustCompile(`\b(net\s+user|net\s+localgroup)\b`), // account manipulation
	// Block long-option shell escape variants.
	regexp.MustCompile(`\bsh\s+--command\b`),
	regexp.MustCompile(`\bbash\s+--command\b`),
	regexp.MustCompile(`\bzsh\s+--command\b`),
	regexp.MustCompile(`\bdash\s+--command\b`),
}

// shellSubstitutionChars matches characters commonly used for shell command
// substitution / evaluation. run_command is intentionally not a full shell, so
// we reject these to reduce injection risk.
var shellSubstitutionChars = regexp.MustCompile("[\\$\\`\\`]")

func isBlacklisted(cmd string) (string, bool) {
	cmdLower := strings.ToLower(cmd)
	for _, re := range blacklistedPatterns {
		if re.MatchString(cmdLower) {
			return re.String(), true
		}
	}
	if shellSubstitutionChars.MatchString(cmd) {
		return "shell substitution characters ($ `) are not allowed", true
	}
	return "", false
}

// commandPathSeparators are shell metacharacters that can separate or wrap a
// path-like argument inside a command string ("cat /etc/passwd && whoami").
// Replaced with spaces before splitting on whitespace, so a path token isn't
// left glued to its neighbor.
var commandPathSeparators = regexp.MustCompile("[;&|()<>`]")

// extractPathTokens returns every whitespace-separated argument in command
// that looks like a filesystem path reference (absolute, "~"-relative, or
// using ".." traversal) — a cheap, tokenizer-free heuristic, not a full
// shell parse. False positives (a non-path argument that happens to contain
// '/', e.g. a URL or a Go module path) are harmless here: they just get
// resolved and checked against the protected-paths list like any other
// candidate, and legitimately fail to match.
func extractPathTokens(command string) []string {
	normalized := commandPathSeparators.ReplaceAllString(command, " ")
	fields := strings.Fields(normalized)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, `"'`)
		// BUG-L2: a "--flag=/path/value"-shaped argument's actual path is
		// the part after "=" (e.g. "--file=/etc/passwd") — the raw token
		// isn't absolute on its own, so commandTargetsProtectedPath used to
		// join it against workingDir instead of recognizing it as pointing
		// at /etc, letting a protected-path target slip through disguised
		// behind a flag. Extract the suffix as the candidate instead of the
		// whole "--flag=..." token whenever it looks path-like.
		if idx := strings.LastIndex(f, "="); idx >= 0 && idx < len(f)-1 {
			if v := f[idx+1:]; strings.HasPrefix(v, "/") || strings.HasPrefix(v, "~") || strings.Contains(v, "/") {
				tokens = append(tokens, v)
				continue
			}
		}
		if strings.HasPrefix(f, "/") || strings.HasPrefix(f, "~") || strings.Contains(f, "/") {
			tokens = append(tokens, f)
		}
	}
	return tokens
}

// commandTargetsProtectedPath scans command for a path-like argument that
// resolves — absolute as-is, "~" expanded to the home directory, anything
// else joined against workingDir (so "../../etc/passwd" traversal is caught
// too) — outside basePath AND under one of defaultProtectedPaths(). This
// mirrors validatePath's actual two-step check (file.go): a resolved path is
// only compared against the protected list once it's already established to
// fall outside the project directory, so an ordinary relative path *inside*
// the project (e.g. "./...") is never flagged just because the project
// itself happens to live under a protected prefix like "/home/". Returns the
// first offending token found, if any.
func commandTargetsProtectedPath(command, workingDir, basePath string) (string, bool) {
	homeDir, _ := os.UserHomeDir()
	for _, tok := range extractPathTokens(command) {
		resolved := tok
		if strings.HasPrefix(resolved, "~") {
			if homeDir == "" {
				continue
			}
			resolved = homeDir + resolved[1:]
		}
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(workingDir, resolved)
		}
		resolved = filepath.Clean(resolved)

		rel, relErr := filepath.Rel(basePath, resolved)
		outside := relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator))
		if !outside {
			continue // inside the project directory — allowed
		}

		cmp := resolved
		if runtime.GOOS == "windows" {
			cmp = strings.ToLower(resolved)
		}
		for _, protected := range defaultProtectedPaths() {
			needle := protected
			if runtime.GOOS == "windows" {
				needle = strings.ToLower(protected)
			}
			if strings.HasPrefix(cmp, needle) {
				return tok, true
			}
		}
	}
	return "", false
}

// CheckBlacklist exposes the same dangerous-command guard run_command uses,
// including the shell-substitution ($ `) block. Appropriate for a command
// string built from live LLM/user input at call time, where that
// substitution block is a real injection defense.
func CheckBlacklist(cmd string) (string, bool) {
	return isBlacklisted(cmd)
}

// CheckDestructivePatterns checks cmd against only the fixed destructive-
// command patterns (rm -rf /, mkfs, sudo, fork bombs, ...) — not the
// shell-substitution ($ `) block CheckBlacklist also applies. For callers
// whose command string is author-trusted static content rather than
// something assembled from live input (e.g. a skill manifest's own
// `command:` field, chosen when the skill was written/installed), that
// substitution block would reject legitimate $VAR usage — including the
// MEMO_SKILL_NAME/MEMO_PROJECT_DIR env vars this codebase itself passes to
// skill tool processes — without adding real protection, since there is no
// LLM-controlled string being interpolated into the command at call time.
func CheckDestructivePatterns(cmd string) (string, bool) {
	cmdLower := strings.ToLower(cmd)
	for _, re := range blacklistedPatterns {
		if re.MatchString(cmdLower) {
			return re.String(), true
		}
	}
	return "", false
}

// PrepareCommand builds a sandboxed *exec.Cmd for running `command` (via
// the platform shell) in workingDir, with a minimal explicit environment
// (no os.Environ() passthrough) — the same construction run_command uses.
// execCtx carries either ctx's own deadline or, if ctx has none, a fresh
// DefaultToolTimeout deadline; callers that need to distinguish a timeout
// from an ordinary failure after cmd.Run() returns should check
// execCtx.Err() == context.DeadlineExceeded. cancel is always non-nil and
// must be deferred by the caller.
func PrepareCommand(ctx context.Context, command, workingDir string) (cmd *exec.Cmd, execCtx context.Context, cancel context.CancelFunc) {
	execCtx = ctx
	cancel = func() {}
	if _, ok := ctx.Deadline(); !ok {
		execCtx, cancel = context.WithTimeout(ctx, DefaultToolTimeout)
	}

	homeDir, _ := os.UserHomeDir()
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/C", command)
		userProfile := os.Getenv("USERPROFILE")
		if userProfile == "" {
			userProfile = homeDir
		}
		temp := os.Getenv("TEMP")
		if temp == "" {
			temp = os.TempDir()
		}
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"USERPROFILE=" + userProfile,
			"SystemRoot=" + os.Getenv("SystemRoot"),
			"USERNAME=" + os.Getenv("USERNAME"),
			"TEMP=" + temp,
			"TMP=" + temp,
			"COMSPEC=" + os.Getenv("COMSPEC"),
			"HOMEDRIVE=" + os.Getenv("HOMEDRIVE"),
			"HOMEPATH=" + os.Getenv("HOMEPATH"),
		}
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", command)
		userPath := os.Getenv("PATH")
		if userPath == "" {
			userPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		}
		home := os.Getenv("HOME")
		if home == "" {
			home = homeDir
		}
		user := os.Getenv("USER")
		if user == "" {
			user = os.Getenv("USERNAME")
		}
		cmd.Env = []string{
			"PATH=" + userPath,
			"HOME=" + home,
			"USER=" + user,
			"LANG=en_US.UTF-8",
		}
	}
	cmd.Dir = workingDir
	return cmd, execCtx, cancel
}

// FormatCommandOutput renders captured stdout/stderr into the truncated
// STDOUT/STDERR text returned to the LLM — shared by run_command and
// internal/skill's tool executor so both truncate/report identically.
func FormatCommandOutput(runErr error, timedOut bool, stdout, stderr string) string {
	var result strings.Builder
	if runErr != nil {
		if timedOut {
			result.WriteString("Command timed out\n")
		} else {
			fmt.Fprintf(&result, "Command failed with error: %v\n", runErr)
		}
	}

	// Truncate COMBINED output if it exceeds the 10MB limit.
	// We proportionally preserve stdout and stderr rather than truncating independently.
	const maxSize = 10 * 1024 * 1024
	total := len(stdout) + len(stderr)
	if total > maxSize {
		result.WriteString("\n... Output truncated, exceeded 10MB limit ...\n")
		ratio := float64(maxSize) / float64(total)
		outLimit := int(float64(len(stdout)) * ratio)
		errLimit := int(float64(len(stderr)) * ratio)
		if outLimit < len(stdout) {
			stdout = stdout[:outLimit]
		}
		if errLimit < len(stderr) {
			stderr = stderr[:errLimit]
		}
	}

	if stdout != "" {
		fmt.Fprintf(&result, "STDOUT:\n%s\n", stdout)
	}
	if stderr != "" {
		fmt.Fprintf(&result, "STDERR:\n%s\n", stderr)
	}

	if result.Len() == 0 {
		return "(Command executed successfully with no output)"
	}
	return result.String()
}

func RunCommand(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args RunCommandArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	// Resolve CWD
	var workingDir string
	if args.CWD != "" {
		if filepath.IsAbs(args.CWD) {
			workingDir = args.CWD
		} else {
			workingDir = filepath.Join(basePath, args.CWD)
		}
	} else {
		workingDir = basePath
	}

	// Security: validate CWD is inside basePath regardless of whether it exists yet.
	realCWD, err := filepath.EvalSymlinks(workingDir)
	if err != nil {
		if os.IsNotExist(err) {
			// The dir doesn't exist yet — resolve as much of the path as
			// actually exists instead of falling back to it unresolved
			// (BUG-C1, same gap as validatePath in file.go): an existing
			// ancestor directory earlier in the path could itself be a
			// symlink pointing outside basePath, and a bare Clean(path)
			// fallback left that completely unresolved, defeating the Rel
			// check below.
			realCWD = resolveExistingAncestor(workingDir)
		} else {
			// On Windows the caller may lack SeCreateSymbolicLinkPrivilege
			// to resolve a junction point — fall back to Clean path same as
			// before; the Rel check below still enforces the boundary as
			// best it can without that resolution.
			realCWD = filepath.Clean(workingDir)
		}
	}
	rel, relErr := filepath.Rel(basePath, realCWD)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("cwd %q is outside the project directory — only %s is accessible.%s", workingDir, basePath, OutsideSandboxHint)
	}
	workingDir = realCWD

	// Security: Blacklist check
	if pattern, blocked := isBlacklisted(args.Command); blocked {
		return "", fmt.Errorf("command is blacklisted for safety: %s", pattern)
	}

	// Security: reject any path-like argument inside the command string that
	// resolves under a protected system directory — the same boundary
	// validatePath already enforces for read_file/write_file/list_directory.
	// Without this, run_command was a complete bypass of that boundary: cwd
	// was validated but nothing stopped e.g. "cat /etc/shadow" or
	// "cat ~/.ssh/id_rsa" from reading straight through it (BUG-M7).
	if target, blocked := commandTargetsProtectedPath(args.Command, workingDir, basePath); blocked {
		return "", fmt.Errorf("access denied: command references %q, outside the project directory — only %s is accessible.%s", target, basePath, OutsideSandboxHint)
	}

	// PrepareCommand honors the caller's own deadline (e.g. the pipeline's
	// 120s per-tool budget) instead of silently truncating it to
	// DefaultToolTimeout; it only falls back to DefaultToolTimeout when the
	// caller passed no deadline at all.
	cmd, execCtx, cancel := PrepareCommand(ctx, args.Command, workingDir)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()
	timedOut := execCtx.Err() == context.DeadlineExceeded

	return FormatCommandOutput(runErr, timedOut, stdout.String(), stderr.String()), nil
}
