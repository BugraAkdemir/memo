// SPDX-License-Identifier: AGPL-3.0-or-later

package routine

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Store is a JSON-file-per-routine persistence layer, mirroring
// internal/taskloop/store.go's shape — routines are a small, user-managed
// list (not a high-churn queue), so a whole-file-per-item store is simpler
// than a single combined file and avoids partial-write corruption of
// unrelated routines.
type Store struct {
	dir  string
	mu   sync.RWMutex
	list map[string]*Routine
}

// NewStore creates (if needed) dir and loads any routines already there.
func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("routine: mkdir: %w", err)
	}
	s := &Store{dir: dir, list: make(map[string]*Routine)}
	if err := s.loadAll(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) path(id string) string {
	return filepath.Join(s.dir, id+".json")
}

func (s *Store) loadAll() error {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return fmt.Errorf("routine: readdir: %w", err)
	}
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(s.dir, e.Name()))
		if err != nil {
			continue
		}
		var r Routine
		if err := json.Unmarshal(data, &r); err != nil {
			continue
		}
		s.list[r.ID] = &r
	}
	return nil
}

func (s *Store) saveLocked(r *Routine) error {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return fmt.Errorf("routine: marshal: %w", err)
	}
	return os.WriteFile(s.path(r.ID), data, 0644)
}

// Create assigns an ID and persists a new routine.
func (s *Store) Create(r Routine) (*Routine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	r.ID = uuid.NewString()
	r.CreatedAt = now
	r.UpdatedAt = now

	if err := s.saveLocked(&r); err != nil {
		return nil, err
	}
	s.list[r.ID] = &r
	out := r
	return &out, nil
}

// Get returns a copy of the routine with the given ID.
func (s *Store) Get(id string) (*Routine, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.list[id]
	if !ok {
		return nil, fmt.Errorf("routine: not found: %s", id)
	}
	out := *r
	return &out, nil
}

// List returns all routines, oldest first.
func (s *Store) List() []Routine {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Routine, 0, len(s.list))
	for _, r := range s.list {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.Before(out[j].CreatedAt) })
	return out
}

// Update persists changes to an existing routine (by ID) and bumps UpdatedAt.
func (s *Store) Update(r Routine) (*Routine, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[r.ID]; !ok {
		return nil, fmt.Errorf("routine: not found: %s", r.ID)
	}
	r.UpdatedAt = time.Now()
	if err := s.saveLocked(&r); err != nil {
		return nil, err
	}
	s.list[r.ID] = &r
	out := r
	return &out, nil
}

// Delete removes a routine.
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[id]; !ok {
		return fmt.Errorf("routine: not found: %s", id)
	}
	delete(s.list, id)
	if err := os.Remove(s.path(id)); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("routine: remove: %w", err)
	}
	return nil
}
