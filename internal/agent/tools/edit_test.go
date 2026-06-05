package tools

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEditFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "edit_test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testFilePath := filepath.Join(tempDir, "test.txt")
	err = os.WriteFile(testFilePath, []byte("hello world\nline 2\nline 3"), 0644)
	if err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	args := map[string]interface{}{
		"path":       testFilePath,
		"old_string": "hello world",
		"new_string": "hello everyone",
	}
	argsJSON, _ := json.Marshal(args)

	_, err = EditFile(argsJSON, tempDir, nil)
	if err != nil {
		t.Fatalf("EditFile failed: %v", err)
	}

	content, _ := os.ReadFile(testFilePath)
	if string(content) != "hello everyone\nline 2\nline 3" {
		t.Errorf("unexpected content: %s", string(content))
	}

	// Test InsertLine
	argsInsert := map[string]interface{}{
		"path":        testFilePath,
		"line_number": 2,
		"content":     "inserted line",
	}
	argsInsertJSON, _ := json.Marshal(argsInsert)

	_, err = InsertLine(argsInsertJSON, tempDir, nil)
	if err != nil {
		t.Fatalf("InsertLine failed: %v", err)
	}

	content, _ = os.ReadFile(testFilePath)
	expected := "hello everyone\ninserted line\nline 2\nline 3"
	if string(content) != expected {
		t.Errorf("unexpected content after insert: %s", string(content))
	}

	// Test DeleteLines
	argsDelete := map[string]interface{}{
		"path":       testFilePath,
		"start_line": 2,
		"end_line":   3,
	}
	argsDeleteJSON, _ := json.Marshal(argsDelete)

	_, err = DeleteLines(argsDeleteJSON, tempDir, nil)
	if err != nil {
		t.Fatalf("DeleteLines failed: %v", err)
	}

	content, _ = os.ReadFile(testFilePath)
	expected = "hello everyone\nline 3"
	if string(content) != expected {
		t.Errorf("unexpected content after delete: %s", string(content))
	}
}
