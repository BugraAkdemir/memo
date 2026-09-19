package app

import (
	"context"

	"memo/internal/agent/tools"
	"memo/internal/browserengine"
)

// browserToolAdapter wraps *browserengine.Manager to satisfy
// tools.InteractiveBrowser — same pattern as calendarToolAdapter in
// learning.go for tools.CalendarClient, kept local to this file since the
// wrapping here is a pure pass-through (no field mapping needed:
// *browserengine.Session already implements tools.BrowserSession directly).
type browserToolAdapter struct {
	m *browserengine.Manager
}

func (a browserToolAdapter) StartSession(ctx context.Context) (tools.BrowserSession, error) {
	return a.m.StartSession(ctx)
}

func (a browserToolAdapter) GetSession() (tools.BrowserSession, bool) {
	return a.m.GetSession()
}

func (a browserToolAdapter) StopSession() error {
	return a.m.StopSession()
}

// IsInstalled satisfies tools.browserInstallChecker (an optional capability
// checked via type assertion, same as fetchpage.go's own use of it) — lets
// BrowserNavigate give an upfront, actionable "install the browser engine
// from Settings" message instead of the raw exec error a failed
// StartSession would otherwise surface.
func (a browserToolAdapter) IsInstalled(ctx context.Context) bool {
	return a.m.IsInstalled(ctx)
}
