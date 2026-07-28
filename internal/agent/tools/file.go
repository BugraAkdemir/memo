package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// ReadFileArgs represents arguments for read_file tool.
type ReadFileArgs struct {
	Path string `json:"path"`
}

func ReadFile(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args ReadFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}

	// 1MB limit for reading files
	if info.Size() > 1024*1024 {
		return "", fmt.Errorf("file is too large (size: %d bytes, limit: 1MB)", info.Size())
	}

	data, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("read failed: %w", err)
	}

	return string(data), nil
}

// WriteFileArgs represents arguments for write_file tool.
type WriteFileArgs struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

func WriteFile(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args WriteFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	// Enforce 10MB size limit for write operations
	if len(args.Content) > 10*1024*1024 {
		return "", fmt.Errorf("content too large: %d bytes (limit: 10MB)", len(args.Content))
	}

	// Create directory if it doesn't exist
	dir := filepath.Dir(fullPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("failed to create directory: %w", err)
	}

	// Backup existing file using BackupManager
	if createBackup != nil {
		if err := createBackup(fullPath); err != nil {
			return "", fmt.Errorf("backup failed, aborting write: %w", err)
		}
	}

	if err := os.WriteFile(fullPath, []byte(args.Content), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return fmt.Sprintf("Successfully wrote %d bytes to %s", len(args.Content), args.Path), nil
}

// DeleteFileArgs represents arguments for delete_file tool.
type DeleteFileArgs struct {
	Path string `json:"path"`
}

func DeleteFile(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args DeleteFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	// Extra check: prevent deleting .git or its contents (use ToSlash for Windows compat)
	slashPath := filepath.ToSlash(fullPath)
	if strings.Contains(slashPath, "/.git/") || strings.HasSuffix(slashPath, "/.git") {
		return "", fmt.Errorf("cannot delete .git directory or files within it")
	}

	if createBackup != nil {
		if err := createBackup(fullPath); err != nil {
			return "", fmt.Errorf("backup failed, aborting delete: %w", err)
		}
	}

	if err := os.RemoveAll(fullPath); err != nil {
		return "", fmt.Errorf("delete failed: %w", err)
	}

	return fmt.Sprintf("Successfully deleted %s", args.Path), nil
}

// ListDirectoryArgs represents arguments for list_directory tool.
type ListDirectoryArgs struct {
	Path      string `json:"path"`
	Recursive bool   `json:"recursive"`
}

func ListDirectory(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args ListDirectoryArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("not a directory: %s", args.Path)
	}

	var result strings.Builder
	count := 0
	maxEntries := 1000

	if args.Recursive {
		err = filepath.WalkDir(fullPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors
			}
			if count >= maxEntries {
				return filepath.SkipDir
			}
			relPath, _ := filepath.Rel(fullPath, path)
			if relPath == "." {
				return nil
			}

			// Skip .git directory
			if d.IsDir() && d.Name() == ".git" {
				return filepath.SkipDir
			}

			prefix := "F"
			if d.IsDir() {
				prefix = "D"
			}
			result.WriteString(fmt.Sprintf("[%s] %s\n", prefix, relPath))
			count++
			return nil
		})
	} else {
		entries, err := os.ReadDir(fullPath)
		if err != nil {
			return "", fmt.Errorf("read dir failed: %w", err)
		}
		for _, d := range entries {
			if count >= maxEntries {
				result.WriteString(fmt.Sprintf("... (truncated, max %d entries)\n", maxEntries))
				break
			}
			prefix := "F"
			if d.IsDir() {
				prefix = "D"
			}
			result.WriteString(fmt.Sprintf("[%s] %s\n", prefix, d.Name()))
			count++
		}
	}

	if err != nil {
		return "", fmt.Errorf("walk failed: %w", err)
	}

	if count == 0 {
		return "(empty directory)", nil
	}
	return result.String(), nil
}

// GetFileInfoArgs represents arguments for get_file_info tool.
type GetFileInfoArgs struct {
	Path string `json:"path"`
}

func GetFileInfo(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args GetFileInfoArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	info, err := os.Stat(fullPath)
	if err != nil {
		return "", fmt.Errorf("stat failed: %w", err)
	}

	result := fmt.Sprintf("Name: %s\nSize: %d bytes\nMode: %s\nModified: %s\nIsDir: %v\n",
		info.Name(),
		info.Size(),
		info.Mode().String(),
		info.ModTime().Format(time.RFC3339),
		info.IsDir(),
	)
	return result, nil
}

