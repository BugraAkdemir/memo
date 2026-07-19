// SPDX-License-Identifier: AGPL-3.0-or-later

package gguf

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"
)

// ggufBuilder assembles a minimal synthetic GGUF byte stream for tests —
// real GGUF files are gigabytes, but ContextLength only ever reads the
// header + metadata section, so a hand-built few-KB stream exercises the
// exact same code path.
type ggufBuilder struct {
	buf     bytes.Buffer
	kvCount uint64
	kvBuf   bytes.Buffer
}

func newGGUFBuilder(version uint32) *ggufBuilder {
	b := &ggufBuilder{}
	binary.Write(&b.buf, binary.LittleEndian, magic)
	binary.Write(&b.buf, binary.LittleEndian, version)
	binary.Write(&b.buf, binary.LittleEndian, uint64(0)) // tensor_count, unused by ContextLength
	return b
}

func (b *ggufBuilder) writeString(s string) {
	binary.Write(&b.kvBuf, binary.LittleEndian, uint64(len(s)))
	b.kvBuf.WriteString(s)
}

func (b *ggufBuilder) kvString(key, val string) {
	b.writeString(key)
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeString))
	b.writeString(val)
	b.kvCount++
}

func (b *ggufBuilder) kvUint32(key string, val uint32) {
	b.writeString(key)
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeUint32))
	binary.Write(&b.kvBuf, binary.LittleEndian, val)
	b.kvCount++
}

// kvStringArray writes an ARRAY-of-STRING value, exercising the same skip
// path a model's (often huge) tokenizer vocabulary takes in a real file.
func (b *ggufBuilder) kvStringArray(key string, vals []string) {
	b.writeString(key)
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeArray))
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeString))
	binary.Write(&b.kvBuf, binary.LittleEndian, uint64(len(vals)))
	for _, v := range vals {
		b.writeString(v)
	}
	b.kvCount++
}

// kvUint32Array writes an ARRAY-of-UINT32 value, exercising the bulk-skip
// path for fixed-size array elements.
func (b *ggufBuilder) kvUint32Array(key string, vals []uint32) {
	b.writeString(key)
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeArray))
	binary.Write(&b.kvBuf, binary.LittleEndian, uint32(typeUint32))
	binary.Write(&b.kvBuf, binary.LittleEndian, uint64(len(vals)))
	for _, v := range vals {
		binary.Write(&b.kvBuf, binary.LittleEndian, v)
	}
	b.kvCount++
}

// writeToTempFile finalizes the stream (header + kv_count + kv pairs) and
// writes it to a temp file, returning its path.
func (b *ggufBuilder) writeToTempFile(t *testing.T) string {
	t.Helper()
	binary.Write(&b.buf, binary.LittleEndian, b.kvCount)
	b.buf.Write(b.kvBuf.Bytes())

	path := filepath.Join(t.TempDir(), "model.gguf")
	if err := os.WriteFile(path, b.buf.Bytes(), 0644); err != nil {
		t.Fatalf("write temp gguf: %v", err)
	}
	return path
}

func TestContextLength_FindsArchSpecificKey(t *testing.T) {
	b := newGGUFBuilder(3)
	b.kvString("general.architecture", "llama")
	b.kvUint32("llama.context_length", 131072)
	// A realistic vocabulary array sitting between the two keys that
	// matter, exercising the array-skip path so it can't desync the reader.
	b.kvStringArray("tokenizer.ggml.tokens", []string{"<s>", "</s>", "hello", "world"})
	b.kvUint32Array("tokenizer.ggml.token_type", []uint32{1, 1, 2, 2})
	path := b.writeToTempFile(t)

	got, err := ContextLength(path)
	if err != nil {
		t.Fatalf("ContextLength: %v", err)
	}
	if got != 131072 {
		t.Errorf("ContextLength = %d, want 131072", got)
	}
}

func TestContextLength_FallsBackToSuffixScanWhenArchKeyMismatched(t *testing.T) {
	b := newGGUFBuilder(3)
	// architecture string that doesn't match its own context_length key —
	// e.g. a custom finetune conversion; the suffix-scan fallback must still
	// find it.
	b.kvString("general.architecture", "some-custom-arch")
	b.kvUint32("mistral.context_length", 32768)
	path := b.writeToTempFile(t)

	got, err := ContextLength(path)
	if err != nil {
		t.Fatalf("ContextLength: %v", err)
	}
	if got != 32768 {
		t.Errorf("ContextLength = %d, want 32768 via suffix-scan fallback", got)
	}
}

func TestContextLength_ReturnsZeroNotErrorWhenKeyMissing(t *testing.T) {
	b := newGGUFBuilder(3)
	b.kvString("general.architecture", "llama")
	b.kvString("general.name", "some model with no context_length key at all")
	path := b.writeToTempFile(t)

	got, err := ContextLength(path)
	if err != nil {
		t.Fatalf("ContextLength returned an error for a merely-incomplete file: %v", err)
	}
	if got != 0 {
		t.Errorf("ContextLength = %d, want 0 (unknown, not an error)", got)
	}
}

func TestContextLength_ErrorsOnBadMagic(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-model.gguf")
	if err := os.WriteFile(path, []byte("not a gguf file at all"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	if _, err := ContextLength(path); err == nil {
		t.Error("ContextLength = nil error, want an error for a non-GGUF file")
	}
}

func TestContextLength_ErrorsOnMissingFile(t *testing.T) {
	if _, err := ContextLength(filepath.Join(t.TempDir(), "does-not-exist.gguf")); err == nil {
		t.Error("ContextLength = nil error, want an error for a missing file")
	}
}
