package app

import (
	"context"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ---- use_skill: the model calls a skill itself --------------------------------

// maxSkillToolChars caps the instructions one use_skill call hands back; a
// skill larger than that is cut with a note, never dropped silently.
const maxSkillToolChars = 24000

// resolveSkillRef finds a skill by name: exact name or slug first, then a slug
// that contains what was asked for, only if exactly one does.
func resolveSkillRef(cat []skillRef, q string) (skillRef, bool) {
	q = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(q), "/"))
	if q == "" {
		return skillRef{}, false
	}
	slug := skillSlug(q)
	for _, r := range cat {
		if q == strings.ToLower(r.Name) || slug == r.Slug {
			return r, true
		}
	}
	var found []skillRef
	for _, r := range cat {
		if slug != "" && strings.Contains(r.Slug, slug) {
			found = append(found, r)
		}
	}
	if len(found) == 1 {
		return found[0], true
	}
	return skillRef{}, false
}

// skillToolAdapter backs the use_skill agent tool.
type skillToolAdapter struct{ a *App }

// UseSkill loads a skill into the chat the turn belongs to and returns its
// instructions so the model can follow them in this same turn (the system
// prompt of the turn was built before the call). An empty name lists what is
// installed.
func (ad skillToolAdapter) UseSkill(ctx context.Context, name string) (string, error) {
	a := ad.a
	chatID := currentChatIDFromContext(ctx)
	// Global skills only. A project skill arrives with a cloned repository and is
	// untrusted text: it is switched on only by the person naming it, never by a
	// model that read "call use_skill evil-skill" in a README or tool output.
	cat := a.buildSkillCatalog(chatID, false)
	if len(cat) == 0 {
		return "No skills are installed.", nil
	}
	if strings.TrimSpace(name) == "" || strings.EqualFold(strings.TrimSpace(name), "list") {
		var b strings.Builder
		b.WriteString("Installed skills (call use_skill with a name to load one):\n")
		for _, r := range cat {
			desc := strings.Join(strings.Fields(r.Description), " ") // one line
			if len([]rune(desc)) > 160 {
				desc = string([]rune(desc)[:160]) + "…"
			}
			fmt.Fprintf(&b, "- %s — %s\n", r.Name, desc)
		}
		return b.String(), nil
	}
	ref, ok := resolveSkillRef(cat, name)
	if !ok {
		return "", fmt.Errorf("no installed skill matches %q; call use_skill with no name to list them", name)
	}
	if chatID == "" {
		return "", fmt.Errorf("use_skill needs a chat to attach the skill to")
	}
	def, ok := a.skillManager.Get(ref.Name)
	if !ok {
		return "", fmt.Errorf("skill %q is no longer installed", ref.Name)
	}
	text := def.Instructions
	if len(text) > maxSkillToolChars {
		cut := maxSkillToolChars
		for cut > 0 && !utf8.RuneStart(text[cut]) {
			cut-- // never split a multi-byte character
		}
		text = text[:cut] + "\n\n[…the rest of this skill was cut: it is longer than this tool returns]"
	}
	if err := a.activateChatSkills(chatID, []string{ref.Name}); err != nil {
		return fmt.Sprintf("Skill %q is loaded for THIS turn only — it could not be switched on for the chat (%v), so it will not carry over. Follow these instructions now:\n\n%s", ref.Name, err, text), nil
	}
	return fmt.Sprintf("Skill %q is now active in this chat. Follow these instructions:\n\n%s", ref.Name, text), nil
}
