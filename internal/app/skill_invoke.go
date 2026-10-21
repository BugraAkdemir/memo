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

// skillUse is a skill a turn called, in the order it was called. Skipped is
// set (to a reason code) when the call could NOT take effect: the turn then
// says so instead of claiming the skill is in use.
type skillUse struct {
	Name        string
	Description string
	Skipped     string
}

// Why a called skill did not take effect. The client maps these to words.
const (
	skillSkipMinimalMode = "minimal_mode" // Minimal Mode sends no skill instructions
	skillSkipNotSaved    = "not_saved"    // the chat's active list could not be updated
)

// The agent-event types for "this turn used a skill" / "could not use it".
const (
	skillEventType        agent.AgentEventType = "skill_used"
	skillSkippedEventType agent.AgentEventType = "skill_skipped"
)

func (u skillUse) event() agent.AgentEvent {
	if u.Skipped != "" {
		return agent.AgentEvent{Type: skillSkippedEventType, ToolName: u.Name, Content: u.Skipped}
	}
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

// normSkillText lower-cases text for matching. A Turkish capital İ becomes a
// plain "i" first: Unicode lower-casing turns it into "i" plus a combining dot,
// which would break every comparison ("GEREKLİ SKİLLERİ" must match like
// "gerekli skilleri"). The Dart side of the slug does the same.
func normSkillText(s string) string {
	return strings.ToLower(strings.ReplaceAll(s, "İ", "i"))
}

// skillSlug is the /name form of a skill name: "Pair Programming" ->
// "pair-programming". The Flutter menu inserts the same slug (skillSlug in
// skill_provider.dart); a shared set of vectors in both test suites keeps them
// identical.
func skillSlug(name string) string {
	var b strings.Builder
	dash := true
	for _, r := range normSkillText(name) {
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

// skillCatalog lists every skill chatID can call by name: the chat's own
// project skills plus the globally installed ones.
func (a *App) skillCatalog(chatID string) []skillRef {
	return a.buildSkillCatalog(chatID, true)
}

// buildSkillCatalog lists the callable skills; withProject adds the chat's
// project skills (.memo/skills), which arrive with a cloned repository.
func (a *App) buildSkillCatalog(chatID string, withProject bool) []skillRef {
	seen := map[string]bool{}
	var out []skillRef
	add := func(name, desc string) {
		if name == "" || seen[name] {
			return
		}
		seen[name] = true
		out = append(out, skillRef{Name: name, Slug: skillSlug(name), Description: strings.TrimSpace(desc)})
	}
	if withProject {
		for _, s := range a.projectSkillList(chatID) {
			add(s.Dir, s.Description)
		}
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
// The caller also rejects a match that is followed by another "/" or "\"
// ("cat /plan/x.go" is a path, not a call to a skill called plan).
var slashToken = regexp.MustCompile(`(?:^|[\s(\[{"'])/([\p{L}\p{N}][\p{L}\p{N}_:.\-]*)`)

// skillWord is the word that marks "a skill" in either language, with
// whatever suffix the language puts on it ("skillini", "skill'leri", "becerisi").
const skillWordPattern = `(?:skill|beceri|yetenek)`

var skillWord = regexp.MustCompile(skillWordPattern)

// skillAfterName / skillBeforeName: the word "skill" right next to a name — at
// most a few punctuation characters between ("frontend-design skillini",
// "the Pair Programming skill", "skill: plan", "skill 'plan'"). Further away it
// is just a sentence that happens to contain both words.
var (
	skillAfterName  = regexp.MustCompile(`^[\s'’":,()\-]{0,3}` + skillWordPattern)
	skillBeforeName = regexp.MustCompile(skillWordPattern + `[\p{L}]*[\s'’":,()\-]{0,3}$`)
)

// autoSkillAsk matches a request for skills with no name: "gerekli skilleri
// kullanarak", "use the relevant skills", "skilleri kullanarak çöz". It must
// read as an instruction: a question about skills ("skill kullanımı nedir",
// "how are skills used") or a sentence that merely mentions them is not one.
// Applied to normSkillText output.
var autoSkillAsk = regexp.MustCompile(
	`(?:^|[^\p{L}])(?:gerekli|uygun|ilgili|lazim|gereken|necessary|relevant|appropriate|suitable|needed)[\s,]+(?:\p{L}+[\s,]+){0,2}` + skillWordPattern +
		`|` + skillWordPattern + `\p{L}*(?:['’]\p{L}+)?[\s,]+(?:kullanarak|kullanip|kullanıp|using|kullan)(?:[\s.,!;:]|$)`)

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

	msg = strings.ReplaceAll(msg, "İ", "i")

	// /name — compared against the slug and the lower-cased name.
	for _, m := range slashToken.FindAllStringSubmatchIndex(msg, -1) {
		if end := m[3]; end < len(msg) && (msg[end] == '/' || msg[end] == '\\') {
			continue // a path: /plan/x.go
		}
		tok := strings.ToLower(strings.TrimRight(msg[m[2]:m[3]], ".:,;-_"))
		if tok == "" {
			continue
		}
		for _, r := range cat {
			if tok == r.Slug || tok == normSkillText(r.Name) {
				take(m[2], r)
				break
			}
		}
	}

	// The skill's name right next to the word "skill".
	lower := normSkillText(msg)
	for _, r := range cat {
		if taken[r.Name] {
			continue
		}
		for _, form := range uniqueStrings(normSkillText(r.Name), r.Slug) {
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
		auto = autoSkillAsk.MatchString(lower)
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
// the word "skill" (or its Turkish forms) right next to it, or -1. A bare
// mention of a skill's name is not a call: "plan" is a skill name and a word
// people write all the time.
func nameNextToSkillWord(text, name string) int {
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
		if skillAfterName.MatchString(text[end:]) || skillBeforeName.MatchString(text[:i]) {
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
	for _, w := range strings.FieldsFunc(normSkillText(text), func(r rune) bool {
		return !(unicode.IsLetter(r) || unicode.IsDigit(r))
	}) {
		if utf8.RuneCountInString(w) >= 4 && !skillStopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// maxPickWords bounds the work of pickSkillsFor on a very long message.
const maxPickWords = 200

func runeWords(words []string) [][]rune {
	out := make([][]rune, len(words))
	for i, w := range words {
		out[i] = []rune(w)
	}
	return out
}

// sameStem is a loose word match that survives suffixes ("performans" /
// "performance", "testleri" / "testing"): equal, or the same first six letters.
func sameStem(ra, rb []rune) bool {
	n := 0
	for n < len(ra) && n < len(rb) && ra[n] == rb[n] {
		n++
	}
	if n == len(ra) && n == len(rb) {
		return true
	}
	return n >= 6 || (n >= 5 && n >= len(ra)-1 && n >= len(rb)-1)
}

// pickSkillsFor picks the skills whose name and description best match what the
// message is about. No model call: it runs on every such message, including in
// modes that forbid Memo-initiated side calls. It is a word match, so an agent
// turn also has use_skill, where the model itself chooses.
func pickSkillsFor(msg string, cat []skillRef, max int) []skillRef {
	words := skillWords(msg)
	if len(words) == 0 || max <= 0 {
		return nil
	}
	seen := map[string]bool{}
	var distinct []string
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			distinct = append(distinct, w)
			if len(distinct) == maxPickWords {
				break
			}
		}
	}
	want := runeWords(distinct)
	type scored struct {
		ref   skillRef
		score int
	}
	var all []scored
	for _, r := range cat {
		nameWords := runeWords(skillWords(r.Name + " " + r.Slug))
		descWords := runeWords(skillWords(r.Description))
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
// active stay as they are. The error says the list could NOT be updated — the
// caller must not then claim the skill is on for the chat.
func (a *App) activateChatSkills(chatID string, names []string) error {
	sm := a.getSessionManager()
	if sm == nil {
		return fmt.Errorf("session manager not initialized")
	}
	if chatID == "" || len(names) == 0 {
		return nil
	}
	if err := sm.AddActiveSkills(chatID, names); err != nil {
		logx.Printf("skill: activate %v in %s: %v", names, chatID, err)
		return err
	}
	return nil
}

type noSkillCallsCtxKey struct{}

// withoutSkillCalls marks a turn nobody typed (a routine, a delegated live-mode
// job): a "/word" or "skills" inside such a prompt is not a call, and switching
// skills on persistently in a chat no one is watching would go unnoticed.
func withoutSkillCalls(ctx context.Context) context.Context {
	return context.WithValue(ctx, noSkillCallsCtxKey{}, true)
}

// skillsForTurn works out which skills this message calls, switches them on for
// the chat (so the system prompt built next carries their instructions) and
// returns ctx carrying them, for the stream notice and the saved message.
func (a *App) skillsForTurn(ctx context.Context, chatID, userMsg string) (context.Context, []skillUse) {
	if chatID == "" || a.skillManager == nil {
		return ctx, nil
	}
	if off, _ := ctx.Value(noSkillCallsCtxKey{}).(bool); off || taskRunConfigFromCtx(ctx) != nil {
		return ctx, nil
	}
	// Cheap exit for the common message: every way to call a skill has a "/" or
	// the word skill in it, and building the catalog reads project skill files.
	if !strings.Contains(userMsg, "/") && !skillWord.MatchString(normSkillText(userMsg)) {
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
	// Minimal Mode sends the model no skill instructions at all, so "using skill
	// X" would be false: say it was skipped, and do not switch it on.
	if a.identity != nil && a.identity.GetMinimalMode() {
		for i := range uses {
			uses[i].Skipped = skillSkipMinimalMode
		}
		return withSkillUses(ctx, uses), uses
	}
	if err := a.activateChatSkills(chatID, names); err != nil {
		for i := range uses {
			uses[i].Skipped = skillSkipNotSaved
		}
	}
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
