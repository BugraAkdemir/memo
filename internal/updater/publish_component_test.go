// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/components"
)

// The publisher (scripts/publish_component.sh) and the installer
// (InstallComponent) are written separately; these tests run one into the other
// so a folder the script packs is proven installable by the code that consumes it.

func engineFolder(t *testing.T, version string) string {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "cpu")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	must := func(name, body string, mode os.FileMode) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), mode); err != nil {
			t.Fatal(err)
		}
	}
	must("llama-server", "#!/bin/sh\necho 'version: 11456 (abc)'\n", 0o755)
	must("libggml.so.0.14.0", "ggml", 0o644)
	must("LLAMA_VERSION", version+"\n", 0o644)
	// A symlink in the folder (as llama.cpp tarballs carry): packed as a real file.
	if err := os.Symlink("libggml.so.0.14.0", filepath.Join(dir, "libggml.so.0")); err != nil {
		t.Fatal(err)
	}
	return dir
}

func (e *publishEnv) runComponent(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{filepath.Join(e.repo, "scripts", "publish_component.sh")}, args...)...)
	cmd.Dir = e.repo
	cmd.Env = []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + t.TempDir(), "GOCACHE=" + goCache(t), "GOFLAGS=-mod=mod",
		"GOPATH=" + os.Getenv("GOPATH"), "GOMODCACHE=" + os.Getenv("GOMODCACHE"),
		"MEMO_UPDATE_SIGNING_KEY=" + e.privKey,
	}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestPublishComponent_WhatItPacksTheInstallerAccepts(t *testing.T) {
	e := newPublishEnv(t)
	dir := engineFolder(t, "b11456")
	out, err := e.runComponent(t, "--dry-run", "--requires-app", "4.7.0", "llama", "b11456", "linux/cpu="+dir)
	if err != nil {
		t.Fatalf("publish failed: %v\n%s", err, out)
	}
	if !strings.Contains(out, `"llama": "b11456"`) || !strings.Contains(out, "nothing uploaded") {
		t.Errorf("output does not tell the maintainer what to do next / that it was a dry run:\n%s", out)
	}
	built := filepath.Join(e.repo, "build_output", "components", "llama", "b11456")
	body, _ := os.ReadFile(filepath.Join(built, "manifest.json"))
	sig, _ := os.ReadFile(filepath.Join(built, "manifest.json.sig"))
	pub, _ := DecodePublic(e.pubKey)
	if err := Verify(pub, body, string(sig)); err != nil {
		t.Fatalf("manifest signature: %v", err)
	}
	m, err := ParseComponentManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if m.Component != "llama" || m.Version != "b11456" || m.RequiresApp != "4.7.0" {
		t.Fatalf("manifest = %+v", m)
	}
	f, ok := m.Files["linux/cpu"]
	if !ok || f.Format != "tar.gz" || !strings.HasPrefix(f.URL, "https://data.memocpp.com/update/components/llama/b11456/") {
		t.Fatalf("file entry = %+v", f)
	}

	// Serve the archive the script built under the URL the manifest names (the
	// manifest's host is the real one, so redirect to a local server by hand) and
	// install it with the real installer.
	archive := filepath.Join(built, "linux-cpu.tar.gz")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.ServeFile(w, r, archive) }))
	defer srv.Close()
	root := filepath.Join(t.TempDir(), "components")
	err = InstallComponent(context.Background(), ComponentInstall{
		OverrideRoot: root, OS: "linux", Folder: "cpu", Component: components.Llama, Version: "b11456",
		File: File{URL: srv.URL + "/x.tar.gz", SHA256: f.SHA256, Size: f.Size, Format: f.Format},
	})
	if err != nil {
		t.Fatalf("the installer rejected what the publisher packed: %v", err)
	}
	dest := filepath.Join(root, "binaries", "linux", "cpu")
	if v, _ := components.ReadMarker(dest, components.Llama); v != "b11456" {
		t.Errorf("installed marker = %q", v)
	}
	if st, err := os.Lstat(filepath.Join(dest, "libggml.so.0")); err != nil || st.Mode()&os.ModeSymlink != 0 {
		t.Errorf("the symlink was not packed as a plain file: %v %v", st, err)
	}
	if st, err := os.Stat(filepath.Join(dest, "llama-server")); err != nil || st.Mode()&0o111 == 0 {
		t.Errorf("llama-server is not executable: %v %v", st, err)
	}
}

func TestPublishComponent_RefusesWhatItCouldNotStandBehind(t *testing.T) {
	e := newPublishEnv(t)
	good := engineFolder(t, "b11456")
	wrongMarker := engineFolder(t, "b9441")
	noBinary := engineFolder(t, "b11456")
	_ = os.Remove(filepath.Join(noBinary, "llama-server"))

	cases := map[string][]string{
		"marker differs from the version": {"--dry-run", "llama", "b11456", "linux/cpu=" + wrongMarker},
		"no engine binary in the folder":  {"--dry-run", "llama", "b11456", "linux/cpu=" + noBinary},
		"folder is not os/name":           {"--dry-run", "llama", "b11456", "cpu=" + good},
		"folder escapes":                  {"--dry-run", "llama", "b11456", "../etc=" + good},
		"unknown component":               {"--dry-run", "sqlite", "b11456", "linux/cpu=" + good},
		"llama tag without the b":         {"--dry-run", "llama", "11456", "linux/cpu=" + good},
		"directory missing":               {"--dry-run", "llama", "b11456", "linux/cpu=/nonexistent/dir"},
		"too few arguments":               {"--dry-run", "llama"},
	}
	for name, args := range cases {
		out, err := e.runComponent(t, args...)
		if err == nil {
			t.Errorf("%s: accepted\n%s", name, out)
		}
	}
	if _, err := os.Stat(filepath.Join(e.repo, "build_output", "components", "llama", "b11456", "manifest.json")); err == nil {
		t.Error("a manifest was signed for a release the script refused")
	}
}

func TestPublishComponent_SeveralFoldersAndWindowsZip(t *testing.T) {
	if _, err := exec.LookPath("zip"); err != nil {
		if _, perr := exec.LookPath("python3"); perr != nil {
			t.Skip("neither zip nor python3 available")
		}
	}
	e := newPublishEnv(t)
	linux := engineFolder(t, "b11456")
	win := filepath.Join(t.TempDir(), "amd")
	_ = os.MkdirAll(win, 0o755)
	_ = os.WriteFile(filepath.Join(win, "llama-server.exe"), []byte("MZ"), 0o755)
	_ = os.WriteFile(filepath.Join(win, "LLAMA_VERSION"), []byte("b11456\n"), 0o644)

	out, err := e.runComponent(t, "--dry-run", "llama", "b11456", "linux/cpu="+linux, "windows/amd="+win)
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	body, _ := os.ReadFile(filepath.Join(e.repo, "build_output", "components", "llama", "b11456", "manifest.json"))
	m, err := ParseComponentManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if m.Files["linux/cpu"].Format != "tar.gz" || m.Files["windows/amd"].Format != "zip" {
		t.Fatalf("formats = %+v / %+v", m.Files["linux/cpu"], m.Files["windows/amd"])
	}
	// The zip really unpacks with the app's own extractor.
	zipPath := filepath.Join(e.repo, "build_output", "components", "llama", "b11456", "windows-amd.zip")
	dst := t.TempDir()
	if err := Extract(zipPath, "zip", dst, ExtractLimits{}); err != nil {
		t.Fatalf("the zip the script made does not unpack: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dst, "LLAMA_VERSION")); err != nil {
		t.Errorf("zip is missing the marker: %v", err)
	}
}
