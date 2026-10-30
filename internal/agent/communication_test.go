package agent

import (
	"context"
	"encoding/json"
	"memo/internal/agent/tools"
	"testing"
)

func TestCommunicationScopeDeniesHiddenToolsAndLeavesSharedRegistryIntact(t *testing.T) {
	registry := NewRegistry()
	scoped := registry.scoped(WithToolScope(context.Background(), []string{"mail_send", "mail_accounts", "telegram_send", "whatsapp_send"}))
	for _, name := range []string{"mail_send", "telegram_send", "whatsapp_send"} {
		def, ok := scoped.Get(name)
		if !ok || def.DangerLevel != Medium {
			t.Fatalf("permission gate missing: %s", name)
		}
	}
	for _, name := range []string{"run_command", "read_file", "write_file", "open_app", "browser_navigate"} {
		if _, ok := scoped.Get(name); ok {
			t.Fatalf("unwanted tool available: %s", name)
		}
		if _, err := scoped.Execute(context.Background(), name, json.RawMessage(`{}`), "", nil); err == nil {
			t.Fatalf("hidden tool executed: %s", name)
		}
	}
	if _, ok := registry.Get("run_command"); !ok {
		t.Fatal("mutated global agent registry")
	}
	for _, def := range ForCodeMode(registry.ToOpenAITools(nil)) {
		switch def.Function.Name {
		case "mail_send", "mail_read", "telegram_send":
			t.Fatalf("personal tool in code mode: %s", def.Function.Name)
		}
	}
}

func TestRestrictedChatCannotOfferOrExecuteCommunicationTools(t *testing.T) {
	ctx := tools.WithCommunicationAccess(context.Background(), false)
	scoped := NewRegistry().scoped(ctx)
	for _, name := range []string{"mail_accounts", "mail_read", "mail_send", "telegram_send", "whatsapp_send", "whatsapp_search"} {
		if _, ok := scoped.Get(name); ok {
			t.Fatalf("restricted channel visible: %s", name)
		}
		if _, err := scoped.Execute(ctx, name, json.RawMessage(`{}`), "", nil); err == nil {
			t.Fatal("restricted tool ran")
		}
	}
}
