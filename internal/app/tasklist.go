package app

import (
	"context"
	"fmt"
	"memo/internal/api"
	"memo/internal/provider"
	"memo/internal/taskloop"
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
	return a.taskloopEngine.Start(ctx, listID)
}

func (a *App) StopTaskList(listID string) {
	if a.taskloopEngine != nil {
		a.taskloopEngine.Stop(listID)
	}
}

func (a *App) buildTaskLoopRunWorker() taskloop.RunWorker {
	return func(ctx context.Context, chatID, prompt string) (string, error) {
		// App only has a single global "active session" — SendMessageStream
		// always acts on whatever chat that happens to be, and agent mode
		// (tool use) is a separate global on/off flag. Both must be pinned
		// to this task list's chat for the duration of the call, and calls
		// must be serialized so two task lists running at once can't send
		// into each other's chats mid-switch.
		a.taskloopRunMu.Lock()
		defer a.taskloopRunMu.Unlock()

		if err := a.SwitchChat(chatID); err != nil {
			return "", fmt.Errorf("görev sohbetine geçilemedi: %w", err)
		}

		a.agentMu.Lock()
		prevAgentEnabled := a.agentEnabled
		a.agentEnabled = true
		a.agentMu.Unlock()
		defer func() {
			a.agentMu.Lock()
			a.agentEnabled = prevAgentEnabled
			a.agentMu.Unlock()
		}()

		ch := a.SendMessageStream(ctx, prompt)
		var sb strings.Builder
		for chunk := range ch {
			if chunk.Error != "" {
				return sb.String(), fmt.Errorf("işçi hatası: %s", chunk.Error)
			}
			if chunk.Content != "" {
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
