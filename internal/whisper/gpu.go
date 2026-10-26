// SPDX-License-Identifier: AGPL-3.0-or-later

package whisper

import (
	"runtime"

	"memo/internal/llama"
)

// hostOnlyArgs keeps whisper-server on the CPU when a ggml GPU backend library
// sits beside it. whisper-server shares its engine folder with llama-server and
// loads every backend library it finds there; a Vulkan plugin placed in the
// folder for llama would otherwise move speech-to-text to the GPU as a side
// effect. That is faster (a 4 s synthetic clip: 2.3 s -> 1.0 s, same text) but
// its accuracy on real speech was not checked, so it keeps the behaviour it
// always had. Linux only: that is the only place the plugin is shipped.
func hostOnlyArgs(bin string) []string {
	if runtime.GOOS == "linux" && llama.HasGPUBackendLib(bin) {
		return []string{"--no-gpu"}
	}
	return nil
}
