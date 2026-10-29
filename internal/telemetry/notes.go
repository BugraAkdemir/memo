// SPDX-License-Identifier: AGPL-3.0-or-later

package telemetry

import "sync/atomic"

// The rest of Memo reports things it did (a provider failed, an update was
// applied, a feature was used) through these package-level functions. They are
// no-ops until a Manager is installed, and the Manager itself keeps nothing
// unless the person agreed — so call sites never need to check anything.

var defaultManager atomic.Pointer[Manager]

// SetDefault installs the Manager the Note* functions report to (nil removes it).
func SetDefault(m *Manager) { defaultManager.Store(m) }

// Default returns the installed Manager, or nil.
func Default() *Manager { return defaultManager.Load() }

func NoteFeature(f string) {
	if m := Default(); m != nil {
		m.NoteFeature(f)
	}
}
func NoteProviderErr(provider, code string) {
	if m := Default(); m != nil {
		m.NoteProviderErr(provider, code)
	}
}
func NoteStartFail(component, code string) {
	if m := Default(); m != nil {
		m.NoteStartFail(component, code)
	}
}
func NoteExit(component, code string) {
	if m := Default(); m != nil {
		m.NoteExit(component, code)
	}
}
func NoteUpdate(event, code string) {
	if m := Default(); m != nil {
		m.NoteUpdate(event, code)
	}
}
func NoteMilestone(name string) {
	if m := Default(); m != nil {
		m.NoteMilestone(name)
	}
}

// NotePanic is the hook behind every recover in Memo: it sanitizes the panic
// (type name and Memo frames only) and counts it.
func NotePanic(r any, stack []byte) {
	if m := Default(); m != nil {
		if c, ok := SanitizePanic(r, stack); ok {
			m.NoteCrash(c)
		}
	}
}
