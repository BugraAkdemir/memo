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

// Manager owns at most one live browser process at a time.
type Manager struct {
	mu        sync.Mutex
	keepAlive bool
	engine    engineHandle // non-nil only when keepAlive and already started
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

// Install downloads a browser engine (chrome-headless-shell — from Google's
// chrome-for-testing CDN on most platforms, from Microsoft's Playwright CDN
// on linux/arm64, which chrome-for-testing doesn't build for at all) if none
// is already available. Blocking — gosearch's browser.Install has no
// progress-callback hook, unlike internal/llama/installer.go's
// downloadFileProgress. A no-op (returns nil quickly) if a browser is
// already installed.
func (m *Manager) Install(ctx context.Context) error {
	err := installFn(ctx, browser.AllowDownload(true))
	if err != nil && errors.Is(err, browser.ErrUnsupportedPlatform) {
		return fmt.Errorf("%w: no automatic download is available for this platform — "+
			"install a system Chromium/Chrome yourself (e.g. \"sudo apt install chromium\" "+
			"on Debian/Raspberry Pi OS, \"brew install --cask chromium\" on macOS) and retry; "+
			"Memo will detect it automatically", err)
	}
	return err
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
