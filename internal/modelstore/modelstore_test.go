package modelstore

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsEmbeddingModel(t *testing.T) {
	tests := []struct {
		filename string
		repoID   string
		want     bool
	}{
		{"nomic-embed-text-v1.5.Q4_K_M.gguf", "nomic-ai/nomic-embed-text-v1.5", true},
		{"bge-small-en-v1.5.Q4_K_M.gguf", "BAAI/bge-small-en-v1.5", true},
		{"e5-mistral-7b-instruct.Q4_K_M.gguf", "intfloat/e5-mistral-7b-instruct", true},
		{"llama-3.2-3b.Q4_K_M.gguf", "meta-llama/llama-3.2-3b", false},
		{"mistral-7b.Q4_K_M.gguf", "mistralai/mistral-7b", false},
		{"mxbai-embed-large-v1.Q4_K_M.gguf", "mixedbread-ai/mxbai-embed-large-v1", true},
		{"", "snowflake-arctic-embed", true},
	}
	for _, tt := range tests {
		got := isEmbeddingModel(tt.filename, tt.repoID)
		if got != tt.want {
			t.Errorf("isEmbeddingModel(%q, %q) = %v, want %v", tt.filename, tt.repoID, got, tt.want)
		}
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"meta-llama/llama-3.2-3b", "meta-llama__llama-3.2-3b"},
		{"nomic-ai/nomic-embed-text-v1.5", "nomic-ai__nomic-embed-text-v1.5"},
		{"simple", "simple"},
		{"", ""},
	}
	for _, tt := range tests {
		got := sanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestUnsanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"meta-llama__llama-3.2-3b", "meta-llama/llama-3.2-3b"},
		{"simple", "simple"},
		{"", ""},
		{"a__b__c", "a/b/c"},
	}
	for _, tt := range tests {
		got := unsanitizePath(tt.input)
		if got != tt.want {
			t.Errorf("unsanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestSanitizeUnsanitizeRoundtrip(t *testing.T) {
	inputs := []string{
		"meta-llama/llama-3.2-3b",
		"nomic-ai/nomic-embed-text-v1.5",
		"BAAI/bge-small-en-v1.5",
		"single",
	}
	for _, input := range inputs {
		sanitized := sanitizePath(input)
		unsanitized := unsanitizePath(sanitized)
		if unsanitized != strings.ReplaceAll(input, "/", "/") {
			// unsanitize replaces __ with /, so "single" stays "single"
			if input != "single" || unsanitized != "single" {
				t.Errorf("roundtrip(%q) → sanitize → unsanitize = %q", input, unsanitized)
			}
		}
	}
}

func TestFormatSpeed(t *testing.T) {
	tests := []struct {
		bps  float64
		want string
	}{
		{0, "0 B/s"},
		{500, "500 B/s"},
		{1500, "1.5 KB/s"},
		{1024 * 1024 * 2, "2.0 MB/s"},
		{1024 * 1024 * 1024 * 3.5, "3.5 GB/s"},
	}
	for _, tt := range tests {
		got := formatSpeed(tt.bps)
		if got != tt.want {
			t.Errorf("formatSpeed(%f) = %q, want %q", tt.bps, got, tt.want)
		}
	}
}

func TestListLocalModelsEmpty(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}
	models := s.ListLocalModels()
	if len(models) != 0 {
		t.Errorf("len(models) = %d, want 0", len(models))
	}
}

func TestListLocalModelsWithFiles(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	os.WriteFile(filepath.Join(dir, "model1.gguf"), []byte("data"), 0644)
	os.WriteFile(filepath.Join(dir, "model2.gguf"), []byte("data2"), 0644)
	os.WriteFile(filepath.Join(dir, "readme.txt"), []byte("info"), 0644)

	models := s.ListLocalModels()
	if len(models) != 2 {
		t.Fatalf("len(models) = %d, want 2", len(models))
	}
}

func TestListLocalModelsSkipsDownloading(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	os.WriteFile(filepath.Join(dir, "model.gguf.downloading"), []byte("partial"), 0644)
	os.WriteFile(filepath.Join(dir, "real.gguf"), []byte("data"), 0644)

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Errorf("len(models) = %d, want 1 (skip .downloading)", len(models))
	}
}

func TestListLocalModelsWithRepoDir(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	repoDir := filepath.Join(dir, "meta-llama__llama-3.2-3b")
	os.MkdirAll(repoDir, 0755)
	os.WriteFile(filepath.Join(repoDir, "llama-3.2-3b.Q4_K_M.gguf"), []byte("data"), 0644)

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].RepoID != "meta-llama/llama-3.2-3b" {
		t.Errorf("RepoID = %q, want meta-llama/llama-3.2-3b", models[0].RepoID)
	}
	if models[0].Filename != "llama-3.2-3b.Q4_K_M.gguf" {
		t.Errorf("Filename = %q", models[0].Filename)
	}
}

func TestDeleteLocalModel(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}
	modelPath := filepath.Join(dir, "model.gguf")
	os.WriteFile(modelPath, []byte("data"), 0644)

	if err := s.DeleteLocalModel(modelPath); err != nil {
		t.Fatalf("DeleteLocalModel() error = %v", err)
	}
	if _, err := os.Stat(modelPath); !os.IsNotExist(err) {
		t.Error("model file still exists after delete")
	}
}

func TestDeleteLocalModelOutsideDir(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	outsideFile := filepath.Join(os.TempDir(), "outside.gguf")
	os.WriteFile(outsideFile, []byte("data"), 0644)
	defer os.Remove(outsideFile)

	err := s.DeleteLocalModel(outsideFile)
	if err == nil {
		t.Error("DeleteLocalModel() should reject paths outside models dir")
	}
}

func TestDeleteLocalModelNonExistent(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	err := s.DeleteLocalModel(filepath.Join(dir, "nonexistent.gguf"))
	if err == nil {
		t.Error("DeleteLocalModel() should error on non-existent file")
	}
}

func TestImportLocalModel(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: filepath.Join(dir, "models")}

	sourcePath := filepath.Join(dir, "source.gguf")
	if err := os.WriteFile(sourcePath, []byte("model data"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := s.ImportLocalModel(sourcePath); err != nil {
		t.Fatalf("ImportLocalModel() error = %v", err)
	}

	destPath := filepath.Join(s.modelsDir, "source.gguf")
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("imported model not found at destination")
	}
}

func TestMaxImportSizeConstant(t *testing.T) {
	// Verify the constant is exactly 50 GiB as documented.
	want := int64(50 * 1024 * 1024 * 1024)
	if maxImportSize != want {
		t.Errorf("maxImportSize = %d, want %d (50 GiB)", maxImportSize, want)
	}
}

func TestImportLocalModelErrorMessages(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: filepath.Join(dir, "models")}

	err := s.ImportLocalModel(filepath.Join(dir, "nonexistent.gguf"))
	if err == nil {
		t.Fatal("expected error for non-existent file")
	}

	err = s.ImportLocalModel(dir)
	if err == nil || !strings.Contains(err.Error(), "directory") {
		t.Errorf("expected directory error, got: %v", err)
	}
}

func TestImportLocalModelDir(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: filepath.Join(dir, "models")}

	err := s.ImportLocalModel(dir)
	if err == nil {
		t.Error("ImportLocalModel() should error on directory")
	}
}

func TestImportLocalModelNonExistent(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: filepath.Join(dir, "models")}

	err := s.ImportLocalModel(filepath.Join(dir, "nonexistent.gguf"))
	if err == nil {
		t.Error("ImportLocalModel() should error on non-existent file")
	}
}
