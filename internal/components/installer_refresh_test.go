// SPDX-License-Identifier: AGPL-3.0-or-later

package components

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The installers cannot `source` a helper (they are piped from curl), so each one
// embeds the block between these markers from scripts/engine_refresh.sh.
const (
	refreshBegin = "# >>> memo-engine-refresh"
	refreshEnd   = "# <<< memo-engine-refresh"
)

func scriptsDir(t *testing.T) string {
	t.Helper()
	p, err := filepath.Abs(filepath.Join("..", "..", "scripts"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func refreshBlock(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	s := string(b)
	i := strings.Index(s, refreshBegin)
	j := strings.Index(s, refreshEnd)
	if i < 0 || j < i {
		t.Fatalf("%s has no %q ... %q block", filepath.Base(path), refreshBegin, refreshEnd)
	}
	return s[i : j+len(refreshEnd)]
}

func TestInstallers_EmbedTheSameEngineRefreshBlock(t *testing.T) {
	dir := scriptsDir(t)
	want := refreshBlock(t, filepath.Join(dir, "engine_refresh.sh"))
	for _, n := range []string{"get-memo.sh", "get-memo-beta.sh", "get-memo-server.sh", "get-memo-server-beta.sh", "get_memo_arm.sh", "update.sh"} {
		got := refreshBlock(t, filepath.Join(dir, n))
		if got != want {
			t.Errorf("%s embeds a different engine-refresh block than scripts/engine_refresh.sh; re-embed it "+
				"(replace everything from %q to %q with the file's block)", n, refreshBegin, refreshEnd)
		}
		b, _ := os.ReadFile(filepath.Join(dir, n))
		if !strings.Contains(string(b), `memo_refresh_engines "$MEMO_HOME/binaries" || true`) {
			t.Errorf("%s never calls memo_refresh_engines", n)
		}
	}
}

// ── a fake release server ────────────────────────────────────────────────────

type fakeEngine struct {
	component, version string
	folder             string // key inside the manifest, e.g. linux/cpu
	files              map[string]string
	corruptSum         bool
	extra              func(tw *tar.Writer)
}

func (f fakeEngine) archive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range f.files {
		mode := int64(0o644)
		if strings.HasPrefix(body, "#!") {
			mode = 0o755
		}
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(body)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(body)); err != nil {
			t.Fatal(err)
		}
	}
	if f.extra != nil {
		f.extra(tw)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// releaseServer announces `engines` in the beacon and serves their manifests and archives.
func releaseServer(t *testing.T, engines ...fakeEngine) (beacon, base string) {
	t.Helper()
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	comps := map[string]string{}
	for _, e := range engines {
		e := e
		comps[e.component] = e.version
		data := e.archive(t)
		sum := sha256.Sum256(data)
		hexsum := hex.EncodeToString(sum[:])
		if e.corruptSum {
			hexsum = strings.Repeat("0", 64)
		}
		archPath := fmt.Sprintf("/components/%s/%s/files.tar.gz", e.component, e.version)
		mux.HandleFunc(archPath, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) })
		manifest := fmt.Sprintf(`{
  "schema": 1, "type": "component", "component": %q, "version": %q,
  "files": {
    %q: {
      "url": %q,
      "sha256": %q,
      "size": %d,
      "format": "tar.gz"
    }
  }
}`, e.component, e.version, e.folder, srv.URL+archPath, hexsum, len(data))
		mux.HandleFunc(fmt.Sprintf("/components/%s/%s/manifest.json", e.component, e.version),
			func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(manifest)) })
	}
	var parts []string
	for k, v := range comps {
		parts = append(parts, fmt.Sprintf("%q: %q", k, v))
	}
	beaconJSON := fmt.Sprintf(`{ "version": "V4.7.0", "components": { %s } }`, strings.Join(parts, ", "))
	mux.HandleFunc("/version.json", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(beaconJSON)) })
	return srv.URL + "/version.json", srv.URL + "/components"
}

func runRefresh(t *testing.T, bin, beacon, base string) string {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(scriptsDir(t), "engine_refresh.sh"), bin)
	cmd.Env = append(os.Environ(), "MEMO_BEACON_URL="+beacon, "MEMO_COMPONENTS_BASE="+base)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("engine_refresh.sh failed (it must never be fatal): %v\n%s", err, out)
	}
	return string(out)
}

func needLinuxBash(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("the installers refresh engines on Linux only")
	}
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash not available")
	}
}

