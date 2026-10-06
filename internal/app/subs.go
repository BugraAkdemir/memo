package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"memo/internal/cliproxy"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/models"
	"memo/internal/provider"
)

// subsProviderName is the Name of the single `custom` provider that fronts the
// CLIProxyAPI sidecar. Every model every signed-in vendor account exposes is
// reachable through it; choosing a model rewrites this config's Model (the same
// switch-by-rewriting pattern the old per-vendor subscription providers used).
//
// It is a Name, not a ProviderType: the sidecar speaks plain OpenAI, so it IS a
// custom provider, and the name is how Memo tells it apart from one the user
// added by hand.
const subsProviderName = "Subscriptions"

// subsGatewayPrefix is the "type" half of the dev gateway's "type/model-id"
// strings for sidecar models ("subs/claude-sonnet-4-6"). It is not a
// ProviderType — resolveGatewayProvider maps it to subsProviderName.
const subsGatewayPrefix = "subs"

// subsManager returns the lazily created sidecar manager. It touches no disk
// until something asks for the binary.
func (a *App) subsManager() *cliproxy.Manager {
	a.subsMu.Lock()
	defer a.subsMu.Unlock()
	if a.subs == nil {
		a.subs = cliproxy.New(config.DataPath("cliproxy"))
	}
	return a.subs
}

func (a *App) lifecycle() context.Context {
	if a.lifecycleCtx != nil {
		return a.lifecycleCtx
	}
	return context.Background()
}

// subsMarker is the enabled provider config for the sidecar. The port can
// change between runs (the saved one may have been taken), so BaseURL is
// rewritten from the manager every time rather than trusted from providers.json.
func subsMarker(m *cliproxy.Manager, model string) provider.ProviderConfig {
	return provider.ProviderConfig{
		Type:    provider.ProviderCustom,
		Name:    subsProviderName,
		BaseURL: m.BaseURL(),
		APIKey:  m.APIKey(),
		Model:   model,
		Enabled: true,
	}
}

// isSubsMarker reports whether p is the Subscriptions provider.
func isSubsMarker(p provider.ProviderConfig) bool {
	return p.Name == subsProviderName && p.Type == provider.ProviderCustom
}

// subsMarkerConfig returns the stored Subscriptions provider, if any.
func (a *App) subsMarkerConfig() (provider.ProviderConfig, bool) {
	if a.providerCfgMgr == nil {
		return provider.ProviderConfig{}, false
	}
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == subsProviderName && p.Type == provider.ProviderCustom {
			return p, true
		}
	}
	return provider.ProviderConfig{}, false
}

// pickDefaultSubscriptionModel chooses the model to use when the user has not
// picked one (right after the first login) or the previous pick vanished from
// the account. A still-valid previous choice always wins. Otherwise: the
// strongest everyday Claude, then a Gemini 3, then any Claude, then any Gemini,
// then whatever is first — a preference order, not a filter; every model stays
// selectable.
func pickDefaultSubscriptionModel(list []cliproxy.Model, prev string) string {
	if len(list) == 0 {
		return ""
	}
	for _, m := range list {
		if m.ID == prev {
			return prev
		}
	}
	for _, prefix := range []string{"claude-sonnet", "gemini-3", "claude-opus", "gemini"} {
		for _, m := range list {
			if strings.HasPrefix(m.ID, prefix) && !strings.Contains(m.ID, "image") && !strings.Contains(m.ID, "lite") {
				return m.ID
			}
		}
	}
	return list[0].ID
}

// SubscriptionsState describes the sidecar for Settings. It never starts
// anything: a GET must be side-effect free, and "not started" is a state.
func (a *App) SubscriptionsState(ctx context.Context) models.SubscriptionsState {
	m := a.subsManager()
	st := models.SubscriptionsState{Providers: cliproxy.Providers}

	if _, ver, err := m.Binary(); err != nil {
		st.Problem = a.subsProblemText(err)
	} else {
		st.Bundled, st.Version = true, ver
	}
	st.Running = m.Running()

	st.Accounts = []models.SubscriptionAccount{}
	for _, ac := range m.Accounts() {
		st.Accounts = append(st.Accounts, models.SubscriptionAccount{Provider: ac.Provider, Email: ac.Email, Project: ac.Project, Disabled: ac.Disabled})
	}

	st.Models = []models.SubscriptionModel{}
	var list []cliproxy.Model
	if st.Running {
		qctx, cancel := context.WithTimeout(ctx, 3*time.Second)
		if l, err := m.Models(qctx); err == nil {
			list = l
		}
		cancel()
	}
	if list == nil {
		list = m.CachedModels()
	}
	for _, md := range list {
		st.Models = append(st.Models, models.SubscriptionModel{ID: md.ID, OwnedBy: md.OwnedBy})
	}

	if p, ok := a.subsMarkerConfig(); ok {
		st.Model = p.Model
	}
	ls := m.LoginStatus()
	st.Login = models.SubscriptionLogin{Provider: ls.Provider, Running: ls.Running, URL: ls.URL, Done: ls.Done, Error: ls.Error}
	return st
}

