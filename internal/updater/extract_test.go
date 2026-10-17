// SPDX-License-Identifier: AGPL-3.0-or-later

package updater

import (
	"archive/tar"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func readFile(t *testing.T, p string) string {
	t.Helper()
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestExtract_TarGzAndZipRoundTrip(t *testing.T) {
	ms := []member{
		{name: "Memo/", typ: tar.TypeDir},
		{name: "Memo/memo-backend", body: "BIN", mode: 0o755},
		{name: "Memo/binaries/linux/cpu/LLAMA_VERSION", body: "b9441\n"},
		{name: "Memo/lib/libx.so.1", body: "so"},
	}
	// zip members for directories are named with a trailing slash.
	zms := []member{
		{name: "Memo/memo-backend", body: "BIN", mode: 0o755},
		{name: "Memo/binaries/linux/cpu/LLAMA_VERSION", body: "b9441\n"},
		{name: "Memo/lib/libx.so.1", body: "so"},
	}
	for _, c := range []struct {
		format, path string
	}{{"tar.gz", buildTarGz(t, ms)}, {"zip", buildZip(t, zms)}} {
		t.Run(c.format, func(t *testing.T) {
			dst := t.TempDir()
			if err := Extract(c.path, c.format, dst, ExtractLimits{}); err != nil {
				t.Fatal(err)
			}
			if got := readFile(t, filepath.Join(dst, "Memo", "binaries", "linux", "cpu", "LLAMA_VERSION")); got != "b9441\n" {
				t.Errorf("content = %q", got)
			}
			if runtime.GOOS != "windows" {
				st, err := os.Stat(filepath.Join(dst, "Memo", "memo-backend"))
				if err != nil || st.Mode()&0o111 == 0 {
					t.Errorf("the executable bit was lost: %v %v", st, err)
				}
			}
		})
	}
}

func TestExtract_RefusesPathsThatEscape(t *testing.T) {
	for _, name := range []string{"../evil", "a/../../evil", "/etc/evil", "Memo/../../evil", `..\evil`} {
		for _, kind := range []string{"tar.gz", "zip"} {
			var path string
			ms := []member{{name: name, body: "x"}}
			if kind == "tar.gz" {
				path = buildTarGz(t, ms)
			} else {
				path = buildZip(t, ms)
			}
			parent := t.TempDir()
			dst := filepath.Join(parent, "dst")
			if err := os.Mkdir(dst, 0o755); err != nil {
				t.Fatal(err)
			}
			if err := Extract(path, kind, dst, ExtractLimits{}); err == nil {
				t.Errorf("%s %q: accepted", kind, name)
			}
			if _, err := os.Stat(filepath.Join(parent, "evil")); err == nil {
				t.Errorf("%s %q: wrote outside the destination", kind, name)
			}
		}
	}
}

func TestExtract_RefusesLinksThatLeave(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	cases := map[string][]member{
		"absolute target": {{name: "l", typ: tar.TypeSymlink, link: "/etc/passwd"}},
		"dotdot target":   {{name: "l", typ: tar.TypeSymlink, link: "../outside"}},
		"deep dotdot":     {{name: "a/b/l", typ: tar.TypeSymlink, link: "../../../x"}},
		"hard link":       {{name: "h", typ: tar.TypeLink, link: "/etc/passwd"}},
		"fifo":            {{name: "p", typ: tar.TypeFifo}},
		"char device":     {{name: "c", typ: tar.TypeChar}},
	}
	for name, ms := range cases {
		parent := t.TempDir()
		dst := filepath.Join(parent, "dst")
		_ = os.Mkdir(dst, 0o755)
		if err := Extract(buildTarGz(t, ms), "tar.gz", dst, ExtractLimits{}); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// The attack a purely lexical link check misses: an earlier member makes
// "x/a" a link back to the root, so a later "x/a/l -> ../../y" looks like it
// stays inside (x/a/../../y == y) but really resolves above the destination.
func TestExtract_ChainedLinkCannotWriteOutside(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	ms := []member{
		{name: "x/", typ: tar.TypeDir},
		{name: "x/a", typ: tar.TypeSymlink, link: ".."},
		{name: "x/a/l", typ: tar.TypeSymlink, link: "../../y"},
		{name: "x/a/l/pwned", body: "gotcha"},
	}
	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")
	_ = os.Mkdir(dst, 0o755)
	// The victim directory the link would reach really exists (two levels above
	// dst), so a missing guard would let the write succeed rather than fail by
	// accident on a dangling link.
	victim := filepath.Join(filepath.Dir(parent), "y")
	if err := os.MkdirAll(victim, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(victim) })

	err := Extract(buildTarGz(t, ms), "tar.gz", dst, ExtractLimits{})
	if err == nil {
		t.Fatal("chained-link archive accepted")
	}
	if _, e := os.Lstat(filepath.Join(victim, "pwned")); e == nil {
		t.Fatal("the archive wrote outside the destination")
	}
}

func TestExtract_FileCannotBeWrittenThroughAPlantedLink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	outside := t.TempDir() // a real directory elsewhere
	parent := t.TempDir()
	dst := filepath.Join(parent, "dst")
	_ = os.Mkdir(dst, 0o755)
	// A link inside the destination that points at an inside dir is allowed...
	ms := []member{
		{name: "real/", typ: tar.TypeDir},
		{name: "link", typ: tar.TypeSymlink, link: "real"},
		{name: "link/ok", body: "fine"},
	}
	if err := Extract(buildTarGz(t, ms), "tar.gz", dst, ExtractLimits{}); err != nil {
		t.Fatalf("a harmless inside link was refused: %v", err)
	}
	if readFile(t, filepath.Join(dst, "real", "ok")) != "fine" {
		t.Error("file did not land in real/")
	}
	// ...but a link planted by hand to the outside (e.g. left over from a
	// previous run) must not be followed by a later member.
	dst2 := filepath.Join(parent, "dst2")
	_ = os.Mkdir(dst2, 0o755)
	if err := os.Symlink(outside, filepath.Join(dst2, "trap")); err != nil {
		t.Fatal(err)
	}
	err := Extract(buildTarGz(t, []member{{name: "trap/evil", body: "x"}}), "tar.gz", dst2, ExtractLimits{})
	if err == nil {
		t.Error("wrote through a symlink to the outside")
	}
	if _, e := os.Stat(filepath.Join(outside, "evil")); e == nil {
		t.Error("file escaped via a pre-existing link")
	}
}

func TestExtract_ZipLinkEscapeRefused(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	ms := []member{{name: "l", zipSym: true, link: "../../outside"}}
	if err := Extract(buildZip(t, ms), "zip", t.TempDir(), ExtractLimits{}); err == nil {
		t.Fatal("zip link escape accepted")
	}
	ok := []member{
		{name: "Memo.app/Contents/Frameworks/F.framework/Versions/A/F", body: "bin"},
		{name: "Memo.app/Contents/Frameworks/F.framework/Versions/Current", zipSym: true, link: "A"},
	}
	dst := t.TempDir()
	if err := Extract(buildZip(t, ok), "zip", dst, ExtractLimits{}); err != nil {
		t.Fatalf("a normal framework link was refused: %v", err)
	}
	if readFile(t, filepath.Join(dst, "Memo.app/Contents/Frameworks/F.framework/Versions/Current/F")) != "bin" {
		t.Error("framework link does not resolve")
	}
}

func TestExtract_Limits(t *testing.T) {
	big := strings.Repeat("a", 5000)
	ms := []member{{name: "a", body: big}, {name: "b", body: big}}
	if err := Extract(buildTarGz(t, ms), "tar.gz", t.TempDir(), ExtractLimits{MaxFiles: 10, MaxBytes: 6000}); err == nil {
		t.Error("tar over the byte limit accepted")
	}
	if err := Extract(buildTarGz(t, ms), "tar.gz", t.TempDir(), ExtractLimits{MaxFiles: 1, MaxBytes: 1 << 20}); err == nil {
		t.Error("tar over the file limit accepted")
	}
	if err := Extract(buildZip(t, ms), "zip", t.TempDir(), ExtractLimits{MaxFiles: 10, MaxBytes: 6000}); err == nil {
		t.Error("zip over the byte limit accepted")
	}
	if err := Extract("x", "rar", t.TempDir(), ExtractLimits{}); err == nil {
		t.Error("unknown format accepted")
	}
}

func TestExtract_OverwritesAnExistingFileInsteadOfFollowingIt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	outside := filepath.Join(t.TempDir(), "victim")
	_ = os.WriteFile(outside, []byte("original"), 0o644)
	dst := t.TempDir()
	// A leaf symlink already sits where the archive wants a regular file.
	if err := os.Symlink(outside, filepath.Join(dst, "f")); err != nil {
		t.Fatal(err)
	}
	if err := Extract(buildTarGz(t, []member{{name: "f", body: "new"}}), "tar.gz", dst, ExtractLimits{}); err != nil {
		t.Fatal(err)
	}
	if readFile(t, outside) != "original" {
		t.Error("the file behind a pre-existing symlink was overwritten")
	}
	if readFile(t, filepath.Join(dst, "f")) != "new" {
		t.Error("the member was not written")
	}
}

func TestTarget_RejectsNULAndOddNames(t *testing.T) {
	// A tar header cannot carry a NUL, so this is checked on the resolver directly.
	root := t.TempDir()
	for _, name := range []string{"a/\x00b", "", "/abs", "../x", "a/../../x"} {
		if _, err := target(root, name); err == nil {
			t.Errorf("%q accepted", name)
		}
	}
	if p, err := target(root, "a/b/../c"); err != nil || p != filepath.Join(root, "a", "c") {
		t.Errorf("a/b/../c -> %q, %v", p, err)
	}
}
