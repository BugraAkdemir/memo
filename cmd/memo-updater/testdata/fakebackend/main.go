// SPDX-License-Identifier: AGPL-3.0-or-later

// fakebackend stands in for memo-backend in the updater's end-to-end test: it
// serves /api/version on $FAKE_PORT, reports the version compiled into it, and
// exits at once when built with -X main.crash=1 (a release that won't start).
package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
)

var (
	version = "V0.0.0"
	// crash, set by -ldflags "-X main.crash=1", makes this build exit at once.
	crash = ""
)

func main() {
	exe, _ := os.Executable()
	if crash != "" {
		fmt.Fprintln(os.Stderr, "fakebackend: built to crash, exiting")
		os.Exit(1)
	}
	_ = os.WriteFile(filepath.Join(filepath.Dir(exe), "backend.pid"), []byte(fmt.Sprint(os.Getpid())), 0o644)
	http.HandleFunc("/api/version", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{"version":%q}`, version)
	})
	_ = http.ListenAndServe("127.0.0.1:"+os.Getenv("FAKE_PORT"), nil)
}
