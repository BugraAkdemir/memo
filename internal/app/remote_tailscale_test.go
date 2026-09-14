// SPDX-License-Identifier: AGPL-3.0-or-later

package app

import (
	"testing"

	"memo/internal/config"
	"memo/internal/webserver"
)

// newTailscaleBindTestApp builds a minimal App wired to a real
// webserver.Server (real net.Listen, no mocking) without going through the
// full Startup() sequence — ensureTailscaleWebServerBind only touches the
// web server's bind state, so it doesn't need memory/providers/etc.
// initialized.
func newTailscaleBindTestApp(t *testing.T) *App {
	t.Helper()
	a := &App{cfg: &config.AppConfig{}}
	a.setWebServer(webserver.New(a))
	return a
}

// TestEnsureTailscaleWebServerBind_RebindsFromLoopback is the regression
// test for the P0 Tailscale/Funnel auth-bypass finding: before the fix,
// SetTailscaleMode/startupTailscale left (or put) the web server on
// 127.0.0.1 while a Tailscale tunnel reverse-proxied real tailnet/Funnel
// traffic to it — and remoteAuthOK (server.go) skips its auth check
// entirely whenever the listener isn't bound to 0.0.0.0, so every request
// arriving through the tunnel bypassed auth regardless of AuthMode.
func TestEnsureTailscaleWebServerBind_RebindsFromLoopback(t *testing.T) {
	a := newTailscaleBindTestApp(t)
	ws := a.getWebServer()
	if err := ws.StartHTTPWithAddr(0, "127.0.0.1"); err != nil {
		t.Fatalf("StartHTTPWithAddr(127.0.0.1): %v", err)
	}
	t.Cleanup(func() { _ = ws.Stop() })

	if got := ws.GetListenAddr(); got != "127.0.0.1" {
		t.Fatalf("sanity check failed: listen addr = %q, want 127.0.0.1", got)
	}

	if err := a.ensureTailscaleWebServerBind(ws.GetPort()); err != nil {
		t.Fatalf("ensureTailscaleWebServerBind: %v", err)
	}

	if got := ws.GetListenAddr(); got != "0.0.0.0" {
		t.Fatalf("listen addr after ensureTailscaleWebServerBind = %q, want 0.0.0.0 — "+
			"a Tailscale/Funnel tunnel reverse-proxying here would bypass auth entirely", got)
	}
	if !ws.IsRunning() {
		t.Fatal("web server not running after rebind")
	}
}

// TestEnsureTailscaleWebServerBind_NoOpWhenAlreadyCorrect confirms a server
// already bound to 0.0.0.0 is left alone (no unnecessary stop/restart, which
// would drop any in-flight connections).
func TestEnsureTailscaleWebServerBind_NoOpWhenAlreadyCorrect(t *testing.T) {
	a := newTailscaleBindTestApp(t)
	ws := a.getWebServer()
	if err := ws.StartHTTPWithAddr(0, "0.0.0.0"); err != nil {
		t.Fatalf("StartHTTPWithAddr(0.0.0.0): %v", err)
	}
	t.Cleanup(func() { _ = ws.Stop() })
	port := ws.GetPort()

	if err := a.ensureTailscaleWebServerBind(port); err != nil {
		t.Fatalf("ensureTailscaleWebServerBind: %v", err)
	}
	if got := ws.GetListenAddr(); got != "0.0.0.0" {
		t.Fatalf("listen addr = %q, want 0.0.0.0", got)
	}
	if got := ws.GetPort(); got != port {
		t.Fatalf("port changed from %d to %d — server was needlessly restarted", port, got)
	}
}

// TestEnsureTailscaleWebServerBind_StartsFromStopped covers the boot-time
// path (startupTailscale), where the web server may not be running yet at
// all.
func TestEnsureTailscaleWebServerBind_StartsFromStopped(t *testing.T) {
	a := newTailscaleBindTestApp(t)
	ws := a.getWebServer()
	t.Cleanup(func() { _ = ws.Stop() })

	if err := a.ensureTailscaleWebServerBind(0); err != nil {
		t.Fatalf("ensureTailscaleWebServerBind: %v", err)
	}
	if !ws.IsRunning() {
		t.Fatal("web server not started")
	}
	if got := ws.GetListenAddr(); got != "0.0.0.0" {
		t.Fatalf("listen addr = %q, want 0.0.0.0", got)
	}
}

// TestEnsureTailscaleWebServerBind_NilWebServerIsNoop matches the
// pre-existing nil tolerance the callers relied on before this fix.
func TestEnsureTailscaleWebServerBind_NilWebServerIsNoop(t *testing.T) {
	a := &App{cfg: &config.AppConfig{}}
	if err := a.ensureTailscaleWebServerBind(12345); err != nil {
		t.Fatalf("ensureTailscaleWebServerBind with nil web server: %v", err)
	}
}
