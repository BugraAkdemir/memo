// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/browser"
	"github.com/chromedp/cdproto/page"
	"github.com/chromedp/chromedp"

	"memo/internal/browserengine"
	"memo/internal/config"
	"memo/internal/logx"
)

// tab is one live browser process with one tab, owned by one Service. It is
// the ONLY thing in this package that starts or stops a browser.
//
// The browser is a real, visible window in a profile folder that belongs to
// Memo (data/webchat/<site>), never the person's own browser profile. It is
// started with chromedp's allocator but WITHOUT chromedp's default option set:
// that set adds --enable-automation (the "controlled by automated software"
// banner and navigator.webdriver) and --headless. This is a normal browser
// window the person signs in to themselves; nothing here tries to hide that
// it is being driven, and nothing tries to defeat a site's checks — when one
// appears the person completes it in the window (see Service.waitForHuman).
type tab struct {
	allocCtx    context.Context
	allocCancel context.CancelFunc
	ctx         context.Context
	cancel      context.CancelFunc
	profileDir  string
	started     time.Time
}

// headless is false in production (the person must be able to see and use the
// window). Tests set it so a suite run does not open windows on a desktop.
var headless = false

// startTab launches the browser. resolve is a seam so tests can point at a
// specific binary or fail on purpose.
func startTab(ctx context.Context, siteID string, resolve func(context.Context) (string, error)) (*tab, error) {
	exe, err := resolve(ctx)
	if err != nil {
		if errors.Is(err, browserengine.ErrBrowserNotInstalled) {
			return nil, newErr(KindNotInstalled, "", err)
		}
		return nil, newErr(KindBrowser, "", err)
	}
	dir := filepath.Join(config.DataPath("webchat"), siteID)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, newErr(KindBrowser, "", fmt.Errorf("profile dir: %w", err))
	}
	opts := []chromedp.ExecAllocatorOption{
		chromedp.ExecPath(exe),
		chromedp.UserDataDir(dir),
		chromedp.Flag("headless", headless),
		chromedp.Flag("no-first-run", true),
		chromedp.Flag("no-default-browser-check", true),
		chromedp.Flag("disable-extensions", true),
		chromedp.WindowSize(1100, 820),
	}
	// Both contexts outlive the request that started them: the browser must
	// still be there for the next message.
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.WithoutCancel(ctx), opts...)
	tabCtx, tabCancel := chromedp.NewContext(allocCtx)
	// The first Run is what actually starts the process; it must use tabCtx
	// itself, not a child that gets cancelled (see browserengine.startSession).
	if err := chromedp.Run(tabCtx); err != nil {
		tabCancel()
		allocCancel()
		return nil, newErr(KindBrowser, "", err)
	}
	logx.Info("WEBCHAT: browser started", "site", siteID, "profile", dir)
	return &tab{allocCtx: allocCtx, allocCancel: allocCancel, ctx: tabCtx, cancel: tabCancel, profileDir: dir, started: time.Now()}, nil
}

// alive reports whether the browser process behind the tab is still usable.
func (t *tab) alive() bool {
	if t == nil {
		return false
	}
	return t.ctx.Err() == nil && t.allocCtx.Err() == nil
}

func (t *tab) close() {
	if t == nil {
		return
	}
	// chromedp.Cancel waits for the browser process to exit, so the profile
	// folder is quiet (and its cookies flushed) by the time we return — the
	// next launch, or a folder removal, must not race a process still writing.
	_ = chromedp.Cancel(t.ctx)
	t.cancel()
	t.allocCancel()
}

// run executes actions on the tab under a bound deadline derived from the
// caller's context. Cancelling the derived context only abandons this one
// call; it does not touch the browser (the first-Run rule above applies only
// to the very first call).
func (t *tab) run(ctx context.Context, timeout time.Duration, actions ...chromedp.Action) error {
	if !t.alive() {
		return errors.New("browser is gone")
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	// Tie the call to the tab as well: chromedp.Run uses the context it is
	// given to find its target, so wrap tab.ctx with the caller's deadline.
	merged, mcancel := mergeCtx(t.ctx, cctx)
	defer mcancel()
	return chromedp.Run(merged, actions...)
}

// mergeCtx returns a context that carries base's chromedp target and is
// cancelled when either base or other is.
func mergeCtx(base, other context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(base)
	stop := context.AfterFunc(other, cancel)
	return ctx, func() { stop(); cancel() }
}

// setWindow minimises or restores the window. Best effort: a window manager
// that refuses is not an error worth surfacing.
func (t *tab) setWindow(ctx context.Context, state browser.WindowState) {
	_ = t.run(ctx, 5*time.Second, chromedp.ActionFunc(func(c context.Context) error {
		id, _, err := browser.GetWindowForTarget().Do(c)
		if err != nil {
			return err
		}
		return browser.SetWindowBounds(id, &browser.Bounds{WindowState: state}).Do(c)
	}))
}

func (t *tab) bringToFront(ctx context.Context) {
	t.setWindow(ctx, browser.WindowStateNormal)
	_ = t.run(ctx, 5*time.Second, chromedp.ActionFunc(func(c context.Context) error {
		return page.BringToFront().Do(c)
	}))
}
