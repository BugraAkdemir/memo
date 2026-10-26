// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"sync"
	"testing"
	"time"
)

func TestClassifyStartError_MapsToTheFixedStartFailureCodes(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{errors.New("llama: start failed: fork/exec /x/llama-server: permission denied"), "permission"},
		{errors.New("llama: start failed: fork/exec /x/llama-server: exec format error"), "unknown"},
		{errors.New("listen tcp 127.0.0.1:8081: bind: address already in use"), "port"},
		{errors.New("Llama.cpp motoru (llama-server) bulunamadı. (Hata: exec: no such file)"), "missing_file"},
		{errors.New("llama-server binary not found — install llama.cpp"), "missing_file"},
		{errors.New("llama: start failed: fork/exec /x/llama-server: no such file or directory"), "missing_file"},
		{errors.New("llama: server process exited during startup"), "crash_at_load"},
		{fmt.Errorf("Model yükleme zaman aşımına uğradı (3 dk). (Hata: %w)", errors.New("llama: server process exited during startup")), "crash_at_load"},
		{errors.New("llama: server failed to become ready within 3m0s"), "timeout"},
		{errors.New("context deadline exceeded"), "timeout"},
		{errors.New("cudaMalloc failed: out of memory"), "oom"},
		{errors.New("fork/exec: cannot allocate memory"), "oom"},
		{errors.New("something nobody anticipated"), "unknown"},
		{nil, ""},
	}
	for _, c := range cases {
		if got := ClassifyStartError(c.err); got != c.want {
			t.Errorf("ClassifyStartError(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}

func TestClassifyExit_DistinguishesNonzeroExitFromSignals(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("signals are not modelled on Windows")
	}
	exitErr := func(script string) error {
		return exec.Command("sh", "-c", script).Run()
	}
	if got := ClassifyExit(exitErr("exit 3")); got != "exit_nonzero" {
		t.Errorf("exit 3 -> %q, want exit_nonzero", got)
	}
	if got := ClassifyExit(exitErr("kill -KILL $$")); got != "oom_killed" {
		t.Errorf("SIGKILL -> %q, want oom_killed", got)
	}
	if got := ClassifyExit(exitErr("kill -SEGV $$")); got != "signal" {
		t.Errorf("SIGSEGV -> %q, want signal", got)
	}
	if got := ClassifyExit(nil); got != "unknown" {
		t.Errorf("exit 0 -> %q, want unknown", got)
	}
	if got := ClassifyExit(errors.New("x")); got != "unknown" {
		t.Errorf("non-exit error -> %q, want unknown", got)
	}
}

func TestServer_ExitHookFiresOnlyForAnUnexpectedExit(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses sh")
	}
	var mu sync.Mutex
	var got []error
	fired := make(chan struct{}, 4)
	s := NewServer(8199, 512)
	s.SetExitHook(func(err error) {
		mu.Lock()
		got = append(got, err)
		mu.Unlock()
		fired <- struct{}{}
	})

	// Unexpected: the process dies on its own.
	cmd := exec.Command("sh", "-c", "exit 7")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waitDone := make(chan struct{})
	s.mu.Lock()
	s.cmd, s.waitDone = cmd, waitDone
	s.mu.Unlock()
	s.monitor(cmd, waitDone)
	select {
	case <-fired:
	case <-time.After(2 * time.Second):
		t.Fatal("hook did not fire for an unexpected exit")
	}

	// Expected: Stop() marked it as stopping.
	cmd2 := exec.Command("sh", "-c", "exit 0")
	if err := cmd2.Start(); err != nil {
		t.Fatal(err)
	}
	waitDone2 := make(chan struct{})
	s.mu.Lock()
	s.cmd, s.waitDone, s.stopping = cmd2, waitDone2, true
	s.mu.Unlock()
	s.monitor(cmd2, waitDone2)
	select {
	case <-fired:
		t.Fatal("hook fired for an intentional stop")
	case <-time.After(200 * time.Millisecond):
	}
	mu.Lock()
	defer mu.Unlock()
	if len(got) != 1 || ClassifyExit(got[0]) != "exit_nonzero" {
		t.Fatalf("hook calls = %v", got)
	}
}
