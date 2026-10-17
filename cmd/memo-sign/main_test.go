// SPDX-License-Identifier: AGPL-3.0-or-later

package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/updater"
)

func sign(t *testing.T, env map[string]string, args ...string) (string, string, int) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, &out, &errb, func(k string) string { return env[k] })
	return out.String(), errb.String(), code
}

func newKey(t *testing.T) (keyFile, pub string) {
	t.Helper()
	keyFile = filepath.Join(t.TempDir(), "k", "signing.key")
	out, errs, code := sign(t, nil, "keygen", "-out", keyFile)
	if code != 0 {
		t.Fatalf("keygen: %s %s", out, errs)
	}
	return keyFile, strings.TrimSpace(out)
}

func writeBlob(t *testing.T, n int) string {
	p := filepath.Join(t.TempDir(), "blob.tar.gz")
	if err := os.WriteFile(p, bytes.Repeat([]byte("memo"), n), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestKeygen_WritesAPrivateKeyOnlyTheOwnerCanReadAndNeverOverwrites(t *testing.T) {
	keyFile, pub := newKey(t)
	st, err := os.Stat(keyFile)
	if err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("key file mode = %v, %v; want 0600", st.Mode(), err)
	}
	if _, err := updater.DecodePublic(pub); err != nil {
		t.Fatalf("printed public key is unusable: %v", err)
	}
	_, errs, code := sign(t, nil, "keygen", "-out", keyFile)
	if code == 0 || !strings.Contains(errs, "refusing to overwrite") {
		t.Fatalf("keygen overwrote an existing signing key: code=%d %s", code, errs)
	}
}

func TestApp_SignsAManifestTheAppAccepts(t *testing.T) {
	keyFile, pub := newKey(t)
	blob := writeBlob(t, 1000)
	out := t.TempDir()
	stdout, errs, code := sign(t, nil, "app",
		"-version", "V4.7.0", "-platform", "linux-amd64", "-kind", "archive", "-format", "tar.gz",
		"-url", "https://data.memocpp.com/memo.tar.gz", "-file", blob, "-root", "Memo", "-out", out, "-key-file", keyFile)
	if code != 0 {
		t.Fatalf("app: %s %s", stdout, errs)
	}
	path := filepath.Join(out, "linux-amd64.json")
	body, _ := os.ReadFile(path)
	sig, _ := os.ReadFile(path + ".sig")

	key, _ := updater.DecodePublic(pub)
	if err := updater.Verify(key, body, string(sig)); err != nil {
		t.Fatalf("what memo-sign wrote does not verify: %v", err)
	}
	m, err := updater.ParseAppManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	wantSum, _ := updater.SHA256File(blob)
	if m.SHA256 != wantSum || m.Size != 4000 || m.Version != "V4.7.0" || m.Root != "Memo" {
		t.Errorf("manifest = %+v", m)
	}
	// The verify subcommand agrees.
	if out, errs, code := sign(t, nil, "verify", "-pub", pub, path); code != 0 {
		t.Fatalf("verify: %s %s", out, errs)
	}
	// And notices tampering.
	if err := os.WriteFile(path, bytes.Replace(body, []byte("V4.7.0"), []byte("V9.9.9"), 1), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, code := sign(t, nil, "verify", "-pub", pub, path); code == 0 {
		t.Fatal("verify accepted a tampered manifest")
	}
}

func TestApp_KeyFromTheEnvironmentAndMissingKey(t *testing.T) {
	keyFile, pub := newKey(t)
	raw, _ := os.ReadFile(keyFile)
	blob := writeBlob(t, 10)
	args := []string{"app", "-version", "V4.7.0", "-platform", "windows-amd64", "-kind", "installer", "-format", "exe",
		"-url", "https://data.memocpp.com/memo.exe", "-file", blob, "-out", t.TempDir()}
	if _, errs, code := sign(t, nil, args...); code == 0 || !strings.Contains(errs, "no signing key") {
		t.Fatalf("signing with no key: code=%d %s", code, errs)
	}
	out := t.TempDir()
	args[len(args)-1] = out
	if _, errs, code := sign(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": string(raw)}, args...); code != 0 {
		t.Fatalf("env key: %s", errs)
	}
	body, _ := os.ReadFile(filepath.Join(out, "windows-amd64.json"))
	sig, _ := os.ReadFile(filepath.Join(out, "windows-amd64.json.sig"))
	key, _ := updater.DecodePublic(pub)
	if err := updater.Verify(key, body, string(sig)); err != nil {
		t.Fatal(err)
	}
}

func TestApp_RefusesToPublishWhatTheAppWouldReject(t *testing.T) {
	keyFile, _ := newKey(t)
	blob := writeBlob(t, 10)
	cases := map[string][]string{
		"http url":     {"-url", "http://data.memocpp.com/memo.tar.gz"},
		"exe on linux": {"-kind", "installer", "-format", "exe"},
		"bad version":  {"-version", "latest"},
		"bad platform": {"-platform", "beos-ppc"},
	}
	for name, override := range cases {
		args := []string{"app", "-version", "V4.7.0", "-platform", "linux-amd64", "-kind", "archive", "-format", "tar.gz",
			"-url", "https://data.memocpp.com/memo.tar.gz", "-file", blob, "-out", t.TempDir(), "-key-file", keyFile}
		args = append(args, override...) // later flags win
		if _, _, code := sign(t, nil, args...); code == 0 {
			t.Errorf("%s: a manifest the app would reject was signed", name)
		}
	}
	if _, errs, code := sign(t, nil, "app", "-version", "V4.7.0"); code == 0 || !strings.Contains(errs, "required") {
		t.Errorf("missing flags: code=%d %s", code, errs)
	}
}

func TestComponent_SignsAMultiFolderManifest(t *testing.T) {
	keyFile, pub := newKey(t)
	cpu, amd := writeBlob(t, 100), writeBlob(t, 200)
	out := t.TempDir()
	_, errs, code := sign(t, nil, "component", "-component", "llama", "-version", "b11456", "-requires-app", "4.7.0",
		"-base-url", "https://data.memocpp.com/update/components/llama/b11456/",
		"-file", "linux/cpu="+cpu, "-file", "linux/amd="+amd, "-out", out, "-key-file", keyFile)
	if code != 0 {
		t.Fatalf("component: %s", errs)
	}
	body, _ := os.ReadFile(filepath.Join(out, "manifest.json"))
	sig, _ := os.ReadFile(filepath.Join(out, "manifest.json.sig"))
	key, _ := updater.DecodePublic(pub)
	if err := updater.Verify(key, body, string(sig)); err != nil {
		t.Fatal(err)
	}
	m, err := updater.ParseComponentManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 2 || m.RequiresApp != "4.7.0" {
		t.Fatalf("manifest = %+v", m)
	}
	f := m.Files["linux/cpu"]
	if f.URL != "https://data.memocpp.com/update/components/llama/b11456/"+filepath.Base(cpu) || f.Size != 400 || f.Format != "tar.gz" {
		t.Errorf("cpu file = %+v", f)
	}
	// :zip suffix selects the zip format.
	out2 := t.TempDir()
	if _, errs, code := sign(t, nil, "component", "-component", "cliproxy", "-version", "v8.0.26", "-base-url", "https://x.example/c",
		"-file", "windows/cliproxy="+cpu+":zip", "-out", out2, "-key-file", keyFile); code != 0 {
		t.Fatalf("zip: %s", errs)
	}
	body2, _ := os.ReadFile(filepath.Join(out2, "manifest.json"))
	if m2, err := updater.ParseComponentManifest(body2); err != nil || m2.Files["windows/cliproxy"].Format != "zip" {
		t.Fatalf("zip manifest: %+v %v", m2, err)
	}
}

func TestComponent_RejectsBadInput(t *testing.T) {
	keyFile, _ := newKey(t)
	blob := writeBlob(t, 10)
	base := []string{"component", "-component", "llama", "-version", "b1", "-base-url", "https://x.example/c", "-out", t.TempDir(), "-key-file", keyFile}
	if _, _, code := sign(t, nil, base...); code == 0 {
		t.Error("no files accepted")
	}
	if _, _, code := sign(t, nil, append(base, "-file", "linux-cpu-no-equals")...); code == 0 {
		t.Error("malformed -file accepted")
	}
	if _, _, code := sign(t, nil, append(base, "-file", "../etc="+blob)...); code == 0 {
		t.Error("a folder that is not os/name accepted")
	}
	if _, _, code := sign(t, nil, append(base, "-file", "linux/cpu=/nonexistent/file")...); code == 0 {
		t.Error("missing file accepted")
	}
}

func TestUsage(t *testing.T) {
	if _, _, code := sign(t, nil); code != 2 {
		t.Errorf("no args: %d", code)
	}
	if _, _, code := sign(t, nil, "bogus"); code != 2 {
		t.Errorf("unknown command: %d", code)
	}
}
