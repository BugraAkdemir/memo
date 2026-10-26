// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Consent is the person's answer to "may Memo send an anonymous daily summary?".
type Consent string

const (
	ConsentUnset Consent = ""    // never asked: nothing is sent, nothing is counted
	ConsentYes   Consent = "yes" // agreed
	ConsentNo    Consent = "no"  // declined
)

// counterWindowDays is how many days of counters are kept. The server accepts a
// report up to 8 days late, so a Memo that was closed for a week can still send
// what it saw.
const counterWindowDays = 10

// CrashAgg is one sanitized crash signature counted within a day.
type CrashAgg struct {
	Sig       string   `json:"sig"`
	Component string   `json:"component"`
	ExcType   string   `json:"exc_type"`
	Frames    []string `json:"frames"`
	N         int      `json:"n"`
}

// DayCounters are the things only Memo itself can count for one UTC day:
// feature use, failures and update outcomes. Maps are keyed "a|b".
type DayCounters struct {
	Features    map[string]bool      `json:"features,omitempty"`
	ProviderErr map[string]int       `json:"provider_err,omitempty"` // "provider|code"
	StartFail   map[string]int       `json:"start_fail,omitempty"`   // "component|code"
	Exit        map[string]int       `json:"exit,omitempty"`         // "component|code"
	Update      map[string]int       `json:"update,omitempty"`       // "event|code"
	Crash       map[string]*CrashAgg `json:"crash,omitempty"`        // by signature
}

// StateData is what telemetry_state.json holds. It is never exported or backed
// up (data/telemetry_state.json is not in backupdata's allow-list) and holds no
// content — an id, an answer, dates and counters.
type StateData struct {
	TID         string                  `json:"tid"`
	Consent     Consent                 `json:"consent"`
	ConsentDay  string                  `json:"consent_day,omitempty"`
	InstallDay  string                  `json:"install_day,omitempty"` // only set for a install that started with telemetry
	LastSentDay string                  `json:"last_sent_day,omitempty"`
	Milestones  Milestones              `json:"milestones"`
	Days        map[string]*DayCounters `json:"days,omitempty"`
	// PendingForget are old install ids the server still has to forget (a reset
	// or a delete made while offline); retried on every cycle until it answers.
	PendingForget []string `json:"pending_forget,omitempty"`
}

// State is the on-disk state, safe for concurrent use.
type State struct {
	mu   sync.Mutex
	path string
	d    StateData
}

func newTID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("telemetry: no randomness: %v", err))
	}
	return hex.EncodeToString(b)
}

// OpenState loads the state file, creating it (with a fresh random id) when it
// does not exist. A file that cannot be read is moved aside and replaced: a
// broken telemetry file must never stop Memo from starting.
func OpenState(path string) (*State, error) {
	s := &State{path: path}
	raw, err := os.ReadFile(path)
	switch {
	case err == nil:
		if jerr := json.Unmarshal(raw, &s.d); jerr != nil || s.d.TID == "" {
			_ = os.Rename(path, fmt.Sprintf("%s.corrupt-%d", path, time.Now().Unix()))
			s.d = StateData{}
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return nil, fmt.Errorf("telemetry: read state: %w", err)
	}
	if s.d.TID == "" {
		s.d = StateData{TID: newTID()}
	}
	if s.d.Days == nil {
		s.d.Days = map[string]*DayCounters{}
	}
	if err := s.saveLocked(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *State) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("telemetry: state dir: %w", err)
	}
	raw, err := json.MarshalIndent(s.d, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, raw, 0o600); err != nil {
		return fmt.Errorf("telemetry: write state: %w", err)
	}
	if err := os.Chmod(tmp, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, s.path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("telemetry: replace state: %w", err)
	}
	return nil
}

// Snapshot returns a deep copy of the state.
func (s *State) Snapshot() StateData {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, _ := json.Marshal(s.d)
	var out StateData
	_ = json.Unmarshal(raw, &out)
	if out.Days == nil {
		out.Days = map[string]*DayCounters{}
	}
	return out
}

func (s *State) mutate(fn func(d *StateData)) error {
	return s.mutateIf(func(d *StateData) bool { fn(d); return true })
}

// mutateIf saves only when fn reports that it changed something: a flag that is
// set on every chat turn must not turn into one disk write per turn.
func (s *State) mutateIf(fn func(d *StateData) bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !fn(&s.d) {
		return nil
	}
	return s.saveLocked()
}

