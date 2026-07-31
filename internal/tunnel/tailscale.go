// SPDX-License-Identifier: AGPL-3.0-or-later

// Package tunnel exposes the local Memo web server to remote clients. The
// Tailscale implementation embeds a userspace Tailscale node (tsnet) directly
// in the Memo binary — no separate daemon or downloaded binary — and gives a
// stable URL that does not change across restarts (unlike ngrok's ephemeral
// URLs).
package tunnel

import (
	"context"
	"fmt"
	"memo/internal/browseropen"
	"memo/internal/logx"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"tailscale.com/client/local"
	"tailscale.com/tsnet"
)

// TailscaleConfig configures a Tailscale tunnel. Leaving AuthKey empty
// triggers interactive browser login instead (see connect below) — this is
// the default, key-free path used by the Settings UI's "Tailscale ile
// bağlan" button; a manually-entered AuthKey remains supported for
// headless/automated setups.
type TailscaleConfig struct {
	Hostname  string // node name on the tailnet (e.g. "memo")
	AuthKey   string // tailnet auth key (tskey-auth-...); empty = interactive login
	Funnel    bool   // expose a public HTTPS URL via Tailscale Funnel
	LocalPort int    // local web server port to reverse-proxy to
	StateDir  string // persistent identity/state directory

	// Interactive marks this attempt as directly triggered by the user
	// clicking "Tailscale ile Bağlan" in the Settings UI — someone is
	// actually watching for a browser prompt right now, so it's fine to
	// auto-open one and to block for up to interactiveLoginTimeout waiting
	// on their approval. False for the boot-time auto-start path
	// (startupTailscale) and any reconnect that inherits its cfg: nothing
	// there is a response to a click, so a fresh interactive login must
	// never surprise-open a browser tab with no human necessarily present,
	// and a resume attempt that turns out to need one should fail fast
	// (silentResumeTimeout) instead of tying up Start() for minutes — which
	// would otherwise also reject a real "Bağlan" click the user makes
	// while a silent boot attempt is still in flight ("tunnel already
	// running/connecting").
	Interactive bool
}

const (
	reconnectMinDelay   = 5 * time.Second
	reconnectMaxDelay   = 60 * time.Second
	healthCheckInterval = 20 * time.Second
	healthCheckTimeout  = 10 * time.Second

	// keyedLoginTimeout bounds the fast, non-interactive path (an auth key
	// authenticates in a couple of seconds against the control plane).
	keyedLoginTimeout = 90 * time.Second
	// interactiveLoginTimeout bounds the key-free, user-initiated path: a
	// human has to see the browser tab, pick an identity provider, and click
	// approve.
	interactiveLoginTimeout = 5 * time.Minute
	// silentResumeTimeout bounds a key-free, non-interactive attempt
	// (cfg.Interactive == false — the boot-time auto-start path). Resuming
	// an already-authenticated node from its persisted StateDir identity is
	// fast, same order of magnitude as the keyed path; if it instead turns
	// out a fresh interactive login is genuinely needed, this path must
	// never wait around for one (see TailscaleConfig.Interactive), so it
	// fails fast instead.
	silentResumeTimeout = 20 * time.Second
	// authURLPollInterval controls how often connect() checks tsnet's local
	// status for a pending login URL while waiting on Up().
	authURLPollInterval = 1 * time.Second
)

// tailscaleConn is what connect() produces: an authenticated node plus the
// listener it's serving on. Kept separate from *Tailscale so a (re)connect
// attempt can be built and validated before touching the struct's state.
type tailscaleConn struct {
	srv     *tsnet.Server
	ln      net.Listener
	dnsName string
	ip4     string
}

// Tailscale manages an embedded tsnet node that reverse-proxies to the local
// web server. Once started, a dropped connection (a killed serve loop, a
// control-plane hiccup) is retried automatically with backoff until Stop()
// is called — the caller does not need to notice a drop and manually
// restart the tunnel.
type Tailscale struct {
	mu             sync.Mutex
	srv            *tsnet.Server
	ln             net.Listener
	publicURL      string
	ipURL          string // http://100.x.x.x — bypasses MagicDNS
	lastErr        string
	running        bool
	stopped        bool   // true once Stop() has been called for the current session
	connecting     bool   // true while a Start() call's connect() is in flight — see Start's doc comment
	generation     int    // bumped on every Start()/Stop() so a stale reconnect goroutine can detect it's been superseded
	pendingAuthURL string // set while an interactive login is awaiting browser approval
}

// NewTailscale creates an (unstarted) Tailscale tunnel manager.
func NewTailscale() *Tailscale { return &Tailscale{} }

