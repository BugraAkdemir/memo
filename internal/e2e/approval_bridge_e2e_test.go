package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/textproto"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"memo/internal/config"
	"memo/internal/mail"
	"memo/internal/twin"
)

// Real App startup/worker, owner HTTP API, SQLite reservation and production SMTP
// transport. Only the remote SMTP peer is a loopback protocol fixture.
func TestApprovalDeliveryAndUndoOverRealHTTPAndSMTP(t *testing.T) {
	for _, mode := range []string{"owner", "automatic", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			listener, err := net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listener.Close()
			received := make(chan string, 1)
			done := make(chan struct{})
			var accepts atomic.Int32
			go func() {
				defer close(done)
				conn, err := listener.Accept()
				if err != nil {
					return
				}
				defer conn.Close()
				accepts.Add(1)
				conn.SetDeadline(time.Now().Add(5 * time.Second))
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
						fmt.Fprint(conn, "354 data\r\n")
						body, err := io.ReadAll(r.DotReader())
						if err != nil {
							return
						}
						received <- string(body)
						fmt.Fprint(conn, "250 accepted\r\n")
					case line == "QUIT":
						fmt.Fprint(conn, "221 bye\r\n")
						return
					default:
						fmt.Fprint(conn, "500 unsupported\r\n")
					}
				}
			}()
			var action twin.Action
			h := newHarness(t, func(cfg *config.AppConfig) {
				cfg.Mail.Enabled = false // no external IMAP service is part of this fixture
				acct := &mail.Account{ID: "owner", Email: "owner@example.com", AuthType: mail.AuthPassword, Password: "fixture", Server: mail.ServerConfig{SMTPHost: "localhost", SMTPPort: listener.Addr().(*net.TCPAddr).Port}}
				if err := mail.NewVault(config.DataPath("mail", "accounts.json"), nil).Save([]*mail.Account{acct}); err != nil {
					t.Fatal(err)
				}
				s, err := twin.NewStore(config.DataPath("twin"))
				if err != nil {
					t.Fatal(err)
				}
				defer s.Close()
				id, err := s.SaveContact(context.Background(), twin.Contact{Name: "Friend", AutonomyPolicy: "full_auto", Aliases: []twin.Alias{{Channel: "mail", Address: "friend@example.com"}}})
				if err != nil {
					t.Fatal(err)
				}
				p, _ := s.Preferences(context.Background())
				p.Autonomy = twin.AutonomySettings{Enabled: true, Mail: true, Mode: "full_auto", GraceSeconds: 60}
				if err = s.SavePreferences(context.Background(), p); err != nil {
					t.Fatal(err)
				}
				action = twin.Action{ID: "approval-e2e", ContactID: id, DeliveryVersion: 1, Channel: "mail", Account: "owner", Recipient: "friend@example.com", SourceAddress: "friend@example.com", State: "pending", Mode: "awaiting_approval", Subject: "E2E question", Body: "Evidenced test reply", InReplyTo: "<source@example.com>", CreatedAt: time.Now()}
				if err = s.SaveAction(context.Background(), action); err != nil {
					t.Fatal(err)
				}
			})
			ctx := context.Background()
			if mode != "owner" {
				s, err := twin.NewStore(config.DataPath("twin"))
				if err != nil {
					t.Fatal(err)
				}
				at := time.Now().Add(-time.Second)
				if mode == "cancel" {
					at = time.Now().Add(time.Minute)
				}
				score := .99
				action.Mode = "grace_period_auto"
				action.ExecuteAt = &at
				action.ScoreStatus = "scored"
				action.DecisionScore = &score
				if ok, err := s.ReserveCandidate(ctx, action.ContactID, "thread", action.ID, time.Now(), true); err != nil || !ok {
					t.Fatal(ok, err)
				}
				if err = s.SaveAction(ctx, action); err != nil {
					t.Fatal(err)
				}
				s.Close()
			}
			if mode == "owner" || mode == "cancel" {
				op := "approve_action"
				if mode == "cancel" {
					op = "cancel_action"
				}
				resp := h.postJSON("/api/v1/twin/"+op, map[string]string{"id": action.ID})
				if resp.StatusCode != 200 {
					t.Fatal(resp.StatusCode)
				}
				resp.Body.Close()
			}
			want := "sent"
			if mode == "cancel" {
				want = "cancelled"
			}
			deadline := time.Now().Add(3 * time.Second)
			for {
				var state struct {
					Actions []twin.Action `json:"actions"`
				}
				decodeInto(t, h.getJSON("/api/twin/state"), &state)
				if len(state.Actions) == 1 && state.Actions[0].State == want {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("queue did not settle", state)
				}
				time.Sleep(20 * time.Millisecond)
			}
			if mode != "cancel" {
				select {
				case body := <-received:
					if !strings.Contains(body, action.Body) || !strings.Contains(body, "In-Reply-To: <source@example.com>") {
						t.Fatal(body)
					}
				case <-time.After(time.Second):
					t.Fatal("SMTP received no reply")
				}
				resp := h.postJSON("/api/twin/approve_action", map[string]string{"id": action.ID})
				if resp.StatusCode != 409 {
					t.Fatal(resp.StatusCode)
				}
				resp.Body.Close()
			} else if accepts.Load() != 0 {
				t.Fatal("cancelled reply reached SMTP")
			}
			listener.Close()
			select {
			case <-done:
			case <-time.After(time.Second):
				t.Fatal("SMTP peer did not stop")
			}
		})
	}
}
