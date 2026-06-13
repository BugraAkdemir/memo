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

var blacklistedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+-rf\s+/\b`),
	regexp.MustCompile(`\brm\s+-rf\s+~\b`),
	regexp.MustCompile(`\brm\s+-rf\s+\.\b`),
	regexp.MustCompile(`\bdd\s`),               // dd (disk destroyer) followed by space
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
	regexp.MustCompile(`\bsh\s+-[ci]\b`),
	regexp.MustCompile(`\bbash\s+-[ci]\b`),
	regexp.MustCompile(`\bzsh\s+-[ci]\b`),
	regexp.MustCompile(`\bdash\s+-[ci]\b`),
	regexp.MustCompile(`\bnc\s+-e\b`),
	regexp.MustCompile(`\bmkfifo\b`),
	regexp.MustCompile(`\bshutdown\b`),
	regexp.MustCompile(`\breboot\b`),
	regexp.MustCompile(`\bhalt\b`),
	regexp.MustCompile(`\bpoweroff\b`),
}

func isBlacklisted(cmd string) (string, bool) {
	cmdLower := strings.ToLower(cmd)
	for _, re := range blacklistedPatterns {
		if re.MatchString(cmdLower) {
			return re.String(), true
		}
	}
	return "", false
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
		if !os.IsNotExist(err) {
			return "", fmt.Errorf("failed to resolve cwd: %w", err)
		}
		// Directory doesn't exist; validate the cleaned path instead.
		realCWD = filepath.Clean(workingDir)
	}
	rel, relErr := filepath.Rel(basePath, realCWD)
	if relErr != nil || strings.HasPrefix(rel, "..") {
		return "", fmt.Errorf("cwd is outside project directory")
	}
	workingDir = realCWD

	// Security: Blacklist check
	if pattern, blocked := isBlacklisted(args.Command); blocked {
		return "", fmt.Errorf("command is blacklisted for safety: %s", pattern)
	}

	// Execution with proper context propagation from caller.
	// Respect the caller's timeout/cancellation while enforcing a hard 60s cap.
	execCtx, cancel := context.WithTimeout(ctx, DefaultToolTimeout)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(execCtx, "cmd", "/C", args.Command)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"USERPROFILE=" + os.Getenv("USERPROFILE"),
			"SystemRoot=" + os.Getenv("SystemRoot"),
		}
	} else {
		cmd = exec.CommandContext(execCtx, "bash", "-c", args.Command)
		// Inherit the user's PATH so that tools like go, npm, node, python etc.
		// installed in non-system locations are discoverable.
		userPath := os.Getenv("PATH")
		if userPath == "" {
			userPath = "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin"
		}
		cmd.Env = []string{
			"PATH=" + userPath,
			"HOME=" + os.Getenv("HOME"),
			"USER=" + os.Getenv("USER"),
			"LANG=en_US.UTF-8",
		}
	}
	cmd.Dir = workingDir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

	var result strings.Builder
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.WriteString(fmt.Sprintf("Command timed out after 60s\n"))
		} else {
			result.WriteString(fmt.Sprintf("Command failed with error: %v\n", err))
		}
	}

	outStr := stdout.String()
	errStr := stderr.String()

	// Truncate COMBINED output if it exceeds the 10MB limit.
	// We proportionally preserve stdout and stderr rather than truncating independently.
	const maxSize = 10 * 1024 * 1024
	total := len(outStr) + len(errStr)
	if total > maxSize {
		result.WriteString(fmt.Sprintf("\n... Output truncated, exceeded 10MB limit ...\n"))
		ratio := float64(maxSize) / float64(total)
		outLimit := int(float64(len(outStr)) * ratio)
		errLimit := int(float64(len(errStr)) * ratio)
		if outLimit < len(outStr) {
			outStr = outStr[:outLimit]
		}
		if errLimit < len(errStr) {
			errStr = errStr[:errLimit]
		}
	}

	if outStr != "" {
		result.WriteString(fmt.Sprintf("STDOUT:\n%s\n", outStr))
	}
	if errStr != "" {
		result.WriteString(fmt.Sprintf("STDERR:\n%s\n", errStr))
	}

	if result.Len() == 0 {
		return "(Command executed successfully with no output)", nil
	}

	return result.String(), nil
}
