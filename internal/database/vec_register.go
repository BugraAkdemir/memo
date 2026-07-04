package database

import (
	"database/sql"
	"fmt"
	"memo/internal/logx"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	sqlite3 "github.com/mattn/go-sqlite3"
)

const driverName = "sqlite3_vec"

var (
	registerOnce sync.Once
	vecAvailable bool
	statusMsg    string
)

func init() {
	registerOnce.Do(func() {
		extPath := findVecFile()
		if extPath == "" {
			statusMsg = "DATABASE: sqlite-vec extension not found, vec0 disabled"
			return
		}

		sql.Register(driverName, &sqlite3.SQLiteDriver{
			Extensions: []string{extPath},
		})

		vecAvailable = true
		statusMsg = fmt.Sprintf("DATABASE: vec driver registered successfully (%s)", extPath)
	})
}

// LogStatus logs the outcome of the sqlite-vec registration that happened in
// this package's init(). Callers choose when — init() always runs before
// main() gets a chance to redirect log output (e.g. the terminal REPL moving
// backend logs to a file), so logging directly from init() would leak onto
// the terminal no matter how early main() tries to redirect.
func LogStatus() {
	logx.Info(statusMsg)
}

func VecAvailable() bool {
	return vecAvailable
}

func DriverName() string {
	if vecAvailable {
		return driverName
	}
	return "sqlite3"
}

// findVecFile searches for the vec0 extension binary and returns an absolute
// path WITHOUT the platform-specific suffix (.so, .dylib, .dll).
// sqlite3_load_extension will append the suffix automatically.
func findVecFile() string {
	osName := runtime.GOOS

	suffix := ".so"
	if osName == "darwin" {
		suffix = ".dylib"
	} else if osName == "windows" {
		suffix = ".dll"
	}

	// Search paths (relative + absolute)
	searchDirs := searchRoots()

	for _, dir := range searchDirs {
		candidates := []string{
			fmt.Sprintf("binaries/%s/vec0", osName),
			fmt.Sprintf("binaries/%s/cpu/vec0", osName),
			fmt.Sprintf("binaries/%s/vec0%s", osName, suffix),
			fmt.Sprintf("binaries/%s/cpu/vec0%s", osName, suffix),
		}
		for _, rel := range candidates {
			target := filepath.Join(dir, rel)
			if _, err := os.Stat(target); err == nil {
				// Use the path without .so suffix for LoadExtension
				loadName := strings.TrimSuffix(target, suffix)
				return loadName
			}
		}
	}
	return ""
}

// searchRoots returns candidate base directories to search for binaries/.
func searchRoots() []string {
	var dirs []string

	// 1. Executable directory
	if exe, err := os.Executable(); err == nil {
		dirs = append(dirs, filepath.Dir(exe))
	}

	// 2. Current working directory
	if wd, err := os.Getwd(); err == nil {
		dirs = append(dirs, wd)
	}

	// 3. Walk up from CWD looking for go.mod (project root)
	if wd, err := os.Getwd(); err == nil {
		dir := wd
		for i := 0; i < 10; i++ {
			if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
				dirs = append(dirs, dir)
				break
			}
			parent := filepath.Dir(dir)
			if parent == dir {
				break
			}
			dir = parent
		}
	}

	return dedup(dirs)
}

func dedup(strs []string) []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range strs {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}
