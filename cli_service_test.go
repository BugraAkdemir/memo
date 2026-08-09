package main

import (
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

func TestSystemdUnitPath_UnderConfigSystemdUser(t *testing.T) {
	path, err := systemdUnitPath()
	if err != nil {
		t.Fatalf("systemdUnitPath: %v", err)
	}
	if !strings.HasSuffix(path, "/.config/systemd/user/memo.service") {
		t.Errorf("systemdUnitPath = %q, want a path ending in /.config/systemd/user/memo.service", path)
	}
}
