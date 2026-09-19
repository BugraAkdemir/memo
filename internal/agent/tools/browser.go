package tools

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
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
	PageText(ctx context.Context) (string, error)
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

// interactiveBrowserInstallChecker is fetchpage.go's browserInstallChecker
// pattern applied to InteractiveBrowser instead of websearch.Browser — kept
// as its own type (not reused directly) since the two vars can be wired up
// independently in tests, so a missing websearch.Browser must not be
// mistaken for a missing InteractiveBrowser or vice versa. Same optional-
// capability-via-type-assertion reasoning either way, satisfied by
// browserToolAdapter in internal/app.
type interactiveBrowserInstallChecker interface {
	IsInstalled(ctx context.Context) bool
}

// interactiveBrowserEngineMissing reports whether starting a session would
// fail specifically because no Chromium-family browser is installed at all
// — as opposed to some other reason (a transient launch error, or a test
// that never wired InteractiveBrowser up to anything install-aware).
// Checked upfront in BrowserNavigate so the model gets the SAME actionable
// "install it from Settings" message fetch_page's browser fallback already
// gives, instead of a raw exec error bubbling up from chromedp.
func interactiveBrowserEngineMissing(ctx context.Context) bool {
	checker, ok := InteractiveBrowser.(interactiveBrowserInstallChecker)
	if !ok {
		return false
	}
	return !checker.IsInstalled(ctx)
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
	if interactiveBrowserEngineMissing(ctx) {
		return "", errors.New(T(
			"Etkileşimli tarayıcı için bir Chromium motoru gerekiyor ama şu an kurulu değil. Kullanıcıya bunu söyle, Ayarlar'dan tek tıkla kurabileceğini belirt ve kurmak isteyip istemediğini sor.",
			"The interactive browser needs a Chromium engine, which isn't installed right now. Tell the user this, mention they can install it from Settings with one click, and ask if they'd like to.",
		))
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

// lastFrameB64 holds the most recently captured screenshot, consumed
// exactly once by emitBrowserFrame (internal/app/browser_frame.go) via
// LastBrowserFrame below. This is deliberately NOT part of what
// BrowserScreenshot returns to the model — see that function's doc comment
// for why: putting a base64 PNG in the tool-result text means it rides
// along in the conversation history sent on every subsequent LLM call for
// the rest of the turn, and a real session doing navigate→screenshot→
// scroll→screenshot a few times measurably ballooned prompt tokens from
// ~25K to ~890K in one turn this way — a real, live, costly bug (it
// coincided with a provider account running out of credit mid-task), not a
// theoretical one. The live pane still gets every frame; the model just
// doesn't pay to carry pixels it can't see anyway (no multimodal tool-
// result content block is wired up yet — see that same doc comment).
var (
	lastFrameMu  sync.Mutex
	lastFrameB64 string
)

// LastBrowserFrame returns the most recent screenshot as base64 PNG, if
// one hasn't already been consumed, and clears it — a single-reader,
// consume-once handoff so a slow SSE consumer never re-sends a stale frame.
// Safe to call even when no frame is pending (returns "", false).
func LastBrowserFrame() (string, bool) {
	lastFrameMu.Lock()
	defer lastFrameMu.Unlock()
	b64 := lastFrameB64
	lastFrameB64 = ""
	return b64, b64 != ""
}

// BrowserScreenshot captures the current viewport of the active session.
// The image itself goes out via LastBrowserFrame/the browser_frame SSE
// event for the *user's* live pane to show — not in this function's own
// return value, which the model actually reads. See lastFrameB64's doc
// comment for the real, measured cost of getting this backwards: no
// multimodal tool-result content block is wired up yet, so the model can't
// see pixels either way: returning them as text was pure token cost for
// zero benefit, not a "the model looks at it" tradeoff.
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
	lastFrameMu.Lock()
	lastFrameB64 = base64.StdEncoding.EncodeToString(shot)
	lastFrameMu.Unlock()
	return T(
		"Ekran görüntüsü alındı — kullanıcının canlı tarayıcı panelinde görünüyor. Bu görselin piksellerini SEN göremiyorsun (henüz görsel/multimodal bir kanal yok) — yerleşim ya da görsel detay hakkında tahmin yürütme. Sayfada ne olduğunu anlamak için browser_get_text kullan.",
		"Screenshot captured — visible in the user's live browser pane. You cannot see its pixels yourself (no visual/multimodal channel is wired up yet) — don't guess at layout or visual detail. Use browser_get_text to actually know what's on the page.",
	), nil
}

// BrowserGetTextArgs is currently empty (whole-page text only) — kept as
// its own type so a selector-scoped variant can be added later without
// changing BrowserGetText's signature.
type BrowserGetTextArgs struct{}

// BrowserGetText reads the active session's page as plain visible text
// (document.body.innerText, truncated) — the model's only real grounding
// for what's on a page and, by extension, what a browser_click selector
// might plausibly be, given there is no visual channel (see
// BrowserScreenshot's doc comment).
func BrowserGetText(ctx context.Context, _ json.RawMessage, _ string, _ func(string) error) (string, error) {
	if InteractiveBrowser == nil {
		return "", browserNotConfigured()
	}
	sess, err := activeSession()
	if err != nil {
		return "", err
	}
	text, err := sess.PageText(ctx)
	if err != nil {
		return "", fmt.Errorf(T("sayfa metni okunamadı: %w", "could not read page text: %w"), err)
	}
	return text, nil
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
