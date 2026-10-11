package app

import (
	"context"
	"errors"
	"time"

	"memo/internal/api"
	"memo/internal/cliproxy"
	"memo/internal/logx"
	"memo/internal/provider"
)

type subscriptionTextRoute struct {
	router        *provider.Router
	model, effort string
}
type subscriptionTextRouteKey struct{}

func subscriptionTextRouteFromCtx(ctx context.Context) *subscriptionTextRoute {
	route, _ := ctx.Value(subscriptionTextRouteKey{}).(*subscriptionTextRoute)
	return route
}

// routeSubscriptionText uses a private, single-provider router for one turn.
// It never changes the saved selection and never falls through to a paid API.
func (a *App) routeSubscriptionText(ctx context.Context, cfg provider.ProviderConfig) (context.Context, error) {
	list := a.subsModelList(ctx)
	gen, ok := imageGeneratorFor(cfg)
	if !ok {
		return ctx, errors.New(a.t("Abonelik görsel sağlayıcısı kullanılamıyor.", "Subscription image provider is unavailable."))
	}
	probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	owner := ""
	for _, m := range list {
		if m.ID == cfg.Model {
			owner = m.OwnedBy
			break
		}
	}
	var chat []cliproxy.Model
	for _, m := range list {
		if m.ID != cfg.Model && !gen.IsImageOnlyModel(probeCtx, m.ID) {
			chat = append(chat, m)
		}
	}
	// Prefer a saved conversation model from the same signed-in vendor, then
	// another chat model of that vendor, then another signed-in vendor.
	pick := ""
	for _, sameVendor := range []bool{true, false} {
		eligible := []cliproxy.Model{}
		for _, m := range chat {
			if !sameVendor || owner == "" || m.OwnedBy == owner {
				eligible = append(eligible, m)
			}
		}
		for i := len(cfg.Models) - 1; i >= 0 && pick == ""; i-- {
			for _, m := range eligible {
				if m.ID == cfg.Models[i].ID {
					pick = m.ID
					break
				}
			}
		}
		if pick == "" {
			pick = pickDefaultSubscriptionModel(eligible, "")
		}
		if pick != "" {
			break
		}
	}
	if pick == "" {
		return ctx, errors.New(a.t("Abonelikler'de bu soruyu yanıtlayacak bir sohbet modeli yok. Bir sohbet hesabı bağla veya sohbet modeli seç.", "No subscription chat model is available to answer this question. Connect a chat account or select a chat model."))
	}
	cfg.Model = pick
	cfg.Models = nil
	cfg.ContextTokens = 0 // a limit configured for the selected image model is unrelated
	cfg.EffortLevel = supportedSubscriptionEffort(pick, cfg.EffortLevel)
	router := provider.NewRouter([]provider.ProviderConfig{cfg})
	router.SetActiveProvider(cfg.Name)
	logx.Printf("SUBSCRIPTIONS: routing text turn to %s/%s", cfg.Name, pick)
	return context.WithValue(ctx, subscriptionTextRouteKey{}, &subscriptionTextRoute{router: router, model: pick, effort: cfg.EffortLevel}), nil
}
func (a *App) resolveAgentProviderForContext(ctx context.Context) (*provider.Router, string, string, error) {
	if route := subscriptionTextRouteFromCtx(ctx); route != nil {
		return route.router, route.model, route.effort, nil
	}
	return a.resolveAgentProvider()
}
func (a *App) subscriptionTextRouteError(ctx context.Context, err error, userMsg, sessionID string) <-chan api.StreamChunk {
	out := make(chan api.StreamChunk, 1)
	msg := "⚠️ " + err.Error()
	a.recordStreamError(userMsg, msg, sessionID)
	out <- api.StreamChunk{Error: msg, Done: true}
	close(out)
	return out
}
