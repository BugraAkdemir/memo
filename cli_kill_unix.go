//go:build !windows

package main

import (
	"fmt"
	"os/exec"
)

// memoProcessPatterns are the pkill -f patterns for Memo-owned processes that
// aren't (or aren't yet) listening on a known port — a model server still
// loading, the desktop app, a backend whose port bind failed.
//
// Every pattern is deliberately anchored on a path fragment or a flag, never
// a bare binary name, because pkill -f matches against the WHOLE command line
// of every process on the machine. A bare "llama-server" pattern also matches
// `tail -f llama-server.log`, an editor with that file open, or a shell whose
// command line merely mentions it — this was not theoretical, a bare pattern
// killed the shell it was being tested from. "rpc-server" is worse still:
// generic enough to belong to unrelated software.
//
// Anchoring on "binaries/" (where Memo keeps every bundled executable) and on
// "--headless" (a flag the `memo --kill` invocation itself never carries, so
// this can't kill the process doing the killing) keeps each match specific to
// this installation.
var memoProcessPatterns = []struct {
	label   string
	pattern string
}{
	{"backend", "memo --headless"},
	{"llama-server", "binaries/.*llama-server"},
	{"whisper-server", "binaries/.*whisper-server"},
	{"rpc-server", "binaries/.*rpc-server"},
	{"masaüstü / desktop app", "memo_flutter"},
}

// killMemoProcesses is the last and least precise of runKill's three passes;
// the graceful shutdown and the port sweep before it do the load-bearing
// work. Best-effort by design: pkill exits non-zero simply because nothing
// matched, and on the rare system without procps it is missing entirely.
func killMemoProcesses() {
	for _, p := range memoProcessPatterns {
		if err := exec.Command("pkill", "-f", p.pattern).Run(); err == nil {
			fmt.Printf("  %s: durduruldu / stopped\n", p.label)
		}
	}
}
