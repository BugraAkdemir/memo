package app

import (
	"os"
	"regexp"
	"strings"

	"memo/internal/taskloop"
)

// roleModels holds the resolved model for each planner/executor role. An empty
// field means "unresolved — the caller falls back to its own default (the
// active provider's model)".
type roleModels struct {
	Planner  string
	Coder    string
	Verifier string
}

// roleDirectiveRe matches the single machine-written line persistRoleChoiceToRules
// keeps in AGENTS.md:
//
//	<!-- memo:taskloop planlayıcı=claude kodlayıcı=local doğrulayıcı=local -->
var roleDirectiveRe = regexp.MustCompile(`(?i)<!--\s*memo:taskloop\s+(.+?)\s*-->`)

// resolveRoleModels resolves the three role models for a list, in priority
// order: Task.md header -> AGENTS.md directive line -> Settings default. A role
// left empty here is filled by the caller with the active provider's model.
func (a *App) resolveRoleModels(listID string) roleModels {
	var rm roleModels

	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return a.roleModelsFromConfig(rm)
	}

	// 1) Task.md headers.
	if tl.TaskMdPath != "" {
		if parsed, err := taskloop.ParseTaskMd(tl.TaskMdPath); err == nil {
			rm.Planner = firstNonEmpty(rm.Planner, parsed.Headers["planlayıcı"], parsed.Headers["planlayici"])
			rm.Coder = firstNonEmpty(rm.Coder, parsed.Headers["kodlayıcı"], parsed.Headers["kodlayici"])
			rm.Verifier = firstNonEmpty(rm.Verifier, parsed.Headers["doğrulayıcı"], parsed.Headers["dogrulayici"])
		}
	}

	// 2) AGENTS.md / CLAUDE.md directive line.
	if root := a.taskListProjectPath(listID); root != "" {
		if rules, err := taskloop.ReadRules(root); err == nil {
			d := parseRoleDirectives(rules)
			rm.Planner = firstNonEmpty(rm.Planner, d.Planner)
			rm.Coder = firstNonEmpty(rm.Coder, d.Coder)
			rm.Verifier = firstNonEmpty(rm.Verifier, d.Verifier)
		}
	}

	// 3) Settings default.
	return a.roleModelsFromConfig(rm)
}

func (a *App) roleModelsFromConfig(rm roleModels) roleModels {
	a.cfgMu.RLock()
	tlc := a.cfg.TaskLoop
	a.cfgMu.RUnlock()
	rm.Planner = firstNonEmpty(rm.Planner, tlc.PlannerModel)
	rm.Coder = firstNonEmpty(rm.Coder, tlc.CoderModel)
	rm.Verifier = firstNonEmpty(rm.Verifier, tlc.VerifierModel)
	return rm
}

// parseRoleDirectives reads the "<!-- memo:taskloop k=v … -->" line out of a
// rules blob. Unknown keys are ignored.
func parseRoleDirectives(rulesText string) roleModels {
	var rm roleModels
	m := roleDirectiveRe.FindStringSubmatch(rulesText)
	if m == nil {
		return rm
	}
	for _, tok := range strings.Fields(m[1]) {
		k, v, ok := strings.Cut(tok, "=")
		if !ok {
			continue
		}
		switch strings.ToLower(strings.TrimSpace(k)) {
		case "planlayıcı", "planlayici", "planner":
			rm.Planner = strings.TrimSpace(v)
		case "kodlayıcı", "kodlayici", "coder":
			rm.Coder = strings.TrimSpace(v)
		case "doğrulayıcı", "dogrulayici", "verifier":
			rm.Verifier = strings.TrimSpace(v)
		}
	}
	return rm
}

// persistRoleChoiceToRules writes/updates the machine directive line in the
// project's AGENTS.md (creating it if missing) so a resolved role choice is
// not asked again. role is "planlayıcı" | "kodlayıcı" | "doğrulayıcı".
func (a *App) persistRoleChoiceToRules(projectRoot, role, model string) error {
	if projectRoot == "" || strings.TrimSpace(model) == "" {
		return nil
	}
	path := projectRoot + string(os.PathSeparator) + "AGENTS.md"
	existing, _ := os.ReadFile(path)
	text := string(existing)

	current := parseRoleDirectives(text)
	switch role {
	case "planlayıcı":
		current.Planner = model
	case "kodlayıcı":
		current.Coder = model
	case "doğrulayıcı":
		current.Verifier = model
	default:
		return nil
	}

	var parts []string
	if current.Planner != "" {
		parts = append(parts, "planlayıcı="+current.Planner)
	}
	if current.Coder != "" {
		parts = append(parts, "kodlayıcı="+current.Coder)
	}
	if current.Verifier != "" {
		parts = append(parts, "doğrulayıcı="+current.Verifier)
	}
	line := "<!-- memo:taskloop " + strings.Join(parts, " ") + " -->"

	if roleDirectiveRe.MatchString(text) {
		text = roleDirectiveRe.ReplaceAllString(text, line)
	} else {
		if text != "" && !strings.HasSuffix(text, "\n") {
			text += "\n"
		}
		text += line + "\n"
	}
	return os.WriteFile(path, []byte(text), 0o644)
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
