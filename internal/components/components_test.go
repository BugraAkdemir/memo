// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadMarker(t *testing.T) {
	dir := t.TempDir()
	cases := []struct {
		name, content string
		want          string
		wantErr       bool
	}{
		{"plain", "b9441", "b9441", false},
		{"trailing newline", "1.8.6\n", "1.8.6", false},
		{"crlf", "v8.0.25\r\n", "v8.0.25", false},
		{"empty", "", "", true},
		{"only whitespace", " \n", "", true},
		{"two lines", "b9441\nb1", "", true},
		{"path traversal", "../../etc", "", true},
		{"space inside", "b94 41", "", true},
		{"too long", strings.Repeat("1", 33), "", true},
		{"leading dot", ".1", "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			write(t, filepath.Join(dir, "LLAMA_VERSION"), c.content)
			got, err := ReadMarker(dir, Llama)
			if (err != nil) != c.wantErr || got != c.want {
				t.Fatalf("ReadMarker(%q) = %q, %v; want %q, err=%v", c.content, got, err, c.want, c.wantErr)
			}
		})
	}
}

func TestReadMarker_MissingFileIsNotAnError(t *testing.T) {
	v, err := ReadMarker(t.TempDir(), Whisper)
	if v != "" || err != nil {
		t.Fatalf("missing marker = %q, %v; want empty, nil", v, err)
	}
}

func TestReadMarker_UnknownComponent(t *testing.T) {
	if _, err := ReadMarker(t.TempDir(), Name("nope")); err == nil {
		t.Fatal("unknown component accepted")
	}
}

func TestDetect_OverrideBeatsBundledOnlyWithAValidMarker(t *testing.T) {
	bundled, over := t.TempDir(), t.TempDir()
	write(t, filepath.Join(bundled, "binaries", "linux", "cpu", "LLAMA_VERSION"), "b9441\n")
	write(t, filepath.Join(over, "binaries", "linux", "cpu", "LLAMA_VERSION"), "b11456\n")

	got := Detect(Llama, "linux", "cpu", over, []string{bundled})
	if got.Version != "b11456" || !got.Override {
		t.Fatalf("override with marker should win: %+v", got)
	}

	// A half-written override (garbage marker) must be ignored, not trusted.
	write(t, filepath.Join(over, "binaries", "linux", "cpu", "LLAMA_VERSION"), "garbage here")
	got = Detect(Llama, "linux", "cpu", over, []string{bundled})
	if got.Version != "b9441" || got.Override {
		t.Fatalf("bad override marker should fall back to bundled: %+v", got)
	}

	// An override folder with no marker at all is ignored too.
	if err := os.Remove(filepath.Join(over, "binaries", "linux", "cpu", "LLAMA_VERSION")); err != nil {
		t.Fatal(err)
	}
	got = Detect(Llama, "linux", "cpu", over, []string{bundled})
	if got.Version != "b9441" || got.Override {
		t.Fatalf("override without marker should fall back to bundled: %+v", got)
	}
}

func TestDetect_FirstExistingBundledRootDecides(t *testing.T) {
	first, second := t.TempDir(), t.TempDir()
	// First root has the folder but no marker (an install from before markers).
	if err := os.MkdirAll(filepath.Join(first, "binaries", "linux", "cpu"), 0o755); err != nil {
		t.Fatal(err)
	}
	write(t, filepath.Join(second, "binaries", "linux", "cpu", "LLAMA_VERSION"), "b9441")

	got := Detect(Llama, "linux", "cpu", "", []string{first, second})
	if !got.Found() || got.Known() || got.Version != "" {
		t.Fatalf("an unmarked folder must be found but unknown: %+v", got)
	}
	if got.Dir != filepath.Join(first, "binaries", "linux", "cpu") {
		t.Fatalf("wrong dir: %s", got.Dir)
	}
}

func TestDetect_Absent(t *testing.T) {
	got := Detect(Whisper, "linux", "cpu", t.TempDir(), []string{t.TempDir()})
	if got.Found() || got.Known() {
		t.Fatalf("nothing on disk, got %+v", got)
	}
	if Detect(Llama, "linux", "", "", nil).Found() {
		t.Fatal("empty folder name must find nothing")
	}
	if Detect(Name("x"), "linux", "cpu", "", nil).Found() {
		t.Fatal("unknown component must find nothing")
	}
}

