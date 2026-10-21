package app

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode/utf8"

	"memo/internal/api"
	"memo/internal/config"
	"memo/internal/identity"
	"memo/internal/sessions"
	"memo/internal/skill"
)

func testSkillCatalog() []skillRef {
	mk := func(name, desc string) skillRef {
		return skillRef{Name: name, Slug: skillSlug(name), Description: desc}
	}
	return []skillRef{
		mk("Pair Programming", "AI-assisted pair programming with driver and navigator modes"),
		mk("codebase-memory", "Use the codebase knowledge graph for structural code queries"),
		mk("frontend-design", "Create distinctive production-grade frontend interfaces"),
		mk("plan", "Plan the work"),
		mk("Skill Builder", "Create new skills"),
		mk("Verification & Quality Assurance", "Verify code quality and test coverage"),
	}
}

func names(refs []skillRef) []string {
	var out []string
	for _, r := range refs {
		out = append(out, r.Name)
	}
	return out
}

func TestSkillSlug(t *testing.T) {
	for in, want := range map[string]string{
		"Pair Programming":                 "pair-programming",
		"codebase-memory":                  "codebase-memory",
		"Verification & Quality Assurance": "verification-quality-assurance",
		"  V3 CLI  ":                       "v3-cli",
		"ecc:go-review":                    "ecc-go-review",
		"İş Planı":                         "iş-planı",
		"GEREKLİ SKİLLER":                  "gerekli-skiller",
		"x² y":                             "x-y",
	} {
		if got := skillSlug(in); got != want {
			t.Errorf("skillSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestDetectSkillCalls(t *testing.T) {
	cat := testSkillCatalog()
	cases := []struct {
		name    string
		msg     string
		want    []string
		wantAut bool
	}{
		{"slash form", "login sorununu /codebase-memory kullanarak çöz", []string{"codebase-memory"}, false},
		{"slash form for a spaced name", "bunu /pair-programming ile yapalım", []string{"Pair Programming"}, false},
		{"slash at the start", "/frontend-design landing page yap", []string{"frontend-design"}, false},
		{"skill word after the name (Turkish suffix)", "login sorununu frontend-design skillini kullanarak çöz", []string{"frontend-design"}, false},
		{"skill word before the name", "use the skill Pair Programming for this bug", []string{"Pair Programming"}, false},
		{"name with spaces next to skill", "Pair Programming becerisini kullan", []string{"Pair Programming"}, false},
		{"two skills keep message order", "önce /frontend-design sonra /codebase-memory", []string{"frontend-design", "codebase-memory"}, false},
		{"unknown slash is not a skill", "src/app/main.go dosyasını aç, /etc/hosts da bak", nil, false},
		{"a path is not a /name", "bak src/codebase-memory/x.go", nil, false},
		{"a bare common word that is also a skill name is not a call", "bir plan yapalım, planın ilk adımı ne olsun", nil, false},
		{"the same word next to 'skill' is a call", "plan skill kullan", []string{"plan"}, false},
		{"ask without a name (Turkish)", "xxx sorunu gerekli skillerini kullanarak çöz", nil, true},
		{"ask without a name (English)", "fix it using the relevant skills", nil, true},
		{"ask without a name, no qualifier", "skilleri kullanarak çöz", nil, true},
		{"nothing about skills", "login sayfası açılmıyor", nil, false},
		{"a path that starts with a skill name is not a call", "cat /plan/x.go ve ls /plan/dir", nil, false},
		{"the name far from the word skill is not a call", "make a plan for my skills", nil, false},
		{"the name after 'and' is not a call", "list my skills and plan", nil, false},
		{"a question about skills is not an ask", "skill kullanımı nedir? How are skills used by memo?", nil, false},
		{"mentioning skills is not an ask", "my skills are useful, write a skill to use markdown", nil, false},
		{"a skill that uses something is not an ask", "I need a skill that uses Go", nil, false},
		{"skill: name", "skill: plan the sprint", []string{"plan"}, false},
		{"capital Turkish İ still reads as an ask", "GEREKLİ SKİLLERİ kullanarak çöz", nil, true},
		{"capital İ in a name", "frontend-design SKİLLİNİ kullan", []string{"frontend-design"}, false},
		{"the word skill alone is not a call", "bu skill sistemi nasıl çalışıyor?", nil, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, auto := detectSkillCalls(c.msg, cat)
			if strings.Join(names(got), "|") != strings.Join(c.want, "|") {
				t.Errorf("named = %v, want %v", names(got), c.want)
			}
			if auto != c.wantAut {
				t.Errorf("auto = %v, want %v", auto, c.wantAut)
			}
		})
	}
}

func TestPickSkillsFor(t *testing.T) {
	cat := testSkillCatalog()
	got := pickSkillsFor("kodun kalitesini ve test kapsamını doğrula, code quality coverage", cat, 3)
	if len(got) == 0 || got[0].Name != "Verification & Quality Assurance" {
		t.Errorf("picked %v, want the verification skill first", names(got))
	}
	if got := pickSkillsFor("tamamen alakasız bir cümle yazıyorum", cat, 3); len(got) != 0 {
		t.Errorf("picked %v for a message about nothing", names(got))
	}
	if got := pickSkillsFor("pair programming driver navigator", cat, 1); len(got) != 1 || got[0].Name != "Pair Programming" {
		t.Errorf("max=1 picked %v", names(got))
	}
}

// A skill called "skill-builder" must reach the model; /skill's own command
// parser used to swallow every message that merely started with "/skill".
func TestHandleSkillCommand_SkillNamedLikeTheCommandIsNotSwallowed(t *testing.T) {
	a, chatID := newTestAppWithActiveSkill(t, "x", 10)
	ctx := context.Background()
	if ch := a.handleSkillCommand(ctx, chatID, "/skill-builder yeni bir skill yaz"); ch != nil {
		t.Fatal("/skill-builder was treated as a /skill command")
	}
	if ch := a.handleSkillCommand(ctx, chatID, "/skills"); ch != nil {
		t.Fatal("/skills was treated as a /skill command")
	}
	for _, cmd := range []string{"/skill", "/skill:off", "/skill:on x", "/skill remove nope", "/skill\n"} {
		if ch := a.handleSkillCommand(ctx, chatID, cmd); ch == nil {
			t.Errorf("%q is a /skill command and must still be handled", cmd)
		} else {
			for range ch {
			}
		}
	}
}

// appWithSkills is an App whose skill manager really has the named skills on
// disk, and whose session manager has two chats.
func appWithSkills(t *testing.T, defs map[string]string) (*App, string, string) {
	t.Helper()
	dir := t.TempDir()
	for name, desc := range defs {
		d := filepath.Join(dir, "skills", name)
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
		body := "---\nname: " + name + "\ndescription: \"" + desc + "\"\ndanger_level: safe\n---\nINSTRUCTIONS FOR " + name + "\n"
		if err := os.WriteFile(filepath.Join(d, "SKILL.md"), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	m := skill.NewManager(dir)
	if err := m.Discover(); err != nil {
		t.Fatal(err)
	}
	sm, err := sessions.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := sm.GetActiveID()
	second := sm.NewChat()
	return &App{cfg: &config.AppConfig{}, skillManager: m, sessions: sm}, first, second
}

// The point of the whole feature: calling a skill switches it on for that chat
// and only that chat, and its instructions are in the very prompt the call was
// made in.
func TestSkillsForTurn_ActivatesPerChat(t *testing.T) {
	a, chatA, chatB := appWithSkills(t, map[string]string{
		"codebase-memory": "Use the codebase knowledge graph",
		"frontend-design": "Build frontend interfaces",
	})
	sm := a.getSessionManager()

	ctx, uses := a.skillsForTurn(context.Background(), chatA, "login sorununu /codebase-memory kullanarak çöz")
	if len(uses) != 1 || uses[0].Name != "codebase-memory" {
		t.Fatalf("uses = %+v", uses)
	}
	if got := sm.GetActiveSkills(chatA); len(got) != 1 || got[0] != "codebase-memory" {
		t.Errorf("chat A active skills = %v, want [codebase-memory]", got)
	}
	if got := sm.GetActiveSkills(chatB); len(got) != 0 {
		t.Errorf("a skill called in chat A leaked into chat B: %v", got)
	}
	if got := skillUsesFromCtx(ctx); len(got) != 1 {
		t.Errorf("the turn's ctx does not carry the skill: %v", got)
	}
	if prompt := a.buildActiveSkillPrompt(chatA); !strings.Contains(prompt, "INSTRUCTIONS FOR codebase-memory") {
		t.Errorf("the skill's instructions are not in the prompt of the turn that called it: %q", prompt)
	}

	// Calling it again does not duplicate it; calling a second one adds it.
	a.skillsForTurn(context.Background(), chatA, "yine /codebase-memory ve /frontend-design")
	if got := sm.GetActiveSkills(chatA); len(got) != 2 {
		t.Errorf("active skills = %v, want exactly the two called", got)
	}

	// A message that calls nothing leaves the chat as it is.
	_, none := a.skillsForTurn(context.Background(), chatB, "merhaba nasılsın")
	if len(none) != 0 || len(sm.GetActiveSkills(chatB)) != 0 {
		t.Errorf("an ordinary message switched something on: %v / %v", none, sm.GetActiveSkills(chatB))
	}
}

func TestSkillsForTurn_AskWithoutNamePicksMatches(t *testing.T) {
	a, chatA, _ := appWithSkills(t, map[string]string{
		"frontend-design": "Build distinctive frontend interfaces and landing pages",
		"database-tuning": "Optimize slow SQL queries",
	})
	_, uses := a.skillsForTurn(context.Background(), chatA, "landing page tasarımını gerekli skilleri kullanarak yap (frontend interfaces)")
	if len(uses) != 1 || uses[0].Name != "frontend-design" {
		t.Fatalf("uses = %+v, want only frontend-design", uses)
	}
}

func TestSkillNoticeAndSavedEvents(t *testing.T) {
	uses := []skillUse{{Name: "codebase-memory", Description: "graph"}}
	chunks := skillNoticeChunks(uses)
	if len(chunks) != 1 || chunks[0].FinishReason != "agent_event" ||
		!strings.Contains(chunks[0].Content, `"type":"skill_used"`) || !strings.Contains(chunks[0].Content, `"tool":"codebase-memory"`) {
		t.Fatalf("notice chunk = %+v", chunks)
	}

	// prependChunks puts it in front of the real stream.
	inner := make(chan api.StreamChunk, 2)
	inner <- api.StreamChunk{Content: "hello"}
	inner <- api.StreamChunk{Done: true}
	close(inner)
	var order []string
	for c := range prependChunks(context.Background(), inner, chunks) {
		order = append(order, c.FinishReason+"/"+c.Content[:min(5, len(c.Content))])
	}
	if len(order) != 3 || !strings.HasPrefix(order[0], "agent_event/") || order[1] != "/hello" {
		t.Errorf("stream order = %v", order)
	}

	// The saved message keeps it, in front of the turn's own events.
	ctx := withSkillUses(context.Background(), uses)
	merged := withSkillEvents(ctx, [][]interface{}{{"tool-event"}})
	if len(merged) != 1 || len(merged[0]) != 2 || merged[0][1] != "tool-event" {
		t.Errorf("merged events = %+v", merged)
	}
	if got := withSkillEvents(context.Background(), [][]interface{}{{"x"}}); len(got) != 1 || len(got[0]) != 1 {
		t.Errorf("no skills called must leave the events alone: %+v", got)
	}
}

func TestUseSkillTool(t *testing.T) {
	a, chatA, chatB := appWithSkills(t, map[string]string{
		"Pair Programming": "Driver and navigator",
		"codebase-memory":  "Graph",
	})
	sm := a.getSessionManager()
	ad := skillToolAdapter{a}
	ctxA := withCurrentChatID(context.Background(), chatA)

	list, err := ad.UseSkill(ctxA, "")
	if err != nil || !strings.Contains(list, "Pair Programming") || !strings.Contains(list, "codebase-memory") {
		t.Fatalf("list = %q, %v", list, err)
	}
	if len(sm.GetActiveSkills(chatA)) != 0 {
		t.Fatal("listing skills switched one on")
	}

	out, err := ad.UseSkill(ctxA, "/pair-programming")
	if err != nil || !strings.Contains(out, "INSTRUCTIONS FOR Pair Programming") {
		t.Fatalf("out = %q, %v", out, err)
	}
	if got := sm.GetActiveSkills(chatA); len(got) != 1 || got[0] != "Pair Programming" {
		t.Errorf("active = %v", got)
	}
	if got := sm.GetActiveSkills(chatB); len(got) != 0 {
		t.Errorf("use_skill in chat A switched a skill on in chat B: %v", got)
	}
	// A unique fragment of the name is enough; an unknown one is an error the
	// model can read.
	if _, err := ad.UseSkill(ctxA, "codebase"); err != nil {
		t.Errorf("unique fragment: %v", err)
	}
	if _, err := ad.UseSkill(ctxA, "does-not-exist"); err == nil || !strings.Contains(err.Error(), "no installed skill") {
		t.Errorf("unknown skill error = %v", err)
	}
}

// A turn must not claim a skill is in use when it is not: Minimal Mode sends
// the model no skill instructions, and a chat whose active list cannot be
// updated will not carry the skill.
func TestSkillsForTurn_SaysSoWhenTheSkillCannotTakeEffect(t *testing.T) {
	a, chatA, _ := appWithSkills(t, map[string]string{"codebase-memory": "Graph"})
	sm := a.getSessionManager()

	a.identity = identity.New("T", "Memo", "casual", "", true)
	ctx, uses := a.skillsForTurn(context.Background(), chatA, "bak /codebase-memory")
	if len(uses) != 1 || uses[0].Skipped != skillSkipMinimalMode {
		t.Fatalf("Minimal Mode: uses = %+v, want one skipped (minimal_mode)", uses)
	}
	if got := sm.GetActiveSkills(chatA); len(got) != 0 {
		t.Errorf("Minimal Mode switched the skill on: %v", got)
	}
	ev := uses[0].event()
	if ev.Type != skillSkippedEventType || ev.ToolName != "codebase-memory" || ev.Content != skillSkipMinimalMode {
		t.Errorf("skipped event = %+v", ev)
	}
	if merged := withSkillEvents(ctx, nil); len(merged) != 1 || len(merged[0]) != 1 {
		t.Errorf("the saved reply must carry the skip notice too: %+v", merged)
	}

	a.identity = identity.New("T", "Memo", "casual", "", false)
	_, uses = a.skillsForTurn(context.Background(), "no-such-chat", "bak /codebase-memory")
	if len(uses) != 1 || uses[0].Skipped != skillSkipNotSaved {
		t.Errorf("a chat whose list cannot be saved: uses = %+v, want one skipped (not_saved)", uses)
	}
	if chunks := skillNoticeChunks(uses); len(chunks) != 1 || !strings.Contains(chunks[0].Content, `"skill_skipped"`) {
		t.Errorf("notice = %+v", chunks)
	}
}

func TestUseSkillTool_DoesNotClaimItIsActiveWhenItIsNot(t *testing.T) {
	a, _, _ := appWithSkills(t, map[string]string{"codebase-memory": "Graph"})
	out, err := skillToolAdapter{a}.UseSkill(withCurrentChatID(context.Background(), "no-such-chat"), "codebase-memory")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out, "is now active in this chat") || !strings.Contains(out, "THIS turn only") ||
		!strings.Contains(out, "INSTRUCTIONS FOR codebase-memory") {
		t.Errorf("tool result = %q", out)
	}
}

func TestUseSkillTool_CutNeverSplitsACharacter(t *testing.T) {
	a, chatA, _ := appWithSkills(t, map[string]string{"big": "x"})
	// One ASCII byte, then 3-byte characters: the cap falls in the middle of one.
	def, _ := a.lookupSkill(chatA, "big")
	def.Instructions = "a" + strings.Repeat("€", maxSkillToolChars)
	out, err := skillToolAdapter{a}.UseSkill(withCurrentChatID(context.Background(), chatA), "big")
	if err != nil {
		t.Fatal(err)
	}
	if !utf8.ValidString(out) || !strings.Contains(out, "was cut") {
		t.Errorf("cut result valid=%v cutNote=%v", utf8.ValidString(out), strings.Contains(out, "was cut"))
	}
}

// Calling a skill by name switches it on for the chat for good, so the model
// must not be able to do it for a project skill — that text came with a cloned
// repository and nobody read it.
func TestUseSkillTool_NeverLoadsAProjectSkill(t *testing.T) {
	a, chatA, _ := appWithSkills(t, map[string]string{"codebase-memory": "Graph"})
	cat := a.buildSkillCatalog(chatA, false)
	if len(cat) != 1 || cat[0].Name != "codebase-memory" {
		t.Fatalf("global-only catalog = %+v", cat)
	}
	out, err := skillToolAdapter{a}.UseSkill(withCurrentChatID(context.Background(), chatA), "")
	if err != nil || strings.Contains(out, "\n\n") {
		t.Errorf("listing = %q, %v", out, err)
	}
}

func TestSkillsForTurn_IgnoresAutomatedTurnsAndQuietMessages(t *testing.T) {
	a, chatA, _ := appWithSkills(t, map[string]string{"codebase-memory": "Graph"})
	sm := a.getSessionManager()

	ctx := withoutSkillCalls(context.Background())
	if _, uses := a.skillsForTurn(ctx, chatA, "run /codebase-memory now"); len(uses) != 0 {
		t.Errorf("a routine/delegated prompt called a skill: %+v", uses)
	}
	if got := sm.GetActiveSkills(chatA); len(got) != 0 {
		t.Errorf("automated turn switched a skill on: %v", got)
	}
	// No "/" and no word skill: not even looked at.
	if _, uses := a.skillsForTurn(context.Background(), chatA, "merhaba nasılsın"); len(uses) != 0 {
		t.Errorf("quiet message called %+v", uses)
	}
}

func TestAddActiveSkills_KeepsEveryConcurrentAddition(t *testing.T) {
	_, chatA, _ := appWithSkills(t, map[string]string{"a": "x"})
	a, _, _ := appWithSkills(t, map[string]string{"a": "x"})
	sm := a.getSessionManager()
	chat := sm.GetActiveID()
	_ = chatA
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_ = sm.AddActiveSkills(chat, []string{fmt.Sprintf("s%d", i), "shared"})
		}(i)
	}
	wg.Wait()
	if got := sm.GetActiveSkills(chat); len(got) != 21 {
		t.Errorf("got %d active skills (%v), want 20 distinct + shared = 21", len(got), got)
	}
}

func TestPickSkillsFor_LongMessageIsFast(t *testing.T) {
	cat := testSkillCatalog()
	msg := strings.Repeat("alpha beta gamma delta epsilon zeta eta theta iota kappa ", 40000)
	start := time.Now()
	pickSkillsFor(msg, cat, 3)
	if d := time.Since(start); d > 2*time.Second {
		t.Errorf("pickSkillsFor on a %d byte message took %v", len(msg), d)
	}
}
