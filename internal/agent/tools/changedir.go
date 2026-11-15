package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// ChangeDirectoryArgs represents arguments for the change_directory tool.
type ChangeDirectoryArgs struct {
	Path            string `json:"path"`
	CreateIfMissing bool   `json:"create_if_missing,omitempty"`
}

// OutsideSandboxHint is appended to every "path is outside the working
// directory" denial (validatePath in file.go, RunCommand's cwd/command
// checks in command.go) so the model learns about change_directory at the
// exact moment it would actually be useful, instead of only when the user
// happens to already know the tool exists and asks for it by name. Without
// this, the model's only fallback was telling the user to manually move the
// file into basePath — technically correct, but not what change_directory
// was built to replace.
const OutsideSandboxHint = " If it looks like the user wants you working in that location, offer to switch your working directory there with change_directory instead of asking them to move the file — describe what you'd switch to and let them confirm."

// hardDenylistedRoots are bare system directories change_directory refuses
// to adopt as the new sandbox root, even with explicit user permission —
// defense in depth, the same posture as run_command's destructive-pattern
// blacklist. defaultProtectedPaths (file.go) only guards *escaping*
// basePath; it is never applied to basePath itself, so without this separate
// check here, approving a single change_directory call could otherwise hand
// every other tool full read/write/delete access to the whole filesystem.
// Subdirectories of these roots, and everything not listed (including /tmp
// and the user's own home directory), are allowed — that access breadth is
// exactly what this tool exists to grant.
func hardDenylistedRoots() []string {
	if runtime.GOOS == "windows" {
		sysDrive := os.Getenv("SystemDrive")
		if sysDrive == "" {
			sysDrive = "C:"
		}
		return []string{sysDrive + `\`, sysDrive + `\Windows`, sysDrive + `\Windows\`}
	}
	return []string{"/", "/etc", "/usr", "/boot", "/dev", "/sys", "/proc", "/var", "/root", "/run"}
}

// resolveChangeDirectoryTarget expands ~, resolves a relative path against
// the current basePath (so "the repo's sibling lib/ folder" style requests
// work naturally), resolves symlinks, and confirms the result is an existing
// directory. It does not consult defaultProtectedPaths/validatePath — those
// answer "can I reach outside basePath", not "can this become the new
// basePath"; see hardDenylistedRoots above for the check that matters here.
func expandChangeDirectoryPath(rawPath, basePath string) (string, error) {
	if strings.TrimSpace(rawPath) == "" {
		return "", fmt.Errorf("path is required")
	}

	expanded := rawPath
	if expanded == "~" || strings.HasPrefix(expanded, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("could not resolve home directory: %w", err)
		}
		expanded = filepath.Join(home, strings.TrimPrefix(expanded, "~"))
	}

	if filepath.IsAbs(expanded) {
		return filepath.Clean(expanded), nil
	}
	return filepath.Join(basePath, expanded), nil
}

func checkNotHardDenylistedRoot(path string) error {
	cmpPath := path
	if runtime.GOOS == "windows" {
		cmpPath = strings.ToLower(path)
	}
	for _, root := range hardDenylistedRoots() {
		needle := root
		if runtime.GOOS == "windows" {
			needle = strings.ToLower(needle)
		}
		if cmpPath == needle {
			return fmt.Errorf("refusing to set the working directory to %q — it is a protected system root, not a project directory", path)
		}
	}
	return nil
}

func resolveChangeDirectoryTarget(rawPath, basePath string) (string, error) {
	fullPath, err := expandChangeDirectoryPath(rawPath, basePath)
	if err != nil {
		return "", err
	}

	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("directory does not exist: %s", fullPath)
		}
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	info, err := os.Stat(realPath)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("%q is a file, not a directory", rawPath)
	}

	if err := checkNotHardDenylistedRoot(realPath); err != nil {
		return "", err
	}

	return realPath, nil
}

// ChangeDirectoryPreview describes the pending directory switch for the
// permission dialog / WhatsApp-Telegram y/n prompt.
func ChangeDirectoryPreview(argsJSON json.RawMessage, basePath string) (string, error) {
	var args ChangeDirectoryArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	target, err := resolveChangeDirectoryTarget(args.Path, basePath)
	if err == nil {
		return fmt.Sprintf("Change working directory to: %s", target), nil
	}
	if args.CreateIfMissing {
		fullPath, pathErr := expandChangeDirectoryPath(args.Path, basePath)
		if pathErr != nil {
			return "", pathErr
		}
		if denyErr := checkNotHardDenylistedRoot(fullPath); denyErr != nil {
			return "", denyErr
		}
		return fmt.Sprintf("Create and switch working directory to: %s", fullPath), nil
	}
	return "", err
}

// ChangeDirectory switches the agent's effective sandbox root for the rest
// of the current conversation. It updates the live sandbox (so later tool
// calls in this same turn already see the new directory — Pipeline.RunStream
// re-reads the sandbox's base path once per iteration) and, when a session is
// attached to ctx, persists the choice so it survives into future turns too.
func ChangeDirectory(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args ChangeDirectoryArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	target, err := resolveChangeDirectoryTarget(args.Path, basePath)
	if err != nil && args.CreateIfMissing {
		fullPath, pathErr := expandChangeDirectoryPath(args.Path, basePath)
		if pathErr != nil {
			return "", pathErr
		}
		if denyErr := checkNotHardDenylistedRoot(fullPath); denyErr != nil {
			return "", denyErr
		}
		if mkErr := os.MkdirAll(fullPath, 0755); mkErr != nil {
			return "", fmt.Errorf("create directory %s: %w", fullPath, mkErr)
		}
		target, err = resolveChangeDirectoryTarget(args.Path, basePath)
	}
	if err != nil {
		return "", err
	}

	sandbox, ok := SandboxSetterFromContext(ctx)
	if !ok {
		return "", fmt.Errorf("internal error: no sandbox available to change_directory")
	}
	sandbox.SetBasePath(target)

	persisted := ""
	if setter, sessionID, ok := WorkingDirSetterFromContext(ctx); ok {
		if err := setter.SetWorkingDir(sessionID, target); err != nil {
			persisted = fmt.Sprintf(" (warning: could not persist for future turns: %v)", err)
		}
	} else if setter, sessionID, ok := ProjectPathSetterFromContext(ctx); ok {
		if err := setter.SetProjectPath(sessionID, target); err != nil {
			// The live sandbox already switched (the more important half —
			// it's what every subsequent tool call this turn actually uses),
			// so a persistence failure is reported but not fatal to the call.
			persisted = fmt.Sprintf(" (warning: could not persist for future turns: %v)", err)
		}
	}

	return fmt.Sprintf("Working directory changed to %s%s", target, persisted), nil
}
