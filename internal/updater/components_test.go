// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"

	"memo/internal/components"
)

// compArchive builds a tar.gz of an engine folder and returns its bytes' file
// path, digest and size.
func compArchive(t *testing.T, ms []member) (path, sum string, size int64) {
	t.Helper()
	path = buildTarGz(t, ms)
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s, err := SHA256File(path)
	if err != nil {
		t.Fatal(err)
	}
	return path, s, int64(len(b))
}

func serveFile(t *testing.T, path string) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, path) }))
	t.Cleanup(srv.Close)
	return srv.URL + "/engine.tar.gz"
}

func llamaFolder(version string) []member {
	return []member{
		{name: "llama-server", body: "server-" + version, mode: 0o644},
		{name: "libggml.so.0", body: "ggml-" + version},
		{name: "LLAMA_VERSION", body: version + "\n"},
	}
}

type events struct{ log []string }

func (e *events) add(s string) { e.log = append(e.log, s) }

type compEnv struct {
	root string
	ev   *events
}

func newCompEnv(t *testing.T) *compEnv {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	return &compEnv{root: filepath.Join(t.TempDir(), "components"), ev: &events{}}
}

func (e *compEnv) install(t *testing.T, version string, ms []member, mut func(*ComponentInstall)) error {
	t.Helper()
	path, sum, size := compArchive(t, ms)
	c := ComponentInstall{
		OverrideRoot: e.root, OS: "linux", Folder: "cpu", Component: components.Llama, Version: version,
		File: File{URL: serveFile(t, path), SHA256: sum, Size: size, Format: "tar.gz"},
		Quiesce: func() (func() error, error) {
			e.ev.add("quiesce")
			return func() error { e.ev.add("resume"); return nil }, nil
		},
	}
	if mut != nil {
		mut(&c)
	}
	return InstallComponent(context.Background(), c)
}

func (e *compEnv) dest() string { return filepath.Join(e.root, "binaries", "linux", "cpu") }

func (e *compEnv) read(t *testing.T, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(e.dest(), rel))
	if err != nil {
		return "<" + err.Error() + ">"
	}
	return string(b)
}

func TestInstallComponent_FreshInstall(t *testing.T) {
	e := newCompEnv(t)
	if err := e.install(t, "b11456", llamaFolder("b11456"), nil); err != nil {
		t.Fatal(err)
	}
	if got := e.read(t, "llama-server"); got != "server-b11456" {
		t.Errorf("llama-server = %q", got)
	}
	if v, _ := components.ReadMarker(e.dest(), components.Llama); v != "b11456" {
		t.Errorf("marker = %q", v)
	}
	if st, err := os.Stat(filepath.Join(e.dest(), "llama-server")); err != nil || st.Mode()&0o111 == 0 {
		t.Errorf("llama-server is not executable: %v %v", st, err)
	}
	if !reflect.DeepEqual(e.ev.log, []string{"quiesce", "resume"}) {
		t.Errorf("events = %v; the engine must be stopped, swapped, then started again", e.ev.log)
	}
	if _, err := os.Stat(e.dest() + ".old"); err == nil {
		t.Error(".old left behind")
	}
	if entries, _ := os.ReadDir(filepath.Join(e.root, ".work")); len(entries) != 0 {
		t.Errorf("scratch files left behind: %v", entries)
	}
}

func TestInstallComponent_ReplacesAnEarlierOverride(t *testing.T) {
	e := newCompEnv(t)
	if err := e.install(t, "b10000", llamaFolder("b10000"), nil); err != nil {
		t.Fatal(err)
	}
	if err := e.install(t, "b11456", llamaFolder("b11456"), nil); err != nil {
		t.Fatal(err)
	}
	if got := e.read(t, "LLAMA_VERSION"); got != "b11456\n" {
		t.Errorf("marker = %q", got)
	}
	if _, err := os.Stat(e.dest() + ".old"); err == nil {
		t.Error("the previous override was kept after a good update")
	}
}

