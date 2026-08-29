package app

import (
	"context"
	"fmt"
	"strings"

	"memo/internal/api"
	"memo/internal/taskloop"
)

// initTaskNotifyBus builds the NotifyBus and its senders. Called from the
// taskloop wiring block in Startup once the engine exists.
func (a *App) initTaskNotifyBus() {
	a.taskNotifyBus = taskloop.NewNotifyBus()
	a.taskNotifyBus.AddSender(&appNotifySender{a: a})
	a.taskNotifyBus.AddSender(&telegramNotifySender{a: a})
	a.taskNotifyBus.AddSender(&whatsappNotifySender{a: a})
}

// dispatchTaskEvent translates one engine onEvent(name,data) into a
// Notification and hands it to the bus. data is "<listID>" or
// "<listID>:<extra>" depending on the event.
func (a *App) dispatchTaskEvent(name, data string) {
	if a.taskNotifyBus == nil {
		return
	}
	listID, extra := data, ""
	if i := strings.IndexByte(data, ':'); i >= 0 {
		listID, extra = data[:i], data[i+1:]
	}

	var event, detail string
	switch name {
	case "taskloop:planning":
		event = "started"
	case "tasklist:finished":
		event = "finished"
		if extra == "failed" {
			event = "failed"
		}
		// The completion notification is a report the model writes about its
		// own run, not a fixed string. Generate it off the engine goroutine so
		// onEvent returns immediately.
		go a.dispatchTaskFinishReport(listID, event)
		return
	case "tasklist:item_started":
		event = "item_started"
	case "tasklist:item_done":
		event = "item_done"
	case "tasklist:item_stuck":
		event = "item_stuck"
	case "taskloop:waiting_limit":
		event, detail = "waiting_limit", extra
	case "taskloop:provider_switched":
		event, detail = "provider_switched", extra
	case "taskloop:config_changed":
		event, detail = "config_changed", strings.TrimSpace(extra)
	case "tasklist:subagent_spawned":
		event = "subagent_spawned"
	default:
		return
	}

	title := listID
	if a.taskloopStore != nil {
		if tl, err := a.taskloopStore.Get(listID); err == nil {
			title = tl.Title
		}
	}
	a.taskNotifyBus.Notify(context.Background(), taskloop.Notification{
		ListID:    listID,
		ListTitle: title,
		Event:     event,
		Detail:    detail,
	})
}

// dispatchTaskFinishReport asks the model to summarize its own task run and
// sends that as the completion notification. Falls back to a factual roll-up
// if the model can't be reached.
func (a *App) dispatchTaskFinishReport(listID, event string) {
	if a.taskNotifyBus == nil || a.taskloopStore == nil {
		return
	}
	tl, err := a.taskloopStore.Get(listID)
	if err != nil {
		return
	}

	header := "✅"
	if event == "failed" {
		header = "❌"
	}
	body := strings.TrimSpace(a.generateTaskReport(tl))
	if body == "" {
		body = factualTaskRollup(tl) // last-resort, still not a canned per-event string
	}
	full := fmt.Sprintf("%s %s\n\n%s", header, tl.Title, body)

	a.taskNotifyBus.Notify(context.Background(), taskloop.Notification{
		ListID:    listID,
		ListTitle: tl.Title,
		Event:     event,
		Body:      full,
	})
}

// reportPrompt is the wrap-up instruction handed to the model.
const reportPrompt = "Bu Self-Driving görev listesi bitti. Kullanıcıya bir KAPANIŞ RAPORU yaz: " +
	"hangi maddeleri tamamladın, hangileri takıldı ve neden, hangi dosyalara/değişikliklere dokundun, " +
	"kullanıcının bilmesi gereken bir şey varsa. Görev maddelerinin diliyle yaz. " +
	"Selamlama ve dolgu cümlesi kullanma, direkt rapora geç. En fazla ~150 kelime."

