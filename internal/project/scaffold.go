// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"

	"memo/internal/fileutil"
)

//go:embed templates/*
var templateFS embed.FS

// RulesVersion is bumped whenever templates/memo_agents.md changes.
const RulesVersion = 1

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
	info := res.Info
	out := &ScaffoldResult{Root: root, ID: res.ID}
	rel := func(p string) string { r, _ := filepath.Rel(root, p); return filepath.ToSlash(r) }
	create := func(p string, data []byte) error {
		if err := fileutil.AtomicWrite(p, data, 0o644); err != nil {
			return err
		}
		out.Created = append(out.Created, rel(p))
		return nil
	}

	// Generic rules: Memo-managed.
	rules := readTemplate("memo_agents.md")
	rulesPath := filepath.Join(Dir(root), "AGENTS.md")
	infoChanged := false
	switch {
	case !exists(rulesPath):
		if err := create(rulesPath, rules); err != nil {
			return nil, err
		}
		info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
	default:
		cur, rerr := os.ReadFile(rulesPath)
		switch {
		case rerr != nil:
			out.Skipped = append(out.Skipped, rel(rulesPath))
		case sha(cur) == sha(rules):
			if info.RulesHash != sha(rules) {
				info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
			}
		case info.RulesHash != "" && sha(cur) == info.RulesHash:
			if err := fileutil.AtomicWrite(rulesPath, rules, 0o644); err != nil {
				return nil, err
			}
			info.RulesVersion, info.RulesHash, infoChanged = RulesVersion, sha(rules), true
			out.Refreshed = append(out.Refreshed, rel(rulesPath))
		default:
			out.Skipped = append(out.Skipped, rel(rulesPath)) // edited by the user
		}
	}

	for _, f := range []struct {
		path string
		tmpl string
		skip bool
	}{
		{filepath.Join(Dir(root), "notes.md"), "notes.md", false},
		{filepath.Join(Dir(root), ".gitignore"), "gitignore.txt", false},
		{filepath.Join(root, "AGENTS.md"), "agents.md", exists(filepath.Join(root, "CLAUDE.md"))},
		{filepath.Join(root, "handoff.md"), "handoff.md", false},
	} {
		if f.skip || exists(f.path) {
			out.Skipped = append(out.Skipped, rel(f.path))
			continue
		}
		if err := create(f.path, readTemplate(f.tmpl)); err != nil {
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
