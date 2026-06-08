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

// RunCommandArgs represents arguments for run_command tool.
type RunCommandArgs struct {
	Command string `json:"command"`
	CWD     string `json:"cwd"`
}

var blacklistedPatterns = []*regexp.Regexp{
	regexp.MustCompile(`\brm\s+-rf\s+/\b`),
	regexp.MustCompile(`\brm\s+-rf\s+~\b`),
	regexp.MustCompile(`\brm\s+-rf\s+\.\b`),
	regexp.MustCompile(`\bdd\b`),
	regexp.MustCompile(`\bmkfs\b`),
	regexp.MustCompile(`\bformat\b`),
	regexp.MustCompile(`\bfdisk\b`),
	regexp.MustCompile(`\bparted\b`),
	regexp.MustCompile(`\bchmod\s+777\b`),
	regexp.MustCompile(`\bchown\b`),
	regexp.MustCompile(`\bsudo\b`),
	regexp.MustCompile(`\bsu\b`),
	regexp.MustCompile(`\bpkexec\b`),
	regexp.MustCompile(`:\{\s*:\|\s*:\s*&\s*;?\s*:\s*\}`), // fork bomb
	regexp.MustCompile(`\bnc\s+-e\b`),
	regexp.MustCompile(`\bbash\s+-i\b`),
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

func RunCommand(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
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

	// Security: validate path
	realCWD, err := filepath.EvalSymlinks(workingDir)
	if err == nil {
		rel, err := filepath.Rel(basePath, realCWD)
		if err != nil || strings.HasPrefix(rel, "..") {
			return "", fmt.Errorf("cwd is outside project directory")
		}
		workingDir = realCWD
	}

	// Security: Blacklist check
	if pattern, blocked := isBlacklisted(args.Command); blocked {
		return "", fmt.Errorf("command is blacklisted for safety: %s", pattern)
	}

	// Execution
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", args.Command)
		cmd.Env = []string{
			"PATH=" + os.Getenv("PATH"),
			"USERPROFILE=" + os.Getenv("USERPROFILE"),
			"SystemRoot=" + os.Getenv("SystemRoot"),
		}
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", args.Command)
		cmd.Env = []string{
			"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
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

	// Truncate if output is too large (10MB limit)
	const maxSize = 10 * 1024 * 1024
	if len(outStr)+len(errStr) > maxSize {
		result.WriteString(fmt.Sprintf("\n... Output truncated, exceeded 10MB limit ...\n"))
		if len(outStr) > maxSize/2 {
			outStr = outStr[:maxSize/2]
		}
		if len(errStr) > maxSize/2 {
			errStr = errStr[:maxSize/2]
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
