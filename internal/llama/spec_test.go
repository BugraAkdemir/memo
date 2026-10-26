// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"reflect"
	"testing"
)

func TestSpeculativeArgs(t *testing.T) {
	requireLinux(t)
	t.Setenv("MEMO_LLAMA_SPECULATIVE", "")
	accepting := probeBinary(t, t.TempDir(), "cpu", noGPU, true)
	rejecting := probeBinary(t, t.TempDir(), "cpu", noGPU, false) // an older build: unknown flag
	unrunnable := withStubBackendLib(t, engine(t, t.TempDir(), "cpu"))

	tests := []struct {
		name      string
		bin       string
		embedding bool
		mmproj    bool
		rpc       bool
		want      []string
	}{
		{"a chat model on a build that knows the flags gets them", accepting, false, false, false, specNgramArgs},
		{"the embedding sidecar never drafts", accepting, true, false, false, nil},
		{"a vision model is left alone: not verified with a projector", accepting, false, true, false, nil},
		{"the swarm coordinator is left alone", accepting, false, false, true, nil},
		{"a build that rejects the flags is never given them", rejecting, false, false, false, nil},
		{"a binary that cannot run gives no verdict and gets nothing", unrunnable, false, false, false, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := speculativeArgs(tt.bin, tt.embedding, tt.mmproj, tt.rpc); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("speculativeArgs() = %v, want %v", got, tt.want)
			}
		})
	}
}

// Escape hatch: a model or build that misbehaves with drafting can be run
// without it, with no config change and no rebuild.
func TestSpeculativeArgs_CanBeSwitchedOff(t *testing.T) {
	requireLinux(t)
	bin := probeBinary(t, t.TempDir(), "cpu", noGPU, true)
	t.Setenv("MEMO_LLAMA_SPECULATIVE", "off")
	if got := speculativeArgs(bin, false, false, false); got != nil {
		t.Fatalf("got %v with MEMO_LLAMA_SPECULATIVE=off, want none", got)
	}
}
