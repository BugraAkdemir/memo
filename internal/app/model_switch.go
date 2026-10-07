package app

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"memo/internal/logx"
	"memo/internal/provider"
)

// Switching models from a chat surface (Telegram, WhatsApp).
//
// The app's own pickers and the REPL list a provider's models and call
// SetProviderModel/SetActiveProvider; a chat message has neither a menu nor a
// list to tap, so the same switch is exposed as numbered text: "/model" lists
// every model one tap away, "/model 3" or "/model gemini" picks one.

// ModelChoice is one model a person can switch to.
type ModelChoice struct {
	Provider string // the configured provider's Name; "" is the local model
	Model    string
	Group    string // the vendor behind a Subscriptions model, else the provider's Name
	Active   bool
	Image    bool // draws pictures; used automatically when one is asked for
}

// maxListedModels caps what one chat message shows; the rest stay reachable by
// number and by search.
const maxListedModels = 60

// ModelChoices lists every model that can be switched to: each enabled
// provider's current model, every live model of the Subscriptions provider, and
// the local model when one is running. The order is stable, so a number shown by
// one call means the same model in the next.
func (a *App) ModelChoices(ctx context.Context) []ModelChoice {
	active := a.GetActiveProvider()
	var out []ModelChoice

	if a.providerCfgMgr != nil {
		for _, cfg := range a.providerCfgMgr.GetAll() {
			if !cfg.Enabled {
				continue
			}
			if isSubsMarker(cfg) {
				out = append(out, a.subsChoices(ctx, cfg, active)...)
				continue
			}
			if strings.TrimSpace(cfg.Model) == "" {
				continue
			}
			out = append(out, ModelChoice{
				Provider: cfg.Name,
				Model:    cfg.Model,
				Group:    cfg.Name,
				Active:   active == cfg.Name,
			})
		}
	}

	a.clientMu.RLock()
	localRunning := a.llamaServer != nil && a.llamaServer.IsRunning()
	a.clientMu.RUnlock()
	if localRunning {
		out = append(out, ModelChoice{Group: "local", Active: active == ""})
	}
	return out
}

func (a *App) subsChoices(ctx context.Context, cfg provider.ProviderConfig, active string) []ModelChoice {
	list := a.subsModelList(ctx)
	if len(list) == 0 {
		if strings.TrimSpace(cfg.Model) == "" {
			return nil
		}
		return []ModelChoice{{Provider: cfg.Name, Model: cfg.Model, Group: cfg.Name, Active: active == cfg.Name}}
	}
	images := map[string]bool{}
	ictx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	for _, m := range a.subsImageModels(ictx, cfg, list) {
		images[m.ID] = true
	}
	out := make([]ModelChoice, 0, len(list))
	for _, m := range list {
		group := m.OwnedBy
		if group == "" {
			group = cfg.Name
		}
		out = append(out, ModelChoice{
			Provider: cfg.Name,
			Model:    m.ID,
			Group:    group,
			Active:   active == cfg.Name && cfg.Model == m.ID,
			Image:    images[m.ID],
		})
	}
	// Stable grouping: the sidecar's order within a vendor, vendors in the order
	// they first appear.
	order := map[string]int{}
	for _, c := range out {
		if _, ok := order[c.Group]; !ok {
			order[c.Group] = len(order)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return order[out[i].Group] < order[out[j].Group] })
	return out
}

// SwitchModel makes c the model chats use from now on.
func (a *App) SwitchModel(c ModelChoice) error {
	if c.Provider == "" {
		a.SetActiveProvider("")
		return nil
	}
	if err := a.SetProviderModel(c.Provider, c.Model); err != nil {
		return err
	}
	a.SetActiveProvider(c.Provider)
	if got := a.GetActiveProvider(); got != c.Provider {
		return fmt.Errorf("provider %q could not be activated", c.Provider)
	}
	return nil
}

// matchModelChoices returns the indexes (into choices) of the models a typed
// query names: an exact model id wins outright, otherwise every word must appear
// in "group provider model".
func matchModelChoices(choices []ModelChoice, query string) []int {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return nil
	}
	var exact []int
	for i, c := range choices {
		if strings.ToLower(c.Model) == q {
			exact = append(exact, i)
		}
	}
	if len(exact) == 1 {
		return exact
	}
	words := strings.Fields(q)
	var out []int
	for i, c := range choices {
		hay := strings.ToLower(c.Group + " " + c.Provider + " " + c.Model)
		if c.Provider == "" {
			hay += " local yerel"
		}
		ok := true
		for _, w := range words {
			if !strings.Contains(hay, w) {
				ok = false
				break
			}
		}
		if ok {
			out = append(out, i)
		}
	}
	return out
}

