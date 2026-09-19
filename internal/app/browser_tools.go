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
