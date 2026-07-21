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

	// writeMu serializes disk writes independently of mu, so a slow write
	// doesn't hold mu (and therefore block concurrent List()/Get() reads —
	// hit by both the mobile poll and the desktop UI) for its whole duration
	// (BUG-L6). Only Update uses it today — it's the hot path, called up to
	// twice per routine per day by RoutineLoop.tick. Create/Delete are rare
	// enough that holding mu across their own writes isn't worth the same
	// treatment.
	writeMu sync.Mutex
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
// The disk write happens under writeMu only, not mu, so concurrent
// List()/Get() calls never block on it (BUG-L6) — mu is only held for the
// two brief map operations (existence check, then applying the result).
func (s *Store) Update(r Routine) (*Routine, error) {
	s.mu.RLock()
	_, ok := s.list[r.ID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("routine: not found: %s", r.ID)
	}

	r.UpdatedAt = time.Now()

	s.writeMu.Lock()
	err := s.saveLocked(&r)
	s.writeMu.Unlock()
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.list[r.ID]; !ok {
		// Deleted concurrently while the write above was in flight — don't
		// resurrect it into the map. Best-effort clean up the stray file we
		// just wrote (a racing Delete's own os.Remove may have already run
		// past this point; either order ends with the file gone).
		_ = os.Remove(s.path(r.ID))
		return nil, fmt.Errorf("routine: not found: %s", r.ID)
	}
	s.list[r.ID] = &r
	out := r
	return &out, nil
}

// SyncUTCOffset updates every routine whose Schedule.UTCOffsetMinutes
// doesn't already match minutes, and returns how many were changed.
//
// Schedule.UTCOffsetMinutes freezes the client's UTC offset at the moment a
// routine is created (see its doc comment, BUG-M4) — that fixed a routine
// created while the user was in a different timezone than the backend host,
// but left it permanently wrong after a DST transition, or if the user later
// relocates: a repeating "time of day" schedule's real intent is almost
// always "this time of day, wherever/whenever I currently am," the same way
// a phone's daily alarm follows the device's current local time rather than
// staying pinned to whatever offset was in effect when the alarm was set.
// Intended to be called with the client's current wall-clock offset each
// time it (re)connects (see RoutineBridge.SyncRoutineUTCOffsets) — a DST
// transition or relocation then self-corrects the next time the app talks to
// the backend, instead of staying frozen forever.
//
// Reuses Update for the actual per-routine write instead of touching s.list
// or disk directly, so this gets Update's existing concurrency handling
// (BUG-L6's writeMu, and its "deleted mid-write" guard) for free. A routine
// deleted concurrently just fails its own Update call here — logged via the
// returned error, skipped, harmless — rather than aborting the whole sync.
func (s *Store) SyncUTCOffset(minutes int) (int, error) {
	s.mu.RLock()
	var toUpdate []Routine
	for _, r := range s.list {
		if r.Schedule.UTCOffsetMinutes == nil || *r.Schedule.UTCOffsetMinutes != minutes {
			toUpdate = append(toUpdate, *r)
		}
	}
	s.mu.RUnlock()

	changed := 0
	var firstErr error
	for _, r := range toUpdate {
		offset := minutes
		r.Schedule.UTCOffsetMinutes = &offset
		if _, err := s.Update(r); err != nil {
			if firstErr == nil {
				firstErr = fmt.Errorf("routine: sync utc offset (id=%s): %w", r.ID, err)
			}
			continue
		}
		changed++
	}
	return changed, firstErr
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