// explicitModelChoice reads "<provider> <model>", "<provider>/<model>" or
// "<provider>:<model>" for a model that is not in the short list (a provider's
// other models are reachable this way without listing hundreds of them).
func (a *App) explicitModelChoice(query string) (ModelChoice, bool) {
	if a.providerCfgMgr == nil {
		return ModelChoice{}, false
	}
	q := strings.TrimSpace(query)
	lq := strings.ToLower(q)
	var names []string
	for _, cfg := range a.providerCfgMgr.GetAll() {
		if cfg.Enabled {
			names = append(names, cfg.Name)
		}
	}
	// A query that is exactly a provider's name names the provider, not a model of
	// a shorter-named one ("OpenCode Go" is not "OpenCode" + model "Go").
	for _, name := range names {
		if strings.ToLower(name) == lq {
			return ModelChoice{}, false
		}
	}
	// Longest name first, so "OpenCode Go qwen" is read as "OpenCode Go" + "qwen".
	sort.Slice(names, func(i, j int) bool { return len(names[i]) > len(names[j]) })
	for _, name := range names {
		ln := strings.ToLower(name)
		if !strings.HasPrefix(lq, ln) || len(q) <= len(name) {
			continue
		}
		sep := q[len(name)]
		if sep != ' ' && sep != '/' && sep != ':' {
			continue
		}
		model := strings.TrimSpace(q[len(name)+1:])
		if model == "" {
			continue
		}
		return ModelChoice{Provider: name, Model: model, Group: name}, true
	}
	return ModelChoice{}, false
}

// selfChatModelCommand answers "/model [number|name]" for a chat surface.
func (a *App) selfChatModelCommand(ctx context.Context, lang, args string) string {
	ctx, cancel := context.WithTimeout(ctx, 25*time.Second)
	defer cancel()

	choices := a.ModelChoices(ctx)
	args = strings.TrimSpace(args)
	if len(choices) == 0 && args == "" {
		return scT(lang, "sc_model_none")
	}
	if args == "" {
		return a.formatModelChoices(lang, choices, nil, "")
	}

	if n, err := strconv.Atoi(args); err == nil {
		if n < 1 || n > len(choices) {
			return fmt.Sprintf(scT(lang, "sc_model_bad_number"), len(choices))
		}
		return a.applyModelChoice(lang, choices[n-1])
	}

	if idx := matchModelChoices(choices, args); len(idx) == 1 {
		return a.applyModelChoice(lang, choices[idx[0]])
	} else if len(idx) > 1 {
		return a.formatModelChoices(lang, choices, idx, args)
	}

	if c, ok := a.explicitModelChoice(args); ok {
		return a.applyModelChoice(lang, c)
	}
	return fmt.Sprintf(scT(lang, "sc_model_not_found"), args)
}

func (a *App) applyModelChoice(lang string, c ModelChoice) string {
	if err := a.SwitchModel(c); err != nil {
		logx.Printf("model switch from chat failed: %v", err)
		return fmt.Sprintf(scT(lang, "sc_model_switch_err"), a.FriendlyError(err.Error()))
	}
	logx.Printf("model switched from chat to %s / %s", c.Provider, c.Model)
	return fmt.Sprintf(scT(lang, "sc_model_switched"), describeChoice(lang, c))
}

func describeChoice(lang string, c ModelChoice) string {
	if c.Provider == "" {
		return scT(lang, "sc_model_local")
	}
	if c.Model == "" {
		return c.Provider
	}
	return c.Provider + " · " + c.Model
}

// formatModelChoices renders the numbered list. only, when set, restricts it to
// those indexes (a search); numbers always index the full list.
func (a *App) formatModelChoices(lang string, choices []ModelChoice, only []int, query string) string {
	show := only
	if show == nil {
		show = make([]int, len(choices))
		for i := range choices {
			show[i] = i
		}
	}

	var b strings.Builder
	if query != "" {
		fmt.Fprintf(&b, scT(lang, "sc_model_many"), query)
		b.WriteString("\n")
	} else {
		current := scT(lang, "sc_model_current_none")
		for _, c := range choices {
			if c.Active {
				current = describeChoice(lang, c)
				break
			}
		}
		fmt.Fprintf(&b, scT(lang, "sc_model_header"), current)
		b.WriteString("\n")
	}

	lastGroup := "\x00"
	shown := 0
	for _, i := range show {
		if shown >= maxListedModels {
			fmt.Fprintf(&b, "\n"+scT(lang, "sc_model_more"), len(show)-shown)
			break
		}
		c := choices[i]
		if c.Group != lastGroup {
			title := c.Group
			if c.Provider == "" {
				title = scT(lang, "sc_model_local")
			}
			fmt.Fprintf(&b, "\n%s\n", title)
			lastGroup = c.Group
		}
		name := c.Model
		if c.Provider == "" {
			name = scT(lang, "sc_model_local")
		}
		line := fmt.Sprintf("%d. %s", i+1, name)
		if c.Image {
			line += " 🎨"
		}
		if c.Active {
			line += " ✅"
		}
		b.WriteString(line + "\n")
		shown++
	}
	b.WriteString("\n" + scT(lang, "sc_model_footer"))
	return strings.TrimRight(b.String(), "\n")
}

// activeModelLabel names what chats are using right now — "Provider · model" —
// for status messages. Empty when the local model (or nothing) is active.
func (a *App) activeModelLabel() string {
	name := a.GetActiveProvider()
	if name == "" {
		return ""
	}
	if cfg, ok := a.providerConfigByName(name); ok && cfg.Model != "" {
		return name + " · " + cfg.Model
	}
	return name
}
