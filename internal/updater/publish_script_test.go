// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// These tests run scripts/publish_update_manifest.sh the way the build
// workflows do — against a COPY of the modules it builds, with a throwaway key
// compiled in as the "public key" and rclone's local backend standing in for R2.

func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		if strings.HasSuffix(p, "_test.go") {
			return nil
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, b, info.Mode())
	})
	if err != nil {
		t.Fatal(err)
	}
}

type publishEnv struct {
	repo, r2, privKey, pubKey string
}

func newPublishEnv(t *testing.T) *publishEnv {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("bash script")
	}
	for _, tool := range []string{"bash", "rclone", "go"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	for _, f := range []string{"go.mod"} {
		b, _ := os.ReadFile(filepath.Join(root, f))
		_ = os.WriteFile(filepath.Join(repo, f), b, 0o644)
	}
	for _, d := range []string{"cmd/memo-sign", "internal/updater", "internal/components", "scripts"} {
		copyTree(t, filepath.Join(root, d), filepath.Join(repo, d))
	}
	_ = os.WriteFile(filepath.Join(repo, "version"), []byte("V4.7.0"), 0o644)

	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	// Compile the throwaway public key into the copy, as pubkey.go does in the app.
	pk := filepath.Join(repo, "internal", "updater", "pubkey.go")
	src, _ := os.ReadFile(pk)
	re := strings.Replace(string(src), `const publicKeyB64 = "`+publicKeyB64+`"`, `const publicKeyB64 = "`+EncodePublic(pub)+`"`, 1)
	if re == string(src) {
		t.Fatal("could not substitute the public key in the copy")
	}
	_ = os.WriteFile(pk, []byte(re), 0o644)
	return &publishEnv{repo: repo, r2: t.TempDir(), privKey: EncodePrivate(priv), pubKey: EncodePublic(pub)}
}

// uploaded is where the fake R2 ("R2:memo/…" on rclone's local backend, which
// the script reaches after it cd's to the repo root) puts a published file.
func (e *publishEnv) uploaded(parts ...string) string {
	return filepath.Join(append([]string{e.repo, "memo"}, parts...)...)
}

func (e *publishEnv) run(t *testing.T, env map[string]string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command("bash", append([]string{filepath.Join(e.repo, "scripts", "publish_update_manifest.sh")}, args...)...)
	cmd.Dir = e.r2 // the local rclone remote is relative to the working directory
	base := []string{
		"PATH=" + os.Getenv("PATH"), "HOME=" + os.Getenv("HOME"), "GOCACHE=" + goCache(t), "GOFLAGS=-mod=mod",
		"GOPATH=" + os.Getenv("GOPATH"), "GOMODCACHE=" + os.Getenv("GOMODCACHE"),
		"RCLONE_CONFIG_R2_TYPE=local", "RUNNER_TEMP=" + t.TempDir(), "R2_BUCKET=memo",
	}
	for k, v := range env {
		base = append(base, k+"="+v)
	}
	cmd.Env = base
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func goCache(t *testing.T) string {
	if v := os.Getenv("GOCACHE"); v != "" {
		return v
	}
	out, err := exec.Command("go", "env", "GOCACHE").Output()
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(out))
}

