package app

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
)

// A skill is switched on by USING it, not from a settings list: the person (or
// the model) calls it, it becomes active for that chat, and a new chat starts
// with none. Three ways to call one from a message:
//
//	"fix X with /pair-programming"             the /name form (what the "/" menu inserts)
//	"fix X using the Pair Programming skill"   the skill's name next to the word "skill"
//	"fix X using the necessary skills"         no name: the best matches are picked
//
// The model can also load one itself with the use_skill tool (agent turns).
// What was used is shown in the stream and kept on the saved message.

// skillRef is one skill a chat can call.
type skillRef struct {
	Name        string // what a chat's active list stores
	Slug        string // lower-case, dash-separated: the /name form
	Description string
}

// skillUse is a skill a turn called, in the order it was called.
type skillUse struct {
	Name        string
	Description string
}

// skillEventType is the agent-event type that says "this turn used a skill".
const skillEventType agent.AgentEventType = "skill_used"

func (u skillUse) event() agent.AgentEvent {
	return agent.AgentEvent{Type: skillEventType, ToolName: u.Name, Content: u.Description}
}

type skillUsesCtxKey struct{}

func withSkillUses(ctx context.Context, uses []skillUse) context.Context {
	if len(uses) == 0 {
		return ctx
	}
	return context.WithValue(ctx, skillUsesCtxKey{}, uses)
}

func skillUsesFromCtx(ctx context.Context) []skillUse {
	u, _ := ctx.Value(skillUsesCtxKey{}).([]skillUse)
	return u
}

// withSkillEvents puts the skills this turn called in front of its saved
// agent events, so a reloaded chat still says which skills were used.
func withSkillEvents(ctx context.Context, events [][]interface{}) [][]interface{} {
	uses := skillUsesFromCtx(ctx)
	if len(uses) == 0 {
		return events
	}
	merged := make([]interface{}, 0, len(uses))
	for _, u := range uses {
		merged = append(merged, u.event())
	}
	for _, e := range events {
		merged = append(merged, e...)
	}
	return [][]interface{}{merged}
}

// skillSlug is the /name form of a skill name: "Pair Programming" ->
// "pair-programming".
func skillSlug(name string) string {
	var b strings.Builder
	dash := true
	for _, r := range strings.ToLower(name) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			dash = false
		} else if !dash {
			b.WriteByte('-')
			dash = true
		}
	}
	return strings.Trim(b.String(), "-")
}

