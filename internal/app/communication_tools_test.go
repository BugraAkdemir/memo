package app

import (
	"context"
	"encoding/json"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"memo/internal/autonomy"
	"memo/internal/mail"
)

func seedCommunicationMail(t *testing.T, a *App) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "accounts.json")
	key := make([]byte, 32)
	v := mail.NewVault(path, key)
	if err := v.Save([]*mail.Account{{ID: "test:owner@example.com", Email: "owner@example.com", Password: "NEVER_EXPOSE_MAIL_PASSWORD", AuthType: mail.AuthPassword}}); err != nil {
		t.Fatal(err)
	}
	var err error
	a.mailManager, err = mail.NewManager(path, key)
	if err != nil {
		t.Fatal(err)
	}
}
func TestCommunicationAvailableWithoutAgentAndUsesNativeToolsWithAgent(t *testing.T) {
	for _, agentOn := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[agentOn], func(t *testing.T) {
			reqs := &capturedRequests{}
			a := newSendMessageTestApp(t, reqs)
			seedCommunicationMail(t, a)
			if err := a.SetAgentEnabled(agentOn); err != nil {
				t.Fatal(err)
			}
			if got := a.SendMessage("E-postayı gönder"); got != "fake agent reply" {
				t.Fatal(got)
			}
			raw := reqs.String()
			for _, name := range []string{"mail_send", "mail_read", "mail_accounts"} {
				if !strings.Contains(raw, `"name":"`+name+`"`) {
					t.Fatalf("native tool absent: %s", name)
				}
			}
			if !agentOn && strings.Contains(raw, `"name":"run_command"`) {
				t.Fatal("plain chat acquired terminal")
			}
			if a.GetAgentEnabled() != agentOn {
				t.Fatal("changed user agent toggle")
			}
			if !strings.Contains(raw, "never use Python") {
				t.Fatal("missing native-tool instruction")
			}
		})
	}
}
func TestCommunicationAccountsHideSecretsAndDLPStillRuns(t *testing.T) {
	a := &App{}
	seedCommunicationMail(t, a)
	accounts, err := a.CallCommunicationTool(context.Background(), "mail_accounts", json.RawMessage(`{}`))
	if err != nil || strings.Contains(accounts, "NEVER_EXPOSE") || !strings.Contains(accounts, "owner@example.com") {
		t.Fatalf("unsafe accounts: %s %v", accounts, err)
	}
	for _, name := range []string{"mail_send", "mail_save_draft"} {
		args, _ := json.Marshal(mail.SendRequest{AccountID: "test:owner@example.com", To: []string{"friend@example.com"}, Body: "sk-" + strings.Repeat("x", 24), Subject: "test"})
		_, err := a.CallCommunicationTool(context.Background(), name, args)
		var block *autonomy.BlockedError
		if !errors.As(err, &block) {
			t.Fatalf("%s bypassed DLP: %v", name, err)
		}
	}
	_, err = a.CallCommunicationTool(context.Background(), "telegram_send", json.RawMessage(`{"text":"hello"}`))
	if err == nil {
		t.Fatal("claimed sent with no linked Telegram")
	}
}
