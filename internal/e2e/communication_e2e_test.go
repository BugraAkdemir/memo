package e2e

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/mail"
)

// Real HTTP/SSE -> native tool -> permission HTTP -> App.MailSend -> real SMTP.
// No email leaves loopback; the fixture checks the actual transmitted message.
func TestCommunicationMailSendWithAgentOffKeepsPermissionsAndNativeTransport(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	delivered := make(chan string, 1)
	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		conn.SetDeadline(time.Now().Add(15 * time.Second))
		r := textproto.NewReader(bufio.NewReader(conn))
		fmt.Fprint(conn, "220 localhost fixture\r\n")
		for {
			line, err := r.ReadLine()
			if err != nil {
				return
			}
			switch {
			case strings.HasPrefix(line, "EHLO"):
				fmt.Fprint(conn, "250-localhost\r\n250 AUTH PLAIN\r\n")
			case strings.HasPrefix(line, "AUTH"):
				fmt.Fprint(conn, "235 authenticated\r\n")
			case strings.HasPrefix(line, "MAIL"), strings.HasPrefix(line, "RCPT"):
				fmt.Fprint(conn, "250 ok\r\n")
			case line == "DATA":
				fmt.Fprint(conn, "354 send data\r\n")
				body, err := io.ReadAll(r.DotReader())
				if err != nil {
					return
				}
				delivered <- string(body)
				fmt.Fprint(conn, "250 accepted\r\n")
			case line == "QUIT":
				fmt.Fprint(conn, "221 bye\r\n")
				return
			default:
				fmt.Fprint(conn, "500 unsupported\r\n")
			}
		}
	}()
	h := newHarness(t, func(cfg *config.AppConfig) {
		cfg.Mail.Enabled = false // no polling competes with this send fixture
		v := mail.NewVault(config.DataPath("mail", "accounts.json"), nil)
		if err := v.Save([]*mail.Account{{ID: "custom:owner@example.com", Email: "owner@example.com", AuthType: mail.AuthPassword, Password: "NEVER_EXPOSE_SMTP_PASSWORD", Server: mail.ServerConfig{SMTPHost: "localhost", SMTPPort: listener.Addr().(*net.TCPAddr).Port}}}); err != nil {
			t.Fatal(err)
		}
	})
	h.SetAgentEnabled(false)
	sawAccounts, sawSend := false, false
	h.Fake.Script = func(_ int, req FakeChatRequest) FakeChatResponse {
		if len(req.Tools) == 0 {
			return FakeChatResponse{Text: "title"}
		}
		schema := string(req.Raw)
		if !strings.Contains(schema, `"name":"mail_send"`) {
			t.Error("mail tool absent with Agent off")
		}
		if strings.Contains(schema, `"name":"run_command"`) {
			t.Error("ordinary chat acquired shell")
		}
		if strings.Contains(schema, "NEVER_EXPOSE_SMTP_PASSWORD") {
			t.Error("mail credentials sent to model")
		}
		for _, msg := range req.Messages {
			s := string(msg)
			if strings.Contains(s, `"role":"tool"`) && strings.Contains(s, `"tool_call_id":"accounts"`) {
				sawAccounts = true
			}
			if strings.Contains(s, `"role":"tool"`) && strings.Contains(s, `"tool_call_id":"send"`) {
				if !strings.Contains(s, `\"sent\":true`) {
					t.Errorf("send failed: %s", s)
				}
				sawSend = true
			}
		}
		if sawSend {
			return FakeChatResponse{Text: "Gönderildi."}
		}
		if !sawAccounts {
			return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "accounts", Name: "mail_accounts", Arguments: `{}`}}}
		}
		return FakeChatResponse{ToolCalls: []FakeToolCall{{ID: "send", Name: "mail_send", Arguments: `{"account_id":"custom:owner@example.com","to":["Buğra <friend@example.com>"],"subject":"Re: acil toplantı","body":"Sorun ne, detay verebilir misin?","in_reply_to":"<original@example.com>"}`}}}
	}
	asked, executed, done := false, false, false
	for ev := range h.SendMessageStreamAsync(h.NewChat(), "Konuştuğumuz e-postayı sen at") {
		if ev.FinishReason == "agent_event" {
			for _, ae := range AgentEvents(t, []SSEEvent{ev}) {
				if ae.Type == "permission_request" && ae.ToolName == "mail_send" {
					asked = true
					h.ResolveAgentPermission(ae.RequestID, "allow_once")
				}
				if ae.Type == "tool_executing" && ae.ToolName == "mail_send" {
					executed = true
				}
			}
		}
		if ev.Done {
			done = true
		}
	}
	if !asked || !executed || !sawSend || !done {
		t.Fatalf("broken lifecycle: permission=%v executed=%v result=%v done=%v", asked, executed, sawSend, done)
	}
	if h.App.GetAgentEnabled() {
		t.Fatal("changed Agent toggle")
	}
	select {
	case body := <-delivered:
		if !strings.Contains(body, "To: friend@example.com") || !strings.Contains(body, "Sorun ne, detay verebilir misin?") || !strings.Contains(body, "In-Reply-To: <original@example.com>") {
			t.Fatalf("lost mail body/thread: %s", body)
		}
	case <-time.After(time.Second):
		t.Fatal("SMTP received no mail")
	}
}
