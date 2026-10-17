// SPDX-License-Identifier: AGPL-3.0-or-later

package llama

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// engine puts a fake llama-server for flavor under root/binaries/<os>/<flavor>.
func engine(t *testing.T, root, flavor string) string {
	t.Helper()
	p := filepath.Join(root, "binaries", runtime.GOOS, flavor, llamaServerBinary())
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

// withOverride points the downloaded-engine tree at a fresh dir and makes the
// working directory the (only) bundled base.
func withOverride(t *testing.T) (over, bundled string) {
	t.Helper()
	over, bundled = t.TempDir(), t.TempDir()
	t.Chdir(bundled)
	prev := overrideRootFn
	overrideRootFn = func() string { return over }
	t.Cleanup(func() { overrideRootFn = prev })
	return over, bundled
}

func samePath(t *testing.T, got, want string) bool {
	t.Helper()
	// EvalSymlinks keeps a relative path relative; compare absolute real paths.
	ga, _ := filepath.Abs(got)
	wa, _ := filepath.Abs(want)
	a, err1 := filepath.EvalSymlinks(ga)
	b, err2 := filepath.EvalSymlinks(wa)
	return err1 == nil && err2 == nil && a == b
}

func TestResolveBinary_ExplicitModePrefersTheDownloadedEngine(t *testing.T) {
	over, bundled := withOverride(t)
	engine(t, bundled, "cpu")
	want := engine(t, over, "cpu")
	got, err := resolveBinary("", "cpu")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("got %q, %v; want the downloaded %q", got, err, want)
	}
}

func TestResolveBinary_ExplicitModeFallsBackToBundledWhenNothingWasDownloadedForIt(t *testing.T) {
	over, bundled := withOverride(t)
	want := engine(t, bundled, "amd")
	engine(t, over, "cpu") // a different flavor was updated
	got, err := resolveBinary("", "amd")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("got %q, %v; want the bundled amd %q", got, err, want)
	}
}

// The property that matters most: auto mode still picks the flavor the bundled
// tree would have picked. A downloaded cpu engine must not move a machine that
// runs the amd engine over to the cpu one.
func TestResolveBinary_AutoNeverSwitchesFlavorBecauseOfADownload(t *testing.T) {
	over, bundled := withOverride(t)
	wantAMD := engine(t, bundled, "amd")
	engine(t, bundled, "cpu")
	engine(t, over, "cpu") // only cpu was downloaded
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, wantAMD) {
		t.Fatalf("auto picked %q, %v; want the bundled amd %q", got, err, wantAMD)
	}
}

func TestResolveBinary_AutoUsesTheDownloadedCopyOfTheFlavorItWouldHavePicked(t *testing.T) {
	over, bundled := withOverride(t)
	engine(t, bundled, "amd")
	engine(t, bundled, "cpu")
	want := engine(t, over, "amd")
	got, err := resolveBinary("", "auto")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("auto picked %q, %v; want the downloaded amd %q", got, err, want)
	}
}

func TestResolveBinary_AutoWithNothingBundledUsesADownloadedEngine(t *testing.T) {
	over, _ := withOverride(t)
	want := engine(t, over, "cpu")
	got, err := resolveBinary("", "")
	if err != nil || !samePath(t, got, want) {
		t.Fatalf("got %q, %v; want %q", got, err, want)
	}
}

func TestBinarySearchBases_OverrideFirstThenBundled(t *testing.T) {
	over, _ := withOverride(t)
	bases := binarySearchBases()
	if len(bases) < 2 || bases[0] != over || bases[1] != "." {
		t.Fatalf("bases = %v; want [%s . …]", bases, over)
	}
	for _, b := range bundledSearchBases() {
		if b == over {
			t.Fatalf("the override root leaked into the bundled bases: %v", bundledSearchBases())
		}
	}
}
