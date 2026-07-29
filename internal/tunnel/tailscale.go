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
	"memo/internal/logx"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"tailscale.com/tsnet"
)

// TailscaleConfig configures a Tailscale tunnel.
type TailscaleConfig struct {
	Hostname  string // node name on the tailnet (e.g. "memo")
	AuthKey   string // tailnet auth key (tskey-auth-...)
	Funnel    bool   // expose a public HTTPS URL via Tailscale Funnel
	LocalPort int    // local web server port to reverse-proxy to
	StateDir  string // persistent identity/state directory
}

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
// web server.
type Tailscale struct {
	mu        sync.Mutex
	srv       *tsnet.Server
	ln        net.Listener
	publicURL string
	ipURL     string // http://100.x.x.x — bypasses MagicDNS
	lastErr   string
	running   bool
}

// NewTailscale creates an (unstarted) Tailscale tunnel manager.
func NewTailscale() *Tailscale { return &Tailscale{} }

// connect brings up a single tsnet node and listener. It does not touch
// *Tailscale state, so it can be safely (re)tried independent of the
// struct's lock.
func connect(cfg TailscaleConfig) (*tailscaleConn, error) {
	srv := &tsnet.Server{
		Hostname: cfg.Hostname,
		AuthKey:  cfg.AuthKey,
		Dir:      cfg.StateDir,
		Logf:     func(string, ...any) {}, // silence verbose tsnet logs
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Up blocks until the node is authenticated and has an address.
	status, err := srv.Up(ctx)
	if err != nil {
		srv.Close()
		return nil, fmt.Errorf("tailscale up: %w", err)
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

// Start brings up the tsnet node and begins serving. It blocks until the node
// is authenticated and the URL is known, or until the timeout elapses.
func (t *Tailscale) Start(cfg TailscaleConfig) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if t.running {
		return fmt.Errorf("tailscale tunnel already running")
	}
	if cfg.AuthKey == "" {
		return fmt.Errorf("tailscale auth key is required")
	}
	if cfg.Hostname == "" {
		cfg.Hostname = "memo"
	}

	conn, err := connect(cfg)
	if err != nil {
		t.lastErr = err.Error()
		return err
	}

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

	target, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", cfg.LocalPort))
	proxy := httputil.NewSingleHostReverseProxy(target)
	ln := conn.ln

	go func() {
		defer logx.Recover("tunnel.Tailscale serve")
		err := http.Serve(ln, proxy)
		t.mu.Lock()
		// Only record this as a live failure if nothing has torn the tunnel
		// down (or replaced it) in the meantime — otherwise this is just the
		// listener closing because Stop() ran, which is expected and not an
		// error condition.
		if t.running && t.ln == ln {
			t.running = false
			if err != nil {
				t.lastErr = err.Error()
			}
		}
		t.mu.Unlock()
		logx.Printf("tunnel: tailscale serve stopped: %v", err)
	}()

	logx.Printf("tunnel: tailscale up at %s (funnel=%v)", t.publicURL, cfg.Funnel)
	return nil
}

// Stop tears down the tunnel.
func (t *Tailscale) Stop() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !t.running {
		return
	}
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
