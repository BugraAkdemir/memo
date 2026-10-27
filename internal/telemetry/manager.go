// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import (
	"context"
	"errors"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"memo/internal/logx"
)

// BuildChannel is "official" in a release build (CI stamps it with
// -ldflags "-X memo/internal/telemetry.BuildChannel=official") and "source"
// everywhere else. A source build never sends unless the person turns it on,
// so community builds cannot pollute the numbers.
var BuildChannel = BuildSource

// Config wires a Manager.
type Config struct {
	StatePath string
	Endpoint  string // default: MEMO_TELEMETRY_URL or DefaultEndpoint
	Sources   Sources
	Info      Info
	Client    *http.Client
	Now       func() time.Time
}

// Status is what Settings and the CLI show.
type Status struct {
	Enabled     bool    `json:"enabled"`       // sharing is on (the default of an official build), and not switched off from outside
	Consent     Consent `json:"consent"`       // "", "yes", "no"
	EnvOff      bool    `json:"env_off"`       // MEMO_TELEMETRY=off
	Build       string  `json:"build"`         // official | source
	Endpoint    string  `json:"endpoint"`      // where reports go
	DeviceID    string  `json:"device_id"`     // first 8 characters of the random id, for support
	ConsentDay  string  `json:"consent_day"`   // the day the answer was given
	LastSentDay string  `json:"last_sent_day"` // newest day the server accepted
}

// PreviewResult is the report that would be sent next, as the exact bytes.
type PreviewResult struct {
	Day   string `json:"day"`
	JSON  []byte `json:"-"`
	Empty bool   `json:"empty"` // nothing happened on any recent day: there is nothing to send
}

// Manager owns the consent, the schedule and the sender.
type Manager struct {
	state     *State
	collector *Collector
	client    *http.Client
	cfg       Config
	now       func() time.Time
	tickMu    sync.Mutex
}

// New opens the state and returns a Manager. Nothing is sent by creating one.
func New(cfg Config) (*Manager, error) {
	st, err := OpenState(cfg.StatePath)
	if err != nil {
		return nil, err
	}
	m := &Manager{state: st, cfg: cfg, client: cfg.Client, now: cfg.Now}
	if m.client == nil {
		m.client = &http.Client{Timeout: 10 * time.Second}
	}
	if m.now == nil {
		m.now = time.Now
	}
	m.collector = &Collector{Sources: cfg.Sources, State: st, Info: cfg.Info}
	// An official build shares by default, for a fresh install and for one that
	// was updated from a version without telemetry alike; the person can turn it
	// off in the setup wizard or in Settings > Privacy. The day of this default
	// counts as the day of the answer, so only days AFTER it are ever reported.
	if cfg.Info.Build == BuildOfficial && st.Snapshot().Consent == ConsentUnset {
		if err := st.SetConsent(ConsentYes, m.today()); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Manager) endpoint() string {
	if m.cfg.Endpoint != "" {
		return strings.TrimRight(m.cfg.Endpoint, "/")
	}
	if u := os.Getenv("MEMO_TELEMETRY_URL"); u != "" {
		return strings.TrimRight(u, "/")
	}
	return DefaultEndpoint
}

// envOff is MEMO_TELEMETRY=off (also "0" / "false" / "no"): it beats every answer, and stops all network use.
func envOff() bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("MEMO_TELEMETRY"))) {
	case "off", "0", "false", "no":
		return true
	}
	return false
}

func (m *Manager) today() string { return m.now().UTC().Format("2006-01-02") }

func (m *Manager) enabled() bool {
	return !envOff() && m.state.Snapshot().Consent == ConsentYes
}

// Status reports the current answer.
func (m *Manager) Status() Status {
	snap := m.state.Snapshot()
	off := envOff()
	id := snap.TID
	if len(id) > 8 {
		id = id[:8]
	}
	return Status{
		Enabled:     !off && snap.Consent == ConsentYes,
		Consent:     snap.Consent,
		EnvOff:      off,
		Build:       m.cfg.Info.Build,
		Endpoint:    m.endpoint(),
		DeviceID:    id,
		ConsentDay:  snap.ConsentDay,
		LastSentDay: snap.LastSentDay,
	}
}

// SetConsent records the person's answer. Only days AFTER the day of the answer
// can ever be reported: what happened before they agreed is never sent.
func (m *Manager) SetConsent(yes bool) error {
	c := ConsentNo
	if yes {
		c = ConsentYes
	}
	return m.state.SetConsent(c, m.today())
}

// SetInstallDay marks the day a brand-new install began (only for an install
// that really started with telemetry; an upgrade leaves it unset).
func (m *Manager) SetInstallDay() error { return m.state.SetInstallDay(m.today()) }

// pendingDays are the completed UTC days still to report, oldest first: after
// the last one the server accepted, after the day of the answer, and at most
// eight days back (the server refuses older).
func (m *Manager) pendingDays() []string {
	snap := m.state.Snapshot()
	today, _ := time.Parse("2006-01-02", m.today())
	start := today.AddDate(0, 0, -8)
	later := func(day string) {
		if t, err := time.Parse("2006-01-02", day); err == nil && t.AddDate(0, 0, 1).After(start) {
			start = t.AddDate(0, 0, 1)
		}
	}
	if snap.LastSentDay != "" {
		later(snap.LastSentDay)
	}
	if snap.ConsentDay != "" {
		later(snap.ConsentDay)
	}
	var days []string
	for d := start; d.Before(today); d = d.AddDate(0, 0, 1) {
		days = append(days, d.Format("2006-01-02"))
	}
	return days
}

