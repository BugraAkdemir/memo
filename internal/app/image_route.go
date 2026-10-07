package app

import (
	"context"
	"errors"
	"strings"
	"time"

	"memo/internal/api"
	"memo/internal/cliproxy"
	"memo/internal/config"
	"memo/internal/logx"
	"memo/internal/provider"
)

// Automatic image routing.
//
// A person chatting with a text model who asks for a picture ("draw a cat",
// "make this photo black and white") should get a picture without first
// switching models, and be back on the text model for the next message. This
// file decides, per turn, whether the message wants a picture (image_intent.go)
// and which image model draws it:
//
//   - On a Subscriptions account the image model of THAT account — the vendor of
//     the model being chatted with first (a Codex chat draws with Codex's image
//     model, an Antigravity chat with Antigravity's), any other signed-in
//     vendor's otherwise. Never the global default: a subscription already pays
//     for its own.
//   - On every other provider, the default image model chosen in Settings
//     (config.ImageConfig).
//
// The chat's active model is never changed; only the one turn is sent elsewhere,
// through the same streamImageGeneration the "active model is an image model"
// case already used, so the picture is stored, shown and sealed the same way.

type forcedImageCtxKey struct{}

// withForcedImage marks a turn whose picture request is explicit (the "/image"
// command), so the wording heuristics are skipped.
func withForcedImage(ctx context.Context) context.Context {
	return context.WithValue(ctx, forcedImageCtxKey{}, true)
}

func imageForced(ctx context.Context) bool {
	v, _ := ctx.Value(forcedImageCtxKey{}).(bool)
	return v
}

// imageTarget is the model an image request goes to.
type imageTarget struct {
	Gen      provider.ImageGenerator
	Model    string
	Provider string // the configured provider's Name
}

// GetImageConfig returns the automatic image routing settings.
func (a *App) GetImageConfig() config.ImageConfig {
	a.cfgMu.RLock()
	defer a.cfgMu.RUnlock()
	return a.cfg.Image
}

// SetImageConfig stores the automatic image routing settings. A default image
// model must name a configured provider; an empty one clears it.
func (a *App) SetImageConfig(c config.ImageConfig) error {
	c.DefaultProvider = strings.TrimSpace(c.DefaultProvider)
	c.DefaultModel = strings.TrimSpace(c.DefaultModel)
	if (c.DefaultProvider == "") != (c.DefaultModel == "") {
		return errors.New("choose both a provider and a model for the default image model, or neither")
	}
	if c.DefaultProvider != "" {
		if _, ok := a.providerConfigByName(c.DefaultProvider); !ok {
			return errors.New("provider \"" + c.DefaultProvider + "\" is not configured")
		}
	}
	a.cfgMu.Lock()
	a.cfg.Image = c
	a.cfgMu.Unlock()
	return config.Save(a.cfg)
}

func (a *App) providerConfigByName(name string) (provider.ProviderConfig, bool) {
	if a.providerCfgMgr == nil {
		return provider.ProviderConfig{}, false
	}
	for _, p := range a.providerCfgMgr.GetAll() {
		if p.Name == name {
			return p, true
		}
	}
	return provider.ProviderConfig{}, false
}

// imageGeneratorFor builds the provider for cfg and returns it as an image
// generator, when its type can draw.
func imageGeneratorFor(cfg provider.ProviderConfig) (provider.ImageGenerator, bool) {
	prov, err := provider.NewProvider(cfg)
	if err != nil {
		return nil, false
	}
	gen, ok := prov.(provider.ImageGenerator)
	return gen, ok
}

// subsImageModels returns the Subscriptions models that draw pictures, in the
// sidecar's own order (newest-looking first within each vendor), asking the
// endpoint rather than guessing from ids (see openai_images.go).
func (a *App) subsImageModels(ctx context.Context, cfg provider.ProviderConfig, list []cliproxy.Model) []cliproxy.Model {
	gen, ok := imageGeneratorFor(cfg)
	if !ok {
		return nil
	}
	var out []cliproxy.Model
	for _, m := range list {
		if gen.IsImageOnlyModel(ctx, m.ID) {
			out = append(out, m)
		}
	}
	return out
}

// subsModelList is the Subscriptions model list without waiting on the sidecar:
// the cache when there is one, else a short live read.
func (a *App) subsModelList(ctx context.Context) []cliproxy.Model {
	m := a.subsManager()
	if list := m.CachedModels(); len(list) > 0 {
		return list
	}
	if !m.Running() {
		return nil
	}
	lctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	list, err := m.Models(lctx)
	if err != nil {
		return nil
	}
	return list
}