// SetConsent records the answer and the day it was given.
func (s *State) SetConsent(c Consent, day string) error {
	return s.mutate(func(d *StateData) { d.Consent, d.ConsentDay = c, day })
}

// SetInstallDay remembers the day this install began, once.
func (s *State) SetInstallDay(day string) error {
	return s.mutate(func(d *StateData) {
		if d.InstallDay == "" {
			d.InstallDay = day
		}
	})
}

// MarkSent records the newest day the server has accepted.
func (s *State) MarkSent(day string) error {
	return s.mutate(func(d *StateData) {
		if day > d.LastSentDay {
			d.LastSentDay = day
		}
	})
}

// SetMilestone keeps the earliest date seen for "provider_added" or "first_message".
func (s *State) SetMilestone(name, day string) error {
	return s.mutateIf(func(d *StateData) bool {
		target := map[string]*string{"provider_added": &d.Milestones.ProviderAdded, "first_message": &d.Milestones.FirstMessage}[name]
		if target != nil && (*target == "" || day < *target) {
			*target = day
			return true
		}
		return false
	})
}

// ResetID gives this install a new random id and a fresh history, keeps the
// person's answer, and returns the OLD id so the caller can ask the server to
// forget it before it is lost.
func (s *State) ResetID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.d.TID
	s.d.TID = newTID()
	s.d.Days = map[string]*DayCounters{}
	s.d.LastSentDay = ""
	return old, s.saveLocked()
}

// AddPendingForget remembers an id the server must still forget.
func (s *State) AddPendingForget(tid string) error {
	return s.mutate(func(d *StateData) {
		for _, x := range d.PendingForget {
			if x == tid {
				return
			}
		}
		d.PendingForget = append(d.PendingForget, tid)
	})
}

// RemovePendingForget clears an id once the server has forgotten it.
func (s *State) RemovePendingForget(tid string) error {
	return s.mutate(func(d *StateData) {
		out := d.PendingForget[:0]
		for _, x := range d.PendingForget {
			if x != tid {
				out = append(out, x)
			}
		}
		d.PendingForget = out
	})
}

// day returns the counters of one day, creating them, and drops days that fell
// out of the window (relative to the day being written).
func (d *StateData) day(day string) *DayCounters {
	if d.Days == nil {
		d.Days = map[string]*DayCounters{}
	}
	if t, err := time.Parse("2006-01-02", day); err == nil {
		cut := t.AddDate(0, 0, -counterWindowDays).Format("2006-01-02")
		for k := range d.Days {
			if k < cut {
				delete(d.Days, k)
			}
		}
	}
	c := d.Days[day]
	if c == nil {
		c = &DayCounters{}
		d.Days[day] = c
	}
	return c
}

func bump(m *map[string]int, key string) {
	if *m == nil {
		*m = map[string]int{}
	}
	(*m)[key]++
}

// AddFeature notes that a feature was used on a day (once per day).
func (s *State) AddFeature(day, feature string) error {
	return s.mutateIf(func(d *StateData) bool {
		c := d.day(day)
		if c.Features[feature] {
			return false
		}
		if c.Features == nil {
			c.Features = map[string]bool{}
		}
		c.Features[feature] = true
		return true
	})
}

func (s *State) AddProviderErr(day, provider, code string) error {
	return s.mutate(func(d *StateData) { bump(&d.day(day).ProviderErr, provider+"|"+code) })
}
func (s *State) AddStartFail(day, component, code string) error {
	return s.mutate(func(d *StateData) { bump(&d.day(day).StartFail, component+"|"+code) })
}
func (s *State) AddExit(day, component, code string) error {
	return s.mutate(func(d *StateData) { bump(&d.day(day).Exit, component+"|"+code) })
}
func (s *State) AddUpdate(day, event, code string) error {
	return s.mutate(func(d *StateData) { bump(&d.day(day).Update, event+"|"+code) })
}

// AddCrash counts one occurrence of a sanitized crash signature.
func (s *State) AddCrash(day string, c CrashAgg) error {
	return s.mutate(func(d *StateData) {
		dc := d.day(day)
		if dc.Crash == nil {
			dc.Crash = map[string]*CrashAgg{}
		}
		if cur := dc.Crash[c.Sig]; cur != nil {
			cur.N++
			return
		}
		c.N = 1
		dc.Crash[c.Sig] = &c
	})
}
