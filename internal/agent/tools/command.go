package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RunCommandArgs represents arguments for run_command tool.
type RunCommandArgs struct {
	Command string `json:"command"`
	CWD     string `json:"cwd"`
}

var blacklistedCommands = []string{
	"rm -rf /",
	"rm -rf ~",
	"rm -rf .",
	"dd",
	"mkfs",
	"format",
	"fdisk",
	"parted",
	"chmod 777",
	"chown",
	"sudo",
	"su",
	"pkexec",
	":(){ :|:& };:", // fork bomb
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
	cmdLower := strings.ToLower(args.Command)
	for _, blacklisted := range blacklistedCommands {
		if strings.Contains(cmdLower, blacklisted) {
			return "", fmt.Errorf("command is blacklisted for safety: %s", blacklisted)
		}
	}

	// Execution
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "bash", "-c", args.Command)
	cmd.Dir = workingDir
	
	// Limit env variables, keep safe path
	cmd.Env = []string{
		"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=" + os.Getenv("HOME"),
		"USER=" + os.Getenv("USER"),
		"LANG=en_US.UTF-8",
	}

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
