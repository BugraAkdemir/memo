package stt

import (
	"context"
	"errors"
	"fmt"
	"memo/internal/logx"
	"sort"
	"sync"
)

// Router manages multiple external STT providers with fallback support.
// Mirrors internal/tts.Router's fallback-chain design one-for-one (priority
// order, auto-disable after 3 consecutive failures, no HealthCheck loop —
// same "not called often enough to justify it" reasoning).
type Router struct {
	mu         sync.RWMutex
	providers  []*providerEntry
	configs    []ProviderConfig
	activeName string
}

type providerEntry struct {
	STTProvider
	cfg       ProviderConfig
	failCount int
	disabled  bool
}

// NewRouter creates a new STT router from configs.
func NewRouter(configs []ProviderConfig) *Router {
	r := &Router{}
	r.UpdateConfigs(configs)
	return r
}

// UpdateConfigs replaces all provider configs and rebuilds the provider list.
func (r *Router) UpdateConfigs(configs []ProviderConfig) {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.configs = configs
	r.providers = nil

	for _, cfg := range configs {
		if !cfg.Enabled {
			continue
		}
		if err := cfg.Validate(); err != nil {
			logx.Printf("STT: invalid provider config for %s: %v", cfg.Type, err)
			continue
		}
		p, err := NewProvider(cfg)
		if err != nil {
			logx.Printf("STT: failed to create provider %s: %v", cfg.Type, err)
			continue
		}
		r.providers = append(r.providers, &providerEntry{
			STTProvider: p,
			cfg:         cfg,
		})
	}

	sort.SliceStable(r.providers, func(i, j int) bool {
		return r.providers[i].cfg.Priority > r.providers[j].cfg.Priority
	})
}

// ActiveProviders returns the list of currently active (enabled) provider configs.
func (r *Router) ActiveProviders() []ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]ProviderConfig, 0, len(r.providers))
	for _, entry := range r.providers {
		if !entry.disabled {
			configs = append(configs, entry.cfg)
		}
	}
	return configs
}

// AllConfigs returns all provider configs (including disabled).
func (r *Router) AllConfigs() []ProviderConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	configs := make([]ProviderConfig, len(r.configs))
	copy(configs, r.configs)
	return configs
}

// Transcribe tries providers in order (by descending priority) with
// fallback, mirroring Router.Synthesize's error handling.
func (r *Router) Transcribe(ctx context.Context, audioData []byte) (string, error) {
	entries := r.getActiveEntries()
	if len(entries) == 0 {
		return "", errors.New("stt: no active provider")
	}

	var lastErr error
	for _, entry := range entries {
		text, err := entry.Transcribe(ctx, audioData)
		if err == nil {
			r.resetFailCount(entry)
			return text, nil
		}

		lastErr = err

		if ctx.Err() != nil {
			return "", ctx.Err()
		}
		if contextCanceledOrDeadline(err) {
			return "", err
		}

		r.recordFailure(entry)
		logx.Printf("STT: %s failed: %v, falling back", entry.Name(), err)
	}

	return "", fmt.Errorf("all stt providers failed: %w", lastErr)
}

// SetActiveProvider restricts the router to only use the given provider by
// name. Pass empty string to allow any enabled provider.
func (r *Router) SetActiveProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeName = name
}

// getActiveEntries returns non-disabled provider entries, sorted by
// descending priority. Callers must not hold r.mu.
func (r *Router) getActiveEntries() []*providerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*providerEntry
	for _, entry := range r.providers {
		if entry.disabled {
			continue
		}
		if r.activeName != "" && entry.cfg.Name != r.activeName {
			continue
		}
		entries = append(entries, entry)
	}

	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].cfg.Priority > entries[j].cfg.Priority
	})
	return entries
}

func (r *Router) recordFailure(entry *providerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()

	entry.failCount++
	if entry.failCount >= 3 {
		entry.disabled = true
		logx.Printf("STT: %s auto-disabled after %d consecutive failures", entry.Name(), entry.failCount)
	}
}

func (r *Router) resetFailCount(entry *providerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.failCount = 0
}

// ReenableProvider re-enables a previously auto-disabled provider by name.
func (r *Router) ReenableProvider(name string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.providers {
		if entry.cfg.Name == name {
			entry.disabled = false
			entry.failCount = 0
			logx.Printf("STT: %s re-enabled", name)
			return
		}
	}
}

// ReenableAllProviders re-enables all previously auto-disabled providers.
func (r *Router) ReenableAllProviders() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.providers {
		entry.disabled = false
		entry.failCount = 0
	}
}

// HasActiveProvider returns true if at least one provider is enabled and not disabled.
func (r *Router) HasActiveProvider() bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, entry := range r.providers {
		if !entry.disabled {
			return true
		}
	}
	return false
}

// contextCanceledOrDeadline returns true when err is a context.Canceled or
// context.DeadlineExceeded — mirrors internal/tts's identical helper.
func contextCanceledOrDeadline(err error) bool {
	return errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
