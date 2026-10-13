// SPDX-License-Identifier: AGPL-3.0-or-later

// Package webchattest serves a stand-in chat page for tests: the DOM hooks the
// site tables in internal/webchat name, with a reply that appears piece by
// piece. It proves Memo's logic against the page SHAPE; it says nothing about
// whether the real sites still look like this.
package webchattest

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"
)

// fakeChat is a stand-in for a chat site's page: the same DOM hooks the
// site tables name (a contenteditable composer, a send button that enables
// once there is text, a stop button while "generating", assistant messages
// carrying data-message-author-role), with a reply that appears piece by
// piece. It is served over real HTTP and driven by a real Chromium. It proves
// Memo's logic against the page shape; it does NOT prove the real sites still
// look like this (see Site.Verified).
type FakeChat struct {
	srv *httptest.Server // URL() is its address
	// behaviour switches
	mode    atomic.Value // "ok" | "login" | "challenge" | "quota" | "silent"
	sends   atomic.Int32
	lastMsg atomic.Value // last user message the page received
}

const fakeReplyHTML = `<p>Hello <strong>world</strong>, here is a list:</p><ul><li>one</li><li>two</li></ul><pre><button>Copy</button><code class="language-go">fmt.Println("hi")</code></pre><p>Done.</p>`

// New starts the fake site; it is closed with the test.
func New(t *testing.T) *FakeChat {
	f := &FakeChat{}
	f.mode.Store("ok")
	mux := http.NewServeMux()
	mux.HandleFunc("/new", func(w http.ResponseWriter, r *http.Request) {
		switch f.mode.Load().(string) {
		case "login":
			fmt.Fprint(w, `<html><title>Home</title><body><a data-testid="login-button" href="/login">Log in</a><div id="prompt-textarea" contenteditable="true" class="ProseMirror"></div></body></html>`)
			return
		case "login_text":
			// What the real signed-out chatgpt.com looks like: a visible
			// "Log in" button with no test id, hidden /auth/login links, and a
			// composer that works for anonymous visitors.
			fmt.Fprint(w, `<html><title>Home</title><body><button>Log in</button><button>Sign up for free</button><a href="/auth/login" style="display:none">Continue with Google</a><div id="prompt-textarea" contenteditable="true" class="ProseMirror"></div><button data-testid="send-button">Send</button></body></html>`)
			return
		case "challenge":
			fmt.Fprint(w, `<html><title>Just a moment...</title><body><div id="challenge-form">checking</div></body></html>`)
			return
		}
		fmt.Fprint(w, fakePage(f.mode.Load().(string)))
	})
	mux.HandleFunc("/sent", func(w http.ResponseWriter, r *http.Request) {
		f.sends.Add(1)
		b := make([]byte, 1<<16)
		n, _ := r.Body.Read(b)
		f.lastMsg.Store(string(b[:n]))
	})
	f.srv = httptest.NewServer(mux)
	t.Cleanup(f.srv.Close)
	return f
}

func fakePage(mode string) string {
	return `<html><title>Chat</title><body>
<div id="thread"></div>
<div id="prompt-textarea" contenteditable="true" class="ProseMirror" style="min-height:40px;border:1px solid #000"></div>
<button data-testid="send-button" id="send" disabled>Send</button>
<button data-testid="stop-button" id="stop" style="display:none">Stop</button>
<script>
var MODE = ` + fmt.Sprintf("%q", mode) + `;
var composer = document.getElementById('prompt-textarea');
var send = document.getElementById('send');
var stop = document.getElementById('stop');
composer.addEventListener('input', function(){ send.disabled = composer.innerText.trim().length === 0; });
composer.addEventListener('paste', function(e){ e.preventDefault(); var t=(e.clipboardData||window.clipboardData).getData('text/plain'); document.execCommand('insertText', false, t); });
send.addEventListener('click', function(){
  var text = composer.innerText;
  fetch('/sent', {method:'POST', body:text});
  composer.innerText=''; send.disabled=true;
  var u=document.createElement('div'); u.setAttribute('data-message-author-role','user'); u.textContent=text; document.getElementById('thread').appendChild(u);
  if (MODE === 'silent') return;
  if (MODE === 'quota') { var q=document.createElement('div'); q.textContent="You've reached your usage limit. Try again after 5:00 PM."; document.body.appendChild(q); return; }
  var a=document.createElement('div'); a.setAttribute('data-message-author-role','assistant'); document.getElementById('thread').appendChild(a);
  stop.style.display='inline';
  var parts = ` + jsParts() + `;
  var i=0;
  (function step(){ if(i<parts.length){ a.innerHTML += parts[i++]; setTimeout(step, 60);} else { stop.style.display='none'; } })();
});
</script></body></html>`
}

// jsParts splits the reply into chunks revealed one after another, cutting
// inside elements' boundaries only (the page appends whole nodes), so a poll
// can land between any two of them.
func jsParts() string {
	re := regexp.MustCompile(`</(p|ul|pre)>`)
	var parts []string
	rest := fakeReplyHTML
	for {
		loc := re.FindStringIndex(rest)
		if loc == nil {
			if rest != "" {
				parts = append(parts, rest)
			}
			break
		}
		parts = append(parts, rest[:loc[1]])
		rest = rest[loc[1]:]
	}
	var sb strings.Builder
	sb.WriteString("[")
	for i, p := range parts {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(fmt.Sprintf("%q", p))
	}
	sb.WriteString("]")
	return sb.String()
}

// URL is the page a Site's StartURL should point at.
func (f *FakeChat) URL() string { return f.srv.URL + "/new" }

// SetMode picks the page behaviour: ok, login, login_text, challenge, quota, silent.
func (f *FakeChat) SetMode(m string) { f.mode.Store(m) }

// Sends is how many messages the page has received.
func (f *FakeChat) Sends() int { return int(f.sends.Load()) }

// LastMessage is the text of the last message the page received.
func (f *FakeChat) LastMessage() string { s, _ := f.lastMsg.Load().(string); return s }
