// SPDX-License-Identifier: AGPL-3.0-or-later

// Package gguf reads just enough of a GGUF model file's header to answer
// questions a client would otherwise have to guess at from the filename:
// how large a context window was this model actually trained for, and does
// it actually support tool/function calling? It deliberately parses only
// the metadata key-value section — never tensor data — which sits at a
// small, self-describing offset right after the header regardless of the
// file's overall size (a GGUF file can be many gigabytes; this reads at
// most a few hundred KB, dominated by skipping the tokenizer's vocabulary
// array).
package gguf

import (
	"bufio"
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"strings"
)

// magic is "GGUF" read as a little-endian uint32.
const magic uint32 = 0x46554747

// GGUF metadata value types, per the format's own spec
// (https://github.com/ggml-org/ggml/blob/master/docs/gguf.md).
const (
	typeUint8   = 0
	typeInt8    = 1
	typeUint16  = 2
	typeInt16   = 3
	typeUint32  = 4
	typeInt32   = 5
	typeFloat32 = 6
	typeBool    = 7
	typeString  = 8
	typeArray   = 9
	typeUint64  = 10
	typeInt64   = 11
	typeFloat64 = 12
)

// fixedSizes gives the byte width of every value type that isn't STRING or
// ARRAY (both variable-length, handled separately).
var fixedSizes = map[uint32]int64{
	typeUint8: 1, typeInt8: 1, typeBool: 1,
	typeUint16: 2, typeInt16: 2,
	typeUint32: 4, typeInt32: 4, typeFloat32: 4,
	typeUint64: 8, typeInt64: 8, typeFloat64: 8,
}

// Metadata is the subset of a GGUF file's own metadata this package
// extracts — everything a client would otherwise have had to guess from
// the filename or repo name.
type Metadata struct {
	// ContextLength is the model's trained context length — the
	// "<architecture>.context_length" key every llama.cpp-supported GGUF
	// file carries (e.g. "llama.context_length", "qwen2.context_length",
	// "gemma2.context_length"). 0 means unknown (an architecture string
	// this reader doesn't recognize the convention for), never "the model
	// supports 0 tokens of context."
	ContextLength int
	// SupportsTools reports whether the model's own embedded chat template
	// (the "tokenizer.chat_template" Jinja template llama.cpp itself uses
	// to format the conversation) references "tool_calls" — the field name
	// essentially every tool-calling chat template convention (Llama 3.1+,
	// Qwen2.5, Hermes, Mistral, ...) uses when rendering or parsing a tool
	// call. This is the model's own declared behavior, read from the
	// actual file — not a guess based on the model's name or family.
	SupportsTools bool
}

// Read parses path's GGUF metadata key-value section once and extracts
// everything this package knows how to read (see Metadata's doc comment).
// Returns a zero Metadata, not an error, for any field this reader
// couldn't determine — only a genuinely malformed/non-GGUF file, or an I/O
// failure, produces a non-nil error.
func Read(path string) (Metadata, error) {
	meta, err := readAllMetadata(path)
	if err != nil {
		return Metadata{}, err
	}

	var m Metadata
	if arch, ok := meta["general.architecture"].(string); ok {
		if n, ok := asContextLength(meta[arch+".context_length"]); ok {
			m.ContextLength = n
		}
	}
	if m.ContextLength == 0 {
		// Fallback for a model whose "general.architecture" is missing or
		// doesn't match its own "<x>.context_length" key exactly (seen on
		// some custom/finetuned conversions) — scan every key by suffix
		// instead of giving up.
		for k, v := range meta {
			if strings.HasSuffix(k, ".context_length") {
				if n, ok := asContextLength(v); ok {
					m.ContextLength = n
					break
				}
			}
		}
	}

	for k, v := range meta {
		if !strings.HasSuffix(k, "chat_template") {
			continue
		}
		if tmpl, ok := v.(string); ok && strings.Contains(strings.ToLower(tmpl), "tool_calls") {
			m.SupportsTools = true
			break
		}
	}

	return m, nil
}

