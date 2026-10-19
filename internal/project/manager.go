// SPDX-License-Identifier: AGPL-3.0-or-later

package project

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"memo/internal/fileutil"
	"memo/internal/logx"
)

// registryEntry is what Memo remembers, per device, about one project folder.
type registryEntry struct {
	ID      string `json:"id"`
	Created string `json:"created"`
	// Declined is set when the user chose "never set up Memo here".
	Declined bool `json:"declined,omitempty"`
}

type registryFile struct {
	Version  int                       `json:"version"`
	Projects map[string]*registryEntry `json:"projects"`
}

// Manager resolves folders to project ids and creates `.memo` folders.
//
// The registry (path → id) lives in Memo's data directory, is per device and is
// never synced: a path means nothing on another machine, while the id inside
// `.memo/project.json` travels with the folder.
type Manager struct {
	guard Guard
	path  string // registry file
	now   func() time.Time

	mu  sync.Mutex
	reg registryFile
}

// NewManager returns a Manager whose per-device registry is stored at
// registryPath (e.g. `<data>/projects.json`). A missing or unreadable registry
// starts empty; an unreadable one is kept aside as `<path>.bad`.
func NewManager(guard Guard, registryPath string) *Manager {
	m := &Manager{guard: guard, path: registryPath, now: time.Now}
	m.load()
	return m
}

func (m *Manager) load() {
	m.reg = registryFile{Version: 1, Projects: map[string]*registryEntry{}}
	if m.path == "" {
		return
	}
	data, err := os.ReadFile(m.path)
	if err != nil {
		return
	}
	var f registryFile
	if err := json.Unmarshal(data, &f); err != nil {
		logx.Printf("PROJECT: registry %s unreadable (%v); starting empty, copy kept as .bad", m.path, err)
		_ = os.WriteFile(m.path+".bad", data, 0o600)
		return
	}
	if f.Projects == nil {
		f.Projects = map[string]*registryEntry{}
	}
	m.reg = f
}

// saveLocked persists the registry. Callers hold m.mu.
func (m *Manager) saveLocked() {
	if m.path == "" {
		return
	}
	data, err := json.MarshalIndent(m.reg, "", "  ")
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o755); err != nil {
		logx.Printf("PROJECT: registry dir: %v", err)
		return
	}
	if err := fileutil.AtomicWrite(m.path, append(data, '\n'), 0o600); err != nil {
		logx.Printf("PROJECT: registry save: %v", err)
	}
}

// Resolved is the answer to "what project is this folder?".
type Resolved struct {
	// Root is the canonical project root (the folder holding `.memo`, or the
	// folder itself when it has not been set up yet).
	Root string
	// ID is the project's stable id. For a folder that is not set up yet it is
	// a pending id kept in the per-device registry; Init writes that same id
	// into project.json, so memory saved before setup keeps belonging to it.
	ID string
	// SetUp is true when `<Root>/.memo/project.json` exists.
	SetUp bool
	// Declined is true when the user said "never set up Memo here".
	Declined bool
	// Info is project.json's content when SetUp.
	Info *Info
}

// Open resolves path (usually a chat's ProjectPath) to its project. It returns
// a *RootError for folders that can never be projects (home, install dir, …)
// and ErrForeignDir for a `.memo` that is not a Memo project folder.
func (m *Manager) Open(path string) (*Resolved, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	root, ok := m.guard.FindRoot(path)
	if !ok {
		var err error
		if root, err = m.guard.Check(path); err != nil {
			return nil, err
		}
	}
	info, err := ReadInfo(root)
	switch {
	case err == nil:
	case errors.Is(err, ErrNoProject):
		info = nil
	default:
		return nil, err
	}
	return m.resolveLocked(root, info)
}

func (m *Manager) resolveLocked(root string, info *Info) (*Resolved, error) {
	res := &Resolved{Root: root, Info: info, SetUp: info != nil}
	if info == nil {
		e := m.reg.Projects[root]
		if e == nil {
			e = &registryEntry{ID: NewID(), Created: m.now().UTC().Format(time.RFC3339)}
			m.reg.Projects[root] = e
			m.saveLocked()
		}
		res.ID, res.Declined = e.ID, e.Declined
		return res, nil
	}

	id := info.ID
	changed := false
	for p, e := range m.reg.Projects {
		if p == root || e.ID != id {
			continue
		}
		if other, err := ReadInfo(p); err == nil && other.ID == id {
			// The same id lives in a second folder that still exists: this one
			// is a copy (cp -r, a template). Give the copy its own identity so
			// two projects never share one memory.
			id = NewID()
			info.ID = id
			if err := writeInfo(root, info); err != nil {
				return nil, err
			}
			changed = true
			logx.Printf("PROJECT: %s is a copy of %s; new id assigned", root, p)
			break
		}
		// The old folder is gone: the project was moved. Forget the old path.
		delete(m.reg.Projects, p)
		changed = true
	}
	e := m.reg.Projects[root]
	if e == nil || e.ID != id {
		declined := e != nil && e.Declined
		e = &registryEntry{ID: id, Created: info.Created, Declined: declined}
		m.reg.Projects[root] = e
		changed = true
	}
	if changed {
		m.saveLocked()
	}
	res.ID, res.Declined = id, e.Declined
	return res, nil
}

// Init sets root up as a project: creates `.memo/project.json` with the folder's
// pending id. It is idempotent for a folder that already is a project, and it
// never writes into a foreign `.memo`.
func (m *Manager) Init(path string) (*Resolved, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	root, err := m.guard.Check(path)
	if err != nil {
		return nil, err
	}
	info, err := ReadInfo(root)
	switch {
	case err == nil:
		return m.resolveLocked(root, info)
	case errors.Is(err, ErrNoProject):
	default:
		return nil, err
	}
	res, err := m.resolveLocked(root, nil)
	if err != nil {
		return nil, err
	}
	info = newInfo(root, res.ID, m.now())
	if err := writeInfo(root, info); err != nil {
		return nil, err
	}
	if e := m.reg.Projects[root]; e != nil {
		e.Declined = false
		m.saveLocked()
	}
	return &Resolved{Root: root, ID: info.ID, SetUp: true, Info: info}, nil
}

// SetDeclined records the user's "never set up Memo in this folder" choice.
func (m *Manager) SetDeclined(path string, declined bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	root, err := m.guard.Check(path)
	if err != nil {
		return err
	}
	e := m.reg.Projects[root]
	if e == nil {
		e = &registryEntry{ID: NewID(), Created: m.now().UTC().Format(time.RFC3339)}
		m.reg.Projects[root] = e
	}
	e.Declined = declined
	m.saveLocked()
	return nil
}