// connect brings up a single tsnet node and listener. It does not touch
// *Tailscale state, so it can be safely (re)tried independent of the
// struct's lock.
//
// When cfg.AuthKey is empty, tsnet.Up blocks waiting for a human to complete
// an interactive login in the browser rather than failing outright. While
// it's blocked, watchAuthURL polls the node's local status for the pending
// login URL and reports it via onAuthURL (nil is fine — the URL is simply
// dropped) so the caller can surface/auto-open it.
func connect(cfg TailscaleConfig, onAuthURL func(string)) (*tailscaleConn, error) {
	srv := &tsnet.Server{
		Hostname: cfg.Hostname,
		AuthKey:  cfg.AuthKey,
		Dir:      cfg.StateDir,
		Logf:     func(string, ...any) {}, // silence verbose tsnet logs
	}

	timeout := keyedLoginTimeout
	if cfg.AuthKey == "" {
		if cfg.Interactive {
			timeout = interactiveLoginTimeout
		} else {
			timeout = silentResumeTimeout
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	if cfg.AuthKey == "" {
		if err := srv.Start(); err != nil {
			srv.Close()
			return nil, fmt.Errorf("tailscale start: %w", err)
		}
		if lc, err := srv.LocalClient(); err == nil {
			watchCtx, watchCancel := context.WithCancel(ctx)
			defer watchCancel()
			go watchAuthURL(watchCtx, lc, onAuthURL)
		}
	}

	// Up blocks until the node is authenticated and has an address.
	status, err := srv.Up(ctx)
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("tailscale up: %w", err)
	}
	if onAuthURL != nil {
		onAuthURL("") // authenticated now — clear any pending login URL
	}

	var ln net.Listener
	if cfg.Funnel {
		ln, err = srv.ListenFunnel("tcp", ":443")
	} else {
		ln, err = srv.Listen("tcp", ":80")
	}
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("tailscale listen: %w", err)
	}

	conn := &tailscaleConn{srv: srv, ln: ln, dnsName: status.Self.DNSName}
	if status.Self != nil {
		for _, ip := range status.Self.TailscaleIPs {
			if ip.Is4() {
				conn.ip4 = ip.String()
				break
			}
		}
	}
	return conn, nil
}

// watchAuthURL polls the node's local status while it's waiting to be
// authenticated and reports the pending login URL (once per distinct value)
// via onAuthURL — the mechanism tsnet itself uses internally for
// printAuthURLLoop, just surfaced to the caller instead of only logged.
func watchAuthURL(ctx context.Context, lc *local.Client, onAuthURL func(string)) {
	if onAuthURL == nil {
		return
	}
	ticker := time.NewTicker(authURLPollInterval)
	defer ticker.Stop()
	last := ""
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			st, err := lc.StatusWithoutPeers(ctx)
			if err != nil || st.AuthURL == "" || st.AuthURL == last {
				continue
			}
			last = st.AuthURL
			onAuthURL(last)
		}
	}
}

// Start brings up the tsnet node and begins serving. It blocks until the node
// is authenticated and the URL is known, or until the timeout elapses. With
// an empty cfg.AuthKey it blocks for up to interactiveLoginTimeout waiting on
// a human to approve the login opened in their browser (see connect).
//
// t.mu is deliberately released before calling connect() — connect() feeds
// discovered auth URLs back through t.setPendingAuthURL, which itself needs
// t.mu, so holding the lock across the (up to interactiveLoginTimeout, i.e.
// minutes-long) blocking call would deadlock that callback against its own
// caller. It would also block Stop()/AuthURL()/IsRunning() for the same
// duration for any other goroutine — in particular, shutdownSync's
// wg.Wait() on Tailscale.Stop(), which is why a stuck interactive login
// used to make the whole app appear to hang on shutdown. The brief
// connecting flag (checked/set under the same short lock as the running
// check) closes the race a naive unlock-then-call would reopen: two
// overlapping Start() calls both slipping past the "not running yet" check.
func (t *Tailscale) Start(cfg TailscaleConfig) error {
	t.mu.Lock()
	if t.running || t.connecting {
		// Set lastErr even for this early rejection — otherwise a caller that
		// only polls LastError()/GetRemoteAccessStatus (e.g. the Settings UI's
		// poll loop after clicking Connect twice in a row) sees no error at
		// all and just spins silently until its own timeout.
		err := fmt.Errorf("tailscale tunnel already running")
		t.lastErr = err.Error()
		t.mu.Unlock()
		return err
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "memo"
	}
	t.connecting = true
	// A fresh attempt starts clean even if a previous run ended in Stop() —
	// Stop() can still flip this back to true mid-connect below, and that's
	// exactly the signal the post-connect check needs.
	t.stopped = false
	t.mu.Unlock()

	conn, err := connect(cfg, func(url string) { t.setPendingAuthURL(url, cfg.Interactive) })

	t.mu.Lock()
	t.connecting = false
	if err != nil {
		t.lastErr = err.Error()
		// This attempt is definitively over (context expired or a real
		// failure) — any auth URL it surfaced is no longer something a
		// click can usefully resume, so don't leave it lingering as if a
		// login were still pending.
		t.pendingAuthURL = ""
		t.mu.Unlock()
		return err
	}
	if t.stopped {
		// Stop() ran while connect() was still in flight (e.g. the user
		// cancelled an interactive login, or the app is shutting down).
		// Don't resurrect a tunnel that was explicitly torn down.
		t.mu.Unlock()
		conn.srv.Close()
		return fmt.Errorf("tailscale tunnel stopped during connect")
	}
	t.generation++
	gen := t.generation
	t.applyConn(conn, cfg)
	publicURL := t.publicURL
	t.mu.Unlock()

	logx.Printf("tunnel: tailscale up at %s (funnel=%v)", publicURL, cfg.Funnel)
	go t.serveAndSupervise(conn.ln, cfg, gen)
	go t.superviseHealth(cfg, gen)
	return nil
}

