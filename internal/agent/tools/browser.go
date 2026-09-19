package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
)

// BrowserSession is the subset of *browserengine.Session these tools need —
// kept local (not importing internal/browserengine) the same way
// CalendarClient/CalendarEvent above decouple this package from
// internal/calendar; App wires the real adapter in after init (see
// browserToolAdapter in internal/app).
type BrowserSession interface {
	Navigate(ctx context.Context, url string) error
	Click(ctx context.Context, selector string) error
	ClickAt(ctx context.Context, x, y float64) error
	Type(ctx context.Context, selector, text string) error
	Scroll(ctx context.Context, dx, dy int) error
	Screenshot(ctx context.Context) ([]byte, error)
	Close() error
}

// InteractiveBrowser is the subset of *browserengine.Manager's interactive-
// session surface these tools need. Set by App after initialization; nil in
// every test that never wires it up, same convention as CalendarClient.
var InteractiveBrowser interface {
	StartSession(ctx context.Context) (BrowserSession, error)
	GetSession() (BrowserSession, bool)
	StopSession() error
}

func browserNotConfigured() error {
	return errors.New(T(
		"etkileşimli tarayıcı bu ortamda yapılandırılmamış",
		"the interactive browser is not configured in this environment",
	))
}

type BrowserNavigateArgs struct {
	URL string `json:"url"`
}

// BrowserNavigate starts (or reuses) the one global interactive session and
// loads url. Does not itself return a screenshot — call browser_screenshot
// afterward to see the result, the same two-step shape Claude Code's own
// browser tools use ("navigate", then "look").
func BrowserNavigate(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	var args BrowserNavigateArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.URL == "" {
		return "", fmt.Errorf("url is required")
	}

	sess, err := InteractiveBrowser.StartSession(ctx)
	if err != nil {
		return "", fmt.Errorf(T("tarayıcı oturumu başlatılamadı: %w", "could not start browser session: %w"), err)
	}
	if err := sess.Navigate(ctx, args.URL); err != nil {
		return "", fmt.Errorf(T("sayfaya gidilemedi: %w", "could not navigate: %w"), err)
	}
	return fmt.Sprintf(T(
		"Tarayıcı %s adresine gitti. Ne göründüğünü görmek için browser_screenshot çağır.",
		"Browser navigated to %s. Call browser_screenshot to see what's currently displayed.",
	), args.URL), nil
}

// activeSession returns the running session, or the same "no active
// session" error BrowserScreenshot already uses — Click/Type/Scroll act on
// an existing session, they don't implicitly start one the way
// BrowserNavigate does (a click before any navigate has nothing to click).
func activeSession() (BrowserSession, error) {
	sess, ok := InteractiveBrowser.GetSession()
	if !ok {
		return nil, errors.New(T(
			"aktif bir tarayıcı oturumu yok — önce browser_navigate çağır",
			"no active browser session — call browser_navigate first",
		))
	}
	return sess, nil
}

type BrowserClickArgs struct {
	Selector string   `json:"selector,omitempty"`
	X        *float64 `json:"x,omitempty"`
	Y        *float64 `json:"y,omitempty"`
}

// BrowserClick clicks either a CSS selector or fixed viewport coordinates —
// exactly one of the two must be given.
func BrowserClick(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	var args BrowserClickArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	hasSelector := args.Selector != ""
	hasCoords := args.X != nil && args.Y != nil
	if hasSelector == hasCoords {
		return "", fmt.Errorf("provide exactly one of selector or (x and y)")
	}

	sess, err := activeSession()
	if err != nil {
		return "", err
	}
	if hasSelector {
		if err := sess.Click(ctx, args.Selector); err != nil {
			return "", fmt.Errorf(T("tıklanamadı: %w", "could not click: %w"), err)
		}
		return fmt.Sprintf(T("%q öğesine tıklandı.", "Clicked %q."), args.Selector), nil
	}
	if err := sess.ClickAt(ctx, *args.X, *args.Y); err != nil {
		return "", fmt.Errorf(T("tıklanamadı: %w", "could not click: %w"), err)
	}
	return fmt.Sprintf(T("(%.0f, %.0f) konumuna tıklandı.", "Clicked at (%.0f, %.0f)."), *args.X, *args.Y), nil
}

type BrowserTypeArgs struct {
	Selector string `json:"selector"`
	Text     string `json:"text"`
}

// BrowserType focuses selector and types text into it as real key events.
func BrowserType(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	var args BrowserTypeArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Selector == "" {
		return "", fmt.Errorf("selector is required")
	}

	sess, err := activeSession()
	if err != nil {
		return "", err
	}
	if err := sess.Type(ctx, args.Selector, args.Text); err != nil {
		return "", fmt.Errorf(T("yazılamadı: %w", "could not type: %w"), err)
	}
	return fmt.Sprintf(T("%q içine yazıldı.", "Typed into %q."), args.Selector), nil
}

type BrowserScrollArgs struct {
	Dx int `json:"dx"`
	Dy int `json:"dy"`
}

// browserDefaultScrollDy is used when a call omits both dx and dy — a
// convenient "scroll down a bit" with no arguments, mirroring a normal
// mouse-wheel nudge rather than forcing the model to always specify pixels.
const browserDefaultScrollDy = 800

// BrowserScroll scrolls the page by (dx, dy) pixels from its current
// position. Omitting both defaults to a small downward scroll.
func BrowserScroll(ctx context.Context, argsJSON json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	var args BrowserScrollArgs
	if len(argsJSON) > 0 {
		if err := json.Unmarshal(argsJSON, &args); err != nil {
			return "", fmt.Errorf("invalid arguments: %w", err)
		}
	}
	if args.Dx == 0 && args.Dy == 0 {
		args.Dy = browserDefaultScrollDy
	}

	sess, err := activeSession()
	if err != nil {
		return "", err
	}
	if err := sess.Scroll(ctx, args.Dx, args.Dy); err != nil {
		return "", fmt.Errorf(T("kaydırılamadı: %w", "could not scroll: %w"), err)
	}
	return T("Sayfa kaydırıldı.", "Scrolled the page."), nil
}

// BrowserScreenshot captures the current viewport of the active session as
// a base64-encoded PNG data URI. Returning the raw image bytes as text is a
// deliberate, known simplification for this checkpoint — Memo's tool-result
// pipeline doesn't yet build a proper multimodal (image) content block for
// providers that support one, so a vision-capable model at least has the
// bytes in reach; a live, low-cost preview for the *user* is a separate
// mechanism (the browser_frame SSE event, added in a later checkpoint) so
// this doesn't need to be cheap on every call.
func BrowserScreenshot(ctx context.Context, _ json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	sess, err := activeSession()
	if err != nil {
		return "", err
	}
	shot, err := sess.Screenshot(ctx)
	if err != nil {
		return "", fmt.Errorf(T("ekran görüntüsü alınamadı: %w", "could not take screenshot: %w"), err)
	}
	return fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(shot)), nil
}

// BrowserClose ends the active interactive session, if any. Always
// succeeds on "nothing to close" rather than erroring — the model should
// never need to check whether a session exists before cleaning one up.
func BrowserClose(ctx context.Context, _ json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	if err := InteractiveBrowser.StopSession(); err != nil {
		return "", fmt.Errorf(T("tarayıcı oturumu kapatılamadı: %w", "could not close browser session: %w"), err)
	}
	return T("Tarayıcı oturumu kapatıldı.", "Browser session closed."), nil
}
