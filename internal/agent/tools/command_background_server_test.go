package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// freePort grabs a port the OS says is free. Racy in principle, fine for a
// test that needs to tell a child process which port to bind.
func freePort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("cannot find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
}

// TestRunCommand_StartsARealServerInBackground is the end-to-end version of
// the production failure: a model testing the app it just wrote starts a real
// HTTP server with "&". Three things have to hold, and each one broke at some
// point while fixing this:
//
//  1. RunCommand returns — the original bug hung here forever, because the
//     server inherited the pipes os/exec creates for a non-file cmd.Stdout.
//  2. the server is still serving afterwards — the first attempt (WaitDelay
//     closing those pipes) unblocked Wait but then killed the server with
//     SIGPIPE on its first log write.
//  3. the tool still works for the next call.
func TestRunCommand_StartsARealServerInBackground(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "blog.py")
	if err := os.WriteFile(script, []byte(`
import sys, http.server, socketserver
port = int(sys.argv[1]) if len(sys.argv) > 1 else 8080
with socketserver.TCPServer(("127.0.0.1", port), http.server.SimpleHTTPRequestHandler) as httpd:
    httpd.serve_forever()
`), 0o644); err != nil {
		t.Fatal(err)
	}

	port := freePort(t)
	// The shape from the 04:33/04:53 log lines: start the app in the
	// background, give it a moment, print something.
	cmdStr := fmt.Sprintf("python3 blog.py %d &\nsleep 2\necho \"Server started\"", port)
	args, _ := json.Marshal(RunCommandArgs{Command: cmdStr})

	type res struct {
		out string
		err error
	}
	ch := make(chan res, 1)
	start := time.Now()
	go func() {
		out, err := RunCommand(context.Background(), args, dir, nil)
		ch <- res{out, err}
	}()

	var r res
	select {
	case r = <-ch:
	case <-time.After(30 * time.Second):
		t.Fatal("HUNG: run_command never returned for a backgrounded server (the production bug)")
	}
	elapsed := time.Since(start)
	t.Logf("run_command returned after %s, err=%v", elapsed.Round(time.Millisecond), r.err)
	t.Logf("output: %s", strings.TrimSpace(r.out))

	if r.err != nil {
		t.Fatalf("unexpected error: %v", r.err)
	}
	if elapsed > 15*time.Second {
		t.Fatalf("returned, but took %s — too slow to be usable", elapsed)
	}
	if !strings.Contains(r.out, "Server started") {
		t.Errorf("the command's own stdout was lost: %q", r.out)
	}

	// The server must still be alive — WaitDelay closes our pipes, it must not
	// kill what the user asked to start.
	defer func() {
		kill, _ := json.Marshal(RunCommandArgs{Command: fmt.Sprintf(`pkill -f "blog.py %d"`, port)})
		if _, err := RunCommand(context.Background(), kill, dir, nil); err != nil {
			t.Logf("cleanup: %v", err)
		}
	}()

	var up bool
	for i := 0; i < 40; i++ {
		if resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/", port)); err == nil {
			resp.Body.Close()
			up = true
			break
		}
		time.Sleep(250 * time.Millisecond)
	}
	if !up {
		t.Fatal("the backgrounded server is not reachable — WaitDelay must not kill it")
	}
	t.Logf("server on :%d is up and serving after run_command returned", port)

	// And the tool still works for the next call.
	next, _ := json.Marshal(RunCommandArgs{Command: `echo still-working`})
	out2, err := RunCommand(context.Background(), next, dir, nil)
	if err != nil || !strings.Contains(out2, "still-working") {
		t.Fatalf("follow-up command broken: out=%q err=%v", out2, err)
	}
}
