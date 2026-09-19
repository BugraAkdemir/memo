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
	sess, ok := InteractiveBrowser.GetSession()
	if !ok {
		return "", errors.New(T(
			"aktif bir tarayıcı oturumu yok — önce browser_navigate çağır",
			"no active browser session — call browser_navigate first",
		))
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
