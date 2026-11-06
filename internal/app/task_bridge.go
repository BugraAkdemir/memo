package app

import (
	"context"
	"fmt"
	"strings"
	"unicode"

	"memo/internal/logx"
	"memo/internal/twin"
)

const (
	taskBridgeTitleRunes = 200
	taskBridgeItemRunes  = 500
	taskBridgeMaxItems   = 10
)

// taskFromAction turns a message the owner is looking at into a Self-Driving
// task list. Text from outside is never copied into a task on its own: the
// owner supplies the title and the steps, chooses the project chat, and the
// list is created idle, so nothing runs until the owner starts it. One message
// can become one list.
//
// It is reachable only through the owner's own twin operations; the external
// autonomous executor has no such command.
func (a *App) taskFromAction(ctx context.Context, s *twin.Store, raw []byte) (any, error) {
	var args struct {
		ID     string   `json:"id"`
		ChatID string   `json:"chat_id"`
		Title  string   `json:"title"`
		Items  []string `json:"items"`
	}
	if err := decodeTwin(raw, &args); err != nil {
		return nil, err
	}
	title := strings.TrimSpace(args.Title)
	if title == "" || len([]rune(title)) > taskBridgeTitleRunes || strings.ContainsAny(title, "\r\n") || hasControl(title) {
		return nil, fmt.Errorf("%w: task title", twin.ErrInvalid)
	}
	if len(args.Items) == 0 || len(args.Items) > taskBridgeMaxItems {
		return nil, fmt.Errorf("%w: task steps", twin.ErrInvalid)
	}
	items := make([]string, 0, len(args.Items))
	for _, it := range args.Items {
		it = strings.TrimSpace(it)
		if it == "" || len([]rune(it)) > taskBridgeItemRunes || hasControl(it) {
			return nil, fmt.Errorf("%w: task step", twin.ErrInvalid)
		}
		items = append(items, it)
	}
	action, err := s.Action(ctx, args.ID)
	if err != nil {
		return nil, err
	}
	if action.Channel != "mail" && action.Channel != "whatsapp" {
		return nil, fmt.Errorf("%w: action channel", twin.ErrInvalid)
	}
	key := "task:" + action.ID
	if done, err := s.IncomingClaimed(ctx, key); err != nil {
		return nil, err
	} else if done {
		return nil, fmt.Errorf("%w: a task was already created from this message", twin.ErrDeliveryConflict)
	}
	// CreateTaskList also checks that the chat is a project (agent) chat.
	list, err := a.CreateTaskList(args.ChatID, title, items)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", twin.ErrInvalid, err)
	}
	// The conversion is spent only after the list exists, so a rejected or
	// failed request never uses up the one allowed attempt. If a parallel
	// request got there first, drop the duplicate.
	if claimed, err := s.ClaimIncoming(ctx, key); err != nil || !claimed {
		_ = a.DeleteTaskList(list.ID)
		if err == nil {
			err = twin.ErrDeliveryConflict
		}
		return nil, err
	}
	logx.Printf("twin: task list %s created from action %s (idle, owner-approved text)", list.ID, action.ID)
	return map[string]string{"task_id": list.ID, "state": list.Status}, nil
}

func hasControl(s string) bool {
	for _, r := range s {
		if unicode.IsControl(r) && r != '\t' {
			return true
		}
	}
	return false
}