func TestDetect_CLIProxyUsesTheExistingVERSIONFile(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "binaries", "linux", "cliproxy", "VERSION"), "v8.0.25\n")
	got := Detect(CLIProxy, "linux", "cliproxy", "", []string{root})
	if got.Version != "v8.0.25" {
		t.Fatalf("cliproxy: %+v", got)
	}
}

func TestOverrideRoot(t *testing.T) {
	if got := OverrideRoot("/d"); got != filepath.Join("/d", "components") {
		t.Fatalf("OverrideRoot = %s", got)
	}
}

func TestCompare(t *testing.T) {
	type tc struct {
		n       Name
		a, b    string
		want    int
		wantErr bool
	}
	for _, c := range []tc{
		{Llama, "b9441", "b11456", -1, false}, // numeric, not lexical ("b9441" > "b11456" as text)
		{Llama, "b11456", "b9441", 1, false},
		{Llama, "b9441", "b9441", 0, false},
		{Llama, "9441", "b9441", 0, true}, // missing b
		{Llama, "bx", "b1", 0, true},
		{Llama, "b-1", "b1", 0, true},
		{Whisper, "1.8.6", "1.10.0", -1, false}, // 10 > 8, not "1.10" < "1.8"
		{Whisper, "1.8.6", "1.8", 1, false},     // missing part reads as 0
		{Whisper, "1.8.0", "1.8", 0, false},
		{CLIProxy, "v8.0.25", "8.0.25", 0, false},
		{CLIProxy, "V8.0.26", "v8.0.25", 1, false},
		{CLIProxy, "v8.0.25", "latest", 0, true},
		{CLIProxy, "", "v1", 0, true},
		{CLIProxy, "v1..2", "v1", 0, true},
		{Name("x"), "1", "1", 0, true},
	} {
		got, err := Compare(c.n, c.a, c.b)
		if (err != nil) != c.wantErr || got != c.want {
			t.Errorf("Compare(%s, %q, %q) = %d, %v; want %d, err=%v", c.n, c.a, c.b, got, err, c.want, c.wantErr)
		}
	}
}

// ── scripts/stamp_engine_versions.sh ────────────────────────────────────────

func stampScript(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts", "stamp_engine_versions.sh"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func runStamp(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{stampScript(t)}, args...)...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func engineTree(t *testing.T) string {
	root := t.TempDir()
	write(t, filepath.Join(root, "linux", "cpu", "llama-server"), "x")
	write(t, filepath.Join(root, "linux", "cpu", "whisper-server"), "x")
	write(t, filepath.Join(root, "linux", "amd", "llama-server"), "x")
	write(t, filepath.Join(root, "windows", "cpu", "llama-server.exe"), "x")
	write(t, filepath.Join(root, "darwin", "metal", "llama-server-arm64"), "x")
	write(t, filepath.Join(root, "linux", "ngrok"), "x") // a file, not an engine folder
	write(t, filepath.Join(root, "linux", "cliproxy", "cli-proxy-api"), "x")
	return root
}

func TestStampScript_WritesMarkersOnlyWhereTheBinaryIs(t *testing.T) {
	root := engineTree(t)
	out, err := runStamp(t, root, "stamp", "--llama", "b9441", "--whisper", "1.8.6", root)
	if err != nil {
		t.Fatalf("stamp failed: %v\n%s", err, out)
	}
	for path, want := range map[string]string{
		filepath.Join(root, "linux", "cpu", "LLAMA_VERSION"):    "b9441",
		filepath.Join(root, "linux", "cpu", "WHISPER_VERSION"):  "1.8.6",
		filepath.Join(root, "linux", "amd", "LLAMA_VERSION"):    "b9441",
		filepath.Join(root, "windows", "cpu", "LLAMA_VERSION"):  "b9441",
		filepath.Join(root, "darwin", "metal", "LLAMA_VERSION"): "b9441",
	} {
		b, err := os.ReadFile(path)
		if err != nil || strings.TrimSpace(string(b)) != want {
			t.Errorf("%s = %q, %v; want %q", path, b, err, want)
		}
	}
	for _, path := range []string{
		filepath.Join(root, "linux", "amd", "WHISPER_VERSION"),    // no whisper-server there
		filepath.Join(root, "linux", "cliproxy", "LLAMA_VERSION"), // not llama
		filepath.Join(root, "windows", "cpu", "WHISPER_VERSION"),
	} {
		if _, err := os.Stat(path); err == nil {
			t.Errorf("%s must not exist", path)
		}
	}
	// What the script writes is what the reader accepts.
	if v, err := ReadMarker(filepath.Join(root, "linux", "cpu"), Llama); err != nil || v != "b9441" {
		t.Errorf("round trip: %q, %v", v, err)
	}
}