func (e *publishEnv) artifact(t *testing.T, n int) string {
	p := filepath.Join(t.TempDir(), "memo.tar.gz")
	if err := os.WriteFile(p, []byte(strings.Repeat("m", n)), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestPublishScript_SignsVerifiesAndUploads(t *testing.T) {
	e := newPublishEnv(t)
	art := e.artifact(t, 5000)
	out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": e.privKey, "GITHUB_REF_NAME": "v4.7.0"},
		"linux-amd64", "archive", "tar.gz", art, "https://data.memocpp.com/memo.tar.gz", "Memo")
	if err != nil {
		t.Fatalf("script failed: %v\n%s", err, out)
	}
	dir := e.uploaded("update", "app", "4.7.0")
	body, err := os.ReadFile(filepath.Join(dir, "linux-amd64.json"))
	if err != nil {
		t.Fatalf("manifest not uploaded: %v\n%s", err, out)
	}
	sig, err := os.ReadFile(filepath.Join(dir, "linux-amd64.json.sig"))
	if err != nil {
		t.Fatalf("signature not uploaded: %v", err)
	}
	pub, _ := DecodePublic(e.pubKey)
	if err := Verify(pub, body, string(sig)); err != nil {
		t.Fatalf("the uploaded signature does not verify: %v", err)
	}
	m, err := ParseAppManifest(body)
	if err != nil {
		t.Fatal(err)
	}
	want, _ := SHA256File(art)
	if m.Version != "V4.7.0" || m.SHA256 != want || m.Size != 5000 || m.Root != "Memo" || m.URL != "https://data.memocpp.com/memo.tar.gz" {
		t.Fatalf("manifest = %+v", m)
	}
}

func TestPublishScript_CheckpointTagPublishesNothing(t *testing.T) {
	e := newPublishEnv(t)
	art := e.artifact(t, 100)
	for _, tag := range []string{"v3.2.1", "v4.6.0", "v4.7.1", "v4.7"} {
		out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": e.privKey, "GITHUB_REF_NAME": tag},
			"linux-amd64", "archive", "tar.gz", art, "https://data.memocpp.com/memo.tar.gz", "Memo")
		if err != nil {
			t.Fatalf("tag %s: a checkpoint tag must be a quiet no-op, got %v\n%s", tag, err, out)
		}
		if !strings.Contains(out, "checkpoint") {
			t.Errorf("tag %s: no notice explaining why nothing was published:\n%s", tag, out)
		}
	}
	if _, err := os.Stat(e.uploaded()); err == nil {
		t.Error("something was uploaded for a checkpoint tag")
	}
}

func TestPublishScript_RefusesAKeyThatIsNotTheCompiledInOne(t *testing.T) {
	e := newPublishEnv(t)
	art := e.artifact(t, 100)
	_, wrong, _ := GenerateKey()
	out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": EncodePrivate(wrong), "GITHUB_REF_NAME": "v4.7.0"},
		"linux-amd64", "archive", "tar.gz", art, "https://data.memocpp.com/memo.tar.gz", "Memo")
	if err == nil {
		t.Fatalf("a secret that does not match the app's public key was accepted:\n%s", out)
	}
	if _, statErr := os.Stat(e.uploaded()); statErr == nil {
		t.Error("a manifest nobody can verify was uploaded")
	}
}

func TestPublishScript_RefusesWithoutAKeyOrFile(t *testing.T) {
	e := newPublishEnv(t)
	art := e.artifact(t, 100)
	if out, err := e.run(t, map[string]string{"GITHUB_REF_NAME": "v4.7.0"},
		"linux-amd64", "archive", "tar.gz", art, "https://data.memocpp.com/memo.tar.gz", "Memo"); err == nil || !strings.Contains(out, "MEMO_UPDATE_SIGNING_KEY") {
		t.Fatalf("no key: err=%v\n%s", err, out)
	}
	if out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": e.privKey, "GITHUB_REF_NAME": "v4.7.0"},
		"linux-amd64", "archive", "tar.gz", "/nonexistent/file", "https://data.memocpp.com/memo.tar.gz", "Memo"); err == nil {
		t.Fatalf("missing artifact accepted:\n%s", out)
	}
	// An http URL is rejected by the same parser the app uses.
	if out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": e.privKey, "GITHUB_REF_NAME": "v4.7.0"},
		"linux-amd64", "archive", "tar.gz", art, "http://data.memocpp.com/memo.tar.gz", "Memo"); err == nil {
		t.Fatalf("an http URL was published:\n%s", out)
	}
}

func TestPublishScript_WindowsInstallerShape(t *testing.T) {
	e := newPublishEnv(t)
	art := filepath.Join(t.TempDir(), "Memo-Setup-v4.7.0.exe")
	_ = os.WriteFile(art, []byte(strings.Repeat("e", 3000)), 0o644)
	out, err := e.run(t, map[string]string{"MEMO_UPDATE_SIGNING_KEY": e.privKey, "GITHUB_REF_NAME": "v4.7.0"},
		"windows-amd64", "installer", "exe", art, "https://data.memocpp.com/memo.exe")
	if err != nil {
		t.Fatalf("%v\n%s", err, out)
	}
	body, err := os.ReadFile(e.uploaded("update", "app", "4.7.0", "windows-amd64.json"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := ParseAppManifest(body)
	if err != nil || m.Kind != KindInstaller || m.Format != "exe" || m.Root != "" {
		t.Fatalf("manifest = %+v, %v", m, err)
	}
}
