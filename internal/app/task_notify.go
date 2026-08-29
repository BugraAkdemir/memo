package app

import (
	"context"
	"fmt"
	"strings"

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

func formatTaskNotification(n taskloop.Notification) string {
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
	s.a.emitEvent("taskloop:notify", n.ListID+"|"+n.Event+"|"+n.Detail)
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
