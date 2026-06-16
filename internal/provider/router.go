package provider

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"
)

// Router manages multiple providers with fallback support.
type Router struct {
	mu            sync.RWMutex
	providers     []*providerEntry
	configs       []ProviderConfig
	activeType    ProviderType // only this type will be used; empty = use any
}

type providerEntry struct {
	Provider
	cfg       ProviderConfig
	failCount int
	disabled  bool
}

// NewRouter creates a new provider router from configs.
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
			log.Printf("PROVIDER: invalid config for %s: %v", cfg.Type, err)
			continue
		}
		p, err := NewProvider(cfg)
		if err != nil {
			log.Printf("PROVIDER: failed to create %s: %v", cfg.Type, err)
			continue
		}
		r.providers = append(r.providers, &providerEntry{
			Provider: p,
			cfg:      cfg,
		})
	}

	// Sort by descending priority so higher-priority providers are tried first.
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

// ChatCompletion tries providers in order with fallback.
func (r *Router) ChatCompletion(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	entries := r.getActiveEntries()
	if len(entries) == 0 {
		return nil, ErrProviderNotAvailable
	}

	var lastErr error
	for _, entry := range entries {
		resp, err := entry.ChatCompletion(ctx, req)
		if err == nil {
			r.resetFailCount(entry)
			return resp, nil
		}

		lastErr = err
		r.recordFailure(entry)

		var pErr *ProviderError
		if strings.Contains(err.Error(), ErrRateLimited.Error()) {
			pErr = &ProviderError{
				Provider: entry.Name(),
				Err:      fmt.Errorf("%w, falling back to next provider", ErrRateLimited),
			}
		} else {
			pErr = &ProviderError{
				Provider: entry.Name(),
				Err:      fmt.Errorf("%w, falling back to next provider", err),
			}
		}

		if pErr != nil {
			log.Printf("PROVIDER: %v", pErr)
		}

		// Check context before trying next provider
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// ChatCompletionStream tries providers in order with fallback for streaming.
func (r *Router) ChatCompletionStream(ctx context.Context, req ChatRequest) (<-chan StreamChunk, error) {
	entries := r.getActiveEntries()
	if len(entries) == 0 {
		return nil, ErrProviderNotAvailable
	}

	var lastErr error
	for _, entry := range entries {
		ch, err := entry.ChatCompletionStream(ctx, req)
		if err == nil {
			r.resetFailCount(entry)
			return ch, nil
		}

		lastErr = err
		r.recordFailure(entry)
		log.Printf("PROVIDER: %s stream error: %v, falling back", entry.Name(), err)

		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}

	return nil, fmt.Errorf("all providers failed: %w", lastErr)
}

// CheckConnection tests each enabled provider and returns their status.
func (r *Router) CheckConnection(ctx context.Context) []ProviderConfig {
	r.mu.RLock()
	entries := r.providers
	r.mu.RUnlock()

	results := make([]ProviderConfig, 0, len(entries))
	for _, entry := range entries {
		cfg := entry.cfg
		// Simple test: list models or basic chat
		_, err := entry.ListModels(ctx)
		if err != nil {
			cfg.Connected = false
			cfg.Error = err.Error()
		} else {
			cfg.Connected = true
		}
		results = append(results, cfg)
	}
	return results
}

// SetActiveProvider restricts the router to only use the given provider type.
// Pass empty string to allow any enabled provider.
func (r *Router) SetActiveProvider(pt ProviderType) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.activeType = pt
}

// getActiveEntries returns non-disabled provider entries.
// If activeType is set, only the matching provider is returned.
func (r *Router) getActiveEntries() []*providerEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var entries []*providerEntry
	for _, entry := range r.providers {
		if entry.disabled {
			continue
		}
		if r.activeType != "" && entry.Name() != r.activeType {
			continue
		}
		entries = append(entries, entry)
	}

	// Sort by descending priority to preserve fallback ordering.
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
		log.Printf("PROVIDER: %s auto-disabled after %d consecutive failures", entry.Name(), entry.failCount)
	}
}

func (r *Router) resetFailCount(entry *providerEntry) {
	r.mu.Lock()
	defer r.mu.Unlock()
	entry.failCount = 0
}

// GetProvider returns an existing provider instance by type.
func (r *Router) GetProvider(pt ProviderType) (Provider, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, entry := range r.providers {
		if entry.Name() == pt && !entry.disabled {
			return entry.Provider, true
		}
	}
	return nil, false
}

// ReenableProvider re-enables a previously auto-disabled provider.
func (r *Router) ReenableProvider(name ProviderType) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.providers {
		if entry.Name() == name {
			entry.disabled = false
			entry.failCount = 0
			log.Printf("PROVIDER: %s re-enabled", name)
			return
		}
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

// ReenableAllProviders re-enables all previously auto-disabled providers.
func (r *Router) ReenableAllProviders() {
	r.mu.Lock()
	defer r.mu.Unlock()

	for _, entry := range r.providers {
		entry.disabled = false
		entry.failCount = 0
	}
}

// HealthCheck runs a periodic health check on all providers.
// It re-enables providers that recover.
func (r *Router) HealthCheck(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			r.mu.RLock()
			entries := r.providers
			r.mu.RUnlock()

			for _, entry := range entries {
				if !entry.disabled {
					continue
				}
				// Try to re-enable and test
				checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
				_, err := entry.ListModels(checkCtx)
				cancel()

				if err == nil {
					r.mu.Lock()
					entry.disabled = false
					entry.failCount = 0
					r.mu.Unlock()
					log.Printf("PROVIDER: %s recovered and re-enabled", entry.Name())
				}
			}
		}
	}
}
