package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"memo/internal/agent"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/taskloop"
)

// runPlanStep is the engine's WithStepRunner callback: one fresh, ephemeral
// coder turn per plan step. No session, no RAG/memory/persona — a bare
// system+user pair, permissions bypassed (starting the task was the consent).
func (a *App) runPlanStep(ctx context.Context, listID string, step taskloop.PlanStep, stateDoc string) (string, error) {
	router, model, effort, err := a.planexecRouting(listID, "coder")
	if err != nil {
		return "", err
	}

	projectPath := a.taskListProjectPath(listID)
	subExec := agent.NewSubAgentExecutor(a.agentExecutor, agent.NewRegistry(), router, projectPath)
	subExec.SetBypassPermissions(true)

	userPrompt := coderStepUserPrompt(projectPath, step, stateDoc)
	if a.taskloopStore != nil {
		if notes := a.taskloopStore.DrainResumeNotes(listID); len(notes) > 0 {
			userPrompt = "# Kullanıcı notları (görev duraklatılmışken yazıldı) — YÜKSEK ÖNCELİK\n\n" +
				strings.Join(notes, "\n") +
				"\n\nBu notlardaki gerçek talimatları uygula: bir şey EKLE/DÜZELT deniyorsa, " +
				"bu adımın dar kapsamının dışına çıkıp gereken dosyaları oluştur/değiştir. " +
				"Salt durum sorularını (\"nerdeyiz\", \"bitti mi\") yok say. Zaten biten işi baştan yapma.\n\n" +
				"---\n\n" +
				userPrompt
		}
	}

	msgs := []provider.Message{
		provider.TextMessage("system", coderStepSystemPrompt()),
		provider.TextMessage("user", userPrompt),
	}

	sctx, scancel := context.WithCancel(ctx)
	defer scancel()
	start := time.Now()
	streamCh, err := subExec.RunStream(sctx, "", model, effort, msgs,
		func(ev agent.AgentEvent) { a.emitStepToolActivity(listID, ev) }, projectPath)
	if err != nil {
		return "", err
	}
	out, usage, derr := drainStreamIdleUsage(streamCh, scancel, a.streamIdleTimeout())
	// Count the turn's prompt + generated output too — the per-tool hook above
	// only sees tool round-trips, not the model's own text.
	promptEst := taskloop.EstTokens(coderStepSystemPrompt()) + taskloop.EstTokens(userPrompt)
	a.taskloopEngine.AddTokens(listID, promptEst+taskloop.EstTokens(out))
	a.recordTaskStreamUsage(start, routerProviderName(router), model, categoryTaskStep, usage, promptEst, out)
	if derr != nil {
		return "", fmt.Errorf("coder: %w", derr)
	}
	return strings.TrimSpace(out), nil
}

// toolVerb maps an agent tool name to a short verb for the live activity
// block — the Go counterpart of chat_message_list.dart's _AgentStatusBadge.
// TR+EN like every other user-visible backend string (AGENTS.md #8); these
// lines are pushed straight into the Flutter task card, so a Turkish-only
// map would have been a hardcoded string in the UI.
var toolVerbs = map[string][2]string{
	"read_file":            {"Dosya okudu", "Read a file"},
	"write_file":           {"Dosya yazdı", "Wrote a file"},
	"edit_file":            {"Dosya düzenledi", "Edited a file"},
	"insert_line":          {"Satır ekledi", "Inserted a line"},
	"delete_lines":         {"Sildi", "Deleted lines"},
	"delete_file":          {"Dosya sildi", "Deleted a file"},
	"run_command":          {"Komut", "Command"},
	"run_command_readonly": {"Komut (salt-okunur)", "Command (read-only)"},
	"search_files":         {"Arama yaptı", "Searched files"},
	"list_directory":       {"Klasör listeledi", "Listed a folder"},
	"get_file_info":        {"Dosya bilgisi", "File info"},
	"change_directory":     {"Klasör değiştirdi", "Changed folder"},
	"share_file":           {"Dosya paylaştı", "Shared a file"},
	"web_search":           {"Web araması", "Web search"},
	"fetch_page":           {"Sayfa okudu", "Read a page"},
	// Task-loop and assistant tools a worker turn can reach. Without an entry
	// here the activity line shows the bare tool id ("edit_task_md"), which is
	// what the live task card used to print.
	"edit_task_md":        {"Task.md güncelledi", "Updated Task.md"},
	"create_task_md":      {"Task.md oluşturdu", "Created Task.md"},
	"get_task_status":     {"Görev durumuna baktı", "Checked task status"},
	"pause_task":          {"Görevi duraklattı", "Paused the task"},
	"resume_task":         {"Görevi sürdürdü", "Resumed the task"},
	"create_routine":      {"Rutin oluşturdu", "Created a routine"},
	"list_routines":       {"Rutinleri listeledi", "Listed routines"},
	"cancel_routine":      {"Rutini iptal etti", "Cancelled a routine"},
	"get_calendar_events": {"Takvime baktı", "Checked the calendar"},
	"whatsapp_send":       {"WhatsApp mesajı gönderdi", "Sent a WhatsApp message"},
	"whatsapp_messages":   {"WhatsApp mesajlarını okudu", "Read WhatsApp messages"},
	"whatsapp_latest":     {"Son WhatsApp mesajları", "Latest WhatsApp messages"},
	"whatsapp_search":     {"WhatsApp'ta aradı", "Searched WhatsApp"},
	"configure_provider":  {"Sağlayıcı ayarını değiştirdi", "Changed a provider setting"},
	"read_env":            {"Ortam değişkeni okudu", "Read an env var"},
}

