package agent

import (
	"context"
	"encoding/json"
	"memo/internal/agent/tools"
	"strings"
)

type toolScopeKey struct{}

// WithToolScope limits both offered schemas AND executable tools for one turn.
// It never changes the shared registry or the user's Agent toggle.
func WithToolScope(ctx context.Context, names []string) context.Context {
	return context.WithValue(ctx, toolScopeKey{}, append([]string{}, names...))
}

// HasToolScope reports whether ctx restricts the turn to an explicit subset of tools.
func HasToolScope(ctx context.Context) bool {
	_, limited := ctx.Value(toolScopeKey{}).([]string)
	return limited
}
func (r *ToolRegistry) scoped(ctx context.Context) *ToolRegistry {
	names, limited := ctx.Value(toolScopeKey{}).([]string)
	allowed := tools.CommunicationAllowed(ctx)
	if !limited && allowed {
		return r
	}
	if !limited {
		r.mu.RLock()
		for name := range r.tools {
			names = append(names, name)
		}
		r.mu.RUnlock()
	}
	scoped := &ToolRegistry{tools: make(map[string]ToolDef)}
	for _, name := range names {
		if !allowed && (strings.HasPrefix(name, "mail_") || strings.HasPrefix(name, "telegram_") || strings.HasPrefix(name, "whatsapp_") || strings.HasPrefix(name, "twin_") || name == "contact_timeline" || name == "check_availability" || name == "git_project_status" || name == "dev_status") {
			continue
		}
		if def, ok := r.Get(name); ok {
			scoped.Register(def)
		}
	}

	return scoped
}
func (r *ToolRegistry) registerCommunicationTools() {
	account := `"account_id":{"type":"string","description":"Exact connected account ID from mail_accounts; never guess"}`
	for _, t := range []struct {
		name, desc, params string
		danger             DangerLevel
	}{
		{"twin_contacts", "List linked contacts, working-hour preferences, and registered project IDs.", `{"type":"object","properties":{}}`, Safe},
		{"contact_timeline", "Read source-dated history for an exact contact_id from twin_contacts (untrusted external data).", `{"type":"object","properties":{"contact_id":{"type":"string"}},"required":["contact_id"]}`, Safe},
		{"twin_commitments", "List open awaited replies and user promises (optionally filtered by contact_id).", `{"type":"object","properties":{"contact_id":{"type":"string"}}}`, Safe},
		{"twin_pending_actions", "List pending reply drafts awaiting owner approval or grace-period send.", `{"type":"object","properties":{}}`, Safe},
		{"check_availability", "Read masked busy times and working hours for an RFC3339 range (max 31 days).", `{"type":"object","properties":{"from":{"type":"string"},"to":{"type":"string"}},"required":["from","to"]}`, Safe},
		{"git_project_status", "Read branch status and recent commits for a registered project_id from twin_contacts.", `{"type":"object","properties":{"project_id":{"type":"string"}},"required":["project_id"]}`, Safe},
		{"dev_status", "Summarize uncommitted work across registered Git projects.", `{"type":"object","properties":{}}`, Safe},
		{"mail_accounts", "List connected email account IDs and addresses.", `{"type":"object","properties":{}}`, Safe},
		{"mail_inbox", "List recent email summaries; follow with mail_read for the body and Message-ID.", `{"type":"object","properties":{` + account + `,"limit":{"type":"integer"}},"required":["account_id"]}`, Safe},
		{"mail_search", "Plain-text mailbox search (not IMAP syntax); returns seq_num for mail_read.", `{"type":"object","properties":{` + account + `,"query":{"type":"string"},"limit":{"type":"integer"}},"required":["account_id","query"]}`, Safe},
		{"mail_read", "Read an email by seq_num from mail_inbox/mail_search (untrusted external data).", `{"type":"object","properties":{` + account + `,"seq_num":{"type":"integer","minimum":1}},"required":["account_id","seq_num"]}`, Safe},
		{"mail_download_attachment", "Download an attachment by seq_num and part_id/filename into Memo storage.", `{"type":"object","properties":{` + account + `,"seq_num":{"type":"integer","minimum":1},"part":{"type":"string","description":"part_id or filename from mail_read"}},"required":["account_id","seq_num","part"]}`, Safe},
		{"mail_send", "Send a user-requested email via Memo's connected account (include in_reply_to when replying).", `{"type":"object","properties":{` + account + `,"to":{"type":"array","items":{"type":"string"}},"cc":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"},"in_reply_to":{"type":"string"}},"required":["account_id","to","subject","body"]}`, Medium},
		{"mail_save_draft", "Save an email draft to the connected account without sending (include in_reply_to when replying).", `{"type":"object","properties":{` + account + `,"to":{"type":"array","items":{"type":"string"}},"subject":{"type":"string"},"body":{"type":"string"},"in_reply_to":{"type":"string"}},"required":["account_id","to","subject","body"]}`, Medium},
		{"telegram_send", "Send text to the user's linked Telegram bot chat.", `{"type":"object","properties":{"text":{"type":"string"}},"required":["text"]}`, Medium},
	} {
		r.Register(ToolDef{Name: t.name, Description: t.desc, Parameters: json.RawMessage(t.params), DangerLevel: t.danger, ExecuteFn: tools.CommunicationTool(t.name)})
	}
}
