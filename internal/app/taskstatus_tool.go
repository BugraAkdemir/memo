package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/taskloop"
)

// taskStatusToolAdapter backs the get_task_status agent tool
// (internal/agent/tools/taskstatus.go). Registered in app.go.
type taskStatusToolAdapter struct{ a *App }

func (ad taskStatusToolAdapter) GetTaskStatus(ctx context.Context) (string, error) {
	return ad.a.TaskStatusForChat(ctx), nil
}

// TaskStatusForChat returns a human-readable snapshot of the Self-Driving
// task(s). It prefers the task bound to the chat whose agent turn is running
// (resolved from ctx) and falls back to every running list. When nothing is
// running it says so plainly — the tool exists so the model reports "I can't
// see a running task" instead of inventing a failure narrative (BUG-PLAN10).
func (a *App) TaskStatusForChat(ctx context.Context) string {
	if a.taskloopEngine == nil {
		return a.t("Görev döngüsü motoru başlatılmamış — çalışan görev yok.",
			"The task loop engine is not initialised — no running task.")
	}
	running := a.taskloopEngine.RunningTasks()
	if len(running) == 0 {
		return a.t("Şu an çalışan bir otonom görev yok.",
			"There is no Self-Driving task running right now.")
	}

	chatID := currentChatIDFromContext(ctx)
	var mine []taskloop.RunningTaskInfo
	for _, r := range running {
		if chatID != "" && r.ChatID == chatID {
			mine = append(mine, r)
		}
	}
	show := mine
	scope := a.t("bu sohbete bağlı görev", "task bound to this chat")
	if len(show) == 0 {
		show = running
		scope = a.t("çalışan tüm görevler", "all running tasks")
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%s (%d):\n", scope, len(show))
	for _, r := range show {
		b.WriteString(formatRunningTask(a, r))
	}
	return strings.TrimRight(b.String(), "\n")
}

func formatRunningTask(a *App, r taskloop.RunningTaskInfo) string {
	var b strings.Builder
	title := r.Title
	if title == "" {
		title = r.ID
	}
	fmt.Fprintf(&b, "• %s — %s", title, r.Phase)
	if r.Mode == taskloop.ModePlanner && r.PlanSteps > 0 {
		fmt.Fprintf(&b, ", %s %d/%d", a.t("adım", "step"), r.PlanStepsDone, r.PlanSteps)
	}
	fmt.Fprintf(&b, ", %s %d/%d", a.t("madde", "item"), r.DoneCount, r.ItemCount)
	if strings.TrimSpace(r.CurrentItem) != "" {
		fmt.Fprintf(&b, "\n  %s: %s", a.t("şu an", "current"), strings.TrimSpace(r.CurrentItem))
	}
	if len(r.SubAgents) > 0 {
		fmt.Fprintf(&b, "\n  %s: %s", a.t("alt-ajanlar", "sub-agents"), strings.Join(r.SubAgents, ", "))
	}
	fmt.Fprintf(&b, "\n  %s: %ds", a.t("geçen süre", "elapsed"), r.ElapsedSec)
	b.WriteByte('\n')
	return b.String()
}
