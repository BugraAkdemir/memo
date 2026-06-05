package tools

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type EditFileArgs struct {
	Path        string `json:"path"`
	OldString   string `json:"old_string,omitempty"`
	NewString   string `json:"new_string,omitempty"`
	StartLine   int    `json:"start_line,omitempty"`
	EndLine     int    `json:"end_line,omitempty"`
	NewContent  string `json:"new_content,omitempty"`
}

func EditFile(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args EditFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	contentStr := string(content)

	var newFileContent string

	if args.OldString != "" {
		if !strings.Contains(contentStr, args.OldString) {
			return "", fmt.Errorf("old_string not found in file")
		}
		newFileContent = strings.Replace(contentStr, args.OldString, args.NewString, 1) // Replace first occurrence
	} else if args.StartLine > 0 && args.EndLine >= args.StartLine {
		lines := strings.Split(contentStr, "\n")
		if args.StartLine > len(lines) {
			return "", fmt.Errorf("start_line %d is out of bounds (file has %d lines)", args.StartLine, len(lines))
		}
		end := args.EndLine
		if end > len(lines) {
			end = len(lines)
		}
		
		var newLines []string
		newLines = append(newLines, lines[:args.StartLine-1]...)
		if args.NewContent != "" {
			newLines = append(newLines, args.NewContent)
		}
		newLines = append(newLines, lines[end:]...)
		newFileContent = strings.Join(newLines, "\n")
	} else {
		return "", fmt.Errorf("must provide either old_string or start_line/end_line")
	}

	if createBackup != nil {
		createBackup(fullPath)
	}

	if err := os.WriteFile(fullPath, []byte(newFileContent), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return fmt.Sprintf("Successfully edited %s", args.Path), nil
}

func EditFilePreview(argsJSON json.RawMessage, basePath string) (string, error) {
	var args EditFileArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", err
	}
	
	var diff string
	if args.OldString != "" {
		diff = fmt.Sprintf("```diff\n- %s\n+ %s\n```", 
			strings.ReplaceAll(args.OldString, "\n", "\n- "), 
			strings.ReplaceAll(args.NewString, "\n", "\n+ "))
	} else if args.StartLine > 0 {
		diff = fmt.Sprintf("Replacing lines %d to %d with:\n```diff\n+ %s\n```", 
			args.StartLine, args.EndLine, 
			strings.ReplaceAll(args.NewContent, "\n", "\n+ "))
	} else {
		return "", fmt.Errorf("invalid arguments")
	}
	
	return fmt.Sprintf("**Preview for `edit_file` on `%s`:**\n%s", args.Path, diff), nil
}

type InsertLineArgs struct {
	Path       string `json:"path"`
	LineNumber int    `json:"line_number"`
	Content    string `json:"content"`
}

func InsertLine(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args InsertLineArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Split(string(content), "\n")
	
	idx := args.LineNumber - 1
	if idx < 0 { idx = 0 }
	if idx > len(lines) { idx = len(lines) }

	var newLines []string
	newLines = append(newLines, lines[:idx]...)
	newLines = append(newLines, args.Content)
	newLines = append(newLines, lines[idx:]...)

	if createBackup != nil {
		createBackup(fullPath)
	}

	if err := os.WriteFile(fullPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return fmt.Sprintf("Successfully inserted line %d in %s", args.LineNumber, args.Path), nil
}

func InsertLinePreview(argsJSON json.RawMessage, basePath string) (string, error) {
	var args InsertLineArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", err
	}
	
	diff := fmt.Sprintf("```diff\n+ %s\n```", strings.ReplaceAll(args.Content, "\n", "\n+ "))
	return fmt.Sprintf("**Preview for `insert_line` on `%s` at line %d:**\n%s", args.Path, args.LineNumber, diff), nil
}

type DeleteLinesArgs struct {
	Path      string `json:"path"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func DeleteLines(argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args DeleteLinesArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}

	fullPath, err := validatePath(args.Path, basePath)
	if err != nil {
		return "", err
	}

	content, err := os.ReadFile(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	lines := strings.Split(string(content), "\n")
	
	if args.StartLine < 1 || args.StartLine > len(lines) {
		return "", fmt.Errorf("start_line %d is out of bounds", args.StartLine)
	}
	end := args.EndLine
	if end > len(lines) {
		end = len(lines)
	}

	var newLines []string
	newLines = append(newLines, lines[:args.StartLine-1]...)
	newLines = append(newLines, lines[end:]...)

	if createBackup != nil {
		createBackup(fullPath)
	}

	if err := os.WriteFile(fullPath, []byte(strings.Join(newLines, "\n")), 0644); err != nil {
		return "", fmt.Errorf("write failed: %w", err)
	}

	return fmt.Sprintf("Successfully deleted lines %d-%d in %s", args.StartLine, args.EndLine, args.Path), nil
}

func DeleteLinesPreview(argsJSON json.RawMessage, basePath string) (string, error) {
	var args DeleteLinesArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", err
	}
	return fmt.Sprintf("**Preview for `delete_lines` on `%s`:**\nWill delete lines %d to %d.", args.Path, args.StartLine, args.EndLine), nil
}
