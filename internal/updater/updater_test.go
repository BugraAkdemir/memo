// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// ── versions ────────────────────────────────────────────────────────────────

func TestParseVersion(t *testing.T) {
	for _, ok := range []string{"V4.7.0", "v4.7.0", "4.7.0", " V10.20.30\n"} {
		if _, err := ParseVersion(ok); err != nil {
			t.Errorf("%q rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{"", "V4.7", "V4.7.0.1", "V4.7.x", "V4..0", "dev", "V4.7.0-beta", "V-1.0.0", "V1.2.1234567", "latest"} {
		if _, err := ParseVersion(bad); err == nil {
			t.Errorf("%q accepted", bad)
		}
	}
}

func TestNewer_NeverOffersADowngradeOrTheSameVersion(t *testing.T) {
	cases := []struct {
		cur, cand string
		want      bool
	}{
		{"V4.6.0", "V4.7.0", true},
		{"V4.6.0", "V4.6.1", true},
		{"V4.9.9", "V5.0.0", true},
		{"V4.10.0", "V4.9.0", false}, // numeric, not lexical
		{"V4.9.0", "V4.10.0", true},
		{"V4.7.0", "V4.7.0", false},
		{"V4.7.0", "V4.6.0", false}, // a downgrade is never an update
	}
	for _, c := range cases {
		got, err := Newer(c.cur, c.cand)
		if err != nil || got != c.want {
			t.Errorf("Newer(%s,%s) = %v,%v; want %v", c.cur, c.cand, got, err, c.want)
		}
	}
	if got, err := Newer("dev build", "V9.0.0"); got || err == nil {
		t.Errorf("an unreadable current version must not be offered an update: %v, %v", got, err)
	}
	if got, err := Newer("V4.6.0", "garbage"); got || err == nil {
		t.Errorf("an unreadable candidate must not be an update: %v, %v", got, err)
	}
}

// ── signatures ──────────────────────────────────────────────────────────────

func TestSignVerify(t *testing.T) {
	pub, priv, err := GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"hello":"world"}`)
	sig, err := Sign(priv, body)
	if err != nil {
		t.Fatal(err)
	}
	if err := Verify(pub, body, sig); err != nil {
		t.Fatalf("good signature rejected: %v", err)
	}
	if err := Verify(pub, body, sig+"\n"); err != nil {
		t.Fatalf("a trailing newline in the .sig file must be tolerated: %v", err)
	}

	tampered := append([]byte(nil), body...)
	tampered[2] ^= 1
	if !errors.Is(Verify(pub, tampered, sig), ErrBadSignature) {
		t.Error("tampered body accepted")
	}
	otherPub, _, _ := GenerateKey()
	if !errors.Is(Verify(otherPub, body, sig), ErrBadSignature) {
		t.Error("signature accepted under a different key")
	}
	for _, junk := range []string{"", "not base64!!", "AAAA", strings.Repeat("A", 200)} {
		if !errors.Is(Verify(pub, body, junk), ErrBadSignature) {
			t.Errorf("junk signature %q accepted", junk)
		}
	}
	if Verify(nil, body, sig) == nil {
		t.Error("a missing public key must fail closed")
	}
}

func TestVerify_SignatureOfTheBareBodyIsNotAManifestSignature(t *testing.T) {
	// Domain separation: a signature the key made over the raw bytes (for some
	// other purpose) must not verify as a manifest signature.
	pub, priv, _ := GenerateKey()
	body := []byte("payload")
	raw := ed25519.Sign(priv, body)
	if Verify(pub, body, b64(raw)) == nil {
		t.Fatal("a context-free signature was accepted")
	}
}

func TestKeyEncoding(t *testing.T) {
	pub, priv, _ := GenerateKey()
	p2, err := DecodePublic(EncodePublic(pub))
	if err != nil || !pub.Equal(p2) {
		t.Fatalf("public key round trip: %v", err)
	}
	k2, err := DecodePrivate(EncodePrivate(priv))
	if err != nil || !priv.Equal(k2) {
		t.Fatalf("private key round trip: %v", err)
	}
	if _, err := DecodePublic("AAAA"); err == nil {
		t.Error("short public key accepted")
	}
	if _, err := DecodePrivate(EncodePublic(pub)); err == nil {
		t.Error("a public key accepted as a private key")
	}
}

// ── manifests ───────────────────────────────────────────────────────────────

const goodSum = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

func appManifestJSON(mut func(map[string]any)) []byte {
	m := map[string]any{
		"schema": 1, "type": "app", "version": "V4.7.0", "platform": "linux-amd64",
		"kind": "archive", "format": "tar.gz",
		"url": "https://data.memocpp.com/memo.tar.gz", "sha256": goodSum, "size": 1234, "root": "Memo",
	}
	if mut != nil {
		mut(m)
	}
	b, _ := json.Marshal(m)
	return b
}

func TestParseAppManifest(t *testing.T) {
	if _, err := ParseAppManifest(appManifestJSON(nil)); err != nil {
		t.Fatalf("good manifest rejected: %v", err)
	}
	bad := map[string]func(map[string]any){
		"wrong schema":    func(m map[string]any) { m["schema"] = 2 },
		"wrong type":      func(m map[string]any) { m["type"] = "component" },
		"bad version":     func(m map[string]any) { m["version"] = "latest" },
		"bad platform":    func(m map[string]any) { m["platform"] = "plan9-386" },
		"http url":        func(m map[string]any) { m["url"] = "http://evil.example/x.tar.gz" },
		"url with creds":  func(m map[string]any) { m["url"] = "https://a:b@data.memocpp.com/x" },
		"file url":        func(m map[string]any) { m["url"] = "file:///etc/passwd" },
		"short sha":       func(m map[string]any) { m["sha256"] = "abcd" },
		"upper sha":       func(m map[string]any) { m["sha256"] = strings.ToUpper(goodSum) },
		"zero size":       func(m map[string]any) { m["size"] = 0 },
		"huge size":       func(m map[string]any) { m["size"] = int64(1) << 40 },
		"unknown kind":    func(m map[string]any) { m["kind"] = "script" },
		"exe on linux":    func(m map[string]any) { m["kind"] = "installer"; m["format"] = "exe" },
		"rar":             func(m map[string]any) { m["format"] = "rar" },
		"root traversal":  func(m map[string]any) { m["root"] = "../x" },
		"root with slash": func(m map[string]any) { m["root"] = "a/b" },
	}
	for name, mut := range bad {
		if _, err := ParseAppManifest(appManifestJSON(mut)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
	if _, err := ParseAppManifest(append(appManifestJSON(nil), []byte(`{"x":1}`)...)); err == nil {
		t.Error("trailing data accepted")
	}
	// A loopback http URL is allowed (the test servers); nothing else is.
	if _, err := ParseAppManifest(appManifestJSON(func(m map[string]any) { m["url"] = "http://127.0.0.1:9/x.tar.gz" })); err != nil {
		t.Errorf("loopback http rejected: %v", err)
	}
	// Unknown extra fields are fine: the signature covers them.
	if _, err := ParseAppManifest(appManifestJSON(func(m map[string]any) { m["future"] = true })); err != nil {
		t.Errorf("unknown field rejected: %v", err)
	}
	// Windows installer.
	if _, err := ParseAppManifest(appManifestJSON(func(m map[string]any) {
		m["platform"] = "windows-amd64"
		m["kind"] = "installer"
		m["format"] = "exe"
		m["url"] = "https://data.memocpp.com/memo.exe"
	})); err != nil {
		t.Errorf("windows installer rejected: %v", err)
	}
}

func compManifestJSON(mut func(map[string]any)) []byte {
	m := map[string]any{
		"schema": 1, "type": "component", "component": "llama", "version": "b11456", "requires_app": "4.7.0",
		"files": map[string]any{
			"linux/cpu": map[string]any{"url": "https://data.memocpp.com/update/components/llama/b11456/linux-cpu.tar.gz", "sha256": goodSum, "size": 99, "format": "tar.gz"},
		},
	}
	if mut != nil {
		mut(m)
	}
	b, _ := json.Marshal(m)
	return b
}

func TestParseComponentManifest(t *testing.T) {
	if _, err := ParseComponentManifest(compManifestJSON(nil)); err != nil {
		t.Fatalf("good manifest rejected: %v", err)
	}
	bad := map[string]func(map[string]any){
		"unknown component": func(m map[string]any) { m["component"] = "sqlite" },
		"traversal version": func(m map[string]any) { m["version"] = "../../x" },
		"bad requires_app":  func(m map[string]any) { m["requires_app"] = "new" },
		"no files":          func(m map[string]any) { m["files"] = map[string]any{} },
		"bad folder": func(m map[string]any) {
			m["files"] = map[string]any{"../etc": map[string]any{"url": "https://x.example/a", "sha256": goodSum, "size": 1, "format": "zip"}}
		},
		"folder w/o os": func(m map[string]any) {
			m["files"] = map[string]any{"cpu": map[string]any{"url": "https://x.example/a", "sha256": goodSum, "size": 1, "format": "zip"}}
		},
		"bad file format": func(m map[string]any) {
			m["files"] = map[string]any{"linux/cpu": map[string]any{"url": "https://x.example/a", "sha256": goodSum, "size": 1, "format": "exe"}}
		},
	}
	for name, mut := range bad {
		if _, err := ParseComponentManifest(compManifestJSON(mut)); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// ── client: beacon + signed manifests ───────────────────────────────────────

type fakeSite struct {
	pub  ed25519.PublicKey
	priv ed25519.PrivateKey
	mux  *http.ServeMux
	srv  *httptest.Server
	hits atomic.Int64
}

func newSite(t *testing.T) *fakeSite {
	t.Helper()
	pub, priv, _ := GenerateKey()
	s := &fakeSite{pub: pub, priv: priv, mux: http.NewServeMux()}
	s.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.hits.Add(1)
		s.mux.ServeHTTP(w, r)
	}))
	t.Cleanup(s.srv.Close)
	return s
}

func (s *fakeSite) serve(path string, body []byte) {
	s.mux.HandleFunc(path, func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(body) })
}

func (s *fakeSite) serveSigned(t *testing.T, path string, body []byte) {
	t.Helper()
	sig, err := Sign(s.priv, body)
	if err != nil {
		t.Fatal(err)
	}
	s.serve(path, body)
	s.serve(path+".sig", []byte(sig))
}

func (s *fakeSite) client() *Client {
	c := NewClient(s.pub)
	c.BeaconURL = s.srv.URL + "/version.json"
	c.BaseURL = s.srv.URL + "/update"
	return c
}

func TestFetchBeacon_SanitisesWhatItShows(t *testing.T) {
	s := newSite(t)
	s.serve("/version.json", []byte(`{
		"version":" V4.7.0 ",
		"mini_version_note_tr":"kısa\u0007 not\nikinci satır",
		"mini_version_note_en":"`+strings.Repeat("x", 5000)+`",
		"required_update": true,
		"components": {"llama":"b11456","whisper":"../../etc","cliproxy":"v8.0.26","sqlite":"1.0.0"}
	}`))
	bc, err := s.client().FetchBeacon(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if bc.Version != "V4.7.0" || !bc.Required {
		t.Errorf("beacon = %+v", bc)
	}
	if strings.ContainsAny(bc.NoteTR, "\x07\n") || bc.NoteTR != "kısa not ikinci satır" {
		t.Errorf("note not cleaned: %q", bc.NoteTR)
	}
	if n := len([]rune(bc.NoteEN)); n != maxNoteRunes {
		t.Errorf("note length = %d, want %d", n, maxNoteRunes)
	}
	if len(bc.Components) != 2 || bc.Components["llama"] != "b11456" || bc.Components["cliproxy"] != "v8.0.26" {
		t.Errorf("components not filtered: %v", bc.Components)
	}
}

func TestFetchBeacon_OldStyleFileStillParses(t *testing.T) {
	s := newSite(t)
	s.serve("/version.json", []byte("{\n    \"version\": \"V4.6.0\"\n}\n"))
	bc, err := s.client().FetchBeacon(context.Background())
	if err != nil || bc.Version != "V4.6.0" || bc.Required || len(bc.Components) != 0 {
		t.Fatalf("%+v, %v", bc, err)
	}
}

func TestFetchBeacon_RejectsGarbageAndOversize(t *testing.T) {
	s := newSite(t)
	s.serve("/version.json", []byte("<html>not json</html>"))
	if _, err := s.client().FetchBeacon(context.Background()); err == nil {
		t.Error("html accepted")
	}
	s2 := newSite(t)
	s2.serve("/version.json", []byte(`{"version":"V1.0.0","pad":"`+strings.Repeat("a", maxBeaconBytes)+`"}`))
	if _, err := s2.client().FetchBeacon(context.Background()); err == nil {
		t.Error("oversized beacon accepted")
	}
	s3 := newSite(t) // 404
	if _, err := s3.client().FetchBeacon(context.Background()); err == nil {
		t.Error("404 accepted")
	}
}

func TestAppManifest_SignedOnly(t *testing.T) {
	s := newSite(t)
	c := s.client()
	v, _ := ParseVersion("V4.7.0")
	body := appManifestJSON(nil)
	s.serveSigned(t, "/update/app/4.7.0/linux-amd64.json", body)

	m, err := c.AppManifest(context.Background(), v, "linux-amd64")
	if err != nil || m.Version != "V4.7.0" {
		t.Fatalf("%+v, %v", m, err)
	}
}

func TestAppManifest_RejectsWhatIsNotSignedByOurKey(t *testing.T) {
	s := newSite(t)
	v, _ := ParseVersion("V4.7.0")
	// Served with a signature from a DIFFERENT key (an attacker who owns the bucket).
	_, evil, _ := GenerateKey()
	body := appManifestJSON(nil)
	sig, _ := Sign(evil, body)
	s.serve("/update/app/4.7.0/linux-amd64.json", body)
	s.serve("/update/app/4.7.0/linux-amd64.json.sig", []byte(sig))
	if _, err := s.client().AppManifest(context.Background(), v, "linux-amd64"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("foreign signature: %v", err)
	}
	// Tampered after signing.
	s2 := newSite(t)
	sig2, _ := Sign(s2.priv, body)
	tampered := appManifestJSON(func(m map[string]any) { m["url"] = "https://evil.example/memo.tar.gz" })
	s2.serve("/update/app/4.7.0/linux-amd64.json", tampered)
	s2.serve("/update/app/4.7.0/linux-amd64.json.sig", []byte(sig2))
	if _, err := s2.client().AppManifest(context.Background(), v, "linux-amd64"); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("tampered manifest: %v", err)
	}
	// No .sig at all.
	s3 := newSite(t)
	s3.serve("/update/app/4.7.0/linux-amd64.json", body)
	if _, err := s3.client().AppManifest(context.Background(), v, "linux-amd64"); err == nil {
		t.Fatal("a manifest with no signature file was accepted")
	}
}

func TestAppManifest_ReplayOfAnOlderSignedManifestIsRefused(t *testing.T) {
	// The attacker can't forge a signature, but can serve a genuine old one
	// (4.6.0's manifest) at the 4.7.0 address.
	s := newSite(t)
	old := appManifestJSON(func(m map[string]any) { m["version"] = "V4.6.0" })
	s.serveSigned(t, "/update/app/4.7.0/linux-amd64.json", old)
	v, _ := ParseVersion("V4.7.0")
	if _, err := s.client().AppManifest(context.Background(), v, "linux-amd64"); err == nil {
		t.Fatal("replayed older manifest accepted as the newer release")
	}
	// Same trick across platforms.
	s2 := newSite(t)
	s2.serveSigned(t, "/update/app/4.7.0/linux-amd64.json", appManifestJSON(func(m map[string]any) { m["platform"] = "darwin-arm64" }))
	if _, err := s2.client().AppManifest(context.Background(), v, "linux-amd64"); err == nil {
		t.Fatal("manifest for another platform accepted")
	}
}

func TestComponentManifest_AndURLValidation(t *testing.T) {
	s := newSite(t)
	c := s.client()
	s.serveSigned(t, "/update/components/llama/b11456/manifest.json", compManifestJSON(nil))
	m, err := c.ComponentManifest(context.Background(), "llama", "b11456")
	if err != nil || m.Version != "b11456" {
		t.Fatalf("%+v, %v", m, err)
	}
	// The name and version come from the unsigned beacon: they must never reach a URL unvalidated.
	for _, bad := range [][2]string{{"llama", "../../x"}, {"../llama", "b1"}, {"llama", "b1/../b2"}, {"sqlite", "1.0"}, {"llama", ""}} {
		if _, err := c.ComponentManifest(context.Background(), bad[0], bad[1]); err == nil {
			t.Errorf("%v accepted", bad)
		}
	}
	// A signed manifest for a different release must not stand in for the one asked for.
	s2 := newSite(t)
	s2.serveSigned(t, "/update/components/llama/b11456/manifest.json", compManifestJSON(func(m map[string]any) { m["version"] = "b9441" }))
	if _, err := s2.client().ComponentManifest(context.Background(), "llama", "b11456"); err == nil {
		t.Error("mismatched release accepted")
	}
}

func TestClient_RefusesPlainHTTPToARemoteHost(t *testing.T) {
	c := NewClient(nil)
	c.BeaconURL = "http://example.com/version.json"
	if _, err := c.FetchBeacon(context.Background()); err == nil {
		t.Fatal("http beacon on a remote host accepted")
	}
}

func TestClient_RefusesARedirectToPlainHTTP(t *testing.T) {
	s := newSite(t)
	s.mux.HandleFunc("/version.json", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://example.com/evil.json", http.StatusFound)
	})
	if _, err := s.client().FetchBeacon(context.Background()); err == nil {
		t.Fatal("redirect to remote http followed")
	}
}

// ── download ────────────────────────────────────────────────────────────────

func payload(n int) []byte {
	b := make([]byte, n)
	for i := range b {
		b[i] = byte(i*7 + i/251)
	}
	return b
}

func sumHex(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

func TestDownload_Succeeds(t *testing.T) {
	data := payload(300_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", timeZero, bytesReader(data))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	var last Progress
	err := Download(context.Background(), DownloadOptions{
		URL: srv.URL + "/f", Dest: dest, SHA256: sumHex(data), Size: int64(len(data)),
		OnProgress: func(p Progress) { last = p },
	})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(dest)
	if sumHex(got) != sumHex(data) {
		t.Fatal("content differs")
	}
	if last.Done != int64(len(data)) || last.Total != int64(len(data)) {
		t.Errorf("final progress = %+v", last)
	}
	if _, err := os.Stat(dest + ".part"); err == nil {
		t.Error("the .part file was left behind")
	}
}

func TestDownload_WrongBytesAreDeletedAndRefused(t *testing.T) {
	data := payload(50_000)
	evil := append([]byte(nil), data...)
	evil[1000] ^= 0xff // same length, different content
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(evil) }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	err := Download(context.Background(), DownloadOptions{URL: srv.URL + "/f", Dest: dest, SHA256: sumHex(data), Size: int64(len(data))})
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if _, e := os.Stat(dest); e == nil {
		t.Error("a file that failed verification was installed at Dest")
	}
	if _, e := os.Stat(dest + ".part"); e == nil {
		t.Error("a file that failed verification was kept for resuming")
	}
}

func TestDownload_LengthLiesAreCaught(t *testing.T) {
	data := payload(10_000)
	// Server sends more than the manifest says.
	long := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(append(data, 1, 2, 3)) }))
	defer long.Close()
	if err := Download(context.Background(), DownloadOptions{URL: long.URL, Dest: filepath.Join(t.TempDir(), "a"), SHA256: sumHex(data), Size: int64(len(data))}); err == nil {
		t.Error("a longer body was accepted")
	}
	// Server sends less (connection cut short).
	short := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "10000")
		_, _ = w.Write(data[:4000])
	}))
	defer short.Close()
	dest := filepath.Join(t.TempDir(), "b")
	if err := Download(context.Background(), DownloadOptions{URL: short.URL, Dest: dest, SHA256: sumHex(data), Size: int64(len(data))}); err == nil {
		t.Error("a truncated body was accepted")
	}
	if _, e := os.Stat(dest); e == nil {
		t.Error("truncated download installed")
	}
}

func TestDownload_ResumesFromAPartialFile(t *testing.T) {
	data := payload(200_000)
	var sawRange atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRange.Store(r.Header.Get("Range"))
		http.ServeContent(w, r, "f", timeZero, bytesReader(data))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(dest+".part", data[:80_000], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: sumHex(data), Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if got, _ := sawRange.Load().(string); got != "bytes=80000-" {
		t.Errorf("Range header = %q", got)
	}
	b, _ := os.ReadFile(dest)
	if sumHex(b) != sumHex(data) {
		t.Fatal("resumed file differs")
	}
}

func TestDownload_ServerThatIgnoresRangeRestarts(t *testing.T) {
	data := payload(100_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write(data) })) // always 200, whole body
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	if err := os.WriteFile(dest+".part", data[:30_000], 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: sumHex(data), Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(dest)
	if sumHex(b) != sumHex(data) {
		t.Fatal("file differs after a Range-less restart")
	}
}

func TestDownload_CorruptPartialIsCaughtByTheDigest(t *testing.T) {
	data := payload(100_000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.ServeContent(w, r, "f", timeZero, bytesReader(data))
	}))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	bad := append([]byte(nil), data[:40_000]...)
	bad[10] ^= 1 // a flipped byte in what was "already downloaded"
	_ = os.WriteFile(dest+".part", bad, 0o600)
	err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: sumHex(data), Size: int64(len(data))})
	if !errors.Is(err, ErrChecksum) {
		t.Fatalf("err = %v, want ErrChecksum", err)
	}
	if _, e := os.Stat(dest + ".part"); e == nil {
		t.Error("the corrupt partial was kept; the next try would fail forever")
	}
}

func TestDownload_AlreadyCompletePartialIsJustVerified(t *testing.T) {
	data := payload(5_000)
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hit = true }))
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f.bin")
	_ = os.WriteFile(dest+".part", data, 0o600)
	if err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: sumHex(data), Size: int64(len(data))}); err != nil {
		t.Fatal(err)
	}
	if hit {
		t.Error("the server was contacted although the file was complete")
	}
}

func TestDownload_HTTPErrorsAndBadOptions(t *testing.T) {
	srv := httptest.NewServer(http.NotFoundHandler())
	defer srv.Close()
	dest := filepath.Join(t.TempDir(), "f")
	if err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: goodSum, Size: 10}); err == nil {
		t.Error("404 accepted")
	}
	if err := Download(context.Background(), DownloadOptions{URL: "http://example.com/x", Dest: dest, SHA256: goodSum, Size: 10}); err == nil {
		t.Error("remote http accepted")
	}
	if err := Download(context.Background(), DownloadOptions{URL: srv.URL, Dest: dest, SHA256: "xyz", Size: 10}); err == nil {
		t.Error("bad digest accepted")
	}
}

func TestDownload_CancelStopsIt(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "1000000")
		_, _ = w.Write(make([]byte, 1000))
		w.(http.Flusher).Flush()
		cancel()
		<-r.Context().Done()
	}))
	defer srv.Close()
	err := Download(ctx, DownloadOptions{URL: srv.URL, Dest: filepath.Join(t.TempDir(), "f"), SHA256: goodSum, Size: 1_000_000})
	if err == nil {
		t.Fatal("a cancelled download reported success")
	}
}

func TestEnsureSpace(t *testing.T) {
	if err := EnsureSpace(t.TempDir(), 1); err != nil {
		t.Errorf("1 byte should fit: %v", err)
	}
	if err := EnsureSpace(t.TempDir(), 1<<62); !errors.Is(err, ErrNoSpace) {
		t.Errorf("an absurd request must be refused with ErrNoSpace, got %v", err)
	}
}
