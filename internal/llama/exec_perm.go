// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"os"
	"runtime"

	"memo/internal/logx"
)

// EnsureExecutable adds the execute bits to a regular file that has none and
// reports whether it changed anything. Best effort: a read-only install or a
// file owned by someone else just stays as it is and the launch fails with the
// ordinary error.
//
// Why Memo has to do this itself: engine files travel through R2 (S3 keeps no
// POSIX mode, so every `rclone copy` back out writes 0644), through zip/tar
// archives that "don't reliably preserve the exec bit", and through hand-copied
// folders. Each launcher script was patched to `chmod +x llama-server*` after
// the fact (52abd6f1, 84e59f2d, 44b01e98...) and each patch missed another
// path or another file: whisper-server and rpc-server are still 0644 in an
// installed copy. The kernel's answer is `fork/exec ...: permission denied`,
// which the UI used to blame on RAM (e367c29f). Fixing the file at the moment
// Memo is about to run it covers every path at once, like cliproxy already does.
func EnsureExecutable(path string) bool {
	if runtime.GOOS == "windows" {
		return false
	}
	fi, err := os.Stat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	perm := fi.Mode().Perm()
	if perm&0o111 != 0 {
		return false
	}
	// Execute wherever read is already allowed: 0644 -> 0755, 0600 -> 0700.
	if err := os.Chmod(path, perm|(perm&0o444)>>2); err != nil {
		logx.Printf("engine: %s is not executable and could not be fixed: %v", path, err)
		return false
	}
	logx.Printf("engine: %s was not executable (archives and R2 drop the mode); fixed", path)
	return true
}

// ensureEngineExecutable applies EnsureExecutable to a file in Memo's own engine
// layout (binaries/<os>/<flavor>/…) only. A custom binary_path or a program
// found on PATH is the user's, and is never re-permissioned.
func ensureEngineExecutable(bin string) {
	if FlavorOfPath(bin) != "" {
		EnsureExecutable(bin)
	}
}
