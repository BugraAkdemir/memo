package app

import "memo/internal/config"

// GetOnboardingComplete reports whether the first-run wizard has been
// completed, per the durable server-side flag (see config.OnboardingConfig's
// doc comment for why this moved out of browser-local storage).
func (a *App) GetOnboardingComplete() bool {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Onboarding.Completed
}

// SetOnboardingComplete marks the first-run wizard done (or, via the
// Settings "run setup wizard again" action, resets it) for every client
// regardless of which origin/browser it connects through.
func (a *App) SetOnboardingComplete(completed bool) error {
	a.cfgMu.Lock()
	a.cfg.Onboarding.Completed = completed
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}

// GetOnboardingState returns the full durable onboarding state (wizard,
// spotlight tour, and launchpad).
func (a *App) GetOnboardingState() (completed, tourSeen, launchpadSeen bool) {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Onboarding.Completed, a.cfg.Onboarding.TourSeen, a.cfg.Onboarding.LaunchpadSeen
}

// SetOnboardingState updates any subset of the onboarding flags and persists
// config.yaml.
func (a *App) SetOnboardingState(completed, tourSeen, launchpadSeen *bool) error {
	a.cfgMu.Lock()
	if completed != nil {
		a.cfg.Onboarding.Completed = *completed
	}
	if tourSeen != nil {
		a.cfg.Onboarding.TourSeen = *tourSeen
	}
	if launchpadSeen != nil {
		a.cfg.Onboarding.LaunchpadSeen = *launchpadSeen
	}
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}
