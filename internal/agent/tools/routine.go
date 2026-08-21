package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Routines is the interface the create_routine tool uses to actually
// create a scheduled automation. Set by App after initialization.
// Deliberately has no target/contact/chat parameter of any kind — unlike
// WhatsAppClient.SendMessage above, which the model can point at any
// contact, a routine's delivery target is never something the model
// supplies. CreateRoutine resolves it internally from ctx (which self-chat
// surface, if any, originated this call — see internal/app's
// selfChatSourceFromContext / App.CreateRoutineFromChat) instead: an
// unattended, model-driven "set up a recurring send to some contact" tool
// call is a real risk (prompt injection from scraped content, a model
// error) that a hardcoded "always targets the conversation that asked"
// contract closes off entirely, not just mitigates.
var Routines interface {
	CreateRoutine(ctx context.Context, text string) (string, error)
}

// CreateRoutineArgs is create_routine's only argument — see the Routines
// doc comment above for why there is nothing else here.
type CreateRoutineArgs struct {
	Text string `json:"text"`
}

func CreateRoutine(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args CreateRoutineArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.Text) == "" {
		return "", fmt.Errorf("text boş olamaz")
	}
	if Routines == nil {
		return "Rutin sistemi hazır değil.", nil
	}
	summary, err := Routines.CreateRoutine(ctx, args.Text)
	if err != nil {
		return "", fmt.Errorf("rutin oluşturulamadı: %w", err)
	}
	return summary, nil
}
