// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

//go:embed templates/*
var templateFS embed.FS

// RulesVersion is bumped whenever templates/memo_agents.md changes.
const RulesVersion = 2

// ScaffoldResult says what Scaffold did, with paths relative to Root.
type ScaffoldResult struct {
	Root      string   `json:"root"`
	ID        string   `json:"id"`
	Created   []string `json:"created"`
	Refreshed []string `json:"refreshed,omitempty"`
	// Skipped files already existed and were left exactly as they were.
	Skipped []string `json:"skipped,omitempty"`
}

// Status describes a folder for the UI: may it be a project, is it set up.
type Status struct {
	Root     string `json:"root,omitempty"`
	ID       string `json:"id,omitempty"`
	Name     string `json:"name,omitempty"`
	SetUp    bool   `json:"set_up"`
	Declined bool   `json:"declined"`
	// Allowed is false for a folder that can never be a project (home, Memo's
	// own folders, a foreign .memo…); Reason then says why.
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason,omitempty"`
}

// UIStatus is Status plus what the app decides about showing the setup prompt.
type UIStatus struct {
	Status
	// ShouldAsk is true when the UI should offer "set up Memo for this project":
	// Minimal Mode is off, the folder can be a project, is not set up yet and was
	// not declined.
	ShouldAsk bool `json:"should_ask"`
}

// Status resolves path for the UI. It never writes anything.
func (m *Manager) Status(path string) Status {
	res, err := m.Open(path)
	if err != nil {
		reason := err.Error()
		if errors.Is(err, ErrForeignDir) {
			reason = "this folder already has a .memo folder that Memo did not create"
		}
		return Status{Allowed: false, Reason: reason}
	}
	st := Status{Root: res.Root, ID: res.ID, SetUp: res.SetUp, Declined: res.Declined, Allowed: true}
	if res.Info != nil {
		st.Name = res.Info.Name
	}
	return st
}

func readTemplate(name string) []byte {
	b, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		panic("project: missing embedded template " + name) // a build defect, not a runtime condition
	}
	return b
}

func sha(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func exists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// existingPath checks directory entries rather than following links. Case variants,
// directories and dangling links all reserve the name and must be preserved.
func existingPath(path string) (string, error) {
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.Name() == filepath.Base(path) {
			return path, nil
		}
	}
	for _, entry := range entries {
		if strings.EqualFold(entry.Name(), filepath.Base(path)) {
			return filepath.Join(filepath.Dir(path), entry.Name()), nil
		}
	}
	return "", nil
}

// createStarter publishes a complete file without ever replacing an existing
// entry, even if another setup or editor creates it after the existence check.
func createStarter(path string, data []byte) (bool, error) {
	return writeProjectFile(path, data, false)
}

// writeProjectFile uses an exclusively created, unpredictable temporary name.
// Refreshes use rename without a truncating-copy fallback; failure preserves
// the destination rather than risking partial replacement on Windows.
func writeProjectFile(path string, data []byte, replace bool) (bool, error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".memo-scaffold-*")
	if err != nil {
		return false, err
	}
	defer os.Remove(tmp.Name())
	if err = tmp.Chmod(0644); err == nil {
		_, err = tmp.Write(data)
	}
	if err == nil {
		err = tmp.Sync()
	}
	closeErr := tmp.Close()
	if err != nil {
		return false, err
	}
	if closeErr != nil {
		return false, closeErr
	}
	if replace {
		return false, os.Rename(tmp.Name(), path)
	}
	if err = os.Link(tmp.Name(), path); os.IsExist(err) {
		return false, nil
	}
	return err == nil, err
}

// RulesPath resolves the existing spelling of the managed rules file.
func RulesPath(root string) (string, error) {
	return existingPath(filepath.Join(Dir(root), "AGENTS.md"))
}

// Scaffold sets path up as a Memo project and writes the starter files:
//
//	.memo/project.json  .memo/AGENTS.md  .memo/notes.md  .memo/.gitignore
//	AGENTS.md  handoff.md        (project root)
//
// Nothing that already exists is overwritten, with one exception: the generic
// rules in `.memo/AGENTS.md` are Memo's own text and are refreshed when a newer
// version ships — but only while nobody has edited the file. The root
// AGENTS.md skeleton is not created when the project already has an AGENTS.md
// or a CLAUDE.md (it already has its rules).
func (m *Manager) Scaffold(path string) (*ScaffoldResult, error) {
	res, err := m.Init(path)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	root := res.Root
	info, err := ReadInfo(root)
	if err != nil {
		return nil, err
	}
	out := &ScaffoldResult{Root: root, ID: res.ID}
	rel := func(p string) string { r, _ := filepath.Rel(root, p); return filepath.ToSlash(r) }
	create := func(p string, data []byte) (bool, error) {
		created, err := createStarter(p, data)
		if err != nil {
			return false, err
		}
		if created {
			out.Created = append(out.Created, rel(p))
		} else {
			out.Skipped = append(out.Skipped, rel(p))
		}
		return created, nil
	}

	// Generic rules: Memo-managed.
	rules := readTemplate("memo_agents.md")
	rulesPath, err := RulesPath(root)
	if err != nil {
		return nil, err
	}
	infoChanged := false
	switch {
	case rulesPath == "":
		created, err := create(filepath.Join(Dir(root), "AGENTS.md"), rules)
		if err != nil {
			return nil, err
		}
		if created {
			info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
		}
	default:
		st, err := os.Lstat(rulesPath)
		if err != nil {
			return nil, err
		}
		// A link or non-regular entry belongs to the user, never to the updater.
		if !st.Mode().IsRegular() {
			out.Skipped = append(out.Skipped, rel(rulesPath))
			break
		}
		cur, rerr := os.ReadFile(rulesPath)
		switch {
		case rerr != nil:
			out.Skipped = append(out.Skipped, rel(rulesPath))
		case sha(cur) == sha(rules):
			if info.RulesHash != sha(rules) {
				info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
			}
		case info.RulesHash != "" && sha(cur) == info.RulesHash:
			if _, err := writeProjectFile(rulesPath, rules, true); err != nil {
				return nil, err
			}
			info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
			out.Refreshed = append(out.Refreshed, rel(rulesPath))
		default:
			out.Skipped = append(out.Skipped, rel(rulesPath)) // edited by the user
		}
	}

	claudePath, err := existingPath(filepath.Join(root, "CLAUDE.md"))
	if err != nil {
		return nil, err
	}
	for _, f := range []struct {
		path string
		tmpl string
		skip bool
	}{
		{filepath.Join(Dir(root), "notes.md"), "notes.md", false},
		{filepath.Join(Dir(root), ".gitignore"), "gitignore.txt", false},
		{filepath.Join(root, "AGENTS.md"), "agents.md", claudePath != ""},
		{filepath.Join(root, "handoff.md"), "handoff.md", false},
	} {
		found, err := existingPath(f.path)
		if err != nil {
			return nil, err
		}
		if f.skip || found != "" {
			if found == "" {
				found = f.path
			}
			out.Skipped = append(out.Skipped, rel(found))
			continue
		}
		if _, err := create(f.path, readTemplate(f.tmpl)); err != nil {
			return nil, err
		}
	}

	if infoChanged {
		if err := writeInfo(root, info); err != nil {
			return nil, err
		}
	}
	return out, nil
}
