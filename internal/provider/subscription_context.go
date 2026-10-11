package provider

import "memo/internal/modelcatalog"

// SubscriptionContextWindow is known metadata, or zero. A caller may use an
// internal safety budget when unknown, but must not present it as capacity.
func SubscriptionContextWindow(cfg ProviderConfig, model string) int {
	if cfg.Name != "Subscriptions" || cfg.Type != ProviderCustom {
		return 0
	}
	if model == "" {
		model = cfg.Model
	}
	if v, ok := modelcatalog.Shared().Cached(model, ""); ok {
		return v.Context
	}
	return 0
}