// slowTools are the tools whose *execution* can take real time (as opposed to
// a file write, which returns instantly once the model has finished producing
// its content). Only these get a "starting" line — pairing start/finish on an
// instant tool would double the log without telling the user anything.
var slowTools = map[string]bool{
	"run_command":          true,
	"run_command_readonly": true,
	"web_search":           true,
	"fetch_page":           true,
	"search_files":         true,
	"whatsapp_send":        true,
	"whatsapp_search":      true,
}

func (a *App) toolVerb(name string) string {
	if v, ok := toolVerbs[name]; ok {
		return a.t(v[0], v[1])
	}
	return name
}

// emitStepToolActivity forwards one coder/worker tool call into the live task
// activity stream.
//
// Finished results and errors always produce a line. A *starting* line is
// emitted only for tools whose execution can actually take a while
// (slowTools) — for those, the pair is the only thing standing between the
// user and "it looks frozen"; for an instant tool like write_file the pair
// would land in the same second and just double the log.
func (a *App) emitStepToolActivity(listID string, ev agent.AgentEvent) {
	a.emitToolActivity(listID, "", ev)
}

// emitSubAgentActivity is emitStepToolActivity's counterpart for a sub-agent
// turn (SubAgentOrchestrator.Spawn — subagent.go), role-tagged so four
// sub-agents running in parallel read as four sub-agents in the log instead
// of one indistinguishable stream. Wired as appSubAgentRunner.Run's onEvent;
// see that call site for why the wiring matters (it used to be a no-op).
func (a *App) emitSubAgentActivity(listID, role string, ev agent.AgentEvent) {
	a.emitToolActivity(listID, role, ev)
}

// emitToolActivity is the shared body: role is a "[coder] " style prefix, or
// "" for a plain worker/coder-step turn that doesn't need one.
func (a *App) emitToolActivity(listID, role string, ev agent.AgentEvent) {
	if a.taskloopEngine == nil {
		return
	}
	prefix := ""
	if role != "" {
		prefix = "[" + role + "] "
	}

	if ev.Type == agent.EventToolExecuting {
		if !slowTools[ev.ToolName] {
			return
		}
		a.taskloopEngine.EmitActivity(listID, "tool_start", prefix+a.toolLine(ev)+" …")
		return
	}

	if ev.Type != agent.EventToolResult && ev.Type != agent.EventToolError {
		return
	}
	// Every tool round-trip pushes its args + result back through the model as
	// context on the next turn — feed that into the list's running token
	// estimate so the chat block has a number that keeps climbing while the
	// planner/coder works (a "not frozen" signal, not a billing figure).
	a.taskloopEngine.AddTokens(listID,
		taskloop.EstTokens(string(ev.Args))+taskloop.EstTokens(ev.Result)+taskloop.EstTokens(ev.Content))
	line := prefix + a.toolLine(ev)
	if ev.Type == agent.EventToolError {
		// toolVerbs stores completed-action phrasing ("Task.md oluşturdu" /
		// "Created Task.md") — reusing it unmodified for a failed call reads
		// as if the action had succeeded, with only the ⚠ glyph to notice
		// otherwise. Live-observed: create_task_md failed with "relative
		// path but this chat has no project folder" and the task card still
		// said "Task.md oluşturdu" — a real error read as a success. Lead
		// with an explicit failure word and the real reason instead.
		reason := ev.Error
		if len(reason) > 120 {
			reason = reason[:120] + "…"
		}
		line = "⚠ " + a.t("Başarısız", "Failed") + " — " + prefix + a.toolVerb(ev.ToolName)
		if reason != "" {
			line += ": " + reason
		}
	}
	a.taskloopEngine.EmitActivity(listID, "tool", line)
}

