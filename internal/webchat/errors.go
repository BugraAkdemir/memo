// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"errors"
	"fmt"
)

// ErrKind sorts every way a web chat turn can fail into a handful of
// situations a person can act on. The raw cause (a chromedp or CDP error, a
// context deadline) is logged and kept in Err.Cause, but it is never what the
// user reads: "rpc error: -32000 ... context deadline exceeded" tells nobody
// what to do, "Sign in to ChatGPT first" does.
type ErrKind string

const (
	KindNotInstalled ErrKind = "not_installed" // no Chromium-family browser on this machine
	KindNeedsLogin   ErrKind = "needs_login"   // the page shows a signed-out state
	KindChallenge    ErrKind = "challenge"     // a human check (Cloudflare, captcha) is on screen
	KindPageChanged  ErrKind = "page_changed"  // the page no longer looks like what the site table describes
	KindQuota        ErrKind = "quota"         // the site says the account's allowance is used up
	KindTimeout      ErrKind = "timeout"       // no (more) answer arrived in time
	KindBusy         ErrKind = "busy"          // another message is already in flight to this site
	KindDisabled     ErrKind = "disabled"      // Beta Features is off
	KindNoConsent    ErrKind = "no_consent"    // the notice has not been confirmed for this site
	KindBrowser      ErrKind = "browser"       // the browser died or could not be driven
)

// Err is the one error type this package returns across its boundary.
type Err struct {
	Kind  ErrKind
	Site  string // display name, e.g. "ChatGPT"
	Cause error  // never shown to the user, only logged
	// Extra is optional detail that is safe to show (for KindQuota: the
	// site's own wording about when the allowance returns).
	Extra string
}

func (e *Err) Error() string { return e.Message() }

func (e *Err) Unwrap() error { return e.Cause }

// Message is the sentence the user sees. Plain English, no stack, no
// internals; the UI may localise by Kind.
func (e *Err) Message() string {
	site := e.Site
	if site == "" {
		site = "The web chat"
	}
	switch e.Kind {
	case KindNotInstalled:
		return "No Chromium-based browser was found on this computer. Install one from Settings › General, then try again."
	case KindNeedsLogin:
		return fmt.Sprintf("You are not signed in to %s. Open Settings › Web chat and press Connect, then sign in in the window that opens.", site)
	case KindChallenge:
		return fmt.Sprintf("%s is asking for a human check. Finish it in the Memo browser window, then send your message again.", site)
	case KindPageChanged:
		return fmt.Sprintf("%s's page looks different from what Memo expects, so the reply could not be read. Your other providers are not affected. Try again later or update Memo.", site)
	case KindQuota:
		msg := fmt.Sprintf("%s says you have reached its usage limit.", site)
		if e.Extra != "" {
			msg += " " + e.Extra
		}
		return msg
	case KindTimeout:
		return fmt.Sprintf("%s did not answer in time. Try again.", site)
	case KindBusy:
		return fmt.Sprintf("Another message is already being sent to %s. Wait for it to finish.", site)
	case KindDisabled:
		return "Web chat is a Beta feature. Turn on Beta Features in Settings to use it."
	case KindNoConsent:
		return "Read and confirm the web chat notice in Settings › Web chat before using it."
	default:
		return fmt.Sprintf("%s could not be reached through the browser. Try again.", site)
	}
}

// KindOf returns the kind of an error produced by this package, or "" for
// anything else.
func KindOf(err error) ErrKind {
	var e *Err
	if errors.As(err, &e) {
		return e.Kind
	}
	return ""
}

func newErr(kind ErrKind, site string, cause error) *Err {
	return &Err{Kind: kind, Site: site, Cause: cause}
}

// Message is the sentence for a kind, without needing an error value.
func Message(kind ErrKind, site *Site) string {
	return (&Err{Kind: kind, Site: site.Name}).Message()
}