// resolveImageTarget finds where a picture request goes right now. ok is false
// when nothing can draw: no image model on the signed-in subscription and no
// default image model configured.
func (a *App) resolveImageTarget(ctx context.Context) (imageTarget, bool) {
	active := a.GetActiveProvider()
	if active != "" {
		if cfg, found := a.providerConfigByName(active); found && isSubsMarker(cfg) {
			if t, ok := a.subsImageTarget(ctx, cfg); ok {
				return t, true
			}
			// The subscription has no image model: fall through to the default.
		}
	}

	ic := a.GetImageConfig()
	if ic.DefaultProvider == "" || ic.DefaultModel == "" {
		return imageTarget{}, false
	}
	cfg, found := a.providerConfigByName(ic.DefaultProvider)
	if !found || !cfg.Enabled {
		return imageTarget{}, false
	}
	cfg.Model = ic.DefaultModel
	gen, ok := imageGeneratorFor(cfg)
	if !ok {
		return imageTarget{}, false
	}
	return imageTarget{Gen: gen, Model: ic.DefaultModel, Provider: cfg.Name}, true
}

// subsImageTarget picks the image model of the signed-in subscription: the
// vendor of the model in use first.
func (a *App) subsImageTarget(ctx context.Context, cfg provider.ProviderConfig) (imageTarget, bool) {
	list := a.subsModelList(ctx)
	if len(list) == 0 {
		return imageTarget{}, false
	}
	images := a.subsImageModels(ctx, cfg, list)
	if len(images) == 0 {
		return imageTarget{}, false
	}
	vendor := ""
	for _, m := range list {
		if m.ID == cfg.Model {
			vendor = m.OwnedBy
			break
		}
	}
	pick := images[0]
	if vendor != "" {
		for _, m := range images {
			if m.OwnedBy == vendor {
				pick = m
				break
			}
		}
	}
	gen, ok := imageGeneratorFor(cfg)
	if !ok {
		return imageTarget{}, false
	}
	return imageTarget{Gen: gen, Model: pick.ID, Provider: cfg.Name}, true
}

// autoRouteImageTurn sends this turn to an image model when the message asks for
// a picture and an image model is available. It returns false — and the caller
// carries on with the ordinary chat path — for everything else, which is almost
// every turn.
func (a *App) autoRouteImageTurn(ctx context.Context, messages []api.Message, userMsg, sessionID string) (<-chan api.StreamChunk, bool) {
	forced := imageForced(ctx)
	if !forced && !a.GetImageConfig().AutoRoute {
		return nil, false
	}
	// Unattended and coding contexts keep to the text model: a task step that
	// happens to say "draw" is not a request for a picture.
	if codeModeActive(ctx) || taskRunConfigFromCtx(ctx) != nil {
		return nil, false
	}

	images := imageInputsFromMessages(messages)
	intent := classifyImageIntent(userMsg, len(images) > 0)
	if forced {
		intent = imageIntentGenerate
		if len(images) > 0 {
			intent = imageIntentEdit
		}
	}
	if intent == imageIntentNone {
		return nil, false
	}

	target, ok := a.resolveImageTarget(ctx)
	if !ok {
		if !forced {
			return nil, false
		}
		out := make(chan api.StreamChunk, 1)
		msg := a.t(
			"⚠️ Görsel üretecek bir model yok. Abonelikler'de bir hesapla giriş yap ya da Ayarlar'dan varsayılan bir görsel modeli seç.",
			"⚠️ There is no model to draw with. Sign in to a subscription, or choose a default image model in Settings.")
		a.recordStreamError(userMsg, msg, sessionID)
		out <- api.StreamChunk{Error: msg, Done: true}
		close(out)
		return out, true
	}

	// Only an edit carries the attached picture: a plain request that merely came
	// with one (a reference the user did not ask to change) is not an edit.
	var sources = images
	if intent != imageIntentEdit {
		sources = nil
	}
	logx.Printf("IMAGE: auto-routing this turn to %s/%s (intent=%d, forced=%v)", target.Provider, target.Model, intent, forced)

	out := make(chan api.StreamChunk, 16)
	go func() {
		defer close(out)
		defer recoverStreamPanic(ctx, out, "autoRouteImageTurn")
		a.streamImageGeneration(ctx, target.Gen, target.Model, userMsg, userMsg, sessionID, sources, out)
	}()
	return out, true
}
