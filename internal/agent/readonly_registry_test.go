package agent

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestNewReadOnlyRegistry_ExcludesMutators(t *testing.T) {
	r := NewReadOnlyRegistry()

	mustHave := []string{"read_file", "list_directory", "search_files", "get_file_info", "read_env", "run_command_readonly", "web_search", "fetch_page"}
	for _, name := range mustHave {
		if _, ok := r.Get(name); !ok {
			t.Errorf("read-only registry missing %q", name)
		}
	}

	mustNotHave := []string{"write_file", "edit_file", "insert_line", "delete_lines", "delete_file", "change_directory", "run_command", "self_clone", "configure_provider", "create_routine", "whatsapp_send", "share_file"}
	for _, name := range mustNotHave {
		if _, ok := r.Get(name); ok {
			t.Errorf("read-only registry exposes mutating tool %q", name)
		}
	}
}

func TestRunCommandReadOnly_RejectsNonAllowlisted(t *testing.T) {
	r := NewReadOnlyRegistry()
	tool, ok := r.Get("run_command_readonly")
	if !ok {
		t.Fatal("run_command_readonly not registered")
	}

	deny := []string{"rm -rf build", "go run main.go", "npm install", "curl http://x", "bash -c 'echo hi'", "echo hi > f"}
	for _, cmd := range deny {
		args, _ := json.Marshal(map[string]string{"command": cmd})
		if _, err := tool.ExecuteFn(context.Background(), args, t.TempDir(), nil); err == nil {
			t.Errorf("run_command_readonly allowed %q", cmd)
		}
	}

	// An allowlisted command is accepted (it may still fail to execute in the
	// test env, but not with the allowlist rejection).
	args, _ := json.Marshal(map[string]string{"command": "git status"})
	_, err := tool.ExecuteFn(context.Background(), args, t.TempDir(), nil)
	if err != nil && strings.Contains(err.Error(), "not on the read-only allowlist") {
		t.Errorf("run_command_readonly rejected an allowlisted command: %v", err)
	}
}
