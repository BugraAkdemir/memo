// Package browserengine manages the lifecycle of the optional headless
// browser (github.com/BugraAkdemir/gosearch/browser) internal/websearch uses
// to render JavaScript-heavy pages gosearch.Fetch alone cannot read.
//
// Two modes, controlled by KeepAlive:
//   - Off (default): a fresh browser is launched per Fetch call and closed
//     immediately after — zero idle memory, a few seconds of startup
//     latency per JS-walled page. Matches Memo's own positioning of being
//     lighter than competitors that keep a browser resident.
//   - On: one browser process is launched lazily on first use and reused
//     across calls (~150-250MB steady RAM, no per-call startup cost).
//
// Either way, the browser Memo launches is a brand-new OS process with its
// own dedicated profile directory (chromedp.NewExecAllocator + a fresh/
// caller-owned UserDataDir — see gosearch/browser/engine.go) — never an
// attach to, or a profile shared with, the user's own personal Chrome/
// Chromium. Closing it never touches anything the user has open themselves,
// and the user closing their own browser has no effect on this one.
package browserengine

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/BugraAkdemir/gosearch"
	"github.com/BugraAkdemir/gosearch/browser"
	"memo/internal/logx"
)

// engineHandle is the subset of *browser.Engine this package uses — kept as
// an interface so tests can substitute a fake without launching a real
// browser. *browser.Engine satisfies it structurally.
type engineHandle interface {
	Fetch(ctx context.Context, url string) (*gosearch.Page, error)
	Close() error
}

// newEngine and installFn sit behind package vars for the same reason
// internal/websearch keeps gosearchSearch/gosearchFetch as vars — tests
// substitute fakes instead of touching the network or launching a browser.
var newEngine = func(ctx context.Context) (engineHandle, error) {
	return browser.New(ctx)
}
var installFn = browser.Install

// InstallProgress reports the state of a browser-engine download. Shape and
// JSON tags mirror modelstore.DownloadProgress on purpose — Flutter reuses
// the exact same rendering conventions (percent, speed, active/error) for
// both, just without modelstore's per-repo/filename keying since there is
// only ever one browser engine to install.
type InstallProgress struct {
	Active     bool    `json:"active"`
	TotalBytes int64   `json:"total_bytes"`
	Downloaded int64   `json:"downloaded"`
	Percent    float64 `json:"percent"`
	Speed      string  `json:"speed"`
	Error      string  `json:"error,omitempty"`
}

// Manager owns at most one live browser process at a time.
type Manager struct {
	mu        sync.Mutex
	keepAlive bool
	engine    engineHandle // non-nil only when keepAlive and already started

	// progressMu guards progress independently of mu — StartInstall's
	// background goroutine updates it continuously and must never contend
	// with (or risk deadlocking against) the engine-lifecycle lock above.
	progressMu sync.Mutex
	progress   InstallProgress
}

// New creates a Manager. keepAlive seeds the initial mode (normally
// config.Browser.KeepAlive).
func New(keepAlive bool) *Manager {
	return &Manager{keepAlive: keepAlive}
}

// Fetch renders url with a browser. In keep-alive mode it reuses the one
// live engine (launching it on first use); otherwise it launches a fresh
// engine for this single call and closes it before returning.
func (m *Manager) Fetch(ctx context.Context, url string) (*gosearch.Page, error) {
	m.mu.Lock()
	if m.keepAlive {
		if m.engine == nil {
			e, err := newEngine(ctx)
			if err != nil {
				m.mu.Unlock()
				logx.Info("BROWSERENGINE: start failed", "mode", "keep-alive", "error", err)
				return nil, err
			}
			m.engine = e
			logx.Info("BROWSERENGINE: engine started", "mode", "keep-alive")
		}
		engine := m.engine
		m.mu.Unlock()
		return engine.Fetch(ctx, url)
	}
	m.mu.Unlock()

	e, err := newEngine(ctx)
	if err != nil {
		logx.Info("BROWSERENGINE: start failed", "mode", "one-shot", "error", err)
		return nil, err
	}
	logx.Info("BROWSERENGINE: engine started", "mode", "one-shot")
	defer func() {
		if err := e.Close(); err != nil {
			logx.Info("BROWSERENGINE: engine close failed", "mode", "one-shot", "error", err)
		} else {
			logx.Info("BROWSERENGINE: engine closed", "mode", "one-shot")
		}
	}()
	return e.Fetch(ctx, url)
}

// SetKeepAlive switches modes. Turning it off closes any currently-running
// persistent engine right away, so the change takes effect immediately
// instead of only on the next Fetch.
func (m *Manager) SetKeepAlive(v bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.keepAlive = v
	if !v && m.engine != nil {
		err := m.engine.Close()
		m.engine = nil
		if err != nil {
			logx.Info("BROWSERENGINE: engine close failed", "reason", "keep-alive turned off", "error", err)
		} else {
			logx.Info("BROWSERENGINE: engine closed", "reason", "keep-alive turned off")
		}
		return err
	}
	return nil
}

