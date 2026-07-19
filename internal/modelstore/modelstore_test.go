package modelstore

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeMinimalGGUF writes a real (if tiny) GGUF file whose only metadata is
// general.architecture + "<arch>.context_length" — enough for
// internal/gguf.ContextLength to parse correctly, without needing to depend
// on that package's own test helpers from a different package.
func writeMinimalGGUF(t *testing.T, path, arch string, ctxLen uint32) {
	t.Helper()
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(0x46554747)) // magic "GGUF"
	binary.Write(&buf, binary.LittleEndian, uint32(3))          // version
	binary.Write(&buf, binary.LittleEndian, uint64(0))          // tensor_count
	binary.Write(&buf, binary.LittleEndian, uint64(2))          // kv_count

	writeStr := func(s string) {
		binary.Write(&buf, binary.LittleEndian, uint64(len(s)))
		buf.WriteString(s)
	}
	writeStr("general.architecture")
	binary.Write(&buf, binary.LittleEndian, uint32(8)) // GGUF STRING type
	writeStr(arch)

	writeStr(arch + ".context_length")
	binary.Write(&buf, binary.LittleEndian, uint32(4)) // GGUF UINT32 type
	binary.Write(&buf, binary.LittleEndian, ctxLen)

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write minimal gguf: %v", err)
	}
}

// writeMinimalGGUFWithChatTemplate is writeMinimalGGUF plus a
// tokenizer.chat_template key, for exercising the GGUF-derived
// SupportsTools detection (internal/gguf.Metadata.SupportsTools).
func writeMinimalGGUFWithChatTemplate(t *testing.T, path, arch string, ctxLen uint32, chatTemplate string) {
	t.Helper()
	var buf bytes.Buffer
	binary.Write(&buf, binary.LittleEndian, uint32(0x46554747)) // magic "GGUF"
	binary.Write(&buf, binary.LittleEndian, uint32(3))          // version
	binary.Write(&buf, binary.LittleEndian, uint64(0))          // tensor_count
	binary.Write(&buf, binary.LittleEndian, uint64(3))          // kv_count

	writeStr := func(s string) {
		binary.Write(&buf, binary.LittleEndian, uint64(len(s)))
		buf.WriteString(s)
	}
	writeStr("general.architecture")
	binary.Write(&buf, binary.LittleEndian, uint32(8))
	writeStr(arch)

	writeStr(arch + ".context_length")
	binary.Write(&buf, binary.LittleEndian, uint32(4))
	binary.Write(&buf, binary.LittleEndian, ctxLen)

	writeStr("tokenizer.chat_template")
	binary.Write(&buf, binary.LittleEndian, uint32(8))
	writeStr(chatTemplate)

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		t.Fatalf("write minimal gguf: %v", err)
	}
}

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