// generateTaskReport has the model write its own completion report. It first
// asks in the task's own chat (full context of everything it actually did);
// if that can't run it falls back to a tool-less summary from the transcript,
// then to a factual roll-up. Never a canned per-event string.
func (a *App) generateTaskReport(tl *taskloop.TaskList) string {
	// Primary: the task's own chat — same client path the worker turns used,
	// with the whole run in context.
	if sm := a.getSessionManager(); sm != nil && sm.SessionExists(tl.ChatID) {
		var sb strings.Builder
		failed := false
		for chunk := range a.SendMessageStreamTo(context.Background(), tl.ChatID, reportPrompt) {
			if chunk.Error != "" {
				failed = true
				break
			}
			// Only real prose chunks — skip the status chunks the stream
			// prepends/interleaves (agent_event JSON, the memory_used count).
			if chunk.FinishReason == "" || chunk.FinishReason == "stop" {
				sb.WriteString(chunk.Content)
			}
			if chunk.Done {
				break
			}
		}
		if out := strings.TrimSpace(sb.String()); out != "" && !failed {
			return out
		}
	}

	// Fallback: tool-less summary from the transcript + item metadata.
	var ctxb strings.Builder
	ctxb.WriteString("GÖREV: " + tl.Title + "\n\nMADDELER:\n")
	for i, it := range tl.Items {
		fmt.Fprintf(&ctxb, "%d. [%s] %s", i+1, it.Status, it.Text)
		if it.Note != "" {
			ctxb.WriteString(" (not: " + it.Note + ")")
		}
		ctxb.WriteString("\n")
	}

	if sm := a.getSessionManager(); sm != nil {
		msgs := sm.GetActiveMessagesForSession(tl.ChatID)
		if len(msgs) > 0 {
			ctxb.WriteString("\nÇALIŞMA DÖKÜMÜ (ajanın turları):\n")
			start := 0
			if len(msgs) > 16 {
				start = len(msgs) - 16
			}
			for _, m := range msgs[start:] {
				c := strings.TrimSpace(m.Content)
				if m.Role == "user" {
					// worker prompts carry a big repeated preamble — keep only
					// the tail (the actual instruction).
					if idx := strings.LastIndex(c, "# Görev"); idx >= 0 {
						c = c[idx:]
					}
					c = truncateStr(c, 300)
				} else {
					c = truncateStr(c, 900)
				}
				if c != "" {
					fmt.Fprintf(&ctxb, "[%s] %s\n", m.Role, c)
				}
			}
		}
	}

	raw := a.callLLMForReview(context.Background(), []api.Message{
		api.NewTextMessage("system",
			"Sen bir Self-Driving görevi bitiren asistansın. Aşağıdaki görev sonucu ve çalışma dökümünden "+
				"kullanıcıya bir KAPANIŞ RAPORU yaz: hangi maddeler tamamlandı, hangileri takıldı ve neden, "+
				"hangi dosyalara/değişikliklere dokunuldu, kullanıcının bilmesi gereken bir şey varsa. "+
				"Görev maddelerinin diliyle yaz. Selamlama ve dolgu cümlesi kullanma, direkt rapora geç. En fazla ~150 kelime."),
		api.NewTextMessage("user", ctxb.String()),
	})
	if strings.HasPrefix(raw, "⚠") {
		return ""
	}
	return strings.TrimSpace(raw)
}

// factualTaskRollup is the plain-facts fallback when no model is reachable.
func factualTaskRollup(tl *taskloop.TaskList) string {
	done, stuck := 0, 0
	var stuckNotes []string
	for _, it := range tl.Items {
		switch it.Status {
		case "done":
			done++
		case "stuck":
			stuck++
			if it.Note != "" {
				stuckNotes = append(stuckNotes, "• "+it.Text+": "+it.Note)
			}
		}
	}
	msg := fmt.Sprintf("%d/%d madde tamamlandı", done, len(tl.Items))
	if stuck > 0 {
		msg += fmt.Sprintf(", %d takıldı", stuck)
		if len(stuckNotes) > 0 {
			msg += ":\n" + strings.Join(stuckNotes, "\n")
		}
	}
	return msg
}

func formatTaskNotification(n taskloop.Notification) string {
	if strings.TrimSpace(n.Body) != "" {
		return n.Body
	}
	label := map[string]string{
		"started":           "başladı",
		"finished":          "tamamlandı ✅",
		"failed":            "başarısız oldu ❌",
		"item_started":      "yeni madde",
		"item_done":         "madde bitti",
		"item_stuck":        "madde takıldı ⚠️",
		"waiting_limit":     "kullanım limiti — bekleniyor",
		"provider_switched": "sağlayıcı değiştirildi",
		"config_changed":    "ayar değişti",
		"subagent_spawned":  "alt-agent'lar açıldı",
	}[n.Event]
	if label == "" {
		label = n.Event
	}
	msg := fmt.Sprintf("📋 %s — %s", n.ListTitle, label)
	if n.Detail != "" {
		msg += " (" + n.Detail + ")"
	}
	return msg
}

type appNotifySender struct{ a *App }

func (s *appNotifySender) SendNotification(ctx context.Context, n taskloop.Notification) error {
	payload := n.Detail
	if strings.TrimSpace(n.Body) != "" {
		payload = n.Body // completion report — the app notification centre wants the full text too
	}
	s.a.emitEvent("taskloop:notify", n.ListID+"|"+n.Event+"|"+payload)
	return nil
}

type telegramNotifySender struct{ a *App }

func (s *telegramNotifySender) SendNotification(ctx context.Context, n taskloop.Notification) error {
	if s.a.tgStore == nil {
		return nil
	}
	st := s.a.tgStore.Get()
	if st.OwnerChatID == 0 {
		return nil
	}
	return s.a.TelegramSend(ctx, st.OwnerChatID, formatTaskNotification(n))
}

type whatsappNotifySender struct{ a *App }

func (s *whatsappNotifySender) SendNotification(ctx context.Context, n taskloop.Notification) error {
	s.a.waMu.Lock()
	cli := s.a.waClient
	s.a.waMu.Unlock()
	if cli == nil || !cli.IsConnected() {
		return nil
	}
	jids := cli.OwnJIDs()
	if len(jids) == 0 {
		return nil
	}
	_, err := s.a.WhatsAppSend(ctx, jids[0], formatTaskNotification(n))
	return err
}
