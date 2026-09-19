package app

import (
	"context"
	"fmt"
)

// This file is the direct (non-agent) counterpart to the browser_navigate/
// browser_click/browser_scroll/browser_close agent tools in
// internal/agent/tools/browser.go — the same underlying *browserengine.
// Manager/Session, driven by the user themselves through BrowserPane's own
// URL bar, click-on-screenshot, and scroll-wheel handling instead of the
// model deciding to call a tool. Deliberately bypasses the agent
// permission-prompt flow entirely: there is nothing to ask permission for
// when the user is the one doing it.

// NavigateBrowserSession starts (or reuses) the interactive browser session
// and loads url. Returns the resulting screenshot and the tab's resolved
// URL (which can differ from the requested one after a redirect) so the
// caller gets a fully up-to-date pane in one round trip.
func (a *App) NavigateBrowserSession(ctx context.Context, rawURL string) (screenshot []byte, resolvedURL string, err error) {
	if a.browserMgr == nil {
		return nil, "", fmt.Errorf("browser engine not initialized")
	}
	sess, err := a.browserMgr.StartSession(ctx)
	if err != nil {
		return nil, "", err
	}
	if err := sess.Navigate(ctx, rawURL); err != nil {
		return nil, "", err
	}
	shot, err := sess.Screenshot(ctx)
	if err != nil {
		return nil, "", err
	}
	url, err := sess.CurrentURL(ctx)
	if err != nil {
		// Best effort — the navigate+screenshot above already succeeded,
		// don't fail the whole call just because reading the URL back
		// failed too.
		url = rawURL
	}
	return shot, url, nil
}

// ClickBrowserSession clicks the interactive session's tab at fixed
// viewport coordinates (BrowserPane maps a tap on the displayed screenshot
// back to these) and returns the resulting screenshot + resolved URL.
// Errors if no session is currently open.
func (a *App) ClickBrowserSession(ctx context.Context, x, y float64) (screenshot []byte, resolvedURL string, err error) {
	if a.browserMgr == nil {
		return nil, "", fmt.Errorf("browser engine not initialized")
	}
	sess, ok := a.browserMgr.GetSession()
	if !ok {
		return nil, "", fmt.Errorf("no active browser session")
	}
	if err := sess.ClickAt(ctx, x, y); err != nil {
		return nil, "", err
	}
	shot, err := sess.Screenshot(ctx)
	if err != nil {
		return nil, "", err
	}
	url, _ := sess.CurrentURL(ctx)
	return shot, url, nil
}

// ScrollBrowserSession scrolls the interactive session's tab by (dx, dy)
// pixels and returns the resulting screenshot — BrowserPane's mouse-wheel-
// over-the-screenshot handler drives this.
func (a *App) ScrollBrowserSession(ctx context.Context, dx, dy int) (screenshot []byte, resolvedURL string, err error) {
	if a.browserMgr == nil {
		return nil, "", fmt.Errorf("browser engine not initialized")
	}
	sess, ok := a.browserMgr.GetSession()
	if !ok {
		return nil, "", fmt.Errorf("no active browser session")
	}
	if err := sess.Scroll(ctx, dx, dy); err != nil {
		return nil, "", err
	}
	shot, err := sess.Screenshot(ctx)
	if err != nil {
		return nil, "", err
	}
	url, _ := sess.CurrentURL(ctx)
	return shot, url, nil
}

// CloseBrowserSession ends the interactive session, if one is open. Always
// succeeds on "nothing to close" rather than erroring, same as the
// browser_close agent tool.
func (a *App) CloseBrowserSession() error {
	if a.browserMgr == nil {
		return nil
	}
	return a.browserMgr.StopSession()
}

// BrowserSessionStatus reports whether an interactive session is currently
// open and, if so, its current URL — used to resync BrowserPane's state
// when the user reopens it rather than always starting blank (e.g. the
// agent left a session open, or the user navigated away from chat and
// back).
func (a *App) BrowserSessionStatus(ctx context.Context) (active bool, url string) {
	if a.browserMgr == nil {
		return false, ""
	}
	sess, ok := a.browserMgr.GetSession()
	if !ok {
		return false, ""
	}
	u, _ := sess.CurrentURL(ctx)
	return true, u
}
