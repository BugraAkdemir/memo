// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"memo/internal/fileutil"
)

// SkillsDirName is the folder under .memo that holds a project's skills.
const SkillsDirName = "skills"

const (
	maxSkillFileBytes    = 256 << 10 // SKILL.md may reach a prompt: cap it
	maxArchiveBytes      = 25 << 20  // compressed download
	maxUnpackedBytes     = 60 << 20  // everything in the archive, uncompressed
	maxArchiveEntries    = 5000
	maxSingleFileBytes   = 5 << 20
	maxSkillTotalBytes   = 5 << 20 // what one installed skill may take on disk
	maxSkillDepthBelowTo = 4
	defaultCodeloadBase  = "https://codeload.github.com"
)

var (
	// ErrNotSetUp means the folder has no .memo yet.
	ErrNotSetUp = errors.New("project: this folder is not set up as a Memo project yet")
	// ErrSkillExists means a skill with that name is already installed here.
	ErrSkillExists = errors.New("project: a skill with that name is already installed")
	// ErrSkillNotFound means no such project skill.
	ErrSkillNotFound = errors.New("project: no such project skill")
)

// SkillMeta is what a SkillParser learns from a SKILL.md.
type SkillMeta struct {
	Name        string
	Description string
	// HasTools is true when the manifest declares tools. A project skill's tools
	// are never registered (see the package docs of skills in internal/app); the
	// flag only lets the UI say so.
	HasTools bool
}

// SkillParser reads a SKILL.md. internal/app supplies the real parser
// (internal/skill) so a skill is accepted or refused exactly as the rest of
// Memo would; this package stays free of that dependency.
type SkillParser func(data []byte, dir string) (SkillMeta, error)

// SkillsDir returns `<root>/.memo/skills`.
func SkillsDir(root string) string { return filepath.Join(Dir(root), SkillsDirName) }

// SkillInfo is an installed project skill.
type SkillInfo struct {
	SkillMeta
	// Dir is the skill's folder name (and the name it is activated by).
	Dir string `json:"dir"`
}

// ListSkills returns the skills installed under root, sorted by name. A folder
// that is a symlink, has no readable SKILL.md or fails parse is skipped.
func ListSkills(root string, parse SkillParser) []SkillInfo {
	base := SkillsDir(root)
	if st, err := os.Lstat(base); err != nil || !st.IsDir() {
		return nil // missing, or a symlink: never follow it
	}
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil
	}
	var out []SkillInfo
	for _, e := range entries {
		if !e.IsDir() || e.Type()&os.ModeSymlink != 0 || !validSkillName(e.Name()) {
			continue
		}
		data, err := ReadSkillFile(root, e.Name())
		if err != nil {
			continue
		}
		meta, err := parse(data, filepath.Join(base, e.Name()))
		if err != nil {
			continue
		}
		out = append(out, SkillInfo{SkillMeta: meta, Dir: e.Name()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Dir < out[j].Dir })
	return out
}

// ReadSkillFile returns the SKILL.md of project skill dir. It refuses symlinks
// and oversized files.
func ReadSkillFile(root, dir string) ([]byte, error) {
	if !validSkillName(dir) {
		return nil, ErrSkillNotFound
	}
	p := filepath.Join(SkillsDir(root), dir, "SKILL.md")
	st, err := os.Lstat(p)
	if err != nil {
		return nil, ErrSkillNotFound
	}
	if !st.Mode().IsRegular() || st.Size() > maxSkillFileBytes {
		return nil, fmt.Errorf("project: %s is not a regular SKILL.md of reasonable size", p)
	}
	return os.ReadFile(p)
}

// RemoveSkill deletes project skill dir.
func (m *Manager) RemoveSkill(projectPath, dir string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	root, info, err := m.rootForSkillsLocked(projectPath)
	if err != nil {
		return err
	}
	_ = info
	if !validSkillName(dir) {
		return ErrSkillNotFound
	}
	target := filepath.Join(SkillsDir(root), dir)
	st, err := os.Lstat(target)
	if err != nil || !st.IsDir() || st.Mode()&os.ModeSymlink != 0 {
		return ErrSkillNotFound
	}
	return os.RemoveAll(target)
}

// rootForSkillsLocked resolves projectPath to a set-up project's root.
func (m *Manager) rootForSkillsLocked(projectPath string) (string, *Info, error) {
	root, ok := m.guard.FindRoot(projectPath)
	if !ok {
		if _, err := m.guard.Check(projectPath); err != nil {
			return "", nil, err
		}
		return "", nil, ErrNotSetUp
	}
	info, err := ReadInfo(root)
	if err != nil {
		return "", nil, err
	}
	return root, info, nil
}