func TestStampScript_CheckFailsOnMissingAndPassesAfterStamp(t *testing.T) {
	root := engineTree(t)
	if out, err := runStamp(t, root, "check", root); err == nil {
		t.Fatalf("check passed on an unstamped tree:\n%s", out)
	}
	if out, err := runStamp(t, root, "stamp", "--llama", "b9441", "--whisper", "1.8.6", root); err != nil {
		t.Fatalf("stamp: %v\n%s", err, out)
	}
	if out, err := runStamp(t, root, "check", root); err != nil {
		t.Fatalf("check failed after stamping: %v\n%s", err, out)
	}
	// Removing one marker must be caught again.
	if err := os.Remove(filepath.Join(root, "linux", "amd", "LLAMA_VERSION")); err != nil {
		t.Fatal(err)
	}
	if out, err := runStamp(t, root, "check", root); err == nil {
		t.Fatalf("check passed with a marker missing:\n%s", out)
	}
}

func TestStampScript_RefusesBadVersionsAndMixedLibraries(t *testing.T) {
	root := engineTree(t)
	if out, err := runStamp(t, root, "stamp", "--llama", "9441", "--whisper", "1.8.6", root); err == nil {
		t.Fatalf("a llama tag without the b was accepted:\n%s", out)
	}
	if out, err := runStamp(t, root, "stamp", "--llama", "b9441", root); err == nil {
		t.Fatalf("stamp without --whisper was accepted although a whisper-server exists:\n%s", out)
	}
	// A folder whose libllama carries a different build number than the tag we
	// were told is refused — the stamp must never claim a version the files deny.
	write(t, filepath.Join(root, "linux", "cpu", "libllama.so.0.0.9441"), "x")
	if out, err := runStamp(t, root, "stamp", "--llama", "b11456", "--whisper", "1.8.6", root); err == nil {
		t.Fatalf("stamp contradicted libllama.so.0.0.9441:\n%s", out)
	}
	if out, err := runStamp(t, root, "stamp", "--llama", "b9441", "--whisper", "1.8.6", root); err != nil {
		t.Fatalf("a matching libllama build number must pass: %v\n%s", err, out)
	}
}

func TestStampScript_OnlyRestrictsToTheNamedFolders(t *testing.T) {
	root := engineTree(t)
	out, err := runStamp(t, root, "stamp", "--llama", "b9441", "--whisper", "1.8.6",
		"--only", "linux/cpu", "--only", "linux/amd", root)
	if err != nil {
		t.Fatalf("stamp --only: %v\n%s", err, out)
	}
	for _, p := range []string{
		filepath.Join(root, "linux", "cpu", "LLAMA_VERSION"),
		filepath.Join(root, "linux", "amd", "LLAMA_VERSION"),
	} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("%s missing: %v", p, err)
		}
	}
	for _, p := range []string{
		filepath.Join(root, "windows", "cpu", "LLAMA_VERSION"),
		filepath.Join(root, "darwin", "metal", "LLAMA_VERSION"),
	} {
		if _, err := os.Stat(p); err == nil {
			t.Errorf("%s was stamped although it was not named", p)
		}
	}
	// check honours --only too: the named folders pass, the whole tree does not.
	if out, err := runStamp(t, root, "check", "--only", "linux/cpu", "--only", "linux/amd", root); err != nil {
		t.Errorf("check on the stamped folders failed: %v\n%s", err, out)
	}
	if _, err := runStamp(t, root, "check", root); err == nil {
		t.Error("check on the whole tree passed although windows and darwin are unstamped")
	}
}