// KeepAlive reports the current mode.
func (m *Manager) KeepAlive() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.keepAlive
}

// IsInstalled reports whether a Chromium-family browser is available
// (system-installed, embedded, or already cached) without launching one or
// downloading anything.
func (m *Manager) IsInstalled(ctx context.Context) bool {
	return installFn(ctx) == nil
}

// StartInstall kicks off a browser-engine download in the background and
// returns immediately — callers poll InstallProgress for status instead of
// blocking on this call. Deliberately detached from ctx's cancellation (only
// its values, via context.WithoutCancel, survive): ctx is normally an HTTP
// request context, and the request ending — the user closing the Settings
// dialog, navigating to chat, the app losing focus — must not abort an
// in-flight download. That exact bug (a client-side navigation silently
// killing a server-side install) was reported live before this existed.
//
// A no-op if a download is already in progress (idempotent — a second
// button press or a second poll-triggered call does nothing new) or if a
// browser is already installed (IsInstalled's cache-aware check short-
// circuits instantly, no network, so re-opening Settings after a completed
// install reports done right away rather than starting a redundant "check").
func (m *Manager) StartInstall(ctx context.Context) {
	m.progressMu.Lock()
	if m.progress.Active {
		m.progressMu.Unlock()
		return
	}
	if m.IsInstalled(ctx) {
		m.progress = InstallProgress{Percent: 100}
		m.progressMu.Unlock()
		return
	}
	m.progress = InstallProgress{Active: true}
	m.progressMu.Unlock()

	installCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Minute)
	logx.GoRecover("browserengine: install", func() {
		defer cancel()
		start := time.Now()
		err := installFn(installCtx, browser.AllowDownload(true), browser.WithProgress(func(downloaded, total int64) {
			var percent float64
			if total > 0 {
				percent = float64(downloaded) / float64(total) * 100
			}
			var speed string
			if elapsed := time.Since(start).Seconds(); elapsed > 0 {
				speed = formatSpeed(float64(downloaded) / elapsed)
			}
			m.progressMu.Lock()
			m.progress.Downloaded = downloaded
			m.progress.TotalBytes = total
			m.progress.Percent = percent
			m.progress.Speed = speed
			m.progressMu.Unlock()
		}))

		m.progressMu.Lock()
		defer m.progressMu.Unlock()
		if err != nil {
			if errors.Is(err, browser.ErrUnsupportedPlatform) {
				err = fmt.Errorf("%w: no automatic download is available for this platform — "+
					"install a system Chromium/Chrome yourself (e.g. \"sudo apt install chromium\" "+
					"on Debian/Raspberry Pi OS, \"brew install --cask chromium\" on macOS) and retry; "+
					"Memo will detect it automatically", err)
			}
			m.progress.Active = false
			m.progress.Error = err.Error()
			logx.Info("BROWSERENGINE: install failed", "error", err)
			return
		}
		m.progress.Active = false
		m.progress.Percent = 100
		m.progress.Error = ""
		logx.Info("BROWSERENGINE: install complete")
	})
}

// InstallProgress returns a snapshot of the current (or most recently
// finished) install attempt. Safe to call whether or not StartInstall was
// ever called — the zero value (Active: false, Percent: 0) is a fine answer
// for "nothing has been attempted yet."
func (m *Manager) InstallProgress() InstallProgress {
	m.progressMu.Lock()
	defer m.progressMu.Unlock()
	return m.progress
}

// formatSpeed renders bytesPerSec as a human-readable rate. Mirrors
// modelstore.formatSpeed's exact output format (memo's model-download
// progress bar already uses this convention — matching it keeps the two
// progress UIs visually consistent) without importing that package for one
// function.
func formatSpeed(bytesPerSec float64) string {
	switch {
	case bytesPerSec >= 1024*1024*1024:
		return fmt.Sprintf("%.1f GB/s", bytesPerSec/(1024*1024*1024))
	case bytesPerSec >= 1024*1024:
		return fmt.Sprintf("%.1f MB/s", bytesPerSec/(1024*1024))
	case bytesPerSec >= 1024:
		return fmt.Sprintf("%.1f KB/s", bytesPerSec/1024)
	default:
		return fmt.Sprintf("%.0f B/s", bytesPerSec)
	}
}

// Stop closes the persistent engine, if one is running. Safe to call
// whether or not one was ever started (idempotent no-op) — meant to be
// wired into App shutdown unconditionally, the same way every other
// engine's Stop is.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.engine == nil {
		return nil
	}
	err := m.engine.Close()
	m.engine = nil
	if err != nil {
		logx.Info("BROWSERENGINE: engine close failed", "reason", "stop", "error", err)
	} else {
		logx.Info("BROWSERENGINE: engine closed", "reason", "stop")
	}
	return err
}
