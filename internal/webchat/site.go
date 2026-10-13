// SPDX-License-Identifier: AGPL-3.0-or-later

package webchat

import (
	"regexp"
	"strings"
)

// Site is everything Memo knows about one web chat: where it lives and how
// its page is recognised. It is DATA, not code, on purpose. The pages change
// without notice; when one does, the fix is a line in a selector list here,
// and every list holds more than one entry so a single renamed attribute
// degrades to the next candidate instead of breaking the provider.
//
// The approach (ordered fallback selectors for the composer, "the assistant's
// message" by role attribute, "still generating" by the stop button, a reply
// counts as finished once it has stopped changing) follows MIT-licensed
// MrBalourd/chatgpt-api-web, which drives the same page through Playwright;
// the claude.ai half follows the session flow of yushangxiao/claude2api. Memo
// drives a real, visible, user-signed-in browser instead of imitating either
// site's private API, so there is nothing to forge and no bot check to solve:
// the person completes sign-in and any human check in the window themselves.
type Site struct {
	ID       string // stable key: "chatgpt", "claude" — also the profile folder name
	Name     string // shown to the user
	StartURL string // a fresh conversation
	// Host is the registrable site the profile is for. A navigation that ends
	// elsewhere (an SSO provider, an error page) is not "the chat page".
	Host string

	// Composer is where the message is typed. Tried in order; the first one
	// that is visible wins.
	Composer []string
	// Send is the send button. If none is found Enter is pressed instead.
	Send []string
	// Stop is the button that exists only while a reply is being produced.
	Stop []string
	// Assistant matches one element per assistant message, in page order.
	// The last one is the reply being read.
	Assistant []string
	// LoginMarkers, when visible, mean the page is in its signed-out state.
	// They win over a visible composer: ChatGPT shows a working composer to
	// signed-out visitors, and answering from that would silently spend an
	// anonymous allowance instead of the person's account.
	LoginMarkers []string
	// LoginText matches the whole visible text of a button or link that only
	// exists for signed-out visitors ("Log in", "Sign up for free"). Seen on
	// the real chatgpt.com: the signed-out page has no data-testid on those
	// buttons and its /auth/login links are hidden, while a composer is fully
	// usable — attribute selectors alone would have read it as signed in.
	LoginText *regexp.Regexp
	// LoginPaths are URL path fragments of the sign-in flow.
	LoginPaths []string
	// ChallengeMarkers are elements of a human-verification interstitial.
	ChallengeMarkers []string
	// ChallengeTitle matches the document title of such an interstitial.
	ChallengeTitle *regexp.Regexp
	// QuotaText matches the wording the site shows when the allowance is
	// used up. Checked only when a reply failed to start, never on every poll.
	QuotaText *regexp.Regexp
	// Verified records whether the selectors above have been seen working
	// against the real page of a signed-in account. false is an honest label,
	// surfaced by Status so the UI can say "experimental".
	Verified bool
}

