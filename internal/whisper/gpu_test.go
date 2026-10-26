// SPDX-License-Identifier: AGPL-3.0-or-later

package whisper

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// whisper-server lives in the same engine folder as llama-server and loads every
// ggml backend library beside it. Since the folder may now carry a Vulkan plugin
// for llama, whisper would silently move from the CPU to the GPU: faster on a
// synthetic clip, but its accuracy on real speech was not checked, so it keeps
// the behaviour it always had.
func TestHostOnlyArgs(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("the GPU plugin is only shipped for Linux")
	}
	withPlugin, plain := t.TempDir(), t.TempDir()
	for _, d := range []string{withPlugin, plain} {
		if err := os.WriteFile(filepath.Join(d, "whisper-server"), []byte("x"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(d, "libggml-cpu-haswell.so"), nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(withPlugin, "libggml-vulkan.so"), nil, 0o644); err != nil {
		t.Fatal(err)
	}

	if got := hostOnlyArgs(filepath.Join(withPlugin, "whisper-server")); !reflect.DeepEqual(got, []string{"--no-gpu"}) {
		t.Fatalf("with a GPU backend beside it: got %v, want --no-gpu", got)
	}
	if got := hostOnlyArgs(filepath.Join(plain, "whisper-server")); got != nil {
		t.Fatalf("with only CPU backends: got %v, want none (behaviour unchanged)", got)
	}
}
