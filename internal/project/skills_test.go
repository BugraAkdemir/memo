// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// testParser is a stand-in for the real SKILL.md parser: front matter with
// name/description and a non-empty body.
func testParser(data []byte, _ string) (SkillMeta, error) {
	s := string(data)
	if !strings.HasPrefix(strings.TrimSpace(s), "---") {
		return SkillMeta{}, errors.New("no front matter")
	}
	var m SkillMeta
	for _, line := range strings.Split(s, "\n") {
		switch {
		case strings.HasPrefix(line, "name:"):
			m.Name = strings.TrimSpace(strings.TrimPrefix(line, "name:"))
		case strings.HasPrefix(line, "description:"):
			m.Description = strings.TrimSpace(strings.TrimPrefix(line, "description:"))
		case strings.HasPrefix(line, "tools:"):
			m.HasTools = true
		}
	}
	if m.Name == "" {
		return SkillMeta{}, errors.New("no name")
	}
	return m, nil
}

func skillMd(name, extra string) string {
	return "---\nname: " + name + "\ndescription: does " + name + "\n" + extra + "---\n\nUse " + name + " carefully.\n"
}

type zipFile struct {
	name string
	body string
	mode os.FileMode // 0 = regular
}

func makeZip(t *testing.T, files ...zipFile) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range files {
		h := &zip.FileHeader{Name: f.name, Method: zip.Deflate}
		if f.mode != 0 {
			h.SetMode(f.mode)
		} else {
			h.SetMode(0o644)
		}
		w, err := zw.CreateHeader(h)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write([]byte(f.body)); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// githubStub serves one archive at /<owner>/<repo>/zip/<ref> and records hits.
func githubStub(t *testing.T, archive []byte) (*httptest.Server, *[]string) {
	t.Helper()
	var hits []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits = append(hits, r.URL.Path)
		if archive == nil {
			http.NotFound(w, r)
			return
		}
		w.Write(archive)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func setUpProject(t *testing.T) (*Manager, string) {
	t.Helper()
	g, home := fakeHome(t)
	m := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	proj := mkdir(t, home, "app")
	if _, err := m.Scaffold(proj); err != nil {
		t.Fatal(err)
	}
	return m, proj
}

func TestParseSkillSource(t *testing.T) {
	ok := []struct {
		in   string
		want SkillSource
	}{
		{"owner/repo", SkillSource{Owner: "owner", Repo: "repo"}},
		{"owner/repo/skills/foo", SkillSource{Owner: "owner", Repo: "repo", Path: "skills/foo"}},
		{"github:owner/repo", SkillSource{Owner: "owner", Repo: "repo"}},
		{"git@github.com:owner/repo.git", SkillSource{Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo", SkillSource{Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo.git", SkillSource{Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo/", SkillSource{Owner: "owner", Repo: "repo"}},
		{"https://github.com/owner/repo/tree/main/skills/foo", SkillSource{Owner: "owner", Repo: "repo", Ref: "main", Path: "skills/foo"}},
		{"https://github.com/owner/repo/tree/v1.2", SkillSource{Owner: "owner", Repo: "repo", Ref: "v1.2"}},
		{"npx skills add owner/repo", SkillSource{Owner: "owner", Repo: "repo"}},
		{"npx skills add https://github.com/owner/repo --skill foo", SkillSource{Owner: "owner", Repo: "repo", Skill: "foo"}},
		{"npx -y skills@latest add owner/repo -s foo -g", SkillSource{Owner: "owner", Repo: "repo", Skill: "foo"}},
		{"bunx skills add owner/repo --skill=foo --agent claude-code", SkillSource{Owner: "owner", Repo: "repo", Skill: "foo"}},
		{"pnpm dlx skills add owner/repo", SkillSource{Owner: "owner", Repo: "repo"}},
		{"  npx   add-skill   owner/repo  ", SkillSource{Owner: "owner", Repo: "repo"}},
	}
	for _, c := range ok {
		got, err := ParseSkillSource(c.in)
		if err != nil || got != c.want {
			t.Errorf("ParseSkillSource(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
	bad := []string{
		"", "   ", "npx skills add", "owner", "owner/",
		"https://gitlab.com/owner/repo", "https://example.com/x/y", "ftp://github.com/o/r",
		"owner/repo extra/second", "owner/repo/../etc", "owner/repo//x", "o wner/repo",
		"npx skills add owner/repo --skill ../x",
	}
	for _, in := range bad {
		if got, err := ParseSkillSource(in); err == nil || !errors.Is(err, ErrSkillSource) {
			t.Errorf("ParseSkillSource(%q) = %+v, %v; want an ErrSkillSource", in, got, err)
		}
	}
}

func TestInstallSkill_DownloadsOnlyTheChosenSkillAsPlainFiles(t *testing.T) {
	m, proj := setUpProject(t)
	srv, hits := githubStub(t, makeZip(t,
		zipFile{name: "repo-main/README.md", body: "readme"},
		zipFile{name: "repo-main/skills/foo/SKILL.md", body: skillMd("foo", "")},
		zipFile{name: "repo-main/skills/foo/references/guide.md", body: "guide"},
		zipFile{name: "repo-main/skills/foo/run.sh", body: "#!/bin/sh\necho hi\n", mode: 0o755},
		zipFile{name: "repo-main/skills/other/SKILL.md", body: skillMd("other", "")},
	))

	res, err := m.InstallSkill(context.Background(), proj,
		SkillSource{Owner: "o", Repo: "r", Path: "skills/foo"}, InstallOptions{Parse: testParser, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.Name != "foo" || res.Files != 3 || res.HasTools {
		t.Fatalf("result = %+v", res)
	}
	if len(*hits) != 1 || (*hits)[0] != "/o/r/zip/HEAD" {
		t.Fatalf("requests = %v; want one archive download of the default branch", *hits)
	}
	base := filepath.Join(proj, ".memo", "skills", "foo")
	for _, f := range []string{"SKILL.md", "references/guide.md", "run.sh"} {
		if _, err := os.Stat(filepath.Join(base, f)); err != nil {
			t.Errorf("missing %s: %v", f, err)
		}
	}
	if exists(filepath.Join(proj, ".memo", "skills", "other")) || exists(filepath.Join(base, "..", "README.md")) {
		t.Error("files outside the chosen skill were written")
	}
	if runtime.GOOS != "windows" {
		if st, _ := os.Stat(filepath.Join(base, "run.sh")); st.Mode()&0o111 != 0 {
			t.Error("a downloaded script was made executable; Memo must never do that")
		}
	}
	if left, _ := filepath.Glob(filepath.Join(proj, ".memo", "skills", ".install-*")); len(left) != 0 {
		t.Errorf("temporary install folder left behind: %v", left)
	}

	list := ListSkills(proj, testParser)
	if len(list) != 1 || list[0].Name != "foo" || list[0].Dir != "foo" {
		t.Fatalf("ListSkills = %+v", list)
	}
}

func TestInstallSkill_SeveralSkillsNeedASelector(t *testing.T) {
	m, proj := setUpProject(t)
	srv, _ := githubStub(t, makeZip(t,
		zipFile{name: "repo-main/skills/alpha/SKILL.md", body: skillMd("alpha", "")},
		zipFile{name: "repo-main/skills/beta/SKILL.md", body: skillMd("beta", "")},
	))
	opt := InstallOptions{Parse: testParser, BaseURL: srv.URL}

	_, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r"}, opt)
	if err == nil || !strings.Contains(err.Error(), "alpha") || !strings.Contains(err.Error(), "beta") {
		t.Fatalf("without a selector: %v; want an error naming both skills", err)
	}
	if exists(filepath.Join(proj, ".memo", "skills", "alpha")) {
		t.Fatal("something was installed although the choice was ambiguous")
	}
	if _, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r", Skill: "nope"}, opt); err == nil {
		t.Fatal("an unknown skill name must fail")
	}
	res, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r", Skill: "beta"}, opt)
	if err != nil || res.Name != "beta" {
		t.Fatalf("with a selector: %+v, %v", res, err)
	}
	if exists(filepath.Join(proj, ".memo", "skills", "alpha")) {
		t.Fatal("the other skill was installed too")
	}
}

func TestInstallSkill_HostileArchivesCannotWriteOutsideTheSkill(t *testing.T) {
	m, proj := setUpProject(t)
	parent := filepath.Dir(proj)
	srv, _ := githubStub(t, makeZip(t,
		zipFile{name: "repo-main/SKILL.md", body: skillMd("evil", "")},
		zipFile{name: "repo-main/../../escape.txt", body: "x"},
		zipFile{name: "repo-main/a/../../escape2.txt", body: "x"},
		zipFile{name: "/abs/escape3.txt", body: "x"},
		zipFile{name: "repo-main/..\\escape4.txt", body: "x"},
		zipFile{name: "repo-main/link", body: "/etc/passwd", mode: os.ModeSymlink | 0o777},
		zipFile{name: "repo-main/ok.md", body: "fine"},
	))
	before := dirListing(t, parent)
	res, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r"}, InstallOptions{Parse: testParser, BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	if res.Files != 2 { // SKILL.md and ok.md only
		t.Errorf("Files = %d, want 2 (hostile entries must be skipped)", res.Files)
	}
	skillDir := filepath.Join(proj, ".memo", "skills", "evil")
	if exists(filepath.Join(skillDir, "link")) {
		t.Error("a symlink entry was materialised")
	}
	after := dirListing(t, parent)
	// Only the new skill folder (and its files) may have appeared.
	for _, line := range strings.Split(after, "\n") {
		if !strings.Contains(before, line) && !strings.HasPrefix(line, skillDir) && !strings.HasPrefix(line, filepath.Join(proj, ".memo", "skills")) {
			t.Errorf("unexpected new path outside the skill: %s", line)
		}
	}
}

func TestInstallSkill_ArchiveLimits(t *testing.T) {
	m, proj := setUpProject(t)
	// A "zip bomb": tiny on the wire, enormous unpacked.
	bomb := makeZip(t,
		zipFile{name: "repo-main/SKILL.md", body: skillMd("bomb", "")},
		zipFile{name: "repo-main/zeros-a.bin", body: strings.Repeat("\x00", 4<<20)},
	)
	big := make([]zipFile, 0, 20)
	big = append(big, zipFile{name: "repo-main/SKILL.md", body: skillMd("bomb", "")})
	for i := 0; i < 18; i++ {
		big = append(big, zipFile{name: "repo-main/z" + string(rune('a'+i)) + ".bin", body: strings.Repeat("\x00", 4<<20)})
	}
	for name, archive := range map[string][]byte{"a few small files are fine": bomb, "unpacks past the limit": makeZip(t, big...)} {
		srv, _ := githubStub(t, archive)
		_, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r"}, InstallOptions{Parse: testParser, BaseURL: srv.URL})
		wantErr := name == "unpacks past the limit"
		if wantErr && (err == nil || !strings.Contains(err.Error(), "unpacks")) {
			t.Errorf("%s: %v; want an unpack-limit error", name, err)
		}
		if !wantErr {
			// 4 MB file is under the per-file cap but the skill total (5 MB) still holds.
			if err != nil {
				t.Errorf("%s: %v", name, err)
			}
			_ = m.RemoveSkill(proj, "bomb")
		}
	}
	if exists(filepath.Join(proj, ".memo", "skills", "bomb")) {
		t.Error("a refused archive left a skill behind")
	}
}

func TestInstallSkill_NeverOverwritesAndNeedsASetUpProject(t *testing.T) {
	m, proj := setUpProject(t)
	srv, _ := githubStub(t, makeZip(t, zipFile{name: "repo-main/SKILL.md", body: skillMd("foo", "")}))
	opt := InstallOptions{Parse: testParser, BaseURL: srv.URL}
	src := SkillSource{Owner: "o", Repo: "r"}

	if _, err := m.InstallSkill(context.Background(), proj, src, opt); err != nil {
		t.Fatal(err)
	}
	mine := filepath.Join(proj, ".memo", "skills", "foo", "SKILL.md")
	if err := os.WriteFile(mine, []byte("my edit"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := m.InstallSkill(context.Background(), proj, src, opt); !errors.Is(err, ErrSkillExists) {
		t.Fatalf("second install = %v, want ErrSkillExists", err)
	}
	if read(t, mine) != "my edit" {
		t.Fatal("an existing skill was overwritten")
	}

	g, home := fakeHome(t)
	m2 := NewManager(g, filepath.Join(t.TempDir(), "projects.json"))
	bare := mkdir(t, home, "bare")
	if _, err := m2.InstallSkill(context.Background(), bare, src, opt); !errors.Is(err, ErrNotSetUp) {
		t.Fatalf("install into a folder that is not set up = %v, want ErrNotSetUp", err)
	}
	if exists(filepath.Join(bare, ".memo")) {
		t.Fatal("installing created .memo; only setup may do that")
	}
	if _, err := m2.InstallSkill(context.Background(), home, src, opt); !errors.Is(err, ErrForbiddenRoot) {
		t.Fatalf("install into home = %v, want ErrForbiddenRoot", err)
	}
}

func TestInstallSkill_AHostileSkillNameCannotChooseItsFolder(t *testing.T) {
	m, proj := setUpProject(t)
	srv, _ := githubStub(t, makeZip(t, zipFile{name: "repo-main/SKILL.md", body: skillMd("../../escape", "")}))
	if _, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r"},
		InstallOptions{Parse: testParser, BaseURL: srv.URL}); err == nil {
		t.Fatal("a skill named ../../escape was accepted")
	}
	if exists(filepath.Join(proj, ".memo", "escape")) || exists(filepath.Join(proj, "escape")) {
		t.Fatal("the skill escaped its folder")
	}
}

func TestInstallSkill_NotFoundAndToolsAreReportedPlainly(t *testing.T) {
	m, proj := setUpProject(t)
	srv, _ := githubStub(t, nil)
	_, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "gone"}, InstallOptions{Parse: testParser, BaseURL: srv.URL})
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("404: %v", err)
	}

	srv2, _ := githubStub(t, makeZip(t, zipFile{name: "repo-main/SKILL.md", body: skillMd("withtools", "tools:\n  - name: x\n")}))
	res, err := m.InstallSkill(context.Background(), proj, SkillSource{Owner: "o", Repo: "r"}, InstallOptions{Parse: testParser, BaseURL: srv2.URL})
	if err != nil || !res.HasTools {
		t.Fatalf("a skill that declares tools must say so: %+v, %v", res, err)
	}
}

func TestSkills_ListAndRemoveNeverFollowSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks need privileges on windows")
	}
	m, proj := setUpProject(t)
	skills := SkillsDir(proj)
	real := mkdir(t, skills, "real")
	if err := os.WriteFile(filepath.Join(real, "SKILL.md"), []byte(skillMd("real", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	outside := mkdir(t, filepath.Dir(proj), "outside-skill")
	if err := os.WriteFile(filepath.Join(outside, "SKILL.md"), []byte(skillMd("outside", "")), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(skills, "linked")); err != nil {
		t.Skip(err)
	}
	list := ListSkills(proj, testParser)
	if len(list) != 1 || list[0].Dir != "real" {
		t.Fatalf("ListSkills followed a symlink: %+v", list)
	}
	if err := m.RemoveSkill(proj, "linked"); !errors.Is(err, ErrSkillNotFound) {
		t.Fatalf("RemoveSkill(symlink) = %v", err)
	}
	if !exists(filepath.Join(outside, "SKILL.md")) {
		t.Fatal("removal reached through a symlink")
	}
	for _, bad := range []string{"../x", "", "a/b", ".."} {
		if err := m.RemoveSkill(proj, bad); !errors.Is(err, ErrSkillNotFound) {
			t.Errorf("RemoveSkill(%q) = %v", bad, err)
		}
	}
	if err := m.RemoveSkill(proj, "real"); err != nil {
		t.Fatal(err)
	}
	if exists(real) {
		t.Fatal("skill not removed")
	}
}
