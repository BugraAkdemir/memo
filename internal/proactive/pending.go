// SPDX-License-Identifier: AGPL-3.0-or-later

package proactive

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// PendingTTL is how long an unanswered suggestion stays "pending" before the
// engine is free to act again. Prevents a single ignored prompt from silencing
// the system forever.
const PendingTTL = 2 * time.Hour

// PendingSuggestion is a proactive prompt awaiting the user's response. The
// mobile app polls for it and renders a notification / chat bubble.
type PendingSuggestion struct {
	ID        string    `json:"id"`
	Message   string    `json:"message"`
	PatternID string    `json:"pattern_id"`
	Action    Action    `json:"action"`
	CreatedAt time.Time `json:"created_at"`
	Responded bool      `json:"responded"`
	Response  string    `json:"response"`
}

// PendingStore persists the single in-flight suggestion to pending.json so it
// survives restarts and is visible to the web/mobile layer.
type PendingStore struct {
	path string
	mu   sync.RWMutex
}

// NewPendingStore returns a store backed by the given file path.
func NewPendingStore(path string) *PendingStore {
	return &PendingStore{path: path}
}

func (s *PendingStore) load() (*PendingSuggestion, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	if len(data) == 0 {
		return nil, nil
	}
	var p PendingSuggestion
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, err
	}
	if p.ID == "" {
		return nil, nil
	}
	return &p, nil
}

func (s *PendingStore) write(p *PendingSuggestion) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

// Set stores a new pending suggestion, replacing any previous one.
func (s *PendingStore) Set(p PendingSuggestion) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}
	if err := s.write(&p); err != nil {
		return fmt.Errorf("proactive.PendingStore.Set: %w", err)
	}
	return nil
}

// Get returns the current unanswered, unexpired suggestion, or nil.
func (s *PendingStore) Get() (*PendingSuggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.load()
	if err != nil {
		return nil, fmt.Errorf("proactive.PendingStore.Get: %w", err)
	}
	if p == nil || p.Responded {
		return nil, nil
	}
	if time.Since(p.CreatedAt) > PendingTTL {
		return nil, nil
	}
	return p, nil
}

// GetRaw returns the stored suggestion regardless of responded/expired state,
// or nil if none exists. Used by the engine to reap ignored prompts.
func (s *PendingStore) GetRaw() (*PendingSuggestion, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, err := s.load()
	if err != nil {
		return nil, fmt.Errorf("proactive.PendingStore.GetRaw: %w", err)
	}
	return p, nil
}

// HasPending reports whether an actionable suggestion is still in flight.
func (s *PendingStore) HasPending() bool {
	p, err := s.Get()
	return err == nil && p != nil
}

// Respond records the user's answer to the pending suggestion. It returns the
// suggestion that was answered (for feedback routing), or nil if the id does not
// match the current pending one.
func (s *PendingStore) Respond(id, response string) (*PendingSuggestion, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	p, err := s.load()
	if err != nil {
		return nil, fmt.Errorf("proactive.PendingStore.Respond: %w", err)
	}
	if p == nil || p.ID != id {
		return nil, nil
	}
	p.Responded = true
	p.Response = response
	if err := s.write(p); err != nil {
		return nil, fmt.Errorf("proactive.PendingStore.Respond: %w", err)
	}
	return p, nil
}

// Clear removes any pending suggestion.
func (s *PendingStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("proactive.PendingStore.Clear: %w", err)
	}
	return nil
}