// toolLine renders "<verb>: <short arg>" for one tool event.
func (a *App) toolLine(ev agent.AgentEvent) string {
	line := a.toolVerb(ev.ToolName)
	if arg := shortToolArg(ev.Args); arg != "" {
		line += ": " + arg
	}
	return line
}

// shortToolArg pulls a compact identifier (a path, a command) out of an agent
// event's args JSON for the activity line.
func shortToolArg(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return ""
	}
	for _, k := range []string{"path", "file", "file_path", "command", "cmd", "query", "url", "pattern", "dir"} {
		if s, ok := m[k].(string); ok && strings.TrimSpace(s) != "" {
			s = strings.TrimSpace(s)
			if len(s) > 80 {
				s = s[:80] + "…"
			}
			return s
		}
	}
	return ""
}

func coderStepSystemPrompt() string {
	return `Sen bir uygulayıcı (coder) ajansın. Sana bir planın TEK bir adımı verilecek.
SADECE o adımı uygula — fazlasını yapma, sıradaki adımlara geçme.
Dosya oku/yaz/düzenle ve gerekiyorsa komut çalıştır. Bittiğinde kısa bir özet ver:
hangi dosyalara dokundun, ne değişti. Repo kurallarına uy.`
}

func coderStepUserPrompt(projectPath string, step taskloop.PlanStep, stateDoc string) string {
	var b strings.Builder
	if rules, err := taskloop.ReadRules(projectPath); err == nil && strings.TrimSpace(rules) != "" {
		b.WriteString("# Repo kuralları\n\n")
		b.WriteString(rules)
		b.WriteString("\n\n")
	}
	if strings.TrimSpace(stateDoc) != "" {
		b.WriteString("# Şu ana kadarki ilerleme (bağlam)\n\n")
		b.WriteString(strings.TrimSpace(stateDoc))
		b.WriteString("\n\n")
	}
	b.WriteString("# Bu adım\n\n")
	b.WriteString(strings.TrimSpace(step.Text))
	b.WriteByte('\n')
	if strings.TrimSpace(step.LiteralContent) != "" {
		b.WriteString("\nBirebir uygulanacak içerik:\n```\n")
		b.WriteString(step.LiteralContent)
		b.WriteString("\n```\n")
	}
	if len(step.AcceptanceChecks) > 0 {
		b.WriteString("\nBu adım şunları sağlamalı:\n")
		for _, c := range step.AcceptanceChecks {
			fmt.Fprintf(&b, "- (%s) %s\n", c.Kind, c.Spec)
		}
	}
	return b.String()
}

// acceptancecheck is the engine's WithAcceptanceChecker callback: run a step's
// checks after the coder turn. Deterministic checks (command / grep) run
// directly in the project dir; fuzzy checks go to one verifier-model call.
func (a *App) acceptancecheck(ctx context.Context, listID string, step taskloop.PlanStep) (bool, string, error) {
	projectPath := a.taskListProjectPath(listID)
	for _, c := range step.AcceptanceChecks {
		switch strings.ToLower(c.Kind) {
		case "command":
			if ok, detail := runCheckCommand(ctx, projectPath, c.Spec, c.Expect, a.acceptanceCommandTimeout()); !ok {
				return false, fmt.Sprintf("command %q: %s", c.Spec, detail), nil
			}
		case "grep":
			if ok, detail := runCheckGrep(ctx, projectPath, c.Spec, c.Expect); !ok {
				return false, fmt.Sprintf("grep %q: %s", c.Spec, detail), nil
			}
		case "fuzzy":
			ok, detail, err := a.runFuzzyCheck(ctx, listID, projectPath, c.Spec)
			if err != nil {
				logx.Printf("TASKLOOP: fuzzy check error (%s): %v — treating as pass", listID, err)
				continue
			}
			if !ok {
				return false, "fuzzy: " + detail, nil
			}
		}
	}
	return true, "", nil
}

func (a *App) acceptanceCommandTimeout() time.Duration {
	a.cfgMu.RLock()
	s := a.cfg.TaskLoop.AcceptanceCommandTimeoutSec
	a.cfgMu.RUnlock()
	if s <= 0 {
		s = 300
	}
	return time.Duration(s) * time.Second
}