// InstallOptions configure InstallSkill.
type InstallOptions struct {
	// Parse reads a SKILL.md; required.
	Parse SkillParser
	// HTTP is the client to download with; nil uses a default with a timeout.
	HTTP *http.Client
	// BaseURL is the archive host; empty means GitHub's codeload. Tests point it
	// at a local server.
	BaseURL string
}

// InstallResult says what was installed.
type InstallResult struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	// Dir is the folder under .memo/skills the skill now lives in.
	Dir      string `json:"dir"`
	Source   string `json:"source"`
	Files    int    `json:"files"`
	HasTools bool   `json:"has_tools"`
}

// InstallSkill downloads a skill from GitHub into `<root>/.memo/skills/<name>`.
//
// Nothing is executed: the repository is fetched as a plain archive over HTTPS,
// unpacked in memory with strict limits (archive size, entry count, unpacked
// size, per-file size; symlinks, absolute and `..` paths refused), and only the
// chosen skill's files are written, without execute permission. The skill's
// instructions are used only when someone activates it; any tools it declares
// are ignored. An existing skill is never overwritten.
func (m *Manager) InstallSkill(ctx context.Context, projectPath string, src SkillSource, opt InstallOptions) (*InstallResult, error) {
	if opt.Parse == nil {
		return nil, errors.New("project: InstallSkill needs a SkillParser")
	}
	m.mu.Lock()
	root, _, err := m.rootForSkillsLocked(projectPath)
	m.mu.Unlock()
	if err != nil {
		return nil, err
	}

	files, err := fetchSkillFiles(ctx, src, opt)
	if err != nil {
		return nil, err
	}
	dir, meta, err := chooseSkill(files, src, opt.Parse)
	if err != nil {
		return nil, err
	}

	// A skill's folder is named after the skill, never after anything in the
	// archive.
	if !validSkillName(meta.Name) {
		return nil, fmt.Errorf("project: %q is not a usable skill name", meta.Name)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	skills := SkillsDir(root)
	if err := os.MkdirAll(skills, 0o755); err != nil {
		return nil, err
	}
	if st, err := os.Lstat(skills); err != nil || !st.IsDir() {
		return nil, fmt.Errorf("project: %s is not a plain folder", skills)
	}
	target := filepath.Join(skills, meta.Name)
	if exists(target) {
		return nil, fmt.Errorf("%w: %s (remove it first to reinstall)", ErrSkillExists, meta.Name)
	}

	tmp, err := os.MkdirTemp(skills, ".install-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tmp)

	n, total := 0, 0
	prefix := dir
	if prefix != "" {
		prefix += "/"
	}
	for rel, data := range files {
		if dir != "" && !strings.HasPrefix(rel, prefix) {
			continue
		}
		sub := strings.TrimPrefix(rel, prefix)
		if sub == "" || !safeRel(sub) {
			continue
		}
		total += len(data)
		if total > maxSkillTotalBytes {
			return nil, fmt.Errorf("project: the skill is larger than %d MB", maxSkillTotalBytes>>20)
		}
		dst := filepath.Join(tmp, filepath.FromSlash(sub))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return nil, err
		}
		if err := fileutil.AtomicWrite(dst, data, 0o644); err != nil {
			return nil, err
		}
		n++
	}
	if err := os.Rename(tmp, target); err != nil {
		return nil, err
	}
	return &InstallResult{
		Name: meta.Name, Description: meta.Description, Dir: meta.Name,
		Source: src.String(), Files: n, HasTools: meta.HasTools,
	}, nil
}

func (src SkillSource) archiveURL(base string) string {
	ref := src.Ref
	if ref == "" {
		ref = "HEAD"
	}
	return strings.TrimRight(base, "/") + "/" + url.PathEscape(src.Owner) + "/" + url.PathEscape(src.Repo) + "/zip/" + url.PathEscape(ref)
}

// fetchSkillFiles downloads src's archive and returns the files under src.Path,
// keyed by slash-separated path relative to the repository root.
func fetchSkillFiles(ctx context.Context, src SkillSource, opt InstallOptions) (map[string][]byte, error) {
	base, custom := opt.BaseURL, opt.BaseURL != ""
	if base == "" {
		base = defaultCodeloadBase
	}
	client := opt.HTTP
	if client == nil {
		client = &http.Client{Timeout: 90 * time.Second}
	}
	// Redirects are only followed to GitHub's own hosts (unless a test points the
	// base elsewhere) and only over https.
	cc := *client
	cc.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) > 5 {
			return errors.New("too many redirects")
		}
		if !custom {
			h := strings.ToLower(req.URL.Hostname())
			if req.URL.Scheme != "https" || !(h == "github.com" || strings.HasSuffix(h, ".github.com") || strings.HasSuffix(h, ".githubusercontent.com")) {
				return fmt.Errorf("refusing redirect to %s", req.URL.Host)
			}
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, src.archiveURL(base), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Memo")
	resp, err := cc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("project: download failed: %w", err)
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode == http.StatusNotFound:
		return nil, fmt.Errorf("project: %s/%s was not found on GitHub (is it public, and is the branch right?)", src.Owner, src.Repo)
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("project: GitHub answered %s", resp.Status)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveBytes+1))
	if err != nil {
		return nil, fmt.Errorf("project: download failed: %w", err)
	}
	if len(data) > maxArchiveBytes {
		return nil, fmt.Errorf("project: the repository archive is larger than %d MB", maxArchiveBytes>>20)
	}
	return unpackArchive(data, src.Path)
}

