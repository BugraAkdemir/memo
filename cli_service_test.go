package main

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestBuildUnitFile_PlainMode(t *testing.T) {
	unit := buildUnitFile("/usr/local/bin/memo", 8090, false)
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/memo --headless --port 8090\n") {
		t.Errorf("unexpected ExecStart line in unit file:\n%s", unit)
	}
	if strings.Contains(unit, "--lan") {
		t.Error("expected --lan to be absent when lan=false")
	}
}

func TestBuildUnitFile_LanMode(t *testing.T) {
	unit := buildUnitFile("/usr/local/bin/memo", 9000, true)
	if !strings.Contains(unit, "ExecStart=/usr/local/bin/memo --headless --port 9000 --lan\n") {
		t.Errorf("unexpected ExecStart line in unit file:\n%s", unit)
	}
}

func TestBuildUnitFile_HasRestartOnFailure(t *testing.T) {
	unit := buildUnitFile("/usr/local/bin/memo", 8090, false)
	if !strings.Contains(unit, "Restart=on-failure") {
		t.Error("expected Restart=on-failure so a crashed backend comes back up on its own")
	}
}

// TestBuildUnitFile_SetsMemoDataDir guards a real bug found live on an RPi
// self-hosted install: without this, systemd --user's default working
// directory ($HOME, since WorkingDirectory= is never set) made
// config.DataDir()'s relative "data" fallback resolve to $HOME/data instead
// of ~/.memo/data — every install/update/uninstall script manages the
// latter, so the account/memory/chat history that was supposed to be wiped
// by uninstall-selfhosted.sh silently survived in the former, invisible to
// every script. binPath is always <MEMO_HOME>/bin/memo (the CLI wrapper
// execs that real path before `service install` ever runs), so
// MEMO_DATA_DIR must be its grandparent directory + "/data".
func TestBuildUnitFile_SetsMemoDataDir(t *testing.T) {
	unit := buildUnitFile("/home/pi/.memo/bin/memo", 8090, true)
	if !strings.Contains(unit, "Environment=MEMO_DATA_DIR=/home/pi/.memo/data\n") {
		t.Errorf("unit file missing correct MEMO_DATA_DIR:\n%s", unit)
	}
}

func TestSystemdUnitPath_UnderConfigSystemdUser(t *testing.T) {
	path, err := systemdUnitPath()
	if err != nil {
		t.Fatalf("systemdUnitPath: %v", err)
	}
	if !strings.HasSuffix(path, "/.config/systemd/user/memo.service") {
		t.Errorf("systemdUnitPath = %q, want a path ending in /.config/systemd/user/memo.service", path)
	}
}

func TestPrintServiceUsage_MentionsRestartAndUserFlag(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w
	printServiceUsage()
	w.Close()
	os.Stderr = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	// BUG-ONB2: the usage text is the one place every user actually reads
	// before trying to restart the service — it must list the subcommand
	// and warn that a bare `systemctl restart memo` (no --user) fails.
	if !strings.Contains(out, "memo service restart") {
		t.Errorf("printServiceUsage() output missing 'memo service restart':\n%s", out)
	}
	if !strings.Contains(out, "--user") {
		t.Errorf("printServiceUsage() output missing a --user mention:\n%s", out)
	}
}