func TestInstallComponent_BadDownloadChangesNothingAndStopsNothing(t *testing.T) {
	e := newCompEnv(t)
	_ = e.install(t, "b10000", llamaFolder("b10000"), nil)
	e.ev.log = nil
	err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) {
		c.File.SHA256 = strings.Repeat("0", 64) // the manifest promises different bytes
	})
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if got := e.read(t, "LLAMA_VERSION"); got != "b10000\n" {
		t.Errorf("the installed engine changed: %q", got)
	}
	if len(e.ev.log) != 0 {
		t.Errorf("the running engine was disturbed by a download that failed verification: %v", e.ev.log)
	}
}

func TestInstallComponent_RefusesAReleaseThatIsNotWhatTheManifestSaid(t *testing.T) {
	e := newCompEnv(t)
	cases := map[string][]member{
		"marker says another version": llamaFolder("b9999"),
		"no marker":                   {{name: "llama-server", body: "x"}},
		"no engine binary":            {{name: "LLAMA_VERSION", body: "b11456\n"}, {name: "README", body: "x"}},
		"garbage marker":              {{name: "llama-server", body: "x"}, {name: "LLAMA_VERSION", body: "../../etc\n"}},
	}
	for name, ms := range cases {
		e.ev.log = nil
		if err := e.install(t, "b11456", ms, nil); err == nil {
			t.Errorf("%s: accepted", name)
		}
		if _, err := os.Stat(e.dest()); err == nil {
			t.Errorf("%s: something was installed", name)
		}
		if len(e.ev.log) != 0 {
			t.Errorf("%s: the engine was stopped for a release that was refused: %v", name, e.ev.log)
		}
	}
}

func TestInstallComponent_NewEngineThatWontStartIsRolledBackToThePreviousOverride(t *testing.T) {
	e := newCompEnv(t)
	_ = e.install(t, "b10000", llamaFolder("b10000"), nil)
	e.ev.log = nil

	resumes := 0
	err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) {
		c.Quiesce = func() (func() error, error) {
			e.ev.add("quiesce")
			return func() error {
				resumes++
				e.ev.add("resume:" + e.read(t, "LLAMA_VERSION"))
				if resumes == 1 {
					return errors.New("llama-server exited: unsupported flag") // the new one fails
				}
				return nil
			}, nil
		}
	})
	if err == nil {
		t.Fatal("a failed start was reported as success")
	}
	if got := e.read(t, "LLAMA_VERSION"); got != "b10000\n" {
		t.Fatalf("not rolled back: marker = %q", got)
	}
	want := []string{"quiesce", "resume:b11456\n", "quiesce", "resume:b10000\n"}
	if !reflect.DeepEqual(e.ev.log, want) {
		t.Errorf("events = %q\nwant     %q", e.ev.log, want)
	}
	if _, err := os.Stat(e.dest() + ".old"); err == nil {
		t.Error(".old left behind after the rollback")
	}
}

func TestInstallComponent_FailureWithNoEarlierOverrideFallsBackToTheBundledCopy(t *testing.T) {
	e := newCompEnv(t)
	err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) {
		n := 0
		c.Quiesce = func() (func() error, error) {
			return func() error {
				n++
				if n == 1 {
					return errors.New("won't start")
				}
				return nil
			}, nil
		}
	})
	if err == nil {
		t.Fatal("failure reported as success")
	}
	if _, err := os.Stat(e.dest()); err == nil {
		t.Error("the override folder was kept; the resolver would keep using a broken engine instead of the bundled one")
	}
}

func TestInstallComponent_VerifyFailureRollsBack(t *testing.T) {
	e := newCompEnv(t)
	_ = e.install(t, "b10000", llamaFolder("b10000"), nil)
	err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) {
		c.Verify = func() error { return errors.New("health check failed") }
	})
	if err == nil {
		t.Fatal("verify failure reported as success")
	}
	if got := e.read(t, "LLAMA_VERSION"); got != "b10000\n" {
		t.Errorf("not rolled back: %q", got)
	}
}