const (
	fakeLlama    = "#!/bin/sh\necho 'version: 0.7.0-dev (build 11669, commit abc)'\n"
	fakeCliproxy = "#!/bin/sh\necho 'Usage of cli-proxy-api'\n"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func TestEngineRefresh_ReplacesAnOlderFolderAndLeavesTheRest(t *testing.T) {
	needLinuxBash(t)
	bin := t.TempDir()
	write(t, filepath.Join(bin, "linux", "cpu", "llama-server"), "#!/bin/sh\necho 'version: 9441 (old)'\n")
	write(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION"), "b9441\n")
	write(t, filepath.Join(bin, "linux", "cpu", "stale.so"), "old")
	write(t, filepath.Join(bin, "linux", "cliproxy", "cli-proxy-api"), fakeCliproxy)
	write(t, filepath.Join(bin, "linux", "cliproxy", "VERSION"), "v8.0.43\n") // already current
	write(t, filepath.Join(bin, "linux", "cpu-arm64", "llama-server"), "x")   // marker missing: unknown, untouched

	beacon, base := releaseServer(t,
		fakeEngine{component: "llama", version: "b11669", folder: "linux/cpu", files: map[string]string{
			"llama-server": fakeLlama, "LLAMA_VERSION": "b11669\n", "libggml.so": "new",
		}},
		fakeEngine{component: "cliproxy", version: "v8.0.43", folder: "linux/cliproxy", files: map[string]string{
			"cli-proxy-api": fakeCliproxy, "VERSION": "v8.0.43\n",
		}},
	)
	out := runRefresh(t, bin, beacon, base)

	if got := read(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION")); got != "b11669" {
		t.Fatalf("llama marker = %q, want b11669\n%s", got, out)
	}
	if _, err := os.Stat(filepath.Join(bin, "linux", "cpu", "stale.so")); err == nil {
		t.Error("the folder must be replaced WHOLE; a file from the old build survived")
	}
	if _, err := os.Stat(filepath.Join(bin, "linux", "cpu", "libggml.so")); err != nil {
		t.Error("the new build's library is missing")
	}
	if !strings.Contains(out, "b9441 → b11669") {
		t.Errorf("no progress line for the llama update:\n%s", out)
	}
	if strings.Contains(out, "cliproxy") {
		t.Errorf("an up-to-date folder was touched:\n%s", out)
	}
	if got := read(t, filepath.Join(bin, "linux", "cpu-arm64", "llama-server")); got != "x" {
		t.Error("a folder without a version marker must never be touched")
	}
	if _, err := os.Stat(filepath.Join(bin, "linux", "cpu.memo-old")); err == nil {
		t.Error("the backup folder was left behind")
	}
}

func TestEngineRefresh_KeepsTheOldFolderWhenAnythingIsWrong(t *testing.T) {
	needLinuxBash(t)
	cases := []struct {
		name string
		eng  fakeEngine
		want string // text expected in the output
	}{
		{"checksum mismatch", fakeEngine{corruptSum: true, files: map[string]string{
			"llama-server": fakeLlama, "LLAMA_VERSION": "b11669\n"}}, "checksum mismatch"},
		{"archive that does not carry the announced marker", fakeEngine{files: map[string]string{
			"llama-server": fakeLlama, "LLAMA_VERSION": "b10000\n"}}, "does not carry"},
		{"engine that does not start", fakeEngine{files: map[string]string{
			"llama-server": "#!/bin/sh\nexit 1\n", "LLAMA_VERSION": "b11669\n"}}, "did not start"},
		{"path traversal", fakeEngine{files: map[string]string{
			"llama-server": fakeLlama, "LLAMA_VERSION": "b11669\n", "../evil": "x"}}, "unsafe paths"},
		{"symlink", fakeEngine{files: map[string]string{
			"llama-server": fakeLlama, "LLAMA_VERSION": "b11669\n"},
			extra: func(tw *tar.Writer) {
				_ = tw.WriteHeader(&tar.Header{Name: "libx.so", Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd", Mode: 0o777})
			}}, "links in"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			bin := t.TempDir()
			write(t, filepath.Join(bin, "linux", "cpu", "llama-server"), "#!/bin/sh\necho 'version: 9441 (old)'\n")
			write(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION"), "b9441\n")
			c.eng.component, c.eng.version, c.eng.folder = "llama", "b11669", "linux/cpu"
			beacon, base := releaseServer(t, c.eng)
			out := runRefresh(t, bin, beacon, base)
			if got := read(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION")); got != "b9441" {
				t.Errorf("marker = %q; the old engine must stay\n%s", got, out)
			}
			if !strings.Contains(out, c.want) {
				t.Errorf("output does not explain why (%q):\n%s", c.want, out)
			}
			if _, err := os.Stat(filepath.Join(bin, "linux", "evil")); err == nil {
				t.Error("a path outside the folder was written")
			}
		})
	}
}

func TestEngineRefresh_NothingAnnouncedOrNoNetworkChangesNothing(t *testing.T) {
	needLinuxBash(t)
	bin := t.TempDir()
	write(t, filepath.Join(bin, "linux", "cpu", "llama-server"), "x")
	write(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION"), "b9441\n")

	beacon, base := releaseServer(t) // announces no components
	runRefresh(t, bin, beacon, base)
	runRefresh(t, bin, "http://127.0.0.1:1/version.json", "http://127.0.0.1:1/components") // unreachable
	if got := read(t, filepath.Join(bin, "linux", "cpu", "LLAMA_VERSION")); got != "b9441" {
		t.Errorf("marker = %q, want b9441", got)
	}
}

func TestEngineOlder(t *testing.T) {
	needLinuxBash(t)
	src := refreshBlock(t, filepath.Join(scriptsDir(t), "engine_refresh.sh"))
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"b9441", "b11669", true}, // numeric, not lexical
		{"b11669", "b9441", false},
		{"b11669", "b11669", false},
		{"v8.0.25", "v8.0.43", true},
		{"v8.0.9", "v8.0.43", true},
		{"v8.0.43", "v8.0.9", false},
		{"1.8.6", "1.9.5", true},
		{"1.9.5", "1.9.5", false},
		{"", "b1", false},
	} {
		cmd := exec.Command("bash", "-c", src+"\nmemo_engine_older \"$1\" \"$2\"", "x", c.a, c.b)
		got := cmd.Run() == nil
		if got != c.want {
			t.Errorf("older(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
