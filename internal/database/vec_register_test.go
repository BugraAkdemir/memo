package database

import (
	"path/filepath"
	"slices"
	"testing"
)

// TestSearchRootsFrom_IncludesParentOfExeDir is a regression test: the
// installed CLI binary lives at ~/.memo/bin/memo, one level deeper than the
// bundled binaries/ tree it ships next to (~/.memo/binaries/vec0.so). Before
// this fix, only the exe's own directory and the CWD were searched, so the
// sqlite-vec extension was never found when running as the CLI — memory
// save/retrieve failed with "no such module: vec0" — while the GUI binary,
// which sits flush with binaries/, worked.
func TestSearchRootsFrom_IncludesParentOfExeDir(t *testing.T) {
	exePath := filepath.Join("/home/user/.memo/bin", "memo")

	dirs := searchRootsFrom(exePath, "")

	wantExeDir := filepath.Join("/home/user/.memo/bin")
	wantParent := filepath.Join("/home/user/.memo")
	if !slices.Contains(dirs, wantExeDir) {
		t.Errorf("dirs = %v, want to contain exe dir %q", dirs, wantExeDir)
	}
	if !slices.Contains(dirs, wantParent) {
		t.Errorf("dirs = %v, want to contain parent dir %q", dirs, wantParent)
	}
}

func TestVecFileCandidates_DarwinPrefersArchSpecific(t *testing.T) {
	arm := vecFileCandidates("darwin", "arm64", ".dylib")
	if len(arm) == 0 || arm[0] != "binaries/darwin/cpu/vec0-arm64.dylib" {
		t.Fatalf("darwin/arm64 first candidate = %v, want binaries/darwin/cpu/vec0-arm64.dylib", arm)
	}
	amd := vecFileCandidates("darwin", "amd64", ".dylib")
	if len(amd) == 0 || amd[0] != "binaries/darwin/cpu/vec0-x86_64.dylib" {
		t.Fatalf("darwin/amd64 first candidate = %v, want binaries/darwin/cpu/vec0-x86_64.dylib", amd)
	}
}
