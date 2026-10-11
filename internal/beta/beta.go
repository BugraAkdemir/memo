// SPDX-License-Identifier: AGPL-3.0-or-later

// Package beta is the one place the rest of the backend asks "are beta
// features on?" without importing the app. The App installs the answer at
// startup (config.AppConfig.Beta, toggled in Settings › Beta Features); code
// that must not depend on the app — the agent tool registry, the system-prompt
// builder — reads it here.
//
// It fails closed: until a gate is installed, beta is off. A feature behind it
// is therefore absent, not half-present, in any process that never wired one
// up (a unit test, a tool run standalone).
package beta

import "sync/atomic"

var gate atomic.Pointer[func() bool]

// SetGate installs the function that reports whether beta features are on.
// Passing nil removes it (beta is then off).
func SetGate(f func() bool) {
	if f == nil {
		gate.Store(nil)
		return
	}
	gate.Store(&f)
}

// Enabled reports whether beta features are on right now. It is read on every
// call, so flipping the switch in Settings takes effect on the next turn
// without a restart.
func Enabled() bool {
	if p := gate.Load(); p != nil {
		return (*p)()
	}
	return false
}
