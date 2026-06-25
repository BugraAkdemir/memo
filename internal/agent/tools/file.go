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

	// Extra check: prevent deleting .git or its contents
	if strings.Contains(fullPath, "/.git/") || strings.HasSuffix(fullPath, "/.git") {
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
		// File might not exist yet (e.g. for write_file), use cleaned full path
		if os.IsNotExist(err) {
			realPath = fullPath
		} else {
			return "", fmt.Errorf("failed to resolve path: %w", err)
		}
	}

	// Ensure the path is within basePath.
	rel, err := filepath.Rel(basePath, realPath)
	if err != nil {
		return "", fmt.Errorf("path is outside project directory")
	}
	if strings.HasPrefix(rel, "..") || rel == ".." {
		// Absolute or relative paths outside the project directory are only
		// allowed if they do not point to a protected system path.
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
				return "", fmt.Errorf("access denied: path is within protected directory (%s)", protected)
			}
		}
		return "", fmt.Errorf("path is outside project directory: %s", targetPath)
	}

	return realPath, nil
}