// TestAuthorFromRepoID is the regression test for a real bug: HF's
// /api/models search response carries no "author" field at all (confirmed
// by hitting it directly), so HFModelResult.Author silently decoded to ""
// for every single search result — the frontend's avatar lookup had
// nothing to search HF's org/user API for, so every Discover list item
// showed the generic "?" empty-author placeholder regardless of the real
// repo owner.
func TestAuthorFromRepoID(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"google/gemma-2b", "google"},
		{"meta-llama/Llama-3.2-3B-Instruct", "meta-llama"},
		{"bartowski/google_gemma-3-4b-it-GGUF", "bartowski"},
		{"no-namespace-at-all", ""},
		{"", ""},
		{"/leading-slash-only", ""},
	}
	for _, tt := range tests {
		if got := authorFromRepoID(tt.id); got != tt.want {
			t.Errorf("authorFromRepoID(%q) = %q, want %q", tt.id, got, tt.want)
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

// TestListLocalModels_PopulatesMaxContextFromGGUFHeader is a regression test
// for the local-model context-size crash: the frontend used to let the user
// type an arbitrary --ctx-size (e.g. 10000000) with no bound, which crashed
// llama-server for any model that couldn't actually support it. This
// asserts ListLocalModels surfaces the model's *real* max context (read
// from its own GGUF header) so a client can build a bounded control instead
// of a free-text field.
func TestListLocalModels_PopulatesMaxContextFromGGUFHeader(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	path := filepath.Join(dir, "model.gguf")
	writeMinimalGGUF(t, path, "llama", 131072)

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].MaxContext != 131072 {
		t.Errorf("MaxContext = %d, want 131072", models[0].MaxContext)
	}
}

// TestListLocalModels_CachesMaxContextInSidecar verifies the result is
// persisted into the .meta.json sidecar with GGUFMetaChecked=true, so a
// model whose architecture isn't recognized (MaxContext genuinely 0) isn't
// re-parsed from disk on every single poll — see modelMeta.GGUFMetaChecked's
// doc comment.
func TestListLocalModels_CachesMaxContextInSidecar(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	path := filepath.Join(dir, "model.gguf")
	writeMinimalGGUF(t, path, "qwen2", 32768)

	if models := s.ListLocalModels(); len(models) != 1 || models[0].MaxContext != 32768 {
		t.Fatalf("first ListLocalModels(): got %+v", models)
	}

	data, err := os.ReadFile(path + ".meta.json")
	if err != nil {
		t.Fatalf("sidecar was not written: %v", err)
	}
	var meta modelMeta
	if err := json.Unmarshal(data, &meta); err != nil {
		t.Fatalf("unmarshal sidecar: %v", err)
	}
	if !meta.GGUFMetaChecked {
		t.Error("sidecar GGUFMetaChecked = false, want true after the first list")
	}
	if meta.MaxContext != 32768 {
		t.Errorf("sidecar MaxContext = %d, want 32768", meta.MaxContext)
	}

	// Overwrite the file with garbage — if a second ListLocalModels() call
	// still reports 32768, it proves the cached sidecar value was used
	// rather than a doomed re-parse of the (now-corrupt) file.
	if err := os.WriteFile(path, []byte("not a gguf file anymore"), 0644); err != nil {
		t.Fatalf("corrupt model file: %v", err)
	}
	models := s.ListLocalModels()
	if len(models) != 1 || models[0].MaxContext != 32768 {
		t.Errorf("second ListLocalModels() = %+v, want cached MaxContext=32768 (not re-parsed)", models)
	}
}

// TestListLocalModels_DetectsToolsFromChatTemplateWithNoHFTags is the
// regression test for the hardcoded-filename-family guessing this replaces
// (frontend's old LocalModel.likelySupportsTools/DiscoverItem.likelySupportsTools):
// a locally-imported model with no .meta.json sidecar at all (so HF tags
// never entered the picture) must still be correctly flagged as
// tool-capable purely from its own embedded chat template.
func TestListLocalModels_DetectsToolsFromChatTemplateWithNoHFTags(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	path := filepath.Join(dir, "model.gguf")
	writeMinimalGGUFWithChatTemplate(t, path, "qwen2", 32768,
		`{%- if tools %}{{ message.tool_calls }}{%- endif %}`)

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if !models[0].SupportsTools {
		t.Error("SupportsTools = false, want true — the model's own chat template references tool_calls")
	}
}

// TestListLocalModels_KeepsHFTagSupportsToolsWhenGGUFTemplateHasNone
// verifies the OR: an HF tag already having confirmed tool support must
// survive even if this particular GGUF's chat template doesn't happen to
// mention tool_calls (some conversions strip/simplify the template) —
// GGUF-derived detection can only add, never take away, a capability HF's
// own tags already confirmed.
func TestListLocalModels_KeepsHFTagSupportsToolsWhenGGUFTemplateHasNone(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	path := filepath.Join(dir, "model.gguf")
	writeMinimalGGUFWithChatTemplate(t, path, "llama", 8192,
		`{% for message in messages %}{{ message.content }}{% endfor %}`)
	if err := saveModelMeta(path, &modelMeta{RepoID: "test/model", SupportsTools: true}); err != nil {
		t.Fatalf("saveModelMeta: %v", err)
	}

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if !models[0].SupportsTools {
		t.Error("SupportsTools = false, want true — an existing HF-tag-confirmed value must survive")
	}
}

// TestListLocalModels_UnrecognizedArchGetsZeroNotError covers a GGUF file
// that parses fine but has no context_length key this reader recognizes —
// callers must see MaxContext=0 ("unknown"), not a crash or a bogus value.
func TestListLocalModels_UnrecognizedArchGetsZeroNotError(t *testing.T) {
	dir := t.TempDir()
	s := &Store{modelsDir: dir}

	// A file that's a well-formed GGUF but not one this test's helper gives
	// a context_length key for at all: reuse writeMinimalGGUF but point the
	// architecture at a key that was never written by writing 0 kv pairs
	// instead. Simplest: just drop in plain garbage bytes past the magic —
	// ListLocalModels must not error out or panic, just record MaxContext=0.
	os.WriteFile(filepath.Join(dir, "unknown.gguf"), []byte("not actually a gguf file"), 0644)

	models := s.ListLocalModels()
	if len(models) != 1 {
		t.Fatalf("len(models) = %d, want 1", len(models))
	}
	if models[0].MaxContext != 0 {
		t.Errorf("MaxContext = %d, want 0 for an unparseable file", models[0].MaxContext)
	}
}
