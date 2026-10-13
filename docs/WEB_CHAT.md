# Web chat (ChatGPT / claude.ai) — Beta

Use your own ChatGPT or Claude **web** chat from Memo instead of an API. Memo opens the site in a real
browser window with its own profile folder (`data/webchat/<site>`), you sign in there yourself, and Memo
types your message into the page and reads the reply back as markdown. The allowance that is spent is the
web chat's, not an API's or Codex's.

> **Read this first.** The sites' terms restrict automated use of their web apps. Your account can be
> limited or closed. Memo does not bypass sign-in or human checks (you do those in the window), it does not
> imitate private APIs and it does not hide that a browser is being driven — but none of that makes it
> permitted. It is **experimental**: the pages change without notice and the feature can stop working.
> Use it only with an account you are willing to risk. The in-app notice must be confirmed per site.

## Using it

1. Settings › Beta Features: turn Beta on. The *Web chat* tab does not exist in Settings until Beta is on.
2. Settings › Web chat: **Read the notice and enable** (tick the box) for the site.
3. **Connect**: a Memo browser window opens. Sign in and finish any human check yourself. The window is
   minimised when the page is ready.
4. **Add to my models**, then pick *ChatGPT (Web)* / *Claude (Web)* in the model selector.

**Which browser.** A Chromium-family browser with a real window: Chrome, Chromium, Edge, Brave, Vivaldi or
Opera. Memo prefers your default browser when it is one of those, otherwise the first installed from that
list; `MEMO_WEBCHAT_BROWSER=/path/to/browser` forces one. It always runs with Memo's own profile folder, so
you sign in once more there (current Chrome refuses a debugging port on your everyday profile, and mixing
the two would put your real tabs and cookies in reach). Firefox and Safari cannot be used: Memo drives the
page over the Chrome DevTools Protocol, which they do not offer. Memo's built-in "browser engine" from
Settings is a headless build with no window, so it cannot be used for sign-in.

Text only. No tool calling, files or pictures. Every turn is a fresh (temporary, for ChatGPT) conversation
that carries the system prompt and the transcript so far in one message, so nothing depends on state kept
on the site. Helper calls (chat titles, fact extraction, mood, proactive checks…) and "Test connection" are
**never** sent to a web chat; titles are made from your first words instead.

## How it is kept from breaking things

| Risk | What is done |
|---|---|
| A page changes | Selectors are data (`internal/webchat/site.go`), every list has fallbacks; all DOM access is one injected script (`page.js`) that returns JSON and never throws. |
| Typing into a signed-out page | The signed-out state wins over a visible composer and is detected by button **text** ("Log in", "Continue with Google"…) as well as attributes. Nothing is typed before sign-in is confirmed. |
| Confusing errors | Every failure is one of a few kinds (not installed, needs login, human check, page changed, quota, timeout, busy, disabled, no consent) with a plain sentence. Raw browser errors go to the log only. |
| Spending the account by accident | Off unless Beta **and** the notice are confirmed. Web chats are never a fallback (`provider.IsOptInOnly`) and never an Auto Route candidate. |
| Taking Memo down | Constructing the provider never starts a browser; panics are recovered; a dead or odd browser is dropped and restarted on the next turn; a failure leaves other providers untouched. |
| Half-streamed markdown | The unfinished last line is held back while the reply streams, so emitted text never has to be taken back. |

## What is verified, and what is not

Verified against a local fake page with a real Chromium (`internal/webchat`, `internal/e2e`): the whole
flow, error kinds, the signed-out rule, restart after the browser dies, the consent gate, no helper calls.
Verified by reading the **real** pages signed out (2026-10-13): chatgpt.com keeps `?temporary-chat=true`,
shows a usable composer and a "Log in" button without a test id; claude.ai/new redirects to `/login`.

**Not verified:** anything behind a sign-in — the real composer, send/stop buttons and assistant-message
hooks of either site, the markdown conversion of real replies, the real quota wording and Cloudflare
behaviour. Both sites carry `Verified=false` and the UI labels them *Experimental* until someone checks
them with a signed-in account (then set `Verified` and fix the selectors that differ).

## Files

`internal/webchat/` — `site.go` (selector tables), `page.js` (injected script), `browser.go` (Chromium
lifecycle), `driver.go` (state detection, send, stream), `provider.go` (`web-chatgpt` / `web-claude`),
`consent.go`, `errors.go`, `webchattest/` (fake page). API: `/api/webchat`. UI:
`frontend/lib/widgets/settings/tabs/web_chat_tab.dart`.

Approach credit: the selector/streaming logic follows MIT-licensed
[MrBalourd/chatgpt-api-web](https://github.com/MrBalourd/chatgpt-api-web); the claude.ai half follows
[yushangxiao/claude2api](https://github.com/yushangxiao/claude2api).
