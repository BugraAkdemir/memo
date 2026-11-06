package tools

import (
	"context"
	"encoding/json"
	"fmt"
)

// Bound per turn rather than globally: concurrent apps/chats retain their own
// channel services and the ordinary permission/audit path remains in charge.
type CommunicationService interface {
	CallCommunicationTool(context.Context, string, json.RawMessage) (string, error)
}
type communicationKey struct{}

func WithCommunicationService(ctx context.Context, service CommunicationService) context.Context {
	return context.WithValue(ctx, communicationKey{}, service)
}
func CommunicationTool(name string) func(context.Context, json.RawMessage, string, func(string) error) (string, error) {
	return func(ctx context.Context, args json.RawMessage, _ string, _ func(string) error) (string, error) {
		if !CommunicationAllowed(ctx) {
			return "", fmt.Errorf("communication access denied")
		}
		service, _ := ctx.Value(communicationKey{}).(CommunicationService)
		if service == nil {
			return "", fmt.Errorf("communication service unavailable")
		}
		return service.CallCommunicationTool(ctx, name, args)
	}
}

// HTTP callers with a recognized restricted account cannot gain access to the
// owner's private channels by asking the chat model instead of using the API.
type communicationAccessKey struct{}

func WithCommunicationAccess(ctx context.Context, allowed bool) context.Context {
	return context.WithValue(ctx, communicationAccessKey{}, allowed)
}
func CommunicationAllowed(ctx context.Context) bool {
	allowed, known := ctx.Value(communicationAccessKey{}).(bool)
	return !known || allowed // internal/local callers retain their established access
}