func (a *App) subsProblemText(err error) string {
	switch {
	case errors.Is(err, cliproxy.ErrNotBundled):
		return a.t(
			"Bu sürüm CLIProxyAPI'yi içermiyor (binaries/<işletim sistemi>/cliproxy bulunamadı).",
			"This build does not include CLIProxyAPI (binaries/<os>/cliproxy not found).",
		)
	case errors.Is(err, cliproxy.ErrUnverified):
		return a.t(
			"Paketlenmiş CLIProxyAPI sağlama toplamı doğrulamasından geçemedi; çalıştırılmadı.",
			"The bundled CLIProxyAPI failed its checksum and was not run.",
		)
	}
	return err.Error()
}

// StartSubscriptionLogin begins a browser sign-in for provider
// (antigravity|claude|codex) and returns the vendor URL to open. When the login
// completes, the sidecar is started and the Subscriptions provider registered
// in the background; the UI polls SubscriptionsState for that.
func (a *App) StartSubscriptionLogin(provider string) (string, error) {
	m := a.subsManager()
	ctx, cancel := context.WithTimeout(a.lifecycle(), 40*time.Second)
	defer cancel()
	url, err := m.StartLogin(ctx, provider)
	if err != nil {
		return "", err
	}
	goRecover("subs.awaitLogin", func() { a.awaitSubscriptionLogin(provider) })
	return url, nil
}

func (a *App) awaitSubscriptionLogin(provider string) {
	m := a.subsManager()
	deadline := time.Now().Add(6 * time.Minute)
	for time.Now().Before(deadline) {
		select {
		case <-a.lifecycle().Done():
			return
		case <-time.After(time.Second):
		}
		st := m.LoginStatus()
		if st.Provider != provider {
			return // replaced or cancelled
		}
		if !st.Done {
			continue
		}
		if st.Error != "" {
			logx.Printf("subs: %s login failed: %s", provider, st.Error)
			return
		}
		if err := a.syncSubscriptions(a.lifecycle()); err != nil {
			logx.Printf("subs: sync after %s login: %v", provider, err)
		}
		return
	}
}

// CancelSubscriptionLogin abandons a sign-in in flight.
func (a *App) CancelSubscriptionLogin() { a.subsManager().CancelLogin() }

// LogoutSubscription removes a vendor's stored credential. With no account left
// the sidecar and its provider entry go too.
func (a *App) LogoutSubscription(provider string) error {
	m := a.subsManager()
	if err := m.Logout(provider); err != nil {
		return err
	}
	return a.syncSubscriptions(a.lifecycle())
}

// syncSubscriptions makes the provider list and the sidecar agree with the
// signed-in accounts. It is idempotent and is the single place that starts the
// sidecar and writes the Subscriptions provider:
//
//   - no account  -> stop the sidecar, remove the provider;
//   - any account -> start the sidecar (if needed), wait for it to list models
//     (the file watcher needs a moment to pick up a fresh credential), keep the
//     user's model if it is still offered, else pick one, and (re)write the
//     provider with the sidecar's CURRENT base URL and key.
func (a *App) syncSubscriptions(ctx context.Context) error {
	m := a.subsManager()
	if !m.HasAccounts() {
		m.Stop()
		if _, ok := a.subsMarkerConfig(); ok {
			if err := a.DeleteProvider(provider.ProviderCustom, subsProviderName); err != nil {
				logx.Printf("subs: remove provider: %v", err)
			}
		}
		return nil
	}

	sctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	if err := m.Start(sctx); err != nil {
		return err
	}

	var list []cliproxy.Model
	for deadline := time.Now().Add(20 * time.Second); ; {
		l, err := m.Models(sctx)
		if err == nil && len(l) > 0 {
			list = l
			break
		}
		if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("sidecar is up but lists no models yet: %w", err)
			}
			return errors.New("sidecar is up but lists no models yet")
		}
		select {
		case <-sctx.Done():
			return sctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	prev := ""
	if p, ok := a.subsMarkerConfig(); ok {
		prev = p.Model
	}
	return a.UpdateProvider(subsMarker(m, pickDefaultSubscriptionModel(list, prev)))
}