// defaultProtectedPaths returns platform-appropriate protected system paths.
func defaultProtectedPaths() []string {
	if runtime.GOOS == "windows" {
		sysDrive := os.Getenv("SystemDrive")
		if sysDrive == "" {
			sysDrive = "C:"
		}
		winDir := os.Getenv("SystemRoot")
		if winDir == "" {
			winDir = sysDrive + `\Windows`
		}
		progData := os.Getenv("ProgramData")
		if progData == "" {
			progData = sysDrive + `\ProgramData`
		}
		progFiles := os.Getenv("ProgramFiles")
		if progFiles == "" {
			progFiles = sysDrive + `\Program Files`
		}
		progFilesX86 := os.Getenv("ProgramFiles(x86)")
		if progFilesX86 == "" {
			progFilesX86 = sysDrive + `\Program Files (x86)`
		}
		return []string{
			winDir + `\`, progFiles + `\`, progFilesX86 + `\`,
			sysDrive + `\Boot\`, progData + `\`,
		}
	}
	return []string{
		"/etc/", "/usr/", "/boot/", "/dev/", "/sys/", "/proc/", "/var/",
		"/home/", "/root/", "/tmp/", "/run/", "/opt/", "/mnt/", "/media/",
	}
}

// resolveExistingAncestor resolves symlinks along path as far as its
// components actually exist on disk, then rejoins any trailing
// not-yet-created components unresolved (there is nothing to resolve for a
// path segment that doesn't exist yet). Used when a plain
// filepath.EvalSymlinks(path) itself fails with os.IsNotExist — shared by
// validatePath (file.go) and RunCommand's CWD resolution (command.go),
// which had the identical gap (BUG-C1).
//
// Recurses up one directory level per call; a legitimate filesystem path
// only has as many components as the OS allows, so this cannot recurse
// meaningfully deep. Falls back to the cleaned, unresolved path once it
// either reaches the filesystem root or hits a non-IsNotExist error partway
// up (e.g. a permission error) — best-effort, matching how the pre-fix code
// already tolerated an unresolved fallback for those cases.
func resolveExistingAncestor(path string) string {
	parent := filepath.Dir(path)
	if parent == path {
		// Reached the filesystem root and even that doesn't exist/resolve.
		return filepath.Clean(path)
	}
	resolvedParent, err := filepath.EvalSymlinks(parent)
	if err != nil {
		if os.IsNotExist(err) {
			resolvedParent = resolveExistingAncestor(parent)
		} else {
			resolvedParent = filepath.Clean(parent)
		}
	}
	return filepath.Join(resolvedParent, filepath.Base(path))
}

// validatePath ensures the path is within the base path, resolves relative
// paths, blocks traversal outside the project, and denies access to protected
// system directories.
func validatePath(targetPath, basePath string) (string, error) {
	if !filepath.IsAbs(basePath) {
		absBase, err := filepath.Abs(basePath)
		if err != nil {
			return "", fmt.Errorf("could not determine absolute base path: %w", err)
		}
		basePath = absBase
	}

	var fullPath string
	if filepath.IsAbs(targetPath) {
		fullPath = filepath.Clean(targetPath)
	} else {
		fullPath = filepath.Join(basePath, targetPath)
	}

	// Resolve symlinks
	realPath, err := filepath.EvalSymlinks(fullPath)
	if err != nil {
		// File might not exist yet (e.g. for write_file) — resolve as much
		// of the path as actually exists instead of falling back to the
		// raw, unresolved path (BUG-C1): EvalSymlinks fails with
		// IsNotExist as soon as the FINAL component is missing, even if an
		// EXISTING ancestor directory earlier in the path is itself a
		// symlink pointing outside basePath. Falling back to fullPath
		// verbatim left that ancestor symlink completely unresolved, so
		// the inside-basePath check a few lines down saw only the
		// project-relative-looking literal path — while the actual
		// write (os.WriteFile, etc.) transparently follows the real,
		// unresolved symlink and lands wherever it points, outside the
		// sandbox entirely.
		if os.IsNotExist(err) {
			realPath = resolveExistingAncestor(fullPath)
		} else {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
	}

	// Ensure the path is within basePath.
	rel, err := filepath.Rel(basePath, realPath)
	if err != nil {
		return "", fmt.Errorf("%q is outside the project directory (%s)", targetPath, basePath)
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		// Absolute or relative paths outside the project directory are only
		// allowed if they do not point to a protected system path. Either
		// branch below still rejects the path — defaultProtectedPaths only
		// picks which message to show, never whether to allow it (see
		// TestValidatePath_RejectsUnlistedAbsolutePathOutsideBase's doc
		// comment). Both messages now name the actual project directory
		// (basePath) explicitly: the old wording just said "outside" without
		// saying outside of *what*, so the caller (a human, or the model
		// itself retrying) had nothing to correct toward and would often
		// blindly retry with a different tool/path instead of the one path
		// that was actually allowed.
		cmpPath := realPath
		if runtime.GOOS == "windows" {
			cmpPath = strings.ToLower(realPath)
		}
		for _, protected := range defaultProtectedPaths() {
			needle := protected
			if runtime.GOOS == "windows" {
				needle = strings.ToLower(protected)
			}
			if strings.HasPrefix(cmpPath, needle) {
				return "", fmt.Errorf("access denied: %q is within a protected system directory (%s) — only files inside %s are accessible", targetPath, protected, basePath)
			}
		}
		return "", fmt.Errorf("%q is outside the project directory — only files inside %s are accessible", targetPath, basePath)
	}

	return realPath, nil
}
