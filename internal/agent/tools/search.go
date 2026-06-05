package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SearchFilesArgs represents arguments for search_files tool.
type SearchFilesArgs struct {
	Pattern string `json:"pattern"`
	Path    string `json:"path"`
}

func SearchFiles(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args SearchFilesArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	searchDir, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var result strings.Builder
	count := 0
	maxResults := 100

	err = filepath.WalkDir(searchDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return fmt.Errorf("search timed out")
		default:
		}

		if d.IsDir() {
			if d.Name() == ".git" || d.Name() == "node_modules" || d.Name() == "vendor" || d.Name() == "build" {
				return filepath.SkipDir
			}
			return nil
		}

		matched, err := filepath.Match(args.Pattern, d.Name())
		if err != nil {
			return fmt.Errorf("invalid pattern: %w", err)
		}

		if matched {
			if count >= maxResults {
				result.WriteString(fmt.Sprintf("... (truncated, max %d results)\n", maxResults))
				return fmt.Errorf("max_results_reached")
			}
			
			relPath, _ := filepath.Rel(basePath, path)
			result.WriteString(fmt.Sprintf("%s\n", relPath))
			count++
		}
		return nil
	})

	if err != nil && err.Error() != "max_results_reached" {
		return "", fmt.Errorf("search failed: %w", err)
	}

	if count == 0 {
		return "No files found matching pattern", nil
	}

	return result.String(), nil
}

// ReadEnvArgs represents arguments for read_env tool.
type ReadEnvArgs struct{}

func ReadEnv(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	// Filter out sensitive environment variables like API keys, tokens, etc.
	sensitiveKeywords := []string{"KEY", "TOKEN", "SECRET", "PASS", "AUTH", "CREDENTIAL"}
	
	var result strings.Builder
	count := 0
	
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		
		key := parts[0]
		isSensitive := false
		
		keyUpper := strings.ToUpper(key)
		for _, keyword := range sensitiveKeywords {
			if strings.Contains(keyUpper, keyword) {
				isSensitive = true
				break
			}
		}
		
		if isSensitive {
			result.WriteString(fmt.Sprintf("%s=********\n", key))
		} else {
			result.WriteString(fmt.Sprintf("%s=%s\n", key, parts[1]))
		}
		count++
	}
	
	return fmt.Sprintf("Environment variables (%d total):\n%s", count, result.String()), nil
}
