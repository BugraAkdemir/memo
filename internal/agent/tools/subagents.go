package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ChatSubAgentService is bound by App to the requesting chat. Model arguments
// never select a chat or project; ownership is checked again by the service.
// Returned strings contain the service's structured JSON result.
type ChatSubAgentService interface {
	SubAgentAction(ctx context.Context, action string, args json.RawMessage) (string, error)
}
type chatSubAgentServiceKey struct{}

// WithChatSubAgentService scopes coordination to one turn, avoiding a mutable
// global service shared by concurrent chats and background task workers.
func WithChatSubAgentService(ctx context.Context, service ChatSubAgentService) context.Context {
	return context.WithValue(ctx, chatSubAgentServiceKey{}, service)
}

// HasChatSubAgentService lets the pipeline advertise coordination tools only
// for main-chat turns that actually support asynchronous child execution.
func HasChatSubAgentService(ctx context.Context) bool {
	service, _ := ctx.Value(chatSubAgentServiceKey{}).(ChatSubAgentService)
	return service != nil
}

// ChatSubAgentAction validates arguments before forwarding to the chat-bound
// service. Strict decoding also rejects model-supplied chat/project overrides.
func ChatSubAgentAction(ctx context.Context, action string, args json.RawMessage) (string, error) {
	service, _ := ctx.Value(chatSubAgentServiceKey{}).(ChatSubAgentService)
	if service == nil {
		return "", errors.New("sub-agent coordination is unavailable for this turn")
	}
	if len(bytes.TrimSpace(args)) == 0 || bytes.TrimSpace(args)[0] != '{' {
		return "", errors.New("sub-agent arguments must be an object")
	}
	var target any
	switch action {
	case "spawn":
		target = &struct {
			Prompt  string   `json:"prompt"`
			Title   string   `json:"title"`
			Write   bool     `json:"write"`
			Isolate bool     `json:"isolate"`
			Skills  []string `json:"skills"`
		}{}
	case "send":
		target = &struct {
			ID     string `json:"id"`
			Prompt string `json:"prompt"`
		}{}
	case "merge", "close", "cancel":
		target = &struct {
			ID string `json:"id"`
		}{}
	case "list":
		target = &struct {
			ID string `json:"id"`
		}{}
	default:
		return "", fmt.Errorf("unknown sub-agent action: %s", action)
	}
	decoder := json.NewDecoder(bytes.NewReader(args))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return "", fmt.Errorf("invalid sub-agent arguments: %w", err)
	}
	if err := decoder.Decode(new(any)); err != io.EOF {
		return "", errors.New("sub-agent arguments must contain one JSON object")
	}
	// The strict decoder above validates types; this map validates the common
	// required fields without modifying the original JSON sent to the service.
	var fields map[string]json.RawMessage
	_ = json.Unmarshal(args, &fields)
	required := []string{}
	if action == "spawn" || action == "send" {
		required = append(required, "prompt")
	}
	if action != "spawn" && action != "list" {
		required = append(required, "id")
	}
	for _, name := range required {
		var value string
		_ = json.Unmarshal(fields[name], &value)
		if strings.TrimSpace(value) == "" {
			return "", fmt.Errorf("%s cannot be empty", name)
		}
	}
	if raw, ok := fields["skills"]; ok {
		var skills []string
		_ = json.Unmarshal(raw, &skills)
		for _, name := range skills {
			if strings.TrimSpace(name) == "" {
				return "", errors.New("skill names cannot be empty")
			}
		}
	}
	return service.SubAgentAction(ctx, action, args)
}
