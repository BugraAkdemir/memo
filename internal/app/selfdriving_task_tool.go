package app

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// selfDrivingTaskToolAdapter backs the start_self_driving_task agent tool
// (internal/agent/tools/selfdrivingtask.go). Registered in app.go.
type selfDrivingTaskToolAdapter struct{ a *App }

func (ad selfDrivingTaskToolAdapter) StartSelfDrivingTask(ctx context.Context, taskMdPath, title string) (string, error) {
	return ad.a.StartSelfDrivingTaskFromChat(ctx, taskMdPath, title)
}

// StartSelfDrivingTaskFromChat creates a task list from taskMdPath and starts
// the autonomous loop, binding it to the chat whose agent turn is currently
// running (resolved from ctx, never a chat the model names). That chat must
// be an Agent chat — it's where the loop's worker turns land and what the
// Tasks tab / task detail screen show.
func (a *App) StartSelfDrivingTaskFromChat(ctx context.Context, taskMdPath, title string) (string, error) {
	chatID := currentChatIDFromContext(ctx)
	if chatID == "" {
		return "", errors.New(a.t(
			"bu sohbetin kimliği çözülemedi; görevi Görevler sekmesinden başlat",
			"could not resolve this chat; start the task from the Tasks tab instead"))
	}
	sm := a.getSessionManager()
	if sm == nil || !sm.IsAgentChat(chatID) {
		return "", errors.New(a.t(
			"otonom görev yalnızca bir Ajan sohbetinden başlatılabilir; önce bu sohbete Ajan sekmesinden bir proje klasörü bağla",
			"a Self-Driving task can only be started from an Agent chat; attach a project folder to this chat from the Agent tab first"))
	}

	tl, err := a.CreateTaskListFromTaskMd(chatID, title, taskMdPath)
	if err != nil {
		return "", err
	}
	// context.Background(), not ctx: the task loop outlives this single agent
	// turn, exactly like the REST start handler (see
	// TestHandleTaskListStart_DoesNotUseRequestContext).
	if err := a.StartTaskList(context.Background(), tl.ID); err != nil {
		return "", err
	}

	done := 0
	for _, it := range tl.Items {
		if it.Status == "done" {
			done++
		}
	}
	var b strings.Builder
	fmt.Fprintf(&b, a.t("🚀 Otonom görev başlatıldı: %s\n%d madde", "🚀 Self-Driving task started: %s\n%d item(s)"), tl.Title, len(tl.Items))
	if done > 0 {
		fmt.Fprintf(&b, a.t(" (%d tanesi zaten işaretliydi)", " (%d already checked off)"), done)
	}
	b.WriteString(a.t(
		"\nİlerlemeyi Görevler sekmesinden izleyebilir ya da bana \"görev durumu\" diye sorabilirsin.",
		"\nTrack progress in the Tasks tab, or ask me for the task status."))
	return b.String(), nil
}
