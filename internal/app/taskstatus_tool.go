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

func (ad taskStatusToolAdapter) PauseChatTask(ctx context.Context) (string, error) {
	return ad.a.pauseOrResumeChatTask(ctx, false), nil
}

func (ad taskStatusToolAdapter) ResumeChatTask(ctx context.Context) (string, error) {
	return ad.a.pauseOrResumeChatTask(ctx, true), nil
}

// chatBoundTaskID returns the id of the Self-Driving task list bound to the
// chat whose agent turn is running (from ctx), preferring a list the engine
// still has in memory, else the most recent list for that chat.
func (a *App) chatBoundTaskID(ctx context.Context) string {
	chatID := currentChatIDFromContext(ctx)
	if chatID == "" || a.taskloopStore == nil {
		return ""
	}
	if a.taskloopEngine != nil {
		for _, r := range a.taskloopEngine.RunningTasks() {
			if r.ChatID == chatID {
				return r.ID
			}
		}
	}
	newestID, newestAt := "", ""
	for _, info := range a.taskloopStore.List() {
		tl, err := a.taskloopStore.Get(info.ID)
		if err != nil || tl.ChatID != chatID {
			continue
		}
		if newestID == "" || info.UpdatedAt > newestAt {
			newestID, newestAt = info.ID, info.UpdatedAt
		}
	}
	return newestID
}

func (a *App) pauseOrResumeChatTask(ctx context.Context, resume bool) string {
	id := a.chatBoundTaskID(ctx)
	if id == "" {
		return a.t("Bu sohbete bağlı bir otonom görev bulamadım.",
			"I couldn't find a Self-Driving task bound to this chat.")
	}
	if a.taskloopEngine == nil {
		return a.t("Görev döngüsü motoru başlatılmamış.", "The task loop engine is not initialised.")
	}
	if resume {
		if a.taskloopEngine.IsRunning(id) {
			return a.t("Görev zaten çalışıyor.", "The task is already running.")
		}
		if err := a.StartTaskList(context.Background(), id); err != nil {
			return a.t("Görev sürdürülemedi: ", "Could not resume the task: ") + err.Error()
		}
		return a.t("▶️ Görev kaldığı adımdan sürüyor.", "▶️ The task is resuming from where it stopped.")
	}
	a.StopTaskList(id)
	return a.t("⏸️ Görev duraklatıldı. Sormak istediğini yaz; \"devam\" deyince kaldığı adımdan sürer.",
		"⏸️ Task paused. Ask what you need; say \"devam\" / \"continue\" to resume from the same step.")
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
