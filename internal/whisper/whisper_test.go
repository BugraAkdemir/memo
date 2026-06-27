package whisper

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNewServer(t *testing.T) {
	s := NewServer(9877)
	if s.port != 9877 {
		t.Errorf("port = %d, want 9877", s.port)
	}
}

func TestNewServerDefaultPort(t *testing.T) {
	s := NewServer(0)
	if s.port != 9877 {
		t.Errorf("default port = %d, want 9877", s.port)
	}
}

func TestIsRunning_NewServer(t *testing.T) {
	s := NewServer(9877)
	if s.IsRunning() {
		t.Error("new server should not be running")
	}
}

func TestGetStatus_NewServer(t *testing.T) {
	s := NewServer(9877)
	status := s.GetStatus()
	if status.Running {
		t.Error("new server should not be running")
	}
	if status.Port != 9877 {
		t.Errorf("port = %d, want 9877", status.Port)
	}
}

func TestWhisperServerBinary(t *testing.T) {
	got := whisperServerBinary()
	if runtime.GOOS == "windows" {
		if got != "whisper-server.exe" {
			t.Errorf("binary = %q, want %q", got, "whisper-server.exe")
		}
	} else {
		if got != "whisper-server" {
			t.Errorf("binary = %q, want %q", got, "whisper-server")
		}
	}
}

func TestBinarySearchBases(t *testing.T) {
	bases := binarySearchBases()
	if len(bases) < 1 {
		t.Error("expected at least one base")
	}
	if bases[0] != "." {
		t.Errorf("first base = %q, want %q", bases[0], ".")
	}
}

func TestWithPrependedEnvPath(t *testing.T) {
	tests := []struct {
		name           string
		env            []string
		key            string
		dir            string
		caseInsensitive bool
		wantContains   string
	}{
		{
			name:           "prepend to existing PATH",
			env:            []string{"PATH=/usr/bin", "HOME=/home/user"},
			key:            "PATH",
			dir:            "/opt/bin",
			caseInsensitive: false,
			wantContains:   "/opt/bin",
		},
		{
			name:           "add new PATH entry",
			env:            []string{"HOME=/home/user"},
			key:            "PATH",
			dir:            "/opt/bin",
			caseInsensitive: false,
			wantContains:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := withPrependedEnvPath(tt.env, tt.key, tt.dir, tt.caseInsensitive)

			if tt.wantContains != "" {
				found := false
				for _, entry := range result {
					if strings.HasPrefix(entry, tt.key+"=") && strings.Contains(entry, tt.wantContains) {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("expected entry with %s containing %s in: %v", tt.key, tt.wantContains, result)
				}
			}
		})
	}
}

func TestWithPrependedEnvPath_CaseInsensitive(t *testing.T) {
	env := []string{"Path=C:\\Windows", "HOME=C:\\Users"}
	result := withPrependedEnvPath(env, "PATH", "C:\\tools", true)

	found := false
	for _, entry := range result {
		if strings.HasPrefix(entry, "Path=") && strings.Contains(entry, "C:\\tools") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected case-insensitive match, got: %v", result)
	}
}

func TestPrependPathValue(t *testing.T) {
	tests := []struct {
		name    string
		current string
		dir     string
		want    string
	}{
		{"empty current", "", "/opt/bin", "/opt/bin"},
		{"prepend", "/usr/bin", "/opt/bin", "/opt/bin:/usr/bin"},
		{"deduplicate", "/opt/bin:/usr/bin", "/opt/bin", "/opt/bin:/usr/bin"},
		{"trailing separator", "/usr/bin/", "/opt/bin", "/opt/bin:/usr/bin"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := prependPathValue(tt.current, tt.dir)
			sep := string(os.PathListSeparator)
			parts := strings.Split(got, sep)

			if parts[0] != tt.dir {
				t.Errorf("first element = %q, want %q", parts[0], tt.dir)
			}
		})
	}
}

func TestSamePathEntry(t *testing.T) {
	tests := []struct {
		a    string
		b    string
		want bool
	}{
		{"/usr/bin", "/usr/bin", true},
		{"/usr/bin", "/usr/local/bin", false},
	}
	for _, tt := range tests {
		got := samePathEntry(tt.a, tt.b)
		if got != tt.want {
			t.Errorf("samePathEntry(%q, %q) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestPingPort_NotRunning(t *testing.T) {
	s := NewServer(19999) // unlikely to be in use
	if s.pingPort() {
		t.Error("pingPort on unused port should return false")
	}
}

func TestResolveBinary_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "whisper-server")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh"), 0755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := resolveBinary(binPath)
	if err != nil {
		t.Fatalf("resolveBinary failed: %v", err)
	}
	if got != binPath {
		t.Errorf("got %q, want %q", got, binPath)
	}
}

func TestResolveBinary_NotFound(t *testing.T) {
	_, err := resolveBinary("/nonexistent/path/whisper-server")
	if err == nil {
		t.Error("expected error for nonexistent binary")
	}
}

func TestResolveModel_ExplicitPath(t *testing.T) {
	dir := t.TempDir()
	modelPath := filepath.Join(dir, "ggml-small.bin")
	if err := os.WriteFile(modelPath, []byte("model data"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got := resolveModel(modelPath)
	if got != modelPath {
		t.Errorf("got %q, want %q", got, modelPath)
	}
}

func TestResolveModel_NotFound(t *testing.T) {
	got := resolveModel("/nonexistent/model.bin")
	if got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}

func TestResolveModel_EmptyConfig(t *testing.T) {
	got := resolveModel("")
	// Should not crash — returns empty when nothing found
	_ = got
}