// What has actually been seen on the real pages (2026-10-13, signed OUT, no
// message sent): chatgpt.com keeps ?temporary-chat=true, shows a visible
// textarea composer and a visible "Log in" button WITHOUT a test id (so
// LoginText is what catches it); claude.ai/new redirects to /login with
// "Continue with Google/Apple/email/SSO". Nothing about signed-in pages —
// the composer's ProseMirror form, the send/stop buttons and the assistant
// message hooks come from the reference repos and are unverified, which is
// why Verified stays false for both.
var (
	chatgptSite = Site{
		ID:       "chatgpt",
		Name:     "ChatGPT",
		StartURL: "https://chatgpt.com/?temporary-chat=true",
		Host:     "chatgpt.com",
		Composer: []string{
			"#prompt-textarea",
			"div.ProseMirror[contenteditable='true']",
			"textarea",
			"div[contenteditable='true'][role='textbox']",
			"div[contenteditable='true']",
		},
		Send: []string{
			"button[data-testid='send-button']",
			"button[aria-label*='Send']",
			"button[aria-label*='Gönder']",
		},
		Stop: []string{
			"button[data-testid='stop-button']",
			"button[aria-label*='Stop']",
			"button[aria-label*='Durdur']",
		},
		Assistant: []string{
			"[data-message-author-role='assistant']",
			"article [data-message-author-role='assistant']",
		},
		LoginMarkers: []string{
			"[data-testid='login-button']",
			"a[href*='/auth/login']",
			"button[data-testid='signup-button']",
		},
		LoginText:        regexp.MustCompile(`(?i)^\s*(log ?in|sign ?up( for free)?|giri[sş] yap|oturum a[cç]|kaydol)\s*$`),
		LoginPaths:       []string{"/auth/", "/log-in", "/login"},
		ChallengeMarkers: []string{"#challenge-form", "iframe[src*='challenges.cloudflare.com']", "#cf-challenge-running"},
		ChallengeTitle:   regexp.MustCompile(`(?i)just a moment|attention required|verify you are human|bir dakika`),
		QuotaText:        regexp.MustCompile(`(?i)(reached|hit|exceeded).{0,40}(limit|cap)|usage (limit|cap)|too many (requests|messages)|limit(e|i)?ne ula[sş]t[ıi]n`),
	}

	claudeSite = Site{
		ID:       "claude",
		Name:     "Claude",
		StartURL: "https://claude.ai/new",
		Host:     "claude.ai",
		Composer: []string{
			"div[contenteditable='true'].ProseMirror",
			"[data-testid='chat-input']",
			"div[contenteditable='true'][role='textbox']",
			"textarea",
			"div[contenteditable='true']",
		},
		Send: []string{
			"button[aria-label='Send message']",
			"button[aria-label*='Send']",
			"button[aria-label*='Gönder']",
		},
		Stop: []string{
			"button[aria-label='Stop response']",
			"button[aria-label*='Stop']",
			"[data-is-streaming='true']",
		},
		Assistant: []string{
			"[data-testid='assistant-message']",
			"div.font-claude-response",
			"div.font-claude-message",
		},
		LoginMarkers:     []string{"input[type='email']", "a[href*='/login']", "button[data-testid='login-with-google']"},
		LoginText:        regexp.MustCompile(`(?i)^\s*(log ?in|sign ?up|continue with (google|email|apple|sso))\s*$`),
		LoginPaths:       []string{"/login", "/logout", "/oauth"},
		ChallengeMarkers: []string{"#challenge-form", "iframe[src*='challenges.cloudflare.com']", "#cf-challenge-running"},
		ChallengeTitle:   regexp.MustCompile(`(?i)just a moment|attention required|verify you are human`),
		QuotaText:        regexp.MustCompile(`(?i)(reached|hit|exceeded).{0,40}(limit|cap)|usage (limit|cap)|limit.{0,20}reset|too many (requests|messages)`),
	}

	allSites = map[string]*Site{
		chatgptSite.ID: &chatgptSite,
		claudeSite.ID:  &claudeSite,
	}
)

// SiteByID returns the table entry for id ("chatgpt", "claude"), or nil.
func SiteByID(id string) *Site { return allSites[id] }

// Sites lists the known sites in a stable order.
func Sites() []*Site { return []*Site{&chatgptSite, &claudeSite} }

// pageConfig is the part of a Site the injected script needs, as JSON.
type pageConfig struct {
	Composer         []string `json:"composer"`
	Send             []string `json:"send"`
	Stop             []string `json:"stop"`
	Assistant        []string `json:"assistant"`
	LoginMarkers     []string `json:"loginMarkers"`
	LoginText        string   `json:"loginText"`
	LoginPaths       []string `json:"loginPaths"`
	ChallengeMarkers []string `json:"challengeMarkers"`
	ChallengeTitle   string   `json:"challengeTitle"`
	QuotaText        string   `json:"quotaText"`
}

func (s *Site) pageConfig() pageConfig {
	pc := pageConfig{
		Composer: s.Composer, Send: s.Send, Stop: s.Stop, Assistant: s.Assistant,
		LoginMarkers: s.LoginMarkers, LoginPaths: s.LoginPaths, ChallengeMarkers: s.ChallengeMarkers,
	}
	pc.LoginText = jsPattern(s.LoginText)
	pc.ChallengeTitle = jsPattern(s.ChallengeTitle)
	pc.QuotaText = jsPattern(s.QuotaText)
	return pc
}

// jsPattern hands a Go regexp to the page. The tables write them with a
// leading (?i), which JavaScript does not understand; the script always
// compiles them case-insensitively, so the prefix is simply dropped. Every
// other construct used in the tables is common to RE2 and JavaScript.
func jsPattern(re *regexp.Regexp) string {
	if re == nil {
		return ""
	}
	return strings.TrimPrefix(re.String(), "(?i)")
}
