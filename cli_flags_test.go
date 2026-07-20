package main

import (
	"net"
	"testing"
)

// memoPorts drives --kill's port sweep, and --kill kills whatever it finds on
// those ports. A duplicate is harmless but a missing or bogus entry is not:
// a zero would be passed to KillByPort, and a forgotten port is an orphan
// left holding it — the exact bug --kill exists to clean up.
func TestMemoPorts_IncludesBackendAndStaysSane(t *testing.T) {
	const backendPort = 18990
	ports := memoPorts(backendPort)

	if len(ports) < 2 {
		t.Fatalf("memoPorts returned %v — expected the backend port plus the model/whisper ports", ports)
	}

	seen := map[int]bool{}
	foundBackend := false
	for _, p := range ports {
		if p <= 0 {
			t.Errorf("memoPorts returned a non-positive port %d in %v", p, ports)
		}
		if seen[p] {
			t.Errorf("memoPorts returned duplicate port %d in %v", p, ports)
		}
		seen[p] = true
		if p == backendPort {
			foundBackend = true
		}
	}
	if !foundBackend {
		t.Errorf("memoPorts(%d) = %v, missing the backend port itself", backendPort, ports)
	}
}

// The backend port is passed in separately from the config-derived ones, so a
// user running the backend on a port that is already in that list must not
// produce a duplicate entry.
func TestMemoPorts_NoDuplicateWhenBackendPortIsAlsoAConfiguredPort(t *testing.T) {
	all := memoPorts(18990)
	if len(all) < 2 {
		t.Skip("not enough configured ports to exercise the overlap")
	}
	overlapping := all[1] // a config-derived port

	ports := memoPorts(overlapping)
	seen := map[int]bool{}
	for _, p := range ports {
		if seen[p] {
			t.Errorf("memoPorts(%d) duplicated port %d: %v", overlapping, p, ports)
		}
		seen[p] = true
	}
}

func TestPortInUse(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	port := ln.Addr().(*net.TCPAddr).Port

	if !portInUse(port) {
		t.Errorf("portInUse(%d) = false while a listener is open on it", port)
	}

	ln.Close()
	if portInUse(port) {
		t.Errorf("portInUse(%d) = true after the listener was closed", port)
	}
}

// Every standalone command must be in standaloneCommandFlags, or its output
// gets interleaved with the internal INFO logging main() otherwise leaves on.
func TestStandaloneCommandFlags_CoversEveryDeclaredCommand(t *testing.T) {
	for _, name := range []string{
		"help", "h", "version", "v", "status", "kill",
		"update", "gui", "github", "bugreport", "bugrep", "docs",
	} {
		if !standaloneCommandFlags[name] {
			t.Errorf("flag %q is a standalone command but missing from standaloneCommandFlags", name)
		}
	}
	// Flags that start a real session must NOT be in the set — silencing logs
	// for those would hide the backend's own output.
	for _, name := range []string{"port", "headless", "p", "chat", "auto-shutdown", "auto-allow"} {
		if standaloneCommandFlags[name] {
			t.Errorf("flag %q starts a session but is listed as a standalone command", name)
		}
	}
}