// startSubscriptions brings the sidecar up at startup when an account is
// already signed in, and clears a stale provider entry when none is. Quiet on
// builds without the sidecar.
func (a *App) startSubscriptions() {
	m := a.subsManager()
	if _, _, err := m.Binary(); err != nil {
		if !m.HasAccounts() {
			return
		}
		logx.Printf("subs: signed-in accounts exist but the sidecar is unusable: %v", err)
		return
	}
	if err := a.syncSubscriptions(a.lifecycle()); err != nil {
		logx.Printf("subs: startup sync: %v", err)
	}
}

// stopSubscriptions ends the sidecar at shutdown.
func (a *App) stopSubscriptions() {
	a.subsMu.Lock()
	m := a.subs
	a.subsMu.Unlock()
	if m != nil {
		m.Stop()
	}
}

// SetProviderModel switches which model a configured provider uses by
// rewriting that provider's stored Model, keeping every other field. Name is
// the provider's Name. The selectors in the UI and the REPL call this; it is
// the one supported way to change a model without re-sending (and clobbering)
// the whole provider config.
func (a *App) SetProviderModel(name, model string) error {
	model = strings.TrimSpace(model)
	if model == "" {
		return errors.New("model is required")
	}
	if a.providerCfgMgr == nil {
		return errors.New("provider system not initialized")
	}
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == name {
			if p.Model == model {
				return nil
			}
			p.Model = model
			return a.UpdateProvider(p)
		}
	}
	return fmt.Errorf("provider %q not found", name)
}

// providerModelsTTL bounds how often a model selector being opened repeatedly
// hits a provider's /models endpoint.
const providerModelsTTL = 60 * time.Second

type providerModelsEntry struct {
	at   time.Time
	list []models.ProviderModel
}

// ListProviderModels lists the models a configured provider offers (by Name)
// and the one it currently uses. For the Subscriptions provider that is the
// sidecar's list with vendor grouping; for any other provider it is that
// provider's own live /models, cached briefly. The stored API key is used on
// the server and never returned, which is why the REST route behind this is
// gated on the models permission.
func (a *App) ListProviderModels(ctx context.Context, name string) ([]models.ProviderModel, string, error) {
	if a.providerCfgMgr == nil {
		return nil, "", errors.New("provider system not initialized")
	}
	var cfg provider.ProviderConfig
	found := false
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == name {
			cfg, found = p, true
			break
		}
	}
	if !found {
		return nil, "", fmt.Errorf("provider %q not found", name)
	}

	if isSubsMarker(cfg) {
		m := a.subsManager()
		var list []cliproxy.Model
		if m.Running() {
			l, err := m.Models(ctx)
			if err != nil {
				return nil, cfg.Model, err
			}
			list = l
		} else {
			list = m.CachedModels()
		}
		out := make([]models.ProviderModel, 0, len(list))
		for _, md := range list {
			out = append(out, models.ProviderModel{ID: md.ID, OwnedBy: md.OwnedBy})
		}
		return out, cfg.Model, nil
	}

	key := name + "|" + string(cfg.Type) + "|" + cfg.BaseURL
	a.provModelsMu.Lock()
	if e, ok := a.provModels[key]; ok && time.Since(e.at) < providerModelsTTL {
		out := append([]models.ProviderModel(nil), e.list...)
		a.provModelsMu.Unlock()
		return out, cfg.Model, nil
	}
	a.provModelsMu.Unlock()

	prov, err := provider.NewProvider(cfg)
	if err != nil {
		return nil, cfg.Model, err
	}
	lctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	ids, err := prov.ListModels(lctx)
	if err != nil {
		return nil, cfg.Model, err
	}
	out := make([]models.ProviderModel, 0, len(ids))
	for _, id := range ids {
		out = append(out, models.ProviderModel{ID: id})
	}
	a.provModelsMu.Lock()
	if a.provModels == nil {
		a.provModels = map[string]providerModelsEntry{}
	}
	a.provModels[key] = providerModelsEntry{at: time.Now(), list: out}
	a.provModelsMu.Unlock()
	return append([]models.ProviderModel(nil), out...), cfg.Model, nil
}