// skillCatalog lists every skill chatID can call: the chat's own project
// skills plus the globally installed ones.
func (a *App) skillCatalog(chatID string) []skillRef {
	seen := map[string]bool{}
	var out []skillRef
	add := func(name, desc string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, skillRef{Name: name, Slug: skillSlug(name), Description: strings.TrimSpace(desc)})
	}
	for _, s := range a.projectSkillList(chatID) {
		add(s.Dir, s.Description)
	}
	if a.skillManager != nil {
		for _, d := range a.skillManager.List() {
			add(d.Manifest.Name, d.Manifest.Description)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// slashToken finds "/name" in a message: at the start or after whitespace or an
// opening bracket/quote, so a path like "src/app/x" or a URL never matches.
var slashToken = regexp.MustCompile(`(?:^|[\s(\[{"'])/([\p{L}\p{N}][\p{L}\p{N}_:.\-]*)`)

// skillWord is the word that marks "a skill" in either language, with
// whatever suffix the language puts on it ("skillini", "skill'leri", "becerisi").
var skillWord = regexp.MustCompile(`(?i)(skill|beceri|yetenek)`)

// autoSkillAsk matches a request for skills with no name: "gerekli skilleri
// kullanarak", "use the relevant skills", "skill'leri kullanarak çöz".
var autoSkillAsk = regexp.MustCompile(
	`(?i)(?:^|[^\p{L}])(gerekli|uygun|ilgili|lazım|lazim|gereken|necessary|relevant|appropriate|suitable|needed|right)[\s,]+(?:\p{L}+[\s,]+){0,2}(skill|beceri|yetenek)` +
		`|(?i)(skill|beceri|yetenek)\p{L}*(?:['’]\p{L}+)?[\s,]+(?:\p{L}+[\s,]+)?(kullan|use|apply)`)

// detectSkillCalls finds which skills a message calls. named lists the skills
// the message names (in message order); auto is true when it asks for skills
// without naming any.
func detectSkillCalls(msg string, cat []skillRef) (named []skillRef, auto bool) {
	if len(cat) == 0 || strings.TrimSpace(msg) == "" {
		return nil, false
	}
	type hit struct {
		pos int
		ref skillRef
	}
	var hits []hit
	taken := map[string]bool{}
	take := func(pos int, r skillRef) {
		if taken[r.Name] {
			return
		}
		taken[r.Name] = true
		hits = append(hits, hit{pos, r})
	}

	// /name — compared against the slug and the lower-cased name.
	for _, m := range slashToken.FindAllStringSubmatchIndex(msg, -1) {
		tok := strings.ToLower(strings.TrimRight(msg[m[2]:m[3]], ".:,;-_"))
		if tok == "" {
			continue
		}
		for _, r := range cat {
			if tok == r.Slug || tok == strings.ToLower(r.Name) {
				take(m[2], r)
				break
			}
		}
	}

	// The skill's name next to the word "skill".
	lower := strings.ToLower(msg)
	for _, r := range cat {
		if taken[r.Name] {
			continue
		}
		for _, form := range uniqueStrings(strings.ToLower(r.Name), r.Slug) {
			if pos := nameNextToSkillWord(lower, form); pos >= 0 {
				take(pos, r)
				break
			}
		}
	}

	sort.Slice(hits, func(i, j int) bool { return hits[i].pos < hits[j].pos })
	for _, h := range hits {
		named = append(named, h.ref)
	}
	if len(named) == 0 {
		auto = autoSkillAsk.MatchString(msg)
	}
	return named, auto
}

func uniqueStrings(in ...string) []string {
	var out []string
	for _, s := range in {
		if s == "" {
			continue
		}
		dup := false
		for _, o := range out {
			if o == s {
				dup = true
			}
		}
		if !dup {
			out = append(out, s)
		}
	}
	return out
}

// nameNextToSkillWord returns where name occurs in text as a whole word with
// the word "skill" (or its Turkish forms) within a few words of it, or -1. A
// bare mention of a skill's name is not a call: "plan" is a skill name and a
// word people write all the time.
func nameNextToSkillWord(text, name string) int {
	const window = 22
	from := 0
	for from < len(text) {
		i := strings.Index(text[from:], name)
		if i < 0 {
			return -1
		}
		i += from
		end := i + len(name)
		from = end
		if !wordBoundary(text, i, end) {
			continue
		}
		lo, hi := i-window, end+window
		if lo < 0 {
			lo = 0
		}
		if hi > len(text) {
			hi = len(text)
		}
		for lo < hi && !utf8.RuneStart(text[lo]) {
			lo++
		}
		for hi < len(text) && !utf8.RuneStart(text[hi]) {
			hi--
		}
		if skillWord.MatchString(text[lo:i]) || skillWord.MatchString(text[end:hi]) {
			return i
		}
	}
	return -1
}

// wordBoundary reports whether text[start:end] is not glued to letters or
// digits on either side.
func wordBoundary(text string, start, end int) bool {
	if start > 0 {
		r, _ := utf8.DecodeLastRuneInString(text[:start])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	if end < len(text) {
		r, _ := utf8.DecodeRuneInString(text[end:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

var skillStopWords = map[string]bool{
	"skill": true, "skills": true, "skilleri": true, "skillerini": true, "kullanarak": true, "gerekli": true,
	"using": true, "with": true, "that": true, "this": true, "please": true, "sorunu": true, "sorun": true,
	"çöz": true, "coz": true, "yapalım": true, "yapalim": true, "uygun": true, "ilgili": true, "necessary": true,
	"relevant": true, "appropriate": true, "needed": true, "beceri": true, "becerileri": true,
}

// skillWords splits text into lower-case words of four or more letters.
func skillWords(text string) []string {
	var out []string
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) {
		if utf8.RuneCountInString(w) >= 4 && !skillStopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// sameStem is a loose word match that survives suffixes ("performans" /
// "performance", "testleri" / "testing"): equal, or the same first six letters.
func sameStem(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	n := 0
	for n < len(ra) && n < len(rb) && ra[n] == rb[n] {
		n++
	}
	return n >= 6 || (n >= 5 && n >= len(ra)-1 && n >= len(rb)-1)
}

// pickSkillsFor picks the skills whose name and description best match what the
// message is about. No model call: it runs on every such message, including in
// modes that forbid Memo-initiated side calls. It is a word match, so an agent
// turn also has use_skill, where the model itself chooses.
func pickSkillsFor(msg string, cat []skillRef, max int) []skillRef {
	want := skillWords(msg)
	if len(want) == 0 || max <= 0 {
		return nil
	}
	type scored struct {
		ref   skillRef
		score int
	}
	var all []scored
	for _, r := range cat {
		nameWords := skillWords(r.Name + " " + r.Slug)
		descWords := skillWords(r.Description)
		score := 0
		for _, w := range want {
			matched := 0
			for _, n := range nameWords {
				if sameStem(w, n) {
					matched = 2
					break
				}
			}
			if matched == 0 {
				for _, d := range descWords {
					if sameStem(w, d) {
						matched = 1
						break
					}
				}
			}
			score += matched
		}
		if score >= 2 {
			all = append(all, scored{r, score})
		}
	}
	sort.SliceStable(all, func(i, j int) bool { return all[i].score > all[j].score })
	if len(all) > max {
		all = all[:max]
	}
	out := make([]skillRef, len(all))
	for i, s := range all {
		out[i] = s.ref
	}
	return out
}

// activateChatSkills adds names to chatID's active skills; those already
// active stay as they are.
func (a *App) activateChatSkills(chatID string, names []string) {
	sm := a.getSessionManager()
	if sm == nil || chatID == "" || len(names) == 0 {
		return
	}
	active := sm.GetActiveSkills(chatID)
	have := map[string]bool{}
	for _, n := range active {
		have[n] = true
	}
	next := append([]string(nil), active...)
	for _, n := range names {
		if !have[n] {
			have[n] = true
			next = append(next, n)
		}
	}
	if len(next) == len(active) {
		return
	}
	if err := sm.SetActiveSkills(chatID, next); err != nil {
		logx.Printf("skill: activate %v in %s: %v", names, chatID, err)
	}
}

// skillsForTurn works out which skills this message calls, switches them on for
// the chat (so the system prompt built next carries their instructions) and
// returns ctx carrying them, for the stream notice and the saved message.
func (a *App) skillsForTurn(ctx context.Context, chatID, userMsg string) (context.Context, []skillUse) {
	if chatID == "" || a.skillManager == nil {
		return ctx, nil
	}
	cat := a.skillCatalog(chatID)
	if len(cat) == 0 {
		return ctx, nil
	}
	named, auto := detectSkillCalls(userMsg, cat)
	if len(named) == 0 && auto {
		named = pickSkillsFor(userMsg, cat, 3)
	}
	if len(named) == 0 {
		return ctx, nil
	}
	uses := make([]skillUse, len(named))
	names := make([]string, len(named))
	for i, r := range named {
		uses[i] = skillUse{Name: r.Name, Description: r.Description}
		names[i] = r.Name
	}
	a.activateChatSkills(chatID, names)
	return withSkillUses(ctx, uses), uses
}

// skillNoticeChunks is the live "using skill X" line for each skill a turn
// called: an ordinary agent_event chunk, the shape every client already reads.
func skillNoticeChunks(uses []skillUse) []api.StreamChunk {
	out := make([]api.StreamChunk, 0, len(uses))
	for _, u := range uses {
		data, err := json.Marshal(u.event())
		if err != nil {
			continue
		}
		out = append(out, api.StreamChunk{Content: string(data), FinishReason: "agent_event"})
	}
	return out
}

// prependChunks sends chunks ahead of everything inner produces.
func prependChunks(ctx context.Context, inner <-chan api.StreamChunk, chunks []api.StreamChunk) <-chan api.StreamChunk {
	if len(chunks) == 0 {
		return inner
	}
	out := make(chan api.StreamChunk, 128)
	go func() {
		defer close(out)
		defer recoverPanic("prependChunks")
		for _, c := range chunks {
			trySend(ctx, out, c)
		}
		forwardStream(ctx, inner, out)
	}()
	return out
}

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
	cat := a.skillCatalog(chatID)
	if len(cat) == 0 {
		return "No skills are installed.", nil
	}
	if strings.TrimSpace(name) == "" || strings.EqualFold(strings.TrimSpace(name), "list") {
		var b strings.Builder
		b.WriteString("Installed skills (call use_skill with a name to load one):\n")
		for _, r := range cat {
			desc := r.Description
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
	def, ok := a.lookupSkill(chatID, ref.Name)
	if !ok {
		return "", fmt.Errorf("skill %q is no longer installed", ref.Name)
	}
	a.activateChatSkills(chatID, []string{ref.Name})
	text := def.Instructions
	if len(text) > maxSkillToolChars {
		text = text[:maxSkillToolChars] + "\n\n[…the rest of this skill was cut: it is longer than this tool returns]"
	}
	return fmt.Sprintf("Skill %q is now active in this chat. Follow these instructions:\n\n%s", ref.Name, text), nil
}