// readAllMetadata parses a GGUF file's header + metadata key-value section
// into a plain map, without interpreting any of it — Read (and any future
// caller needing another metadata key) derives specific answers from this.
func readAllMetadata(path string) (map[string]any, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 64*1024)

	var gotMagic, version uint32
	if err := binary.Read(r, binary.LittleEndian, &gotMagic); err != nil {
		return nil, fmt.Errorf("gguf: read magic: %w", err)
	}
	if gotMagic != magic {
		return nil, fmt.Errorf("gguf: not a GGUF file (bad magic)")
	}
	if err := binary.Read(r, binary.LittleEndian, &version); err != nil {
		return nil, fmt.Errorf("gguf: read version: %w", err)
	}
	if version < 2 {
		// v1 used 32-bit tensor/kv counts and hasn't been produced by any
		// current tool in years — not worth a second code path for a format
		// nothing writes anymore.
		return nil, fmt.Errorf("gguf: unsupported GGUF version %d", version)
	}

	var tensorCount, kvCount uint64
	if err := binary.Read(r, binary.LittleEndian, &tensorCount); err != nil {
		return nil, fmt.Errorf("gguf: read tensor count: %w", err)
	}
	if err := binary.Read(r, binary.LittleEndian, &kvCount); err != nil {
		return nil, fmt.Errorf("gguf: read kv count: %w", err)
	}

	meta := make(map[string]any, kvCount)
	for i := uint64(0); i < kvCount; i++ {
		key, err := readString(r)
		if err != nil {
			return nil, fmt.Errorf("gguf: read key %d: %w", i, err)
		}
		val, err := readValue(r)
		if err != nil {
			return nil, fmt.Errorf("gguf: read value for %q: %w", key, err)
		}
		meta[key] = val
	}
	return meta, nil
}

func asContextLength(v any) (int, bool) {
	switch n := v.(type) {
	case uint32:
		return int(n), n > 0
	case uint64:
		return int(n), n > 0
	case int32:
		return int(n), n > 0
	case int64:
		return int(n), n > 0
	}
	return 0, false
}

func readString(r io.Reader) (string, error) {
	var length uint64
	if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
		return "", err
	}
	buf := make([]byte, length)
	if _, err := io.ReadFull(r, buf); err != nil {
		return "", err
	}
	return string(buf), nil
}

// readValue reads one metadata value, dispatching on its own leading
// uint32 type tag. Array contents are walked (never returned — nothing this
// package needs lives inside an array) so the reader stays correctly
// aligned for the next key; a fixed-size array element is skipped in one
// bulk read rather than one binary.Read per element, since GGUF vocabulary
// arrays alone commonly run into the hundreds of thousands of entries.
func readValue(r *bufio.Reader) (any, error) {
	var valType uint32
	if err := binary.Read(r, binary.LittleEndian, &valType); err != nil {
		return nil, err
	}

	switch valType {
	case typeString:
		return readString(r)
	case typeArray:
		var elemType uint32
		if err := binary.Read(r, binary.LittleEndian, &elemType); err != nil {
			return nil, err
		}
		var length uint64
		if err := binary.Read(r, binary.LittleEndian, &length); err != nil {
			return nil, err
		}
		switch elemType {
		case typeString:
			for i := uint64(0); i < length; i++ {
				if _, err := readString(r); err != nil {
					return nil, err
				}
			}
		case typeArray:
			return nil, fmt.Errorf("gguf: nested arrays are not valid GGUF")
		default:
			size, ok := fixedSizes[elemType]
			if !ok {
				return nil, fmt.Errorf("gguf: unknown array element type %d", elemType)
			}
			if _, err := io.CopyN(io.Discard, r, int64(length)*size); err != nil {
				return nil, err
			}
		}
		return nil, nil
	}

	size, ok := fixedSizes[valType]
	if !ok {
		return nil, fmt.Errorf("gguf: unknown value type %d", valType)
	}
	buf := make([]byte, size)
	if _, err := io.ReadFull(r, buf); err != nil {
		return nil, err
	}
	switch valType {
	case typeUint8:
		return buf[0], nil
	case typeInt8:
		return int8(buf[0]), nil
	case typeBool:
		return buf[0] != 0, nil
	case typeUint16:
		return binary.LittleEndian.Uint16(buf), nil
	case typeInt16:
		return int16(binary.LittleEndian.Uint16(buf)), nil
	case typeUint32:
		return binary.LittleEndian.Uint32(buf), nil
	case typeInt32:
		return int32(binary.LittleEndian.Uint32(buf)), nil
	case typeUint64:
		return binary.LittleEndian.Uint64(buf), nil
	case typeInt64:
		return int64(binary.LittleEndian.Uint64(buf)), nil
	}
	// FLOAT32/FLOAT64: bit pattern is read and discarded correctly above,
	// nothing in this package ever needs a float metadata value.
	return nil, nil
}
