package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"memo/internal/agent/tools"
	"memo/internal/taskloop"
)

// taskMdEditorAdapter backs the create_task_md / edit_task_md agent tools
// (internal/agent/tools/taskmd_tools.go). Registered in app.go.
type taskMdEditorAdapter struct{ a *App }

func (ad taskMdEditorAdapter) CreateTaskMd(ctx context.Context, req tools.CreateTaskMdRequest) (string, error) {
	return ad.a.createTaskMd(ctx, req)
}
func (ad taskMdEditorAdapter) EditTaskMd(ctx context.Context, req tools.EditTaskMdRequest) (string, error) {
	return ad.a.editTaskMd(ctx, req)
}

// resolveTaskMdPath picks the file path: an explicit req path (absolute, or
// relative to the current agent chat's project dir), else "<project>/Task.md".
func (a *App) resolveTaskMdPath(ctx context.Context, reqPath string) (string, error) {
	projectDir := ""
	if chatID := currentChatIDFromContext(ctx); chatID != "" {
		if sm := a.getSessionManager(); sm != nil {
			projectDir = sm.GetProjectPath(chatID)
		}
	}
	p := strings.TrimSpace(reqPath)
	if p == "" {
		if projectDir == "" {
			return "", errors.New(a.t(
				"dosya yolu verilmedi ve bu sohbetin bir proje klasörü yok",
				"no path given and this chat has no project folder"))
		}
		return filepath.Join(projectDir, "Task.md"), nil
	}
	if filepath.IsAbs(p) {
		return filepath.Clean(p), nil
	}
	if projectDir == "" {
		return "", errors.New(a.t(
			"göreli yol verildi ama bu sohbetin bir proje klasörü yok",
			"a relative path was given but this chat has no project folder"))
	}
	return filepath.Join(projectDir, p), nil
}

func (a *App) createTaskMd(ctx context.Context, req tools.CreateTaskMdRequest) (string, error) {
	path, err := a.resolveTaskMdPath(ctx, req.Path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(path); err == nil {
		return "", fmt.Errorf(a.t("%s zaten var — düzenlemek için edit_task_md kullan", "%s already exists — use edit_task_md to change it"), path)
	}

	headers := map[string]string{}
	set := func(k, v string) {
		if s := strings.TrimSpace(v); s != "" {
			headers[k] = s
		}
	}
	set("bildirim", req.Notify)
	// Default chat-created task lists to planner/executor mode: its coder
	// turns run in isolated sub-agent turns (nothing written to this chat),
	// whereas worker mode streams every round into the chat history and
	// floods it. Worker mode stays available via an explicit "# mod: worker".
	mode := strings.TrimSpace(req.Mode)
	if mode == "" {
		mode = taskloop.ModePlanner
	}
	set("mod", mode)
	set("planlayıcı", req.PlannerModel)
	set("kodlayıcı", req.CoderModel)
	set("doğrulayıcı", req.VerifierModel)
	set("hafıza", req.Memory)
	if req.AutoApprove {
		headers["onay"] = "otomatik"
	}

	var items []taskloop.TaskMdItem
	for _, it := range req.Items {
		if s := strings.TrimSpace(it); s != "" {
			items = append(items, taskloop.TaskMdItem{Text: s})
		}
	}
	if len(items) == 0 {
		return "", errors.New(a.t("en az bir madde gerek", "at least one item is required"))
	}

	body := taskloop.RenderTaskMd(taskloop.TaskMdDoc{
		Headers: headers,
		Intro:   strings.TrimSpace(req.Intro),
		Items:   items,
	})
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf(a.t("Task.md yazıldı: %s (%d madde)", "Wrote Task.md: %s (%d items)"), path, len(items)), nil
}

func (a *App) editTaskMd(ctx context.Context, req tools.EditTaskMdRequest) (string, error) {
	path, err := a.resolveTaskMdPath(ctx, req.Path)
	if err != nil {
		return "", err
	}
	parsed, err := taskloop.ParseTaskMd(path)
	if err != nil {
		return "", err
	}

	switch req.Op {
	case "check_item":
		it, err := nthItem(parsed.Items, req.ItemIndex)
		if err != nil {
			return "", err
		}
		if err := taskloop.MarkItemDone(path, it.Line); err != nil {
			return "", err
		}
		return fmt.Sprintf(a.t("%d. madde işaretlendi", "item %d checked off"), req.ItemIndex), nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	lines := strings.Split(string(data), "\n")

	switch req.Op {
	case "add_item":
		text := strings.TrimSpace(req.Text)
		if text == "" {
			return "", errors.New(a.t("add_item için text gerek", "add_item needs text"))
		}
		newLine := "- [ ] " + text
		insertAt := len(lines)
		if n := len(parsed.Items); n > 0 {
			insertAt = parsed.Items[n-1].Line // 1-based line == index after it
		}
		lines = insertLines(lines, insertAt, []string{newLine})

	case "set_header":
		key := strings.ToLower(strings.TrimSpace(req.HeaderKey))
		val := strings.TrimSpace(req.HeaderValue)
		if key == "" || val == "" {
			return "", errors.New(a.t("set_header için header_key ve header_value gerek", "set_header needs header_key and header_value"))
		}
		hdr := "# " + key + ": " + val
		replaced := false
		for i, ln := range lines {
			t := strings.TrimSpace(strings.ToLower(ln))
			if strings.HasPrefix(t, "# "+key) && strings.Contains(ln, ":") {
				lines[i] = hdr
				replaced = true
				break
			}
		}
		if !replaced {
			lines = insertLines(lines, 0, []string{hdr})
		}

	case "split_item":
		it, err := nthItem(parsed.Items, req.ItemIndex)
		if err != nil {
			return "", err
		}
		subs := nonEmptyStrings(req.SubItems)
		if len(subs) == 0 {
			return "", errors.New(a.t("split_item için sub_items gerek", "split_item needs sub_items"))
		}
		idx := it.Line - 1
		if idx >= 0 && idx < len(lines) && !strings.Contains(strings.ToLower(lines[idx]), "[parallel]") {
			lines[idx] = strings.TrimRight(lines[idx], " ") + " [parallel]"
		}
		pad := strings.Repeat(" ", it.Indent+2)
		subLines := make([]string, len(subs))
		for i, s := range subs {
			subLines[i] = pad + "- [ ] " + s
		}
		lines = insertLines(lines, it.Line, subLines)
	}

	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return "", err
	}
	// Re-parse so the reply reflects the real post-edit state.
	after, err := taskloop.ParseTaskMd(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf(a.t("Task.md güncellendi: %s (%d madde)", "Updated Task.md: %s (%d items)"), path, len(after.Items)), nil
}

func nthItem(items []taskloop.ParsedItem, oneBased int) (taskloop.ParsedItem, error) {
	if oneBased < 1 || oneBased > len(items) {
		return taskloop.ParsedItem{}, fmt.Errorf("item_index %d out of range (1..%d)", oneBased, len(items))
	}
	return items[oneBased-1], nil
}

func nonEmptyStrings(ss []string) []string {
	var out []string
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			out = append(out, strings.TrimSpace(s))
		}
	}
	return out
}

// insertLines returns lines with extra spliced in after 1-based position `at`
// (at == 0 prepends, at >= len appends).
func insertLines(lines []string, at int, extra []string) []string {
	if at < 0 {
		at = 0
	}
	if at > len(lines) {
		at = len(lines)
	}
	out := make([]string, 0, len(lines)+len(extra))
	out = append(out, lines[:at]...)
	out = append(out, extra...)
	out = append(out, lines[at:]...)
	return out
}
