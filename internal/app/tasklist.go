package app

import (
	"context"
	"fmt"
	"memo/internal/api"
	"memo/internal/logx"
	"memo/internal/provider"
	"memo/internal/taskloop"
	"path/filepath"
	"strings"
	"time"
)

func (a *App) CreateTaskList(chatID, title string, items []string) (*taskloop.TaskList, error) {
	if a.taskloopStore == nil {
		return nil, fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	sm := a.getSessionManager()
	if sm == nil || !sm.IsAgentChat(chatID) {
		return nil, fmt.Errorf("görev listesi yalnızca bir ajan sohbetine bağlanabilir; önce Ajan sekmesinden bir proje sohbeti seçin")
	}
	return a.taskloopStore.Create(chatID, title, items)
}

// CreateTaskListFromTaskMd parses a Task.md file, seeds a task list from its
// checkbox items, and records the notification level (from the "# bildirim:"
// header) plus the file path so item completion can mirror "[ ]" -> "[x]" back
// into it. Already-checked items are stored as done. A file with no checkboxes
// is an error.
func (a *App) CreateTaskListFromTaskMd(chatID, title, taskMdPath string) (*taskloop.TaskList, error) {
	if a.taskloopStore == nil {
		return nil, fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	sm := a.getSessionManager()
	if sm == nil || !sm.IsAgentChat(chatID) {
		return nil, fmt.Errorf("görev listesi yalnızca bir ajan sohbetine bağlanabilir; önce Ajan sekmesinden bir proje sohbeti seçin")
	}
	parsed, err := taskloop.ParseTaskMd(taskMdPath)
	if err != nil {
		return nil, err
	}
	if len(parsed.Items) == 0 {
		return nil, fmt.Errorf("Task.md içinde onay kutusu maddesi bulunamadı: %s", taskMdPath)
	}
	texts := make([]string, len(parsed.Items))
	for i, it := range parsed.Items {
		texts[i] = it.Text
	}
	if strings.TrimSpace(title) == "" {
		title = filepath.Base(taskMdPath)
	}
	tl, err := a.taskloopStore.CreateWithMeta(chatID, title, texts, parsed.NotifyLevel, taskMdPath)
	if err != nil {
		return nil, err
	}
	// "# mod: planlayıcı" opts a Task.md into planner/executor mode.
	switch strings.ToLower(strings.TrimSpace(parsed.Headers["mod"])) {
	case "planlayıcı", "planlayici", "planner":
		if err := a.taskloopStore.SetMode(tl.ID, taskloop.ModePlanner); err != nil {
			logx.Printf("taskloop: set mode planlayıcı %s: %v", tl.ID, err)
		}
	}
	for i, it := range parsed.Items {
		if i >= len(tl.Items) {
			break
		}
		// Record the source line so completion can flip the checkbox in place.
		if err := a.taskloopStore.SetItemLine(tl.ID, tl.Items[i].ID, it.Line); err != nil {
			logx.Printf("taskloop: seed item line %s: %v", tl.Items[i].ID, err)
		}
		// Carry over pre-checked items so a partially-done Task.md resumes
		// rather than redoing completed work.
		if it.Status == "done" {
			if err := a.taskloopStore.SetItemDone(tl.ID, tl.Items[i].ID); err != nil {
				logx.Printf("taskloop: seed done item %s: %v", tl.Items[i].ID, err)
			}
		}
	}
	return a.taskloopStore.Get(tl.ID)
}

// memoSystemGuidance returns the instructions of the built-in memo-system
// skill for the task loop's planning/self-heal prompts. It is read directly
// rather than activated, so it never appears in the user's normal skill list.
func (a *App) memoSystemGuidance() string {
	if a.skillManager == nil {
		return ""
	}
	if def, ok := a.skillManager.Get("memo-system"); ok {
		return def.Instructions
	}
	return ""
}

func (a *App) GetTaskList(id string) (*taskloop.TaskList, error) {
	if a.taskloopStore == nil {
		return nil, fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	return a.taskloopStore.Get(id)
}

func (a *App) ListTaskLists() []taskloop.TaskListInfo {
	if a.taskloopStore == nil {
		return nil
	}
	return a.taskloopStore.List()
}

func (a *App) DeleteTaskList(id string) error {
	if a.taskloopStore == nil {
		return fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	if a.taskloopEngine != nil && a.taskloopEngine.IsRunning(id) {
		a.taskloopEngine.Stop(id)
	}
	return a.taskloopStore.Delete(id)
}

func (a *App) StartTaskList(ctx context.Context, listID string) error {
	if a.taskloopEngine == nil {
		return fmt.Errorf("görev döngüsü motoru başlatılmamış")
	}
	if a.taskloopStore == nil {
		return fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return err
	}
	sm := a.getSessionManager()
	if sm == nil || !sm.IsAgentChat(tl.ChatID) {
		return fmt.Errorf("bu listenin bağlı olduğu sohbet artık bir ajan sohbeti değil (silinmiş olabilir); listeyi yeniden oluşturun")
	}
	if a.taskNotifyBus != nil {
		a.taskNotifyBus.SetLevel(listID, tl.NotifyLevel)
	}
	return a.taskloopEngine.Start(ctx, listID)
}

func (a *App) StopTaskList(listID string) {
	if a.taskloopEngine != nil {
		a.taskloopEngine.Stop(listID)
	}
}

// CancelTaskList stops a list and marks it cancelled (terminal — unlike pause,
// it won't resume).
func (a *App) CancelTaskList(listID string) error {
	if a.taskloopEngine == nil || a.taskloopStore == nil {
		return fmt.Errorf("görev döngüsü motoru başlatılmamış")
	}
	a.taskloopEngine.Stop(listID)
	return a.taskloopStore.SetStatus(listID, "cancelled")
}

// SkipCurrentItem abandons whatever item a list is on and moves to the next.
func (a *App) SkipCurrentItem(listID string) error {
	if a.taskloopEngine == nil {
		return fmt.Errorf("görev döngüsü motoru başlatılmamış")
	}
	return a.taskloopEngine.SkipCurrent(listID)
}

// InjectTaskMessage sends a free-text instruction into a task list's own
// agent chat and returns the assistant's reply.
func (a *App) InjectTaskMessage(ctx context.Context, listID, text string) (string, error) {
	if a.taskloopStore == nil {
		return "", fmt.Errorf("görev listesi sistemi başlatılmamış")
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for chunk := range a.SendMessageStreamTo(context.Background(), tl.ChatID, text) {
		if chunk.Error != "" {
			return sb.String(), fmt.Errorf("%s", chunk.Error)
		}
		sb.WriteString(chunk.Content)
		if chunk.Done {
			break
		}
	}
	return strings.TrimSpace(sb.String()), nil
}

// ListRunningTasks returns a live view of every executing task list.
func (a *App) ListRunningTasks() []taskloop.RunningTaskInfo {
	if a.taskloopEngine == nil {
		return nil
	}
	return a.taskloopEngine.RunningTasks()
}

func (a *App) runningTaskList() []taskloop.RunningTaskInfo { return a.ListRunningTasks() }

func (a *App) buildTaskLoopRunWorker() taskloop.RunWorker {
	return func(ctx context.Context, chatID, prompt string) (string, error) {
		// SendMessageStreamTo (docs/plans/PLAN_chatid_refactor.md Faz 3)
		// targets chatID directly and activates tool execution because the
		// chat itself is an agent chat — no SwitchChat, no global
		// agent-mode flag to flip and race back. taskloopRunMu now only
		// serializes task-list turns against each other, so two lists
		// running at once queue in order instead of both racing streamMu
		// and one silently failing its turn with a "please wait" error.
		a.taskloopRunMu.Lock()
		defer a.taskloopRunMu.Unlock()

		ch := a.SendMessageStreamTo(ctx, chatID, prompt)
		var sb strings.Builder
		for chunk := range ch {
			if chunk.Error != "" {
				return sb.String(), fmt.Errorf("işçi hatası: %s", chunk.Error)
			}
			// Only real assistant prose — skip the status chunks the stream
			// interleaves (agent_event tool JSON, the memory_used count),
			// otherwise the chief reviews polluted input.
			if chunk.Content != "" && (chunk.FinishReason == "" || chunk.FinishReason == "stop") {
				sb.WriteString(chunk.Content)
			}
			if chunk.Done {
				break
			}
		}
		result := sb.String()
		if result == "" {
			return "", fmt.Errorf("işçi boş çıktı döndü")
		}
		return result, nil
	}
}

func (a *App) buildTaskLoopReviewChief() taskloop.ReviewChief {
	return func(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
		a.providerMu.RLock()
		orch := a.orchestraConductor
		orchEnabled := orch != nil && orch.Config().Enabled
		a.providerMu.RUnlock()

		if orchEnabled {
			return a.reviewChiefViaOrchestra(ctx, itemText, workerOutput)
		}
		return a.reviewChiefViaLocal(ctx, itemText, workerOutput)
	}
}

func (a *App) reviewChiefViaOrchestra(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
	a.providerMu.RLock()
	cfg := a.orchestraConductor.Config()
	a.providerMu.RUnlock()

	prov, err := a.orchestraConductor.CreateProviderForType(cfg.ChiefType, cfg.ChiefModel)
	if err != nil {
		return false, "", fmt.Errorf("chief provider oluşturulamadı: %w", err)
	}

	req := provider.ChatRequest{
		Model: cfg.ChiefModel,
		Messages: []provider.Message{
			provider.TextMessage("system", taskloop.ChiefReviewSystemPrompt()),
			provider.TextMessage("user", taskloop.ChiefReviewPrompt(itemText, workerOutput)),
		},
		Temperature: 0.2,
		MaxTokens:   1024,
	}
	if pCfg := a.orchestraConductor.FindProviderConfig(cfg.ChiefType); pCfg != nil {
		req.EffortLevel = pCfg.EffortLevel
	}

	reviewCtx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	resp, err := prov.ChatCompletion(reviewCtx, req)
	if err != nil {
		return false, "", fmt.Errorf("chief inceleme hatası: %w", err)
	}

	return taskloop.ExtractAndParseReview(resp.Content)
}

func (a *App) reviewChiefViaLocal(ctx context.Context, itemText, workerOutput string) (bool, string, error) {
	msgs := []api.Message{
		api.NewTextMessage("system", taskloop.ChiefReviewSystemPrompt()),
		api.NewTextMessage("user", taskloop.ChiefReviewPrompt(itemText, workerOutput)),
	}

	raw := a.callLLMForReview(ctx, msgs)
	if strings.HasPrefix(raw, "\u26a0") {
		return false, "", fmt.Errorf("CEO LLM hatası: %s", raw)
	}

	return taskloop.ExtractAndParseReview(raw)
}

func (a *App) callLLMForReview(ctx context.Context, messages []api.Message) string {
	a.providerMu.RLock()
	activeName := a.activeProviderName
	providerRouter := a.providerRouter
	a.providerMu.RUnlock()

	if activeName != "" && providerRouter != nil {
		pctx, cancel := context.WithTimeout(ctx, 120*time.Second)
		defer cancel()

		pMsgs := make([]provider.Message, len(messages))
		for i, m := range messages {
			pMsgs[i] = provider.Message{Role: m.Role, Content: m.Content}
		}

		req := provider.ChatRequest{
			Messages:    pMsgs,
			Temperature: 0.2,
			MaxTokens:   1024,
		}

		resp, err := providerRouter.ChatCompletion(pctx, req)
		if err != nil {
			return "⚠️ " + err.Error()
		}
		return resp.Content
	}

	lctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()

	a.clientMu.RLock()
	llmClient := a.client
	a.clientMu.RUnlock()

	if llmClient == nil {
		return "⚠️ Yerel model yüklenmemiş."
	}

	resp, err := llmClient.ChatCompletion(lctx, messages, 0.2, 1.0, 1024)
	if err != nil {
		return "⚠️ " + err.Error()
	}
	if len(resp.Choices) == 0 {
		return "⚠️ Boş yanıt"
	}
	return resp.Choices[0].Message.GetTextContent()
}