// setPendingAuthURL records the interactive-login URL tsnet is currently
// waiting on (or clears it, given "") and, the first time a given URL shows
// up during a user-initiated (autoOpen) attempt, opens it in the user's
// default browser — so the whole "Tailscale ile bağlan" flow needs no manual
// copy/paste of a key or a link. autoOpen is false for the boot-time
// auto-start path (see TailscaleConfig.Interactive): the URL is still
// recorded so GetRemoteAccessStatus/the Settings UI can surface a "log in
// again" link, but nothing pops a browser tab the user didn't ask for. Safe
// to call from connect()'s watcher goroutine concurrently with the rest of
// *Tailscale.
func (t *Tailscale) setPendingAuthURL(authURL string, autoOpen bool) {
	t.mu.Lock()
	isNew := authURL != "" && authURL != t.pendingAuthURL
	t.pendingAuthURL = authURL
	t.mu.Unlock()

	if isNew && autoOpen {
		if err := browseropen.OpenURL(authURL); err != nil {
			logx.Printf("tunnel: tailscale could not auto-open login URL (%v): %s", err, authURL)
		}
	}
}

// AuthURL returns the pending interactive-login URL, or "" when no login is
// currently awaiting browser approval.
func (t *Tailscale) AuthURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.pendingAuthURL
}

// applyConn installs a freshly connected node's derived state. Caller must
// hold t.mu.
func (t *Tailscale) applyConn(conn *tailscaleConn, cfg TailscaleConfig) {
	t.pendingAuthURL = ""
	dnsName := strings.TrimSuffix(conn.dnsName, ".")
	if cfg.Funnel {
		t.publicURL = "https://" + dnsName
	} else {
		t.publicURL = "http://" + dnsName
	}
	// Also expose the raw tailnet IP (http://100.x.x.x) so clients can connect
	// without depending on MagicDNS being enabled.
	t.ipURL = ""
	if conn.ip4 != "" {
		t.ipURL = "http://" + conn.ip4
	}
	t.srv = conn.srv
	t.ln = conn.ln
	t.running = true
	t.lastErr = ""
}

// serveAndSupervise proxies over ln until it fails, then — unless the tunnel
// was intentionally stopped or superseded by a newer Start() in the
// meantime — hands off to reconnectLoop instead of leaving the tunnel dead.
func (t *Tailscale) serveAndSupervise(ln net.Listener, cfg TailscaleConfig, gen int) {
	defer logx.Recover("tunnel.Tailscale serve")
	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.LocalPort))
	proxy := httputil.NewSingleHostReverseProxy(target)

	err := http.Serve(ln, proxy)

	t.mu.Lock()
	if t.stopped || t.generation != gen {
		t.mu.Unlock()
		return
	}
	t.running = false
	if err != nil {
		t.lastErr = err.Error()
	}
	t.mu.Unlock()
	logx.Printf("tunnel: tailscale serve stopped, reconnecting: %v", err)

	t.reconnectLoop(cfg, gen)
}