// Preview returns the report that would go next, byte for byte. Before an
// answer (or after a "no") it shows what yesterday's would have been, so the
// choice can be made with the content in view. It never touches the network.
func (m *Manager) Preview(ctx context.Context) (*PreviewResult, error) {
	var days []string
	if m.enabled() {
		days = m.pendingDays()
	} else {
		today, _ := time.Parse("2006-01-02", m.today())
		for i := 1; i <= 8; i++ {
			days = append(days, today.AddDate(0, 0, -i).Format("2006-01-02"))
		}
	}
	for _, day := range days {
		r, err := m.collector.Build(ctx, day)
		if errors.Is(err, ErrUnsupported) {
			return &PreviewResult{Empty: true}, nil
		}
		if err != nil {
			return nil, err
		}
		if r == nil {
			continue
		}
		raw, err := encode(*r)
		if err != nil {
			return nil, err
		}
		return &PreviewResult{Day: day, JSON: raw}, nil
	}
	return &PreviewResult{Empty: true}, nil
}

// Tick runs one cycle: retry pending forgets, then send every pending day.
// Safe to call often; a cycle with nothing to do makes no request.
func (m *Manager) Tick(ctx context.Context) error {
	m.tickMu.Lock()
	defer m.tickMu.Unlock()
	if envOff() {
		return nil
	}
	installErr := m.sendInstall(ctx) // independent of the answer; see install.go
	m.retryForgets(ctx)
	if !m.enabled() {
		return installErr
	}
	for _, day := range m.pendingDays() {
		r, err := m.collector.Build(ctx, day)
		if errors.Is(err, ErrUnsupported) {
			return nil
		}
		if err != nil {
			return err
		}
		if r == nil { // an idle day: nothing to send, and no need to look at it again
			_ = m.state.MarkSent(day)
			continue
		}
		raw, err := encode(*r)
		if err != nil {
			return err
		}
		out, err := m.post(ctx, "/v1/report", raw)
		switch out {
		case outAccepted:
			_ = m.state.MarkSent(day)
		case outRefused:
			logx.Printf("telemetry: the server refused the report of %s; it is dropped: %v", day, err)
			_ = m.state.MarkSent(day) // never resend the same bytes
		default:
			return err // keep the day, try again next cycle
		}
	}
	return installErr
}

func (m *Manager) retryForgets(ctx context.Context) {
	for _, tid := range m.state.Snapshot().PendingForget {
		if err := m.forget(ctx, tid); err != nil {
			return // offline: try again next cycle
		}
		_ = m.state.RemovePendingForget(tid)
	}
}

// ResetID gives this install a new random id. The server is asked to forget the
// old one first; if that cannot be done now (offline) the old id is remembered
// and retried, so a reset never leaves data behind for good.
func (m *Manager) ResetID(ctx context.Context) error {
	old, err := m.state.ResetID()
	if err != nil {
		return err
	}
	if envOff() {
		return m.state.AddPendingForget(old)
	}
	if ferr := m.forget(ctx, old); ferr != nil {
		return m.state.AddPendingForget(old)
	}
	return nil
}

// DeleteData asks the server to delete everything it holds for this install,
// switches sharing off and starts over with a new id.
func (m *Manager) DeleteData(ctx context.Context) error {
	if err := m.ResetID(ctx); err != nil {
		return err
	}
	return m.state.SetConsent(ConsentNo, m.today())
}

// Start runs Tick shortly after startup and then every few hours until ctx ends.
// The first cycle waits a random 0-90 s so a fleet that starts at the same
// moment does not hit the server at the same moment.
func (m *Manager) Start(ctx context.Context) {
	delay := time.Duration(rand.Intn(90)) * time.Second
	timer := time.NewTimer(delay)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			if err := m.Tick(ctx); err != nil {
				logx.Printf("telemetry: %v (will retry)", err)
			}
			timer.Reset(3 * time.Hour)
		}
	}
}

// ---------------------------------------------------------------- counters

func (m *Manager) note(fn func(day string) error) {
	if !m.enabled() {
		return // counters are not even kept unless sharing is on
	}
	if err := fn(m.today()); err != nil {
		logx.Printf("telemetry: counter: %v", err)
	}
}

func (m *Manager) NoteFeature(f string) {
	m.note(func(d string) error { return m.state.AddFeature(d, f) })
}
func (m *Manager) NoteProviderErr(provider, code string) {
	m.note(func(d string) error { return m.state.AddProviderErr(d, provider, code) })
}
func (m *Manager) NoteStartFail(component, code string) {
	m.note(func(d string) error { return m.state.AddStartFail(d, component, code) })
}
func (m *Manager) NoteExit(component, code string) {
	m.note(func(d string) error { return m.state.AddExit(d, component, code) })
}
func (m *Manager) NoteUpdate(event, code string) {
	m.note(func(d string) error { return m.state.AddUpdate(d, event, code) })
}
func (m *Manager) NoteCrash(c CrashAgg) {
	m.note(func(d string) error { return m.state.AddCrash(d, c) })
}

// NoteMilestone records "provider_added" / "first_message" (earliest date wins).
func (m *Manager) NoteMilestone(name string) {
	m.note(func(d string) error { return m.state.SetMilestone(name, d) })
}