func safeRel(p string) bool {
	if p == "" || strings.HasPrefix(p, "/") || strings.Contains(p, "\\") || strings.Contains(p, "\x00") {
		return false
	}
	for _, seg := range strings.Split(p, "/") {
		if seg == "" || seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// unpackArchive reads a GitHub zip (every entry sits under one "<repo>-<ref>/"
// folder, which is stripped) and returns the regular files under only.
func unpackArchive(data []byte, only string) (map[string][]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return nil, fmt.Errorf("project: the download is not a valid archive: %w", err)
	}
	if len(zr.File) > maxArchiveEntries {
		return nil, fmt.Errorf("project: the archive has more than %d entries", maxArchiveEntries)
	}
	only = strings.Trim(only, "/")
	out := map[string][]byte{}
	var unpacked uint64
	for _, f := range zr.File {
		unpacked += f.UncompressedSize64
		if unpacked > maxUnpackedBytes {
			return nil, fmt.Errorf("project: the archive unpacks to more than %d MB", maxUnpackedBytes>>20)
		}
		if f.FileInfo().IsDir() {
			continue
		}
		// Anything that is not a plain file (symlink, device…) is refused.
		if !f.Mode().IsRegular() {
			continue
		}
		name := strings.ReplaceAll(f.Name, "\\", "/")
		i := strings.Index(name, "/")
		if i <= 0 { // no folder to strip, or an absolute path: refuse both
			continue
		}
		rel := name[i+1:]
		if !safeRel(rel) {
			continue
		}
		if only != "" && rel != only && !strings.HasPrefix(rel, only+"/") {
			continue
		}
		if f.UncompressedSize64 > maxSingleFileBytes {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, err
		}
		b, err := io.ReadAll(io.LimitReader(rc, maxSingleFileBytes+1))
		rc.Close()
		if err != nil || len(b) > maxSingleFileBytes {
			continue
		}
		out[rel] = b
	}
	return out, nil
}

// chooseSkill finds the skill to install among files. dir is its folder
// relative to the repository root ("" = the root itself).
func chooseSkill(files map[string][]byte, src SkillSource, parse SkillParser) (dir string, meta SkillMeta, err error) {
	var cands []skillCand
	for rel, data := range files {
		if path.Base(rel) != "SKILL.md" || len(data) > maxSkillFileBytes {
			continue
		}
		d := path.Dir(rel)
		if d == "." {
			d = ""
		}
		below := strings.Trim(strings.TrimPrefix(d, strings.Trim(src.Path, "/")), "/")
		if below != "" && strings.Count(below, "/")+1 > maxSkillDepthBelowTo {
			continue
		}
		m, perr := parse(data, d)
		if perr != nil {
			continue
		}
		cands = append(cands, skillCand{d, m})
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].dir < cands[j].dir })
	if len(cands) == 0 {
		return "", SkillMeta{}, fmt.Errorf("project: no valid SKILL.md found in %s", src)
	}
	if src.Skill != "" {
		for _, c := range cands {
			if strings.EqualFold(c.meta.Name, src.Skill) || strings.EqualFold(path.Base(c.dir), src.Skill) {
				return c.dir, c.meta, nil
			}
		}
		return "", SkillMeta{}, fmt.Errorf("project: no skill called %q in %s (found: %s)", src.Skill, src, candNames(cands))
	}
	if len(cands) > 1 {
		return "", SkillMeta{}, fmt.Errorf("project: %s holds %d skills (%s); say which one to install", src, len(cands), candNames(cands))
	}
	return cands[0].dir, cands[0].meta, nil
}

type skillCand struct {
	dir  string
	meta SkillMeta
}

func candNames(c []skillCand) string {
	out := make([]string, len(c))
	for i, x := range c {
		out[i] = x.meta.Name
	}
	return strings.Join(out, ", ")
}