// reconnectLoop retries connect() with exponential backoff until it succeeds
// or the tunnel is stopped/superseded, then resumes serving on the new
// connection.
func (t *Tailscale) reconnectLoop(cfg TailscaleConfig, gen int) {
	delay := reconnectMinDelay
	for {
		time.Sleep(delay)

		t.mu.Lock()
		superseded := t.stopped || t.generation != gen
		t.mu.Unlock()
		if superseded {
			return
		}

		conn, err := connect(cfg, func(url string) { t.setPendingAuthURL(url, cfg.Interactive) })
		if err != nil {
			t.mu.Lock()
			if t.stopped || t.generation != gen {
				t.mu.Unlock()
				return
			}
			t.lastErr = err.Error()
			t.pendingAuthURL = ""
			t.mu.Unlock()
			logx.Printf("tunnel: tailscale reconnect attempt failed: %v", err)
			delay *= 2
			if delay > reconnectMaxDelay {
				delay = reconnectMaxDelay
			}
			continue
		}

		t.mu.Lock()
		if t.stopped || t.generation != gen {
			t.mu.Unlock()
			conn.srv.Close()
			return
		}
		t.applyConn(conn, cfg)
		t.mu.Unlock()

		logx.Printf("tunnel: tailscale reconnected at %s", t.PublicURL())
		go t.serveAndSupervise(conn.ln, cfg, gen)
		go t.superviseHealth(cfg, gen)
		return
	}
}

// superviseHealth periodically checks the tsnet backend's own connection
// state. A serve loop failure (handled by serveAndSupervise) only catches a
// killed listener — it says nothing about the node having quietly fallen off
// the tailnet (expired key, revoked node, control-plane issue) while the
// local listener keeps accepting connections that then can't actually reach
// anything. On a detected drop this closes the stale connection and starts
// reconnectLoop directly, same as a serve failure would.
func (t *Tailscale) superviseHealth(cfg TailscaleConfig, gen int) {
	defer logx.Recover("tunnel.Tailscale health check")
	ticker := time.NewTicker(healthCheckInterval)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		if t.stopped || t.generation != gen {
			t.mu.Unlock()
			return
		}
		srv := t.srv
		t.mu.Unlock()

		if srv == nil || isBackendHealthy(srv) {
			continue
		}

		t.mu.Lock()
		if t.stopped || t.generation != gen {
			t.mu.Unlock()
			return
		}
		// Bump the generation before tearing anything down: closing staleLn
		// below wakes serveAndSupervise's blocked http.Serve, and it must see
		// a stale generation so it doesn't *also* race its own reconnectLoop
		// against the one this function starts.
		t.generation++
		newGen := t.generation
		t.running = false
		t.lastErr = "tailscale backend unhealthy, reconnecting"
		staleSrv, staleLn := t.srv, t.ln
		t.srv, t.ln = nil, nil
		t.mu.Unlock()

		logx.Printf("tunnel: tailscale health check failed, reconnecting")
		if staleLn != nil {
			staleLn.Close()
		}
		if staleSrv != nil {
			staleSrv.Close()
		}
		t.reconnectLoop(cfg, newGen)
		return // the reconnectLoop launched above starts its own supervisors
	}
}

// isBackendHealthy reports whether the tsnet node still considers itself
// connected and running on the tailnet.
func isBackendHealthy(srv *tsnet.Server) bool {
	lc, err := srv.LocalClient()
	if err != nil {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), healthCheckTimeout)
	defer cancel()
	st, err := lc.StatusWithoutPeers(ctx)
	if err != nil {
		return false
	}
	return st.BackendState == "Running"
}

// Stop tears down the tunnel and cancels any in-flight reconnect attempt.
func (t *Tailscale) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running && t.stopped {
		return
	}
	t.stopped = true
	t.generation++ // supersede a reconnectLoop that's mid-backoff or mid-connect
	t.running = false
	if t.ln != nil {
		t.ln.Close()
		t.ln = nil
	}
	if t.srv != nil {
		t.srv.Close()
		t.srv = nil
	}
	t.publicURL = ""
	t.ipURL = ""
	t.pendingAuthURL = ""
}

// PublicURL returns the stable URL, or "" when not running.
func (t *Tailscale) PublicURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.publicURL
}

// IPURL returns the raw tailnet IP URL (http://100.x.x.x), or "" when not
// running. This works even when MagicDNS is disabled.
func (t *Tailscale) IPURL() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.ipURL
}

// IsRunning reports whether the tunnel is actually up and serving right now —
// not just "was started at some point and never explicitly Stop()-ed".
func (t *Tailscale) IsRunning() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.running
}

// LastError returns the last error string, if any.
func (t *Tailscale) LastError() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.lastErr
}