func runCheckCommand(ctx context.Context, dir, spec, expect string, timeout time.Duration) (bool, string) {
	if timeout <= 0 {
		timeout = 300 * time.Second
	}
	cctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(cctx, "bash", "-lc", spec)
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	out := buf.String()
	if err != nil {
		return false, fmt.Sprintf("exit error: %v\n%s", err, tail(out, 500))
	}
	// "present"/"absent" are grep semantics; planners routinely copy them onto
	// command checks where they mean nothing — for a command, exit 0 is the
	// pass. Only a real, non-sentinel expect string is matched against stdout.
	e := strings.ToLower(strings.TrimSpace(expect))
	if expect != "" && e != "present" && e != "absent" && !strings.Contains(out, expect) {
		return false, fmt.Sprintf("output missing %q", expect)
	}
	return true, ""
}

func runCheckGrep(ctx context.Context, dir, pattern, expect string) (bool, string) {
	cctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	target := dir
	if target == "" {
		target = "."
	}
	cmd := exec.CommandContext(cctx, "grep", "-rIl", "--", pattern, target)
	err := cmd.Run()
	found := err == nil
	wantAbsent := strings.EqualFold(strings.TrimSpace(expect), "absent")
	if wantAbsent {
		if found {
			return false, "pattern present but expected absent"
		}
		return true, ""
	}
	if !found {
		return false, "pattern not found"
	}
	return true, ""
}

func (a *App) runFuzzyCheck(ctx context.Context, listID, projectPath, criterion string) (bool, string, error) {
	ctxInfo := ""
	if projectPath != "" {
		// A git diff is the best signal when the project is a repo; most
		// Self-Driving targets are NOT, so fall back to a recent-files listing
		// so the verifier is never judging blind (that made every fuzzy check
		// fail with "no change summary").
		if out, err := exec.CommandContext(ctx, "git", "-C", projectPath, "diff", "--stat").Output(); err == nil && len(strings.TrimSpace(string(out))) > 0 {
			ctxInfo = "git diff --stat:\n" + string(out)
		} else {
			ctxInfo = "Proje dosyaları (git yok):\n" + projectFileListing(projectPath, 60)
		}
	}
	msgs := []api.Message{
		api.NewTextMessage("system", `Sen bir denetleyicisin. Bir kabul ölçütü ve projenin güncel durumu (git diff ya da dosya listesi) verilecek.
Gerekirse dosyaları okuduğunu VARSAY — elinde listesi var. SADECE şu JSON'u döndür: {"approved": true, "feedback": ""}.
Ölçüt açıkça karşılanmıyorsa approved:false + kısa feedback; emin değilsen approved:true (şüpheden yararlandır).`),
		api.NewTextMessage("user", fmt.Sprintf("Ölçüt: %s\n\nProje durumu:\n%s", criterion, tail(ctxInfo, 3000))),
	}
	raw := a.callLLMForReviewWith(ctx, msgs, categoryTaskReview, a.planexecRoleRouter(listID, "verifier"))
	if strings.HasPrefix(raw, "⚠") {
		return false, "", fmt.Errorf("verifier LLM: %s", raw)
	}
	pass, reason, err := taskloop.ExtractAndParseReview(raw)
	if err != nil {
		return false, "", err
	}
	return pass, reason, nil
}

// compactPlanState is the engine's WithStateCompactor callback.
func (a *App) compactPlanState(ctx context.Context, listID, current string) (string, error) {
	msgs := []api.Message{
		api.NewTextMessage("system", `Aşağıdaki ilerleme kaydını kısalt. Kararları, dokunulan dosyaları ve keşfedilen tuzakları KORU; tekrarları ve ayrıntıyı at. Markdown döndür.`),
		api.NewTextMessage("user", current),
	}
	raw := a.callLLMForReviewWith(ctx, msgs, categoryCompaction, a.planexecRoleRouter(listID, "verifier"))
	if strings.HasPrefix(raw, "⚠") {
		return "", fmt.Errorf("compactor LLM: %s", raw)
	}
	return strings.TrimSpace(raw), nil
}

func tail(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return "…" + s[len(s)-n:]
}

// projectFileListing returns up to `max` non-hidden files under root, "path
// (size, mtime)" per line, newest first — a blind-verifier fallback when the
// project isn't a git repo.
func projectFileListing(root string, max int) string {
	type fi struct {
		rel  string
		size int64
		mod  time.Time
	}
	var files []fi
	_ = filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		rel, _ := filepath.Rel(root, p)
		if strings.HasPrefix(rel, ".") || strings.Contains(rel, "/.") || strings.Contains(rel, "__pycache__") {
			return nil
		}
		files = append(files, fi{rel, info.Size(), info.ModTime()})
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return files[i].mod.After(files[j].mod) })
	if len(files) > max {
		files = files[:max]
	}
	var b strings.Builder
	for _, f := range files {
		fmt.Fprintf(&b, "%s (%d B, %s)\n", f.rel, f.size, f.mod.Format("15:04:05"))
	}
	return b.String()
}
