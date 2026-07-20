//go:build windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
)

// killMemoProcesses is the Windows counterpart of the Unix sweep: taskkill by
// image name for each binary Memo spawns.
//
// memo.exe is the delicate one — `memo --kill` IS a memo.exe, so an
// unfiltered taskkill would terminate this process partway through the sweep.
// The "PID ne <self>" filter excludes it; every other image name can never be
// this process, so they need no filter.
//
// /T takes each match's child tree down with it, which matters for a backend
// that spawned llama-server before being killed. Failures are ignored on
// purpose: taskkill exits non-zero simply because nothing matched.
func killMemoProcesses() {
	self := strconv.Itoa(os.Getpid())

	// Matched by image name, which on Windows is the executable's own file
	// name — so unlike the Unix pkill -f pass this cannot match a process
	// that merely mentions the name in its arguments. rpc-server.exe is
	// nonetheless left out: the name is generic enough to belong to
	// unrelated software, and it is covered by the port pass anyway.
	targets := []struct {
		label string
		image string
		args  []string
	}{
		{"backend", "memo.exe", []string{"/FI", "PID ne " + self}},
		{"llama-server", "llama-server.exe", nil},
		{"whisper-server", "whisper-server.exe", nil},
		{"masaüstü / desktop app", "memo_flutter.exe", nil},
	}

	for _, t := range targets {
		args := append([]string{"/F", "/T", "/IM", t.image}, t.args...)
		if err := exec.Command("taskkill", args...).Run(); err == nil {
			fmt.Printf("  %s: durduruldu / stopped\n", t.label)
		}
	}
}
