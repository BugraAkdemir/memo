package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Routines is the interface the routine tools use to actually create, list,
// and cancel scheduled automations. Set by App after initialization.
//
// CreateRoutine deliberately has no target/contact/chat parameter of any
// kind — unlike WhatsAppClient.SendMessage above, which the model can point
// at any contact, a routine's delivery target is never something the model
// supplies. It resolves internally from ctx (which self-chat surface, if
// any, originated this call — see internal/app's selfChatSourceFromContext
// / App.CreateRoutineFromChat) instead: an unattended, model-driven "set up
// a recurring send to some contact" tool call is a real risk (prompt
// injection from scraped content, a model error) that a hardcoded "always
// targets the conversation that asked" contract closes off entirely, not
// just mitigates.
//
// ListRoutines/DeleteRoutine have no such restriction to make — listing is
// read-only, and deleting only ever acts on a routine ID the model must
// have already seen via ListRoutines' own output (there's no way to guess
// one), operating on the user's own existing automations rather than
// reaching toward any third party.
var Routines interface {
	CreateRoutine(ctx context.Context, text string) (string, error)
	ListRoutines(ctx context.Context) (string, error)
	DeleteRoutine(ctx context.Context, id string) (string, error)
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

// ListRoutines takes no arguments — it always lists every routine, there is
// nothing for the model to filter by that would need a parameter.
func ListRoutines(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	if Routines == nil {
		return "Rutin sistemi hazır değil.", nil
	}
	out, err := Routines.ListRoutines(ctx)
	if err != nil {
		return "", fmt.Errorf("rutinler listelenemedi: %w", err)
	}
	return out, nil
}

// DeleteRoutineArgs identifies the routine to cancel purely by its id — the
// model can only have learned a real id by having already called
// list_routines first, since ids aren't guessable/derivable from anything
// else.
type DeleteRoutineArgs struct {
	ID string `json:"id"`
}

func DeleteRoutine(ctx context.Context, argsJSON json.RawMessage, basePath string, createBackup func(string) error) (string, error) {
	var args DeleteRoutineArgs
	if err := json.Unmarshal(argsJSON, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if strings.TrimSpace(args.ID) == "" {
		return "", fmt.Errorf("id boş olamaz — önce list_routines ile gerçek id'yi öğren")
	}
	if Routines == nil {
		return "Rutin sistemi hazır değil.", nil
	}
	out, err := Routines.DeleteRoutine(ctx, args.ID)
	if err != nil {
		return "", fmt.Errorf("rutin silinemedi: %w", err)
	}
	return out, nil
}
