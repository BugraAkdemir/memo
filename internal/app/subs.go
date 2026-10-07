package app

import (
	"context"
	"errors"
	"fmt"
	"os"
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
	// Whatever is first — but never an image-only model: it cannot chat, and
	// picking one made the first message after a restart draw a picture.
	for _, m := range list {
		if !strings.Contains(m.ID, "image") {
			return m.ID
		}
	}
	return list[0].ID
}

// modelListed reports whether id is one of list's models.
func modelListed(list []cliproxy.Model, id string) bool {
	for _, m := range list {
		if m.ID == id {
			return true
		}
	}
	return false
}

// subsVendorOf is the owned_by a sign-in's models carry in the sidecar's list.
var subsVendorOf = map[string]string{
	cliproxy.ProviderAntigravity: "antigravity",
	cliproxy.ProviderCodex:       "openai",
	cliproxy.ProviderClaude:      "anthropic",
}

// subsListComplete reports whether list has at least one model for every
// account that is signed in. The sidecar fills its list vendor by vendor while
// it warms up — a restart can see the Codex models for a while before the
// Antigravity ones — and a list missing a whole vendor says nothing about
// whether the user's previous pick (a Claude on Antigravity, say) is gone.
func subsListComplete(list []cliproxy.Model, accounts []cliproxy.Account) bool {
	have := map[string]bool{}
	for _, md := range list {
		have[md.OwnedBy] = true
	}
	for _, ac := range accounts {
		if ac.Disabled {
			continue
		}
		if vendor, ok := subsVendorOf[ac.Provider]; ok && !have[vendor] {
			return false
		}
	}
	return true
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
	quotas := a.subsQuotas(ctx, len(list) > 0, 2*time.Second)
	for _, md := range list {
		sm := models.SubscriptionModel{ID: md.ID, OwnedBy: md.OwnedBy}
		if q, ok := quotas.For(md.ID, md.OwnedBy); ok {
			r := q.Remaining
			sm.Remaining, sm.ResetAt, sm.QuotaWindow = &r, q.ResetAt, q.Window
		}
		st.Models = append(st.Models, sm)
	}

	if p, ok := a.subsMarkerConfig(); ok {
		st.Model = p.Model
	}
	ls := m.LoginStatus()
	st.Login = models.SubscriptionLogin{Provider: ls.Provider, Running: ls.Running, URL: ls.URL, Done: ls.Done, Error: ls.Error}
	return st
}

// subsQuotas returns the allowance figures, never failing and never holding the
// caller for more than maxWait: a vendor that reports none, an expired token or a
// slow network just means no percentages (or the previous ones). With maxWait 0
// it returns what is cached at once and refreshes in the background — what keeps
// opening the model picker instant.
func (a *App) subsQuotas(ctx context.Context, haveModels bool, maxWait time.Duration) cliproxy.QuotaSet {
	if !haveModels {
		return cliproxy.QuotaSet{}
	}
	return a.subsManager().QuotaSnapshot(ctx, maxWait)
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

	// A cold sidecar takes ~30s before it lists a freshly started account's
	// models (it refreshes the credential and its model catalogue first), so
	// give it well over that.
	sctx, cancel := context.WithTimeout(ctx, subsStartTimeout)
	defer cancel()
	if err := m.Start(sctx); err != nil {
		return err
	}

	// If a previous session already chose a model, register the provider NOW,
	// with the sidecar's current port and key: chat works the moment the
	// process is healthy instead of 30s later. The wait below then only has to
	// confirm (or replace) that choice once the list is known.
	prev := ""
	if p, ok := a.subsMarkerConfig(); ok && p.Model != "" {
		prev = p.Model
		if err := a.UpdateProvider(subsMarker(m, prev)); err != nil {
			logx.Printf("subs: early provider registration: %v", err)
		}
	}

	var list []cliproxy.Model
	for deadline := time.Now().Add(subsModelsTimeout); ; {
		l, err := m.Models(sctx)
		if err == nil && len(l) > 0 {
			list = l
			// A user's earlier pick that is not in this list may simply not have
			// been listed yet: keep waiting while a signed-in vendor is missing,
			// instead of replacing the choice (and saving the replacement).
			stillWarming := prev != "" && !subsListComplete(l, m.Accounts()) && !modelListed(l, prev)
			if !stillWarming || time.Now().After(deadline) {
				break
			}
		} else if time.Now().After(deadline) {
			if err != nil {
				return fmt.Errorf("%w: %v", errSubsNoModelsYet, err)
			}
			return errSubsNoModelsYet
		}
		select {
		case <-sctx.Done():
			return sctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}

	return a.UpdateProvider(subsMarker(m, pickDefaultSubscriptionModel(list, prev)))
}

// Timing for syncSubscriptions — vars so tests can shorten them.
var (
	subsStartTimeout  = 150 * time.Second
	subsModelsTimeout = 120 * time.Second
	subsRetryEvery    = 45 * time.Second
	subsRetryAttempts = 6
)

// errSubsNoModelsYet means the sidecar is up and signed in but has not listed any
// model yet — worth retrying, unlike a failure to start at all.
var errSubsNoModelsYet = errors.New("sidecar is up but lists no models yet")

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
	err := a.syncSubscriptions(a.lifecycle())
	// A sidecar that is up but has not listed models yet is worth a few more
	// tries (it settles by itself); anything else (cannot start, no binary) is
	// reported once and left for the user, who can sign in again.
	for attempt := 1; errors.Is(err, errSubsNoModelsYet) && attempt <= subsRetryAttempts; attempt++ {
		logx.Printf("subs: startup sync: %v — retry %d/%d in %s", err, attempt, subsRetryAttempts, subsRetryEvery)
		select {
		case <-a.lifecycle().Done():
			return
		case <-time.After(subsRetryEvery):
		}
		err = a.syncSubscriptions(a.lifecycle())
	}
	if err != nil {
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
		quotas := a.subsQuotas(ctx, len(list) > 0, models.QuotaWaitFromContext(ctx))
		out := make([]models.ProviderModel, 0, len(list))
		for _, md := range list {
			pm := models.ProviderModel{ID: md.ID, OwnedBy: md.OwnedBy}
			if q, ok := quotas.For(md.ID, md.OwnedBy); ok {
				r := q.Remaining
				pm.Remaining, pm.ResetAt, pm.QuotaWindow = &r, q.ResetAt, q.Window
			}
			out = append(out, pm)
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

// purgeLegacySubscriptionData removes what the per-vendor subscription providers
// (gemini-sub, claude-sub) left on disk: an encrypted OAuth token each, under
// data/geminisub and data/claudesub. Nothing reads them any more, so keeping a
// refresh token around would be a liability with no use. Idempotent and quiet
// when there is nothing to remove. (The vendor grant itself is not revoked —
// do that from the Google / Claude account pages if it matters.)
func purgeLegacySubscriptionData() {
	for _, dir := range []string{"geminisub", "claudesub"} {
		p := config.DataPath(dir)
		if _, err := os.Stat(p); err != nil {
			continue
		}
		if err := os.RemoveAll(p); err != nil {
			logx.Printf("subs: could not remove legacy %s data: %v", dir, err)
			continue
		}
		logx.Printf("subs: removed legacy %s data (replaced by Settings > Subscriptions)", dir)
	}
}
