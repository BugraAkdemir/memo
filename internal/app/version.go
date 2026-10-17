package app

import "strings"

// GetVersion returns the application version string.
func (a *App) GetVersion() string {
	return strings.TrimSpace(a.version)
}