func TestInstallComponent_CouldNotStopTheEngineChangesNothing(t *testing.T) {
	e := newCompEnv(t)
	_ = e.install(t, "b10000", llamaFolder("b10000"), nil)
	err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) {
		c.Quiesce = func() (func() error, error) { return nil, errors.New("process would not die") }
	})
	if err == nil {
		t.Fatal("reported success")
	}
	if got := e.read(t, "LLAMA_VERSION"); got != "b10000\n" {
		t.Errorf("files changed although the engine could not be stopped: %q", got)
	}
}

func TestInstallComponent_RejectsOddArguments(t *testing.T) {
	e := newCompEnv(t)
	for _, folder := range []string{"", "..", "a/b", `a\b`} {
		err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) { c.Folder = folder })
		if err == nil {
			t.Errorf("folder %q accepted", folder)
		}
	}
	if err := e.install(t, "b11456", llamaFolder("b11456"), func(c *ComponentInstall) { c.OverrideRoot = "" }); err == nil {
		t.Error("empty override root accepted")
	}
}

// ── PruneOverrides ──────────────────────────────────────────────────────────

func markFolder(t *testing.T, root, folder string, marks map[string]string) {
	t.Helper()
	for file, v := range marks {
		put(t, filepath.Join(root, "binaries", "linux", folder), file, v+"\n", 0o644)
	}
}

func TestPruneOverrides(t *testing.T) {
	over, bundled := t.TempDir(), t.TempDir()

	// bundled caught up with the override → remove it
	markFolder(t, over, "cpu", map[string]string{"LLAMA_VERSION": "b10000"})
	markFolder(t, bundled, "cpu", map[string]string{"LLAMA_VERSION": "b11456"})
	// bundled still older → keep
	markFolder(t, over, "amd", map[string]string{"LLAMA_VERSION": "b11456"})
	markFolder(t, bundled, "amd", map[string]string{"LLAMA_VERSION": "b9441"})
	// equal → remove
	markFolder(t, over, "nvidia", map[string]string{"LLAMA_VERSION": "b9441"})
	markFolder(t, bundled, "nvidia", map[string]string{"LLAMA_VERSION": "b9441"})
	// bundled copy has no marker → cannot compare → keep
	markFolder(t, over, "metal", map[string]string{"LLAMA_VERSION": "b11456"})
	if err := os.MkdirAll(filepath.Join(bundled, "binaries", "linux", "metal"), 0o755); err != nil {
		t.Fatal(err)
	}
	// a two-component folder: llama is covered but whisper's override is newer → keep the whole folder
	markFolder(t, over, "cpu-arm64", map[string]string{"LLAMA_VERSION": "b9441", "WHISPER_VERSION": "1.9.0"})
	markFolder(t, bundled, "cpu-arm64", map[string]string{"LLAMA_VERSION": "b11456", "WHISPER_VERSION": "1.8.6"})

	removed := PruneOverrides(over, "linux", []string{bundled})
	got := map[string]bool{}
	for _, r := range removed {
		got[filepath.Base(r)] = true
	}
	if !got["cpu"] || !got["nvidia"] || len(got) != 2 {
		t.Errorf("removed %v; want exactly cpu and nvidia", removed)
	}
	for _, keep := range []string{"amd", "metal", "cpu-arm64"} {
		if _, err := os.Stat(filepath.Join(over, "binaries", "linux", keep)); err != nil {
			t.Errorf("%s was removed but should have been kept", keep)
		}
	}
	// A leftover .old folder is never touched by pruning (InstallComponent owns it).
	if PruneOverrides(t.TempDir(), "linux", nil) != nil {
		t.Error("pruning an empty tree returned something")
	}
}
